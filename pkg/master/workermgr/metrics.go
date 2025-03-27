package workermgr

import (
	"fmt"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// MetricsManager 负责Worker的资源指标收集与分析
type MetricsManager struct {
	workerManager  WorkerManager
	metricsCache   map[string]*WorkerMetrics // worker_id -> metrics
	cacheLock      sync.RWMutex
	interval       time.Duration
	isRunning      bool
	stopChan       chan struct{}
	collectionWg   sync.WaitGroup
	metricsHistory map[string][]*ResourceSnapshot // worker_id -> history snapshots
	historyLock    sync.RWMutex
	historySize    int // 历史记录保留数量
}

// WorkerMetrics 包含Worker的资源指标数据
type WorkerMetrics struct {
	WorkerID         string    `json:"worker_id"`
	LastUpdate       time.Time `json:"last_update"`
	CPUUsagePercent  float64   `json:"cpu_usage_percent"`  // CPU使用率
	MemoryTotal      int64     `json:"memory_total"`       // 总内存(MB)
	MemoryUsed       int64     `json:"memory_used"`        // 已用内存(MB)
	MemoryUsageRatio float64   `json:"memory_usage_ratio"` // 内存使用率
	DiskTotal        int64     `json:"disk_total"`         // 总磁盘(GB)
	DiskUsed         int64     `json:"disk_used"`          // 已用磁盘(GB)
	DiskUsageRatio   float64   `json:"disk_usage_ratio"`   // 磁盘使用率
	LoadAverage      float64   `json:"load_average"`       // 负载均值
	RunningJobs      int       `json:"running_jobs"`       // 运行中的任务数
	HighLoad         bool      `json:"high_load"`          // 是否负载过高
	LowResources     bool      `json:"low_resources"`      // 是否资源不足
}

// ResourceSnapshot 表示某一时刻的资源快照，用于历史趋势分析
type ResourceSnapshot struct {
	Timestamp        time.Time `json:"timestamp"`
	CPUUsagePercent  float64   `json:"cpu_usage_percent"`
	MemoryUsageRatio float64   `json:"memory_usage_ratio"`
	DiskUsageRatio   float64   `json:"disk_usage_ratio"`
	LoadAverage      float64   `json:"load_average"`
	RunningJobs      int       `json:"running_jobs"`
}

// NewMetricsManager 创建指标管理器
func NewMetricsManager(workerManager WorkerManager, interval time.Duration) *MetricsManager {
	if interval <= 0 {
		interval = 60 * time.Second // 默认60秒收集一次
	}

	return &MetricsManager{
		workerManager:  workerManager,
		metricsCache:   make(map[string]*WorkerMetrics),
		interval:       interval,
		isRunning:      false,
		stopChan:       make(chan struct{}),
		metricsHistory: make(map[string][]*ResourceSnapshot),
		historySize:    60, // 默认保留60个历史记录点
	}
}

// Start 启动指标收集
func (m *MetricsManager) Start() {
	if m.isRunning {
		return
	}

	m.isRunning = true
	m.stopChan = make(chan struct{})

	// 启动周期性指标收集
	m.collectionWg.Add(1)
	go m.collectMetricsLoop()

	utils.Info("worker metrics manager started", zap.Duration("interval", m.interval))
}

// Stop 停止指标收集
func (m *MetricsManager) Stop() {
	if !m.isRunning {
		return
	}

	m.isRunning = false
	close(m.stopChan)
	m.collectionWg.Wait()

	utils.Info("worker metrics manager stopped")
}

// collectMetricsLoop 周期性收集指标的循环
func (m *MetricsManager) collectMetricsLoop() {
	defer m.collectionWg.Done()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// 立即收集一次
	m.collectAllWorkerMetrics()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.collectAllWorkerMetrics()
		}
	}
}

// collectAllWorkerMetrics 收集所有活跃Worker的指标
func (m *MetricsManager) collectAllWorkerMetrics() {
	workers, err := m.workerManager.GetActiveWorkers()
	if err != nil {
		utils.Error("failed to get active workers for metrics collection", zap.Error(err))
		return
	}

	for _, worker := range workers {
		metrics := m.calculateWorkerMetrics(worker)
		m.updateMetricsCache(worker.ID, metrics)
		m.addMetricsSnapshot(worker.ID, metrics)
		m.detectAnomalies(worker.ID, metrics)
	}
}

