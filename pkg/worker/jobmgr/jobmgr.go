package jobmgr

import (
	"fmt"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// WorkerJobManager 实现IWorkerJobManager接口
type WorkerJobManager struct {
	etcdClient  *utils.EtcdClient      // etcd客户端
	workerID    string                 // 当前Worker的ID
	handlers    []IJobEventHandler     // 任务事件处理器
	jobCache    map[string]*models.Job // 任务缓存
	runningJobs map[string]*models.Job // 当前正在运行的任务
	isRunning   bool                   // 标记是否正在运行
	stopChan    chan struct{}          // 用于停止监听的通道
	lock        sync.RWMutex           // 保护并发访问
}

// NewWorkerJobManager 创建Worker的任务管理器
func NewWorkerJobManager(etcdClient *utils.EtcdClient, workerID string) IWorkerJobManager {
	return &WorkerJobManager{
		etcdClient:  etcdClient,
		workerID:    workerID,
		handlers:    make([]IJobEventHandler, 0),
		jobCache:    make(map[string]*models.Job),
		runningJobs: make(map[string]*models.Job),
		stopChan:    make(chan struct{}),
	}
}

// Start 启动任务管理器
func (m *WorkerJobManager) Start() error {
	m.lock.Lock()

	if m.isRunning {
		return nil // 已经在运行
	}

	utils.Info("Starting job manager init")
	m.isRunning = true
	m.stopChan = make(chan struct{})
	m.lock.Unlock()

	// 加载所有当前任务
	if err := m.loadJobs(); err != nil {
		utils.Warn("failed to load initial jobs", zap.Error(err))
		// 继续启动，不因初始加载失败而阻止服务启动
	}

	utils.Info("Jobs loaded successfully")

	// 启动监听协程
	utils.Info("About to start watching jobs")
	go m.WatchJobs()
	utils.Info("Watch goroutine started")

	utils.Info("worker job manager started", zap.String("worker_id", m.workerID))
	return nil
}

// Stop 停止任务管理器
func (m *WorkerJobManager) Stop() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if !m.isRunning {
		return nil // 已经停止
	}

	m.isRunning = false
	close(m.stopChan)

	utils.Info("worker job manager stopped", zap.String("worker_id", m.workerID))
	return nil
}

// WatchJobs 监听任务变化
func (m *WorkerJobManager) WatchJobs() {
	utils.Info("starting job watch", zap.String("worker_id", m.workerID))

	// 监听所有任务的变化
	m.etcdClient.WatchWithPrefix(constants.JobPrefix, func(eventType, key, value string) {
		// 解析任务ID
		jobID := key[len(constants.JobPrefix):]

		// 根据事件类型处理
		switch eventType {
		case "PUT": // 任务创建或更新
			if value == "" {
				utils.Warn("received empty job value", zap.String("job_id", jobID))
				return
			}

			// 解析任务数据
			job, err := models.FromJSON(value)
			if err != nil {
				utils.Error("failed to parse job data",
					zap.String("job_id", jobID),
					zap.Error(err))
				return
			}

			// 检查任务是否分配给当前Worker
			if m.IsJobAssignedToWorker(job) {
				m.handleJobUpdate(job)
			}

		case "DELETE": // 任务删除
			m.handleJobDelete(jobID)
		}
	})
}

// GetJob 根据任务ID获取任务
func (m *WorkerJobManager) GetJob(jobID string) (*models.Job, error) {
	// 先从缓存中查找
	m.lock.RLock()
	job, exists := m.jobCache[jobID]
	m.lock.RUnlock()

	if exists {
		return job, nil
	}

	// 缓存中不存在，从etcd获取
	jobKey := constants.JobPrefix + jobID
	jobJSON, err := m.etcdClient.Get(jobKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get job from etcd: %w", err)
	}

	if jobJSON == "" {
		return nil, fmt.Errorf("job not found")
	}

	// 解析任务
	job, err = models.FromJSON(jobJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse job data: %w", err)
	}

	// 更新缓存
	m.lock.Lock()
	m.jobCache[jobID] = job
	m.lock.Unlock()

	return job, nil
}

// ListJobs 获取所有待处理的任务
func (m *WorkerJobManager) ListJobs() ([]*models.Job, error) {
	// 从etcd获取所有任务
	kvs, err := m.etcdClient.GetWithPrefix(constants.JobPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs from etcd: %w", err)
	}

	jobs := make([]*models.Job, 0, len(kvs))

	for _, value := range kvs {
		job, err := models.FromJSON(value)
		if err != nil {
			utils.Error("failed to parse job data", zap.Error(err))
			continue
		}

		// 只返回分配给当前Worker的任务
		if m.IsJobAssignedToWorker(job) {
			jobs = append(jobs, job)
		}
	}

	// 更新缓存
	m.lock.Lock()
	for _, job := range jobs {
		m.jobCache[job.ID] = job
	}
	m.lock.Unlock()

	return jobs, nil
}

