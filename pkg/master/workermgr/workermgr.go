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

// WorkerChangeHandler 是Worker变更处理器的类型定义
type WorkerChangeHandler func(eventType string, worker *models.Worker)

// WorkerManager 定义Worker管理器接口
type WorkerManager interface {
	// Worker管理操作
	ListWorkers(page, pageSize int64) ([]*models.Worker, int64, error)
	GetWorker(workerID string) (*models.Worker, error)
	EnableWorker(workerID string) error
	DisableWorker(workerID string) error

	// 状态管理
	GetActiveWorkers() ([]*models.Worker, error)
	CheckWorkerHealth(workerID string) (bool, error)
	UpdateWorkerLabels(workerID string, labels map[string]string) error

	// 事件处理
	WatchWorkerChanges(handler WorkerChangeHandler)
	NotifyStatusChange(worker *models.Worker)

	// 系统操作
	Start() error
	Stop() error
}

// MasterWorkerManager 实现WorkerManager接口，负责Master节点的Worker管理
type MasterWorkerManager struct {
	workerRepo          repo.IWorkerRepo
	watchHandlers       []WorkerChangeHandler
	etcdClient          *utils.EtcdClient
	watchHandlerLock    sync.RWMutex
	isRunning           bool
	stopChan            chan struct{}
	healthCheckWg       sync.WaitGroup
	healthCheckInterval time.Duration
}

// NewWorkerManager 创建一个新的Worker管理器
func NewWorkerManager(workerRepo repo.IWorkerRepo, healthCheckInterval time.Duration, etcdClient *utils.EtcdClient) WorkerManager {
	if healthCheckInterval <= 0 {
		healthCheckInterval = 30 * time.Second // 默认30秒检查一次
	}

	return &MasterWorkerManager{
		workerRepo:          workerRepo,
		watchHandlers:       make([]WorkerChangeHandler, 0),
		etcdClient:          etcdClient,
		isRunning:           false,
		stopChan:            make(chan struct{}),
		healthCheckInterval: healthCheckInterval,
	}
}

// ListWorkers 获取Worker列表
func (m *MasterWorkerManager) ListWorkers(page, pageSize int64) ([]*models.Worker, int64, error) {
	workers, err := m.workerRepo.ListAll()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list workers: %w", err)
	}

	// 简单的内存分页
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = constants.DefaultPageSize
	}

	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize
	total := int64(len(workers))

	if startIndex >= total {
		return []*models.Worker{}, total, nil
	}

	if endIndex > total {
		endIndex = total
	}

	return workers[startIndex:endIndex], total, nil
}

// GetWorker 获取指定Worker详情
func (m *MasterWorkerManager) GetWorker(workerID string) (*models.Worker, error) {
	worker, err := m.workerRepo.GetByID(workerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get worker: %w", err)
	}
	return worker, nil
}

// EnableWorker 启用Worker
func (m *MasterWorkerManager) EnableWorker(workerID string) error {
	err := m.workerRepo.EnableWorker(workerID)
	if err != nil {
		return fmt.Errorf("failed to enable worker: %w", err)
	}

	utils.Info("worker enabled successfully", zap.String("worker_id", workerID))
	return nil
}

// DisableWorker 禁用Worker
func (m *MasterWorkerManager) DisableWorker(workerID string) error {
	err := m.workerRepo.DisableWorker(workerID)
	if err != nil {
		return fmt.Errorf("failed to disable worker: %w", err)
	}

	utils.Info("worker disabled successfully", zap.String("worker_id", workerID))
	return nil
}

// GetActiveWorkers 获取所有活跃的Worker
func (m *MasterWorkerManager) GetActiveWorkers() ([]*models.Worker, error) {
	return m.workerRepo.ListActive()
}

// CheckWorkerHealth 检查Worker健康状态
func (m *MasterWorkerManager) CheckWorkerHealth(workerID string) (bool, error) {
	worker, err := m.workerRepo.GetByID(workerID)
	if err != nil {
		return false, fmt.Errorf("failed to get worker: %w", err)
	}

	return worker.IsActive(), nil
}

// UpdateWorkerLabels 更新Worker标签
func (m *MasterWorkerManager) UpdateWorkerLabels(workerID string, labels map[string]string) error {
	worker, err := m.workerRepo.GetByID(workerID)
	if err != nil {
		return fmt.Errorf("failed to get worker: %w", err)
	}

	// 更新标签
	if worker.Labels == nil {
		worker.Labels = make(map[string]string)
	}

	for k, v := range labels {
		worker.Labels[k] = v
	}

	// 保存更新后的Worker
	workerJSON, err := worker.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize worker: %w", err)
	}

	err = m.etcdClient.Put(worker.Key(), workerJSON)
	if err != nil {
		return fmt.Errorf("failed to update worker labels: %w", err)
	}

	utils.Info("worker labels updated",
		zap.String("worker_id", workerID),
		zap.Any("labels", labels))

	return nil
}