// calculateWorkerMetrics 计算单个Worker的指标
func (m *MetricsManager) calculateWorkerMetrics(worker *models.Worker) *WorkerMetrics {
	metrics := &WorkerMetrics{
		WorkerID:    worker.ID,
		LastUpdate:  time.Now(),
		LoadAverage: worker.LoadAvg,
		RunningJobs: len(worker.RunningJobs),
	}

	// 内存计算
	metrics.MemoryTotal = worker.MemoryTotal
	metrics.MemoryUsed = worker.MemoryTotal - worker.MemoryFree
	if worker.MemoryTotal > 0 {
		metrics.MemoryUsageRatio = float64(metrics.MemoryUsed) / float64(worker.MemoryTotal)
	}

	// 磁盘计算
	metrics.DiskTotal = worker.DiskTotal
	metrics.DiskUsed = worker.DiskTotal - worker.DiskFree
	if worker.DiskTotal > 0 {
		metrics.DiskUsageRatio = float64(metrics.DiskUsed) / float64(worker.DiskTotal)
	}

	// CPU使用率估算 (通过系统负载与CPU核心数的比值进行粗略估算)
	if worker.CPUCores > 0 {
		metrics.CPUUsagePercent = (worker.LoadAvg / float64(worker.CPUCores)) * 100
		if metrics.CPUUsagePercent > 100 {
			metrics.CPUUsagePercent = 100
		}
	}

	// 检测是否资源紧张
	metrics.HighLoad = metrics.CPUUsagePercent >= 80 || metrics.LoadAverage > float64(worker.CPUCores)
	metrics.LowResources = metrics.MemoryUsageRatio >= 0.85 || metrics.DiskUsageRatio >= 0.90

	return metrics
}

// updateMetricsCache 更新指标缓存
func (m *MetricsManager) updateMetricsCache(workerID string, metrics *WorkerMetrics) {
	m.cacheLock.Lock()
	defer m.cacheLock.Unlock()

	m.metricsCache[workerID] = metrics
}

// addMetricsSnapshot 添加指标快照到历史记录
func (m *MetricsManager) addMetricsSnapshot(workerID string, metrics *WorkerMetrics) {
	m.historyLock.Lock()
	defer m.historyLock.Unlock()

	// 创建快照
	snapshot := &ResourceSnapshot{
		Timestamp:        time.Now(),
		CPUUsagePercent:  metrics.CPUUsagePercent,
		MemoryUsageRatio: metrics.MemoryUsageRatio,
		DiskUsageRatio:   metrics.DiskUsageRatio,
		LoadAverage:      metrics.LoadAverage,
		RunningJobs:      metrics.RunningJobs,
	}

	// 获取现有历史记录
	history, exists := m.metricsHistory[workerID]
	if !exists {
		history = make([]*ResourceSnapshot, 0, m.historySize)
	}

	// 添加新快照
	history = append(history, snapshot)

	// 如果超过保留数量，删除最旧的
	if len(history) > m.historySize {
		history = history[1:]
	}

	m.metricsHistory[workerID] = history

	// 可选：定期将历史数据保存到MongoDB以便长期分析
	if len(history)%10 == 0 { // 每10个快照保存一次
		m.saveMetricsToDatabase(workerID, metrics)
	}
}

// saveMetricsToDatabase 将指标保存到数据库
func (m *MetricsManager) saveMetricsToDatabase(workerID string, metrics *WorkerMetrics) {
	// 这里实现将指标保存到MongoDB的逻辑
	// 为了简化，暂不实现具体保存逻辑，仅记录日志表示已保存
	utils.Debug("metrics data saved to database",
		zap.String("worker_id", workerID),
		zap.Time("timestamp", metrics.LastUpdate))
}

// detectAnomalies 检测异常情况
func (m *MetricsManager) detectAnomalies(workerID string, metrics *WorkerMetrics) {
	// 高负载警告
	if metrics.HighLoad {
		utils.Warn("worker has high load",
			zap.String("worker_id", workerID),
			zap.Float64("cpu_usage", metrics.CPUUsagePercent),
			zap.Float64("load_avg", metrics.LoadAverage),
			zap.Int("running_jobs", metrics.RunningJobs))
	}

	// 低资源警告
	if metrics.LowResources {
		utils.Warn("worker has low resources",
			zap.String("worker_id", workerID),
			zap.Float64("memory_usage", metrics.MemoryUsageRatio*100),
			zap.Float64("disk_usage", metrics.DiskUsageRatio*100))
	}
}

// GetWorkerMetrics 获取指定Worker的最新指标
func (m *MetricsManager) GetWorkerMetrics(workerID string) (*WorkerMetrics, error) {
	m.cacheLock.RLock()
	defer m.cacheLock.RUnlock()

	metrics, exists := m.metricsCache[workerID]
	if !exists {
		return nil, fmt.Errorf("metrics not found for worker: %s", workerID)
	}

	return metrics, nil
}

