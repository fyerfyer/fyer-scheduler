package jobmgr

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

// JobChangeHandler 是任务变更处理器的类型定义
type JobChangeHandler func(eventType string, job *models.Job)

// JobManager 定义任务管理器接口
type JobManager interface {
	// 任务CRUD操作
	CreateJob(job *models.Job) error
	GetJob(jobID string) (*models.Job, error)
	UpdateJob(job *models.Job) error
	DeleteJob(jobID string) error
	ListJobs(page, pageSize int64) ([]*models.Job, int64, error)
	ListJobsByStatus(status string, page, pageSize int64) ([]*models.Job, int64, error)

	// 任务状态操作
	EnableJob(jobID string) error
	DisableJob(jobID string) error

	// 手动触发操作
	TriggerJob(jobID string) error
	CancelJob(jobID, reason string) error

	// 状态跟踪相关
	WatchJobChanges(handler JobChangeHandler)
	GetDueJobs() ([]*models.Job, error)

	// 启动和停止
	Start() error
	Stop() error
}

// MasterJobManager 实现JobManager接口，负责Master节点的任务管理
type MasterJobManager struct {
	jobRepo          repo.IJobRepo
	logRepo          repo.ILogRepo
	workerRepo       repo.IWorkerRepo
	watchHandlers    []JobChangeHandler
	etcdClient       *utils.EtcdClient
	watchHandlerLock sync.RWMutex
	isRunning        bool
	stopChan         chan struct{}
}

// NewJobManager 创建一个新的任务管理器
func NewJobManager(jobRepo repo.IJobRepo, logRepo repo.ILogRepo, workerRepo repo.IWorkerRepo, etcdClient *utils.EtcdClient) JobManager {
	return &MasterJobManager{
		jobRepo:       jobRepo,
		logRepo:       logRepo,
		workerRepo:    workerRepo,
		etcdClient:    etcdClient,
		watchHandlers: make([]JobChangeHandler, 0),
		isRunning:     false,
		stopChan:      make(chan struct{}),
	}
}

// CreateJob 创建新任务
func (m *MasterJobManager) CreateJob(job *models.Job) error {
	// 任务验证
	if err := job.Validate(); err != nil {
		return fmt.Errorf("job validation failed: %w", err)
	}

	// 计算下次运行时间（如果有cron表达式）
	if job.CronExpr != "" {
		if err := job.CalculateNextRunTime(); err != nil {
			return fmt.Errorf("failed to calculate next run time: %w", err)
		}
	}

	// 保存任务
	if err := m.jobRepo.Save(job); err != nil {
		return fmt.Errorf("failed to save job: %w", err)
	}

	utils.Info("job created successfully",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name))

	return nil
}

// GetJob 获取任务详情
func (m *MasterJobManager) GetJob(jobID string) (*models.Job, error) {
	job, err := m.jobRepo.GetByID(jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}
	return job, nil
}

// UpdateJob 更新任务
func (m *MasterJobManager) UpdateJob(job *models.Job) error {
	// 验证任务是否存在
	existingJob, err := m.jobRepo.GetByID(job.ID)
	if err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	// 任务验证
	if err := job.Validate(); err != nil {
		return fmt.Errorf("job validation failed: %w", err)
	}

	// 保留创建时间
	job.CreateTime = existingJob.CreateTime
	job.UpdateTime = time.Now()

	// 计算下次运行时间
	if job.CronExpr != "" && job.CronExpr != existingJob.CronExpr {
		if err := job.CalculateNextRunTime(); err != nil {
			return fmt.Errorf("failed to calculate next run time: %w", err)
		}
	}

	// 保存任务
	if err := m.jobRepo.Save(job); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	utils.Info("job updated successfully",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name))

	return nil
}

// DeleteJob 删除任务
func (m *MasterJobManager) DeleteJob(jobID string) error {
	// 验证任务是否存在
	_, err := m.jobRepo.GetByID(jobID)
	if err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	// 删除任务
	if err := m.jobRepo.Delete(jobID); err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	utils.Info("job deleted successfully", zap.String("job_id", jobID))

	return nil
}

// ListJobs 获取任务列表
func (m *MasterJobManager) ListJobs(page, pageSize int64) ([]*models.Job, int64, error) {
	// 实际上，我们需要从MongoDB查询完整的任务列表
	// 这里的实现暂时使用etcd列出所有任务，但在生产环境中应该使用分页查询
	jobs, err := m.jobRepo.ListAll()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list jobs: %w", err)
	}

	// 简单的内存分页（实际实现应使用数据库分页）
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = constants.DefaultPageSize
	}

	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize
	total := int64(len(jobs))

	if startIndex >= total {
		return []*models.Job{}, total, nil
	}

	if endIndex > total {
		endIndex = total
	}

	return jobs[startIndex:endIndex], total, nil
}

// ListJobsByStatus 按状态获取任务列表
func (m *MasterJobManager) ListJobsByStatus(status string, page, pageSize int64) ([]*models.Job, int64, error) {
	return m.jobRepo.ListByStatus(status, page, pageSize)
}