// WatchWorkerChanges 添加Worker变更处理器
func (m *MasterWorkerManager) WatchWorkerChanges(handler WorkerChangeHandler) {
	m.watchHandlerLock.Lock()
	defer m.watchHandlerLock.Unlock()

	m.watchHandlers = append(m.watchHandlers, handler)
}

// Start 启动Worker管理器
func (m *MasterWorkerManager) Start() error {
	if m.isRunning {
		return nil // 已经在运行中
	}

	m.isRunning = true
	m.stopChan = make(chan struct{})

	// 启动Worker变更监控
	go m.watchWorkers()

	// 启动健康检查
	m.healthCheckWg.Add(1)
	go m.healthCheckLoop()

	utils.Info("worker manager started")
	return nil
}

// Stop 停止Worker管理器
func (m *MasterWorkerManager) Stop() error {
	if !m.isRunning {
		return nil
	}

	m.isRunning = false
	close(m.stopChan)

	// 等待健康检查结束
	m.healthCheckWg.Wait()

	utils.Info("worker manager stopped")
	return nil
}

// watchWorkers 监控Worker变更
func (m *MasterWorkerManager) watchWorkers() {
	utils.Info("starting worker change watcher")

	// 使用etcd客户端设置前缀监听
	m.etcdClient.WatchWithPrefix(constants.WorkerPrefix, func(eventType, key, value string) {
		// 只处理有效事件
		if eventType != "PUT" && eventType != "DELETE" {
			return
		}

		// 提取Worker ID
		workerID := key[len(constants.WorkerPrefix):]

		var worker *models.Worker
		var err error

		// 获取Worker详情（如果不是删除事件）
		if eventType == "PUT" {
			worker, err = models.WorkerFromJSON(value)
			if err != nil {
				utils.Error("failed to parse worker data from event",
					zap.String("worker_id", workerID),
					zap.Error(err))
				return
			}
		}

		// 通知所有处理器
		m.notifyHandlers(eventType, worker)
	})
}

// healthCheckLoop 定期进行Worker健康检查
func (m *MasterWorkerManager) healthCheckLoop() {
	defer m.healthCheckWg.Done()

	ticker := time.NewTicker(m.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkAllWorkersHealth()
		}
	}
}

// checkAllWorkersHealth 检查所有Worker的健康状态
func (m *MasterWorkerManager) checkAllWorkersHealth() {
	workers, err := m.workerRepo.ListAll()
	if err != nil {
		utils.Error("failed to list workers for health check", zap.Error(err))
		return
	}

	for _, worker := range workers {
		// 跳过已禁用的Worker
		if worker.Status == constants.WorkerStatusDisabled {
			continue
		}

		// 检查心跳是否超时
		heartbeatTimeout := time.Duration(constants.WorkerHeartbeatTimeout) * time.Second
		isActive := time.Since(worker.LastHeartbeat) <= heartbeatTimeout

		// 如果状态与当前不符，更新状态
		if isActive && worker.Status != constants.WorkerStatusOnline {
			worker.SetStatus(constants.WorkerStatusOnline)
			m.updateWorkerStatus(worker)
		} else if !isActive && worker.Status != constants.WorkerStatusOffline {
			worker.SetStatus(constants.WorkerStatusOffline)
			m.updateWorkerStatus(worker)
		}
	}
}

// updateWorkerStatus 更新Worker状态
func (m *MasterWorkerManager) updateWorkerStatus(worker *models.Worker) {
	workerJSON, err := worker.ToJSON()
	if err != nil {
		utils.Error("failed to serialize worker",
			zap.String("worker_id", worker.ID),
			zap.Error(err))
		return
	}

	err = m.etcdClient.Put(worker.Key(), workerJSON)
	if err != nil {
		utils.Error("failed to update worker status",
			zap.String("worker_id", worker.ID),
			zap.String("status", worker.Status),
			zap.Error(err))
		return
	}

	utils.Info("worker status updated",
		zap.String("worker_id", worker.ID),
		zap.String("status", worker.Status))
}

// notifyHandlers 通知所有处理器Worker变更
func (m *MasterWorkerManager) notifyHandlers(eventType string, worker *models.Worker) {
	m.watchHandlerLock.RLock()
	defer m.watchHandlerLock.RUnlock()

	for _, handler := range m.watchHandlers {
		go handler(eventType, worker)
	}
}

// NotifyStatusChange 通知Worker状态变更
func (m *MasterWorkerManager) NotifyStatusChange(worker *models.Worker) {
	m.notifyHandlers("STATUS_CHANGE", worker)
}
