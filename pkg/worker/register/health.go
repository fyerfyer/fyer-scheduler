package register

import (
	"context"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// HealthChecker 负责Worker的健康检查和心跳管理
type HealthChecker struct {
	workerRegister      *WorkerRegister
	resourceCollector   IResourceCollector
	heartbeatInterval   time.Duration
	heartbeatTimeout    time.Duration
	healthCheckInterval time.Duration
	isRunning           bool
	stopChan            chan struct{}
	lock                sync.RWMutex
	lastHeartbeatTime   time.Time
	lastHeartbeatError  error
	consecutiveErrors   int
}

// NewHealthChecker 创建一个新的健康检查器
func NewHealthChecker(
	workerRegister *WorkerRegister,
	resourceCollector IResourceCollector,
	heartbeatInterval time.Duration,
) *HealthChecker {
	if heartbeatInterval <= 0 {
		// 默认心跳间隔为5秒
		heartbeatInterval = time.Duration(constants.WorkerHeartbeatInterval) * time.Second
	}

	return &HealthChecker{
		workerRegister:      workerRegister,
		resourceCollector:   resourceCollector,
		heartbeatInterval:   heartbeatInterval,
		heartbeatTimeout:    time.Duration(constants.WorkerHeartbeatTimeout) * time.Second,
		healthCheckInterval: 30 * time.Second, // 默认30秒执行一次完整健康检查
		stopChan:            make(chan struct{}),
		lastHeartbeatTime:   time.Now(),
	}
}

// Start 启动健康检查和心跳发送
func (h *HealthChecker) Start() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	if h.isRunning {
		return nil // 已经在运行中
	}

	h.isRunning = true
	h.stopChan = make(chan struct{})

	// 启动心跳发送协程
	go h.heartbeatLoop()

	// 启动完整健康检查协程
	go h.healthCheckLoop()

	utils.Info("health checker started",
		zap.Duration("heartbeat_interval", h.heartbeatInterval),
		zap.Duration("health_check_interval", h.healthCheckInterval))

	return nil
}

// Stop 停止健康检查和心跳发送
func (h *HealthChecker) Stop() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	if !h.isRunning {
		return nil
	}

	h.isRunning = false
	close(h.stopChan)

	utils.Info("health checker stopped")
	return nil
}

// heartbeatLoop 定期发送心跳
func (h *HealthChecker) heartbeatLoop() {
	ticker := time.NewTicker(h.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopChan:
			return
		case <-ticker.C:
			err := h.sendHeartbeat()
			if err != nil {
				h.handleHeartbeatError(err)
			} else {
				h.resetHeartbeatErrors()
			}
		}
	}
}

// sendHeartbeat 发送心跳并更新资源信息
func (h *HealthChecker) sendHeartbeat() error {
	// 首先更新资源信息
	err := h.workerRegister.UpdateResources()
	if err != nil {
		utils.Warn("failed to update resources before heartbeat",
			zap.Error(err))
		// 继续发送心跳，即使资源更新失败
	}

	// 发送心跳
	err = h.workerRegister.Heartbeat()

	h.lock.Lock()
	h.lastHeartbeatTime = time.Now()
	h.lastHeartbeatError = err
	h.lock.Unlock()

	if err != nil {
		utils.Error("failed to send heartbeat",
			zap.Error(err))
		return err
	}

	utils.Debug("heartbeat sent successfully")
	return nil
}

// handleHeartbeatError 处理心跳发送错误
func (h *HealthChecker) handleHeartbeatError(err error) {
	h.lock.Lock()
	h.consecutiveErrors++
	h.lock.Unlock()

	// 如果连续错误次数超过阈值，尝试重新注册
	if h.consecutiveErrors >= 3 {
		utils.Warn("multiple heartbeat failures, attempting to re-register",
			zap.Int("consecutive_errors", h.consecutiveErrors),
			zap.Error(err))

		// 尝试重新注册
		_, registerErr := h.workerRegister.Register()
		if registerErr != nil {
			utils.Error("failed to re-register worker",
				zap.Error(registerErr))
		} else {
			utils.Info("worker re-registered successfully after heartbeat failures")
			h.resetHeartbeatErrors()
		}
	}
}

// resetHeartbeatErrors 重置连续错误计数
func (h *HealthChecker) resetHeartbeatErrors() {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.consecutiveErrors = 0
}

// healthCheckLoop 执行完整的健康检查
func (h *HealthChecker) healthCheckLoop() {
	ticker := time.NewTicker(h.healthCheckInterval)
	defer ticker.Stop()

	// 立即执行一次健康检查
	h.performHealthCheck()

	for {
		select {
		case <-h.stopChan:
			return
		case <-ticker.C:
			h.performHealthCheck()
		}
	}
}

