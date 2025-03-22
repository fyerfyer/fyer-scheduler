package jobmgr

import (
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// JobEventType 定义任务事件类型
type JobEventType int

const (
	// JobEventSave 表示任务保存事件（新增或更新）
	JobEventSave JobEventType = iota
	// JobEventDelete 表示任务删除事件
	JobEventDelete
	// JobEventKill 表示任务终止事件
	JobEventKill
)

// JobEvent 表示一个任务事件
type JobEvent struct {
	// 事件类型
	Type JobEventType
	// 任务信息
	Job *models.Job
}

// IJobEventHandler 定义任务事件处理器接口
type IJobEventHandler interface {
	// HandleJobEvent 处理任务事件
	HandleJobEvent(event *JobEvent)
}

// IWorkerJobManager 定义Worker节点任务管理器接口
type IWorkerJobManager interface {
	// Start 启动任务管理器
	Start() error

	// Stop 停止任务管理器
	Stop() error

	// WatchJobs 监听任务变化
	WatchJobs()

	// GetJob 根据任务ID获取任务
	GetJob(jobID string) (*models.Job, error)

	// ListJobs 获取所有待处理的任务
	ListJobs() ([]*models.Job, error)

	// ListRunningJobs 获取正在运行的任务
	ListRunningJobs() ([]*models.Job, error)

	// RegisterHandler 注册任务事件处理器
	RegisterHandler(handler IJobEventHandler)

	// ReportJobStatus 报告任务状态
	ReportJobStatus(job *models.Job, status string) error

	// KillJob 终止任务
	KillJob(jobID string) error

	// IsJobAssignedToWorker 检查任务是否分配给当前Worker
	IsJobAssignedToWorker(job *models.Job) bool

	// GetWorkerID 获取当前Worker ID
	GetWorkerID() string
}

// JobLock 任务锁接口
type JobLock interface {
	// TryLock 尝试获取任务锁
	TryLock() (bool, error)

	// Unlock 释放任务锁
	Unlock() error

	// GetLeaseID 获取租约ID
	GetLeaseID() clientv3.LeaseID
}

// JobLockOptions 任务锁选项
type JobLockOptions struct {
	// JobID 任务ID
	JobID string

	// WorkerID 工作节点ID
	WorkerID string

	// TTL 锁超时时间(秒)
	TTL int64

	// MaxRetries 最大重试次数
	MaxRetries int

	// RetryDelay 重试间隔(毫秒)
	RetryDelay time.Duration
}

// JobChangedCallback 定义任务变更回调函数类型
type JobChangedCallback func(jobEvent *JobEvent)