// EnableJob 启用任务
func (m *MasterJobManager) EnableJob(jobID string) error {
	if err := m.jobRepo.EnableJob(jobID); err != nil {
		return fmt.Errorf("failed to enable job: %w", err)
	}

	// 获取更新后的任务并计算下次运行时间
	job, err := m.jobRepo.GetByID(jobID)
	if err != nil {
		utils.Warn("failed to get job after enabling", zap.String("job_id", jobID), zap.Error(err))
		return nil
	}

	// 计算下次运行时间
	if job.CronExpr != "" {
		if err := job.CalculateNextRunTime(); err != nil {
			utils.Warn("failed to calculate next run time after enabling",
				zap.String("job_id", jobID),
				zap.Error(err))
		} else {
			// 更新下次运行时间
			if err := m.jobRepo.Save(job); err != nil {
				utils.Warn("failed to update next run time after enabling",
					zap.String("job_id", jobID),
					zap.Error(err))
			}
		}
	}

	utils.Info("job enabled successfully", zap.String("job_id", jobID))
	return nil
}

// DisableJob 禁用任务
func (m *MasterJobManager) DisableJob(jobID string) error {
	if err := m.jobRepo.DisableJob(jobID); err != nil {
		return fmt.Errorf("failed to disable job: %w", err)
	}
	utils.Info("job disabled successfully", zap.String("job_id", jobID))
	return nil
}

// TriggerJob 手动触发任务执行
func (m *MasterJobManager) TriggerJob(jobID string) error {
	// 获取任务信息
	job, err := m.jobRepo.GetByID(jobID)
	if err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	// 如果任务已被禁用，返回错误
	if !job.Enabled {
		return fmt.Errorf("cannot trigger disabled job")
	}

	// 更新任务状态为运行中
	job.SetStatus(constants.JobStatusRunning)
	if err := m.jobRepo.Save(job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	utils.Info("job triggered manually", zap.String("job_id", jobID), zap.String("name", job.Name))
	return nil
}

// CancelJob 取消任务执行
func (m *MasterJobManager) CancelJob(jobID, reason string) error {
	// 获取任务信息
	job, err := m.jobRepo.GetByID(jobID)
	if err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	// 只有正在运行的任务才能取消
	if job.Status != constants.JobStatusRunning {
		return fmt.Errorf("can only cancel running jobs")
	}

	// 更新任务状态为已取消
	job.SetStatus(constants.JobStatusCancelled)
	if err := m.jobRepo.Save(job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	utils.Info("job cancelled",
		zap.String("job_id", jobID),
		zap.String("name", job.Name),
		zap.String("reason", reason))
	return nil
}

// WatchJobChanges 添加任务变更处理器
func (m *MasterJobManager) WatchJobChanges(handler JobChangeHandler) {
	m.watchHandlerLock.Lock()
	defer m.watchHandlerLock.Unlock()

	m.watchHandlers = append(m.watchHandlers, handler)
}

// GetDueJobs 获取到期需要执行的任务
func (m *MasterJobManager) GetDueJobs() ([]*models.Job, error) {
	return m.jobRepo.ListByNextRunTime(time.Now())
}

// Start 启动任务管理器
func (m *MasterJobManager) Start() error {
	if m.isRunning {
		return nil
	}

	m.isRunning = true
	m.stopChan = make(chan struct{})

	// 启动任务变更监控
	go m.watchJobs()

	utils.Info("job manager started")
	return nil
}

// Stop 停止任务管理器
func (m *MasterJobManager) Stop() error {
	if !m.isRunning {
		return nil
	}

	m.isRunning = false
	close(m.stopChan)

	utils.Info("job manager stopped")
	return nil
}

// watchJobs 监控任务变更
func (m *MasterJobManager) watchJobs() {
	utils.Info("starting job change watcher")

	// 使用etcd客户端设置前缀监听
	m.etcdClient.WatchWithPrefix(constants.JobPrefix, func(eventType, key, value string) {
		// 只处理有效事件
		if eventType != "PUT" && eventType != "DELETE" {
			return
		}

		// 提取任务ID
		jobID := key[len(constants.JobPrefix):]

		var job *models.Job
		var err error

		// 获取任务详情（如果不是删除事件）
		if eventType == "PUT" {
			job, err = models.FromJSON(value)
			if err != nil {
				utils.Error("failed to parse job data from event",
					zap.String("job_id", jobID),
					zap.Error(err))
				return
			}
		}

		// 通知所有处理器
		m.notifyHandlers(eventType, job)
	})
}

// notifyHandlers 通知所有处理器任务变更
func (m *MasterJobManager) notifyHandlers(eventType string, job *models.Job) {
	m.watchHandlerLock.RLock()
	defer m.watchHandlerLock.RUnlock()

	for _, handler := range m.watchHandlers {
		go handler(eventType, job)
	}
}
