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

// WorkerDiscovery 负责发现和跟踪Worker节点
type WorkerDiscovery struct {
	workerManager     WorkerManager
	workerRepo        repo.IWorkerRepo
	discoveredWorkers map[string]*models.Worker
	etcdClient        *utils.EtcdClient
	mongoClient       *utils.MongoDBClient
	workerLock        sync.RWMutex
	handlers          []WorkerChangeHandler
	handlerLock       sync.RWMutex
}

// NewWorkerDiscovery 创建一个新的Worker发现器
func NewWorkerDiscovery(workerManager WorkerManager, workerRepo repo.IWorkerRepo,
	etcdClient *utils.EtcdClient, mongoClient *utils.MongoDBClient) *WorkerDiscovery {
	return &WorkerDiscovery{
		workerManager:     workerManager,
		workerRepo:        workerRepo,
		etcdClient:        etcdClient,
		mongoClient:       mongoClient,
		discoveredWorkers: make(map[string]*models.Worker),
		handlers:          make([]WorkerChangeHandler, 0),
	}
}

// Start 启动Worker发现服务
func (d *WorkerDiscovery) Start() {
	utils.Info("starting worker discovery service")

	// 设置etcd监听，监控Worker注册事件
	d.watchWorkerRegistry()

	// 加载现有Worker
	d.loadExistingWorkers()
}

// RegisterHandler 注册Worker变更处理器
func (d *WorkerDiscovery) RegisterHandler(handler WorkerChangeHandler) {
	d.handlerLock.Lock()
	defer d.handlerLock.Unlock()

	d.handlers = append(d.handlers, handler)
}

// watchWorkerRegistry 监控Worker注册信息变化
func (d *WorkerDiscovery) watchWorkerRegistry() {
	d.etcdClient.WatchWithPrefix(constants.WorkerPrefix, func(eventType, key, value string) {
		// 只处理PUT和DELETE事件
		if eventType != "PUT" && eventType != "DELETE" {
			return
		}

		// 提取Worker ID
		workerID := key[len(constants.WorkerPrefix):]

		if eventType == "PUT" {
			d.handleWorkerAddOrUpdate(workerID, value)
		} else if eventType == "DELETE" {
			d.handleWorkerRemoval(workerID)
		}
	})

	utils.Info("worker registry watch started")
}

// loadExistingWorkers 加载已存在的Worker节点
func (d *WorkerDiscovery) loadExistingWorkers() {
	workers, err := d.workerRepo.ListAll()
	if err != nil {
		utils.Error("failed to load existing workers", zap.Error(err))
		return
	}

	// 更新内存中的Worker列表
	d.workerLock.Lock()
	defer d.workerLock.Unlock()

	for _, worker := range workers {
		d.discoveredWorkers[worker.ID] = worker
		utils.Info("loaded existing worker",
			zap.String("worker_id", worker.ID),
			zap.String("hostname", worker.Hostname),
			zap.String("ip", worker.IP),
			zap.String("status", worker.Status))
	}

	utils.Info("loaded existing workers", zap.Int("count", len(d.discoveredWorkers)))
}

// handleWorkerAddOrUpdate 处理Worker添加或更新事件
func (d *WorkerDiscovery) handleWorkerAddOrUpdate(workerID, workerJSON string) {
	worker, err := models.WorkerFromJSON(workerJSON)
	if err != nil {
		utils.Error("failed to parse worker data",
			zap.String("worker_id", workerID),
			zap.Error(err))
		return
	}

	// 更新内存中的Worker
	d.workerLock.Lock()
	isNewWorker := d.discoveredWorkers[workerID] == nil
	d.discoveredWorkers[workerID] = worker
	d.workerLock.Unlock()

	// 记录Worker添加或更新事件
	if isNewWorker {
		utils.Info("new worker discovered",
			zap.String("worker_id", worker.ID),
			zap.String("hostname", worker.Hostname),
			zap.String("ip", worker.IP))
	} else {
		utils.Debug("worker updated",
			zap.String("worker_id", worker.ID),
			zap.String("status", worker.Status))
	}

	// 通知处理器
	d.notifyHandlers("PUT", worker)
}

// handleWorkerRemoval 处理Worker移除事件
func (d *WorkerDiscovery) handleWorkerRemoval(workerID string) {
	d.workerLock.Lock()
	worker, exists := d.discoveredWorkers[workerID]
	if exists {
		delete(d.discoveredWorkers, workerID)
	}
	d.workerLock.Unlock()

	if exists {
		utils.Info("worker removed",
			zap.String("worker_id", workerID),
			zap.String("hostname", worker.Hostname),
			zap.String("ip", worker.IP))

		// 通知处理器
		d.notifyHandlers("DELETE", worker)

		// 更新Worker状态为离线
		worker.SetStatus(constants.WorkerStatusOffline)
		worker.UpdateTime = time.Now()

		// 保存到MongoDB以保留历史记录
		_, err := worker.ToJSON()
		if err == nil {
			// 只在MongoDB中保留记录，但从etcd中删除
			d.mongoClient.UpdateOne(
				"workers",
				map[string]interface{}{"id": worker.ID},
				map[string]interface{}{"$set": worker},
			)
		}
	}
}

// notifyHandlers 通知所有处理器Worker变更
func (d *WorkerDiscovery) notifyHandlers(eventType string, worker *models.Worker) {
	d.handlerLock.RLock()
	handlers := make([]WorkerChangeHandler, len(d.handlers))
	copy(handlers, d.handlers)
	d.handlerLock.RUnlock()

	for _, handler := range handlers {
		go handler(eventType, worker)
	}
}

// GetDiscoveredWorkers 获取当前发现的所有Worker
func (d *WorkerDiscovery) GetDiscoveredWorkers() []*models.Worker {
	d.workerLock.RLock()
	defer d.workerLock.RUnlock()

	workers := make([]*models.Worker, 0, len(d.discoveredWorkers))
	for _, worker := range d.discoveredWorkers {
		workers = append(workers, worker)
	}

	return workers
}

// GetWorkerByID 根据ID获取Worker
func (d *WorkerDiscovery) GetWorkerByID(workerID string) (*models.Worker, error) {
	d.workerLock.RLock()
	worker, exists := d.discoveredWorkers[workerID]
	d.workerLock.RUnlock()

	if !exists {
		return nil, fmt.Errorf("worker not found: %s", workerID)
	}

	return worker, nil
}

// GetWorkersByStatus 获取指定状态的Worker
func (d *WorkerDiscovery) GetWorkersByStatus(status string) []*models.Worker {
	d.workerLock.RLock()
	defer d.workerLock.RUnlock()

	var workers []*models.Worker
	for _, worker := range d.discoveredWorkers {
		if worker.Status == status {
			workers = append(workers, worker)
		}
	}

	return workers
}

// GetActiveWorkerCount 获取活跃Worker数量
func (d *WorkerDiscovery) GetActiveWorkerCount() int {
	d.workerLock.RLock()
	defer d.workerLock.RUnlock()

	count := 0
	for _, worker := range d.discoveredWorkers {
		if worker.IsActive() {
			count++
		}
	}

	return count
}