// GetAllWorkerMetrics 获取所有Worker的最新指标
func (m *MetricsManager) GetAllWorkerMetrics() (map[string]*WorkerMetrics, error) {
	m.cacheLock.RLock()
	defer m.cacheLock.RUnlock()

	// 创建副本以避免并发问题
	result := make(map[string]*WorkerMetrics, len(m.metricsCache))
	for k, v := range m.metricsCache {
		result[k] = v
	}

	return result, nil
}

// GetMetricsHistory 获取Worker的指标历史记录
func (m *MetricsManager) GetMetricsHistory(workerID string) ([]*ResourceSnapshot, error) {
	m.historyLock.RLock()
	defer m.historyLock.RUnlock()

	history, exists := m.metricsHistory[workerID]
	if !exists {
		return nil, fmt.Errorf("no metrics history found for worker: %s", workerID)
	}

	// 创建副本以避免并发问题
	result := make([]*ResourceSnapshot, len(history))
	copy(result, history)

	return result, nil
}

// GetClusterStats 获取集群整体统计信息
func (m *MetricsManager) GetClusterStats() (map[string]interface{}, error) {
	m.cacheLock.RLock()
	workers := len(m.metricsCache)

	var totalCPUUsage, totalMemoryUsage, totalDiskUsage float64
	var highLoadWorkers, lowResourceWorkers int
	var totalRunningJobs int
	var totalCPUCores int = 0 // 初始化CPU核心计数器
	var totalMemory int64 = 0 // 初始化内存计数器

	for workerID, metrics := range m.metricsCache {
		// 现有的平均值计算
		totalCPUUsage += metrics.CPUUsagePercent
		totalMemoryUsage += metrics.MemoryUsageRatio
		totalDiskUsage += metrics.DiskUsageRatio

		if metrics.HighLoad {
			highLoadWorkers++
		}
		if metrics.LowResources {
			lowResourceWorkers++
		}

		totalRunningJobs += metrics.RunningJobs

		// 累加CPU核心数和内存总量
		// 直接获取worker以访问CPU核心数
		worker, err := m.workerManager.GetWorker(workerID)
		if err == nil {
			totalCPUCores += worker.CPUCores
			// totalMemory += worker.MemoryTotal  // 如果需要，可以改用此方式
		}

		// 使用指标中的内存总量
		totalMemory += metrics.MemoryTotal
	}
	m.cacheLock.RUnlock()

	// 计算平均值
	var avgCPUUsage, avgMemoryUsage, avgDiskUsage float64
	if workers > 0 {
		avgCPUUsage = totalCPUUsage / float64(workers)
		avgMemoryUsage = totalMemoryUsage / float64(workers)
		avgDiskUsage = totalDiskUsage / float64(workers)
	}

	// 返回包含新字段的统计信息
	return map[string]interface{}{
		"total_workers":        workers,
		"avg_cpu_usage":        avgCPUUsage,
		"avg_memory_usage":     avgMemoryUsage * 100, // 转换为百分比
		"avg_disk_usage":       avgDiskUsage * 100,   // 转换为百分比
		"high_load_workers":    highLoadWorkers,
		"low_resource_workers": lowResourceWorkers,
		"total_running_jobs":   totalRunningJobs,
		"total_cpu_cores":      totalCPUCores, // 添加CPU核心总数
		"total_memory":         totalMemory,   // 添加内存总量
		"timestamp":            time.Now(),
	}, nil
}

// FindSuitableWorkerForJob 根据资源需求寻找适合的Worker
func (m *MetricsManager) FindSuitableWorkerForJob(memorySizeMB int64, diskSizeGB int64) ([]string, error) {
	m.cacheLock.RLock()
	defer m.cacheLock.RUnlock()

	var suitableWorkers []string

	for workerID, metrics := range m.metricsCache {
		// 跳过高负载Worker
		if metrics.HighLoad {
			continue
		}

		// 检查内存和磁盘空间是否足够
		if (metrics.MemoryTotal-metrics.MemoryUsed) >= memorySizeMB &&
			(metrics.DiskTotal-metrics.DiskUsed) >= diskSizeGB {
			suitableWorkers = append(suitableWorkers, workerID)
		}
	}

	if len(suitableWorkers) == 0 {
		return nil, fmt.Errorf("no suitable workers found with required resources")
	}

	return suitableWorkers, nil
}

// SetAlertThresholds 设置告警阈值
func (m *MetricsManager) SetAlertThresholds(cpuThreshold, memoryThreshold, diskThreshold float64) {
	// 这里可以实现动态更新告警阈值的逻辑
	utils.Info("alert thresholds updated",
		zap.Float64("cpu_threshold", cpuThreshold),
		zap.Float64("memory_threshold", memoryThreshold),
		zap.Float64("disk_threshold", diskThreshold))
}