// performHealthCheck 执行完整的健康检查
func (h *HealthChecker) performHealthCheck() {
	utils.Debug("performing health check")

	// 检查是否已超过心跳超时时间
	h.checkHeartbeatTimeout()

	// 检查系统资源
	h.checkSystemResources()

	// 检查是否可以连接到etcd
	h.checkEtcdConnection()
}

// checkHeartbeatTimeout 检查是否超过心跳超时
func (h *HealthChecker) checkHeartbeatTimeout() {
	h.lock.RLock()
	lastHeartbeat := h.lastHeartbeatTime
	h.lock.RUnlock()

	if time.Since(lastHeartbeat) > h.heartbeatTimeout {
		utils.Warn("heartbeat timeout detected",
			zap.Time("last_heartbeat", lastHeartbeat),
			zap.Duration("timeout", h.heartbeatTimeout))

		// 尝试立即发送心跳
		err := h.sendHeartbeat()
		if err != nil {
			utils.Error("failed to send heartbeat after timeout",
				zap.Error(err))
		}
	}
}

// checkSystemResources 检查系统资源使用情况
func (h *HealthChecker) checkSystemResources() {
	// 使用资源收集器获取最新资源信息
	if h.resourceCollector != nil {
		resourceInfo := h.resourceCollector.GetResourceInfo()

		// 检查系统资源是否处于警告水平
		if resourceInfo.MemoryFree < resourceInfo.MemoryTotal/10 { // 可用内存低于10%
			utils.Warn("low memory warning",
				zap.Int64("memory_free_mb", resourceInfo.MemoryFree),
				zap.Int64("memory_total_mb", resourceInfo.MemoryTotal))
		}

		if resourceInfo.DiskFree < resourceInfo.DiskTotal/10 { // 可用磁盘低于10%
			utils.Warn("low disk space warning",
				zap.Int64("disk_free_gb", resourceInfo.DiskFree),
				zap.Int64("disk_total_gb", resourceInfo.DiskTotal))
		}

		// 检查系统负载
		if resourceInfo.LoadAvg > float64(resourceInfo.CPUCores)*1.5 {
			utils.Warn("high system load",
				zap.Float64("load_avg", resourceInfo.LoadAvg),
				zap.Int("cpu_cores", resourceInfo.CPUCores))
		}
	}
}

// checkEtcdConnection 检查是否可以连接到etcd
func (h *HealthChecker) checkEtcdConnection() {
	// 通过Worker注册器获取etcd客户端状态
	worker := h.workerRegister.GetWorker()
	if worker == nil {
		utils.Error("worker is not available")
		return
	}

	// 使用一个简单的Get操作检查连接
	// 这里使用了上下文超时来避免阻塞
	_, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 这里实际上我们没有直接访问etcdClient，因为它在WorkerRegister中是私有的
	// 我们通过检查心跳状态间接判断etcd连接状态
	h.lock.RLock()
	lastError := h.lastHeartbeatError
	h.lock.RUnlock()

	if lastError != nil {
		utils.Warn("etcd connection might have issues based on last heartbeat",
			zap.Error(lastError))
	}
}

// GetHealthStatus 获取健康状态信息
func (h *HealthChecker) GetHealthStatus() map[string]interface{} {
	h.lock.RLock()
	defer h.lock.RUnlock()

	status := map[string]interface{}{
		"is_running":            h.isRunning,
		"last_heartbeat_time":   h.lastHeartbeatTime,
		"consecutive_errors":    h.consecutiveErrors,
		"heartbeat_interval":    h.heartbeatInterval.String(),
		"health_check_interval": h.healthCheckInterval.String(),
	}

	// 添加资源信息
	if h.resourceCollector != nil {
		resourceInfo := h.resourceCollector.GetResourceInfo()
		status["resource_info"] = resourceInfo
	}

	// 添加Worker状态信息
	if h.workerRegister != nil {
		worker := h.workerRegister.GetWorker()
		if worker != nil {
			status["worker_status"] = worker.Status
			status["worker_id"] = worker.ID
		}
	}

	return status
}

// IsHealthy 检查Worker是否健康
func (h *HealthChecker) IsHealthy() bool {
	h.lock.RLock()
	defer h.lock.RUnlock()

	// 检查基本运行状态
	if !h.isRunning {
		return false
	}

	// 检查心跳状态
	if time.Since(h.lastHeartbeatTime) > h.heartbeatTimeout {
		return false
	}

	// 检查连续错误次数
	if h.consecutiveErrors >= 3 {
		return false
	}

	return true
}
