package workermgr

import (
	"fmt"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// HealthChecker 负责Worker节点的健康检查
type HealthChecker struct {
	workerManager      WorkerManager
	workerRepo         repo.IWorkerRepo
	etcdClient         utils.EtcdClient
	interval           time.Duration
	heartbeatTimeout   time.Duration
	isRunning          bool
	stopChan           chan struct{}
	statusChangesMutex sync.RWMutex
	statusChanges      map[string]string // workerID -> newStatus
	healthCheckWg      sync.WaitGroup
}

// NewHealthChecker 创建一个新的健康检查器
func NewHealthChecker(workerManager WorkerManager, workerRepo repo.IWorkerRepo,
	interval time.Duration, etcdClient utils.EtcdClient) *HealthChecker {
	if interval <= 0 {
		interval = 30 * time.Second // 默认30秒检查一次
	}

	heartbeatTimeout := time.Duration(constants.WorkerHeartbeatTimeout) * time.Second

	return &HealthChecker{
		workerManager:    workerManager,
		workerRepo:       workerRepo,
		etcdClient:       etcdClient,
		interval:         interval,
		heartbeatTimeout: heartbeatTimeout,
		isRunning:        false,
		stopChan:         make(chan struct{}),
		statusChanges:    make(map[string]string),
	}
}

// Start 启动健康检查器
func (h *HealthChecker) Start() {
	if h.isRunning {
		return
	}

	h.isRunning = true
	h.stopChan = make(chan struct{})

	h.healthCheckWg.Add(1)
	go h.runHealthCheck()

	utils.Info("worker health checker started", zap.Duration("interval", h.interval))
}

// Stop 停止健康检查器
func (h *HealthChecker) Stop() {
	if !h.isRunning {
		return
	}

	h.isRunning = false
	close(h.stopChan)
	h.healthCheckWg.Wait()
	utils.Info("worker health checker stopped")
}

// runHealthCheck 执行健康检查循环
func (h *HealthChecker) runHealthCheck() {
	defer h.healthCheckWg.Done()

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// 立即执行一次健康检查
	h.checkAllWorkers()

	for {
		select {
		case <-h.stopChan:
			return
		case <-ticker.C:
			h.checkAllWorkers()
		}
	}
}

// checkAllWorkers 检查所有Worker的健康状态
func (h *HealthChecker) checkAllWorkers() {
	workers, err := h.workerRepo.ListAll()
	if err != nil {
		utils.Error("failed to list workers for health check", zap.Error(err))
		return
	}

	for _, worker := range workers {
		// 跳过已禁用的Worker
		if worker.Status == constants.WorkerStatusDisabled {
			continue
		}

		h.checkWorkerHealth(worker)
	}

	// 处理状态变更
	h.processStatusChanges()
}

// checkWorkerHealth 检查单个Worker的健康状态
func (h *HealthChecker) checkWorkerHealth(worker *models.Worker) {
	utils.Info("checking worker health",
		zap.String("worker_id", worker.ID),
		zap.String("current_status", worker.Status),
		zap.Time("last_heartbeat", worker.LastHeartbeat),
		zap.Float64("heartbeat_age_seconds", time.Since(worker.LastHeartbeat).Seconds()))

	isActive := h.isWorkerActive(worker)
	currentStatus := worker.Status

	// 检测状态变化
	if isActive && currentStatus != constants.WorkerStatusOnline {
		// Worker恢复在线
		h.registerStatusChange(worker.ID, constants.WorkerStatusOnline)
		utils.Info("worker is back online",
			zap.String("worker_id", worker.ID),
			zap.String("hostname", worker.Hostname),
			zap.String("ip", worker.IP))
	} else if !isActive && currentStatus != constants.WorkerStatusOffline {
		// Worker离线
		h.registerStatusChange(worker.ID, constants.WorkerStatusOffline)
		utils.Warn("worker is offline",
			zap.String("worker_id", worker.ID),
			zap.String("hostname", worker.Hostname),
			zap.String("ip", worker.IP),
			zap.Time("last_heartbeat", worker.LastHeartbeat),
			zap.Duration("heartbeat_age", time.Since(worker.LastHeartbeat)))
	}
}

// isWorkerActive 判断Worker是否活跃
func (h *HealthChecker) isWorkerActive(worker *models.Worker) bool {
	// 如果被禁用，则不活跃
	if worker.Status == constants.WorkerStatusDisabled {
		return false
	}

	// 检查心跳是否超时
	if time.Since(worker.LastHeartbeat) > h.heartbeatTimeout {
		return false
	}

	return true
}

// registerStatusChange 记录Worker状态变更
func (h *HealthChecker) registerStatusChange(workerID, newStatus string) {
	h.statusChangesMutex.Lock()
	defer h.statusChangesMutex.Unlock()
	h.statusChanges[workerID] = newStatus
}

// processStatusChanges 处理所有状态变更
func (h *HealthChecker) processStatusChanges() {
	h.statusChangesMutex.Lock()
	changes := make(map[string]string)
	for k, v := range h.statusChanges {
		changes[k] = v
	}
	h.statusChanges = make(map[string]string)
	h.statusChangesMutex.Unlock()

	// 更新所有已变更的Worker状态
	for workerID, newStatus := range changes {
		h.updateWorkerStatus(workerID, newStatus)
	}
}

// updateWorkerStatus 更新Worker的状态
func (h *HealthChecker) updateWorkerStatus(workerID, newStatus string) {
	worker, err := h.workerRepo.GetByID(workerID)
	if err != nil {
		utils.Error("failed to get worker for status update",
			zap.String("worker_id", workerID),
			zap.Error(err))
		return
	}

	// 更新状态
	worker.SetStatus(newStatus)

	// 保存到etcd
	workerJSON, err := worker.ToJSON()
	if err != nil {
		utils.Error("failed to serialize worker",
			zap.String("worker_id", workerID),
			zap.Error(err))
		return
	}

	err = h.etcdClient.Put(worker.Key(), workerJSON)
	if err != nil {
		utils.Error("failed to update worker status",
			zap.String("worker_id", workerID),
			zap.String("status", newStatus),
			zap.Error(err))
		return
	}

	utils.Info("worker status updated",
		zap.String("worker_id", workerID),
		zap.String("status", newStatus))

	// 通知状态变更
	h.notifyStatusChange(worker)
}

// notifyStatusChange 通知Worker状态变更
func (h *HealthChecker) notifyStatusChange(worker *models.Worker) {
	// 通过事件处理器通知状态变更
	h.workerManager.NotifyStatusChange(worker)
}

// CheckWorkerHealth 检查指定Worker的健康状态
func (h *HealthChecker) CheckWorkerHealth(workerID string) (bool, error) {
	worker, err := h.workerRepo.GetByID(workerID)
	if err != nil {
		return false, fmt.Errorf("failed to get worker: %w", err)
	}

	isActive := h.isWorkerActive(worker)
	return isActive, nil
}

// GetUnhealthyWorkers 获取不健康的Worker列表
func (h *HealthChecker) GetUnhealthyWorkers() ([]*models.Worker, error) {
	workers, err := h.workerRepo.ListAll()
	if err != nil {
		return nil, fmt.Errorf("failed to list workers: %w", err)
	}

	var unhealthy []*models.Worker
	for _, worker := range workers {
		if worker.Status != constants.WorkerStatusDisabled && !h.isWorkerActive(worker) {
			unhealthy = append(unhealthy, worker)
		}
	}

	return unhealthy, nil
}

// ProcessHeartbeat 处理Worker心跳
func (h *HealthChecker) ProcessHeartbeat(workerID string, resources map[string]interface{}) error {
	worker, err := h.workerRepo.GetByID(workerID)
	if err != nil {
		return fmt.Errorf("failed to get worker: %w", err)
	}

	// 更新心跳时间
	worker.UpdateHeartbeat()

	// 更新资源信息（如果提供）
	if resources != nil {
		if cpu, ok := resources["cpu_load"].(float64); ok {
			worker.LoadAvg = cpu
		}
		if memFree, ok := resources["mem_free"].(int64); ok {
			worker.MemoryFree = memFree
		}
		if diskFree, ok := resources["disk_free"].(int64); ok {
			worker.DiskFree = diskFree
		}
	}

	// 如果Worker之前离线，现在恢复在线
	if worker.Status == constants.WorkerStatusOffline {
		worker.SetStatus(constants.WorkerStatusOnline)
		utils.Info("worker is back online after heartbeat",
			zap.String("worker_id", worker.ID),
			zap.String("hostname", worker.Hostname))
	}

	// 保存更新后的Worker
	return h.workerRepo.Heartbeat(worker)
}