// ListRunningJobs 获取正在运行的任务
func (m *WorkerJobManager) ListRunningJobs() ([]*models.Job, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	jobs := make([]*models.Job, 0, len(m.runningJobs))
	for _, job := range m.runningJobs {
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// RegisterHandler 注册任务事件处理器
func (m *WorkerJobManager) RegisterHandler(handler IJobEventHandler) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.handlers = append(m.handlers, handler)
	utils.Info("job event handler registered")
}

// ReportJobStatus 报告任务状态
func (m *WorkerJobManager) ReportJobStatus(job *models.Job, status string) error {
	// 更新任务状态
	job.SetStatus(status)
	job.UpdateTime = time.Now()

	// 确保任务被标记为由当前Worker执行
    if status == constants.JobStatusRunning {
        if job.Env == nil {
            job.Env = make(map[string]string)
        }
        job.Env["EXECUTOR_WORKER_ID"] = m.workerID
    }

	// 将更新后的任务状态保存到etcd
	jobJSON, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	if err := m.etcdClient.Put(job.Key(), jobJSON); err != nil {
		return fmt.Errorf("failed to update job status in etcd: %w", err)
	}

	// 更新本地缓存
	m.lock.Lock()
	m.jobCache[job.ID] = job

	// 根据状态更新运行中的任务列表
	if status == constants.JobStatusRunning {
		m.runningJobs[job.ID] = job
	} else {
		delete(m.runningJobs, job.ID)
	}
	m.lock.Unlock()

	utils.Info("job status updated",
		zap.String("job_id", job.ID),
		zap.String("status", status))

	return nil
}

// KillJob 终止任务
func (m *WorkerJobManager) KillJob(jobID string) error {
	// 通过etcd发出终止信号
	killKey := constants.JobPrefix + jobID + "/kill"
	killValue := fmt.Sprintf("kill_signal_%d", time.Now().Unix()) // 添加时间戳
    err := m.etcdClient.Put(killKey, killValue)
	if err != nil {
		return fmt.Errorf("failed to send kill signal: %w", err)
	}

	utils.Info("kill signal sent for job", zap.String("job_id", jobID))
	return nil
}

// IsJobAssignedToWorker 检查任务是否分配给当前Worker
func (m *WorkerJobManager) IsJobAssignedToWorker(job *models.Job) bool {
	// 检查任务是否指定了特定Worker
	if job.Env != nil {
		if assignedWorkerID, exists := job.Env["ASSIGNED_WORKER_ID"]; exists {
			return assignedWorkerID == m.workerID
		}

		if workerID, exists := job.Env["WORKER_ID"]; exists {
            return workerID == m.workerID
        }
	}

	// 如果任务没有指定Worker，则根据其他规则判断
	// 1. 检查任务是否有标签要求
	// 2. 检查任务是否已经在本地运行
	// 3. 可以实现更复杂的分配规则

	// 简单实现：如果任务已在运行中列表，则认为是分配给当前Worker的
	m.lock.RLock()
	_, isRunning := m.runningJobs[job.ID]
	m.lock.RUnlock()

	if isRunning {
		return true
	}

	// 针对特定状态的任务，检查是否应该由本Worker处理
	if job.Status == constants.JobStatusRunning {
		// 检查是否由本Worker执行
		return job.Env != nil && job.Env["EXECUTOR_WORKER_ID"] == m.workerID
	}

	// 默认：只有在没有Worker ID的情况下，才允许其他Worker处理
	// 这可以根据实际需求进行调整
	if job.Env != nil && (job.Env["ASSIGNED_WORKER_ID"] != "" || job.Env["WORKER_ID"] != "") {
        return false  // If any worker assignment exists but doesn't match, reject
    }

	return true // 默认允许所有Worker处理所有任务
}

// GetWorkerID 获取当前Worker ID
func (m *WorkerJobManager) GetWorkerID() string {
	return m.workerID
}

// loadJobs 从etcd加载所有任务
func (m *WorkerJobManager) loadJobs() error {
	utils.Info("loading initial jobs from etcd")

	jobs, err := m.ListJobs()
	if err != nil {
		return err
	}

	utils.Info("loaded jobs from etcd", zap.Int("count", len(jobs)))
	return nil
}

// handleJobUpdate 处理任务更新事件
func (m *WorkerJobManager) handleJobUpdate(job *models.Job) {
	utils.Debug("job update received",
		zap.String("job_id", job.ID),
		zap.String("status", job.Status))

	// 更新缓存
	m.lock.Lock()
	m.jobCache[job.ID] = job

	// 如果任务正在运行，更新运行中的任务列表
	if job.Status == constants.JobStatusRunning {
		m.runningJobs[job.ID] = job
	} else {
		delete(m.runningJobs, job.ID)
	}
	m.lock.Unlock()

	// 创建任务事件
	event := &JobEvent{
		Type: JobEventSave,
		Job:  job,
	}

	// 通知所有处理器
	m.notifyHandlers(event)
}

// handleJobDelete 处理任务删除事件
func (m *WorkerJobManager) handleJobDelete(jobID string) {
	utils.Debug("job delete received", zap.String("job_id", jobID))

	// 先获取任务的当前状态（如果缓存中有）
	m.lock.RLock()
	job, exists := m.jobCache[jobID]
	m.lock.RUnlock()

	if !exists {
		// 如果缓存中没有，则可能是要删除一个从未处理过的任务
		utils.Debug("deleted job not found in cache", zap.String("job_id", jobID))
		return
	}

	// 从缓存移除
	m.lock.Lock()
	delete(m.jobCache, jobID)
	delete(m.runningJobs, jobID)
	m.lock.Unlock()

	// 创建任务事件
	event := &JobEvent{
		Type: JobEventDelete,
		Job:  job,
	}

	// 通知所有处理器
	m.notifyHandlers(event)
}

// notifyHandlers 通知所有注册的事件处理器
func (m *WorkerJobManager) notifyHandlers(event *JobEvent) {
	m.lock.RLock()
	handlers := make([]IJobEventHandler, len(m.handlers))
	copy(handlers, m.handlers)
	m.lock.RUnlock()

	for _, handler := range handlers {
		handler.HandleJobEvent(event)
	}
}
