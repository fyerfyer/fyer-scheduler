package scheduler

import (
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/joblock"
)

// IScheduler 调度器接口，负责管理任务的调度执行
type IScheduler interface {
	// Start 启动调度器
	Start() error

	// Stop 停止调度器
	Stop() error

	// AddJob 添加任务到调度器
	AddJob(job *models.Job) error

	// RemoveJob 从调度器移除任务
	RemoveJob(jobID string) error

	// TriggerJob 立即触发一个任务执行
	TriggerJob(jobID string) error

	// GetStatus 获取调度器状态
	GetStatus() SchedulerStatus

	// UpdateJob 更新已有任务
	UpdateJob(job *models.Job) error

	// GetJob 获取任务详情
	GetJob(jobID string) (*ScheduledJob, bool)

	// ListJobs 列出所有调度的任务
	ListJobs() []*ScheduledJob
}

// IJobQueue 任务队列接口，负责任务的排序和管理
type IJobQueue interface {
	// Push 添加任务到队列
	Push(job *ScheduledJob) error

	// Pop 获取并移除队首任务
	Pop() (*ScheduledJob, error)

	// Peek 查看队首任务但不移除
	Peek() (*ScheduledJob, error)

	// Remove 从队列中移除指定任务
	Remove(jobID string) error

	// Update 更新队列中的任务
	Update(job *ScheduledJob) error

	// Contains 检查队列是否包含指定任务
	Contains(jobID string) bool

	// Size 获取队列中的任务数量
	Size() int

	// Clear 清空队列
	Clear()

	// GetAll 获取所有任务（不移除）
	GetAll() []*ScheduledJob

	// GetDueJobs 获取已到期需要执行的任务
	GetDueJobs(now time.Time) []*ScheduledJob

	// RemoveDueJobs 移除并返回已到期的任务
	RemoveDueJobs(now time.Time) []*ScheduledJob
}

// ScheduledJob 表示一个被调度的任务
type ScheduledJob struct {
	// Job 原始任务信息
	Job *models.Job

	// NextRunTime 下次执行时间
	NextRunTime time.Time

	// IsRunning 任务是否正在运行
	IsRunning bool

	// LastExecutionID 最近一次执行ID
	LastExecutionID string

	// ExecutionCount 执行次数统计
	ExecutionCount int

	// Lock 任务锁，用于分布式环境下确保任务只被执行一次
	Lock *joblock.JobLock
}

// SchedulerStatus 调度器状态
type SchedulerStatus struct {
	// IsRunning 调度器是否正在运行
	IsRunning bool

	// JobCount 任务总数
	JobCount int

	// RunningJobCount 正在运行的任务数
	RunningJobCount int

	// NextRunTime 下一次调度检查时间
	NextCheckTime time.Time

	// StartTime 调度器启动时间
	StartTime time.Time

	// LastScheduleTime 上次调度检查时间
	LastScheduleTime time.Time

	// FailCount 调度失败次数
	FailCount int
}

// JobExecutionFunc 任务执行函数类型定义
type JobExecutionFunc func(*ScheduledJob) error

// JobQueueOptions 任务队列配置选项
type JobQueueOptions struct {
	// Capacity 队列容量，0表示无限制
	Capacity int

	// SortByPriority 是否按优先级排序
	SortByPriority bool
}

// SchedulerOptions 调度器配置选项
type SchedulerOptions struct {
	// CheckInterval 调度检查间隔
	CheckInterval time.Duration

	// MaxConcurrent 最大并发执行任务数，0表示无限制
	MaxConcurrent int

	// PreemptExpiredJobs 是否抢占过期任务
	PreemptExpiredJobs bool

	// EnablePersistence 是否启用持久化
	EnablePersistence bool

	// MaxRetries 最大重试次数
	MaxRetries int

	// RetryDelay 重试间隔
	RetryDelay time.Duration

	// LogExecutionDetails 是否记录执行详情
	LogExecutionDetails bool

	// Name 调度器名称
	Name string

	// FailStrategy 任务失败策略
	FailStrategy FailStrategy

	// JobQueueCapacity 任务队列容量
	JobQueueCapacity int

	// Timeout 任务执行超时时间
	Timeout time.Duration
}

// NewSchedulerOptions 创建默认的调度器选项
func NewSchedulerOptions() *SchedulerOptions {
	return &SchedulerOptions{
		CheckInterval:       5 * time.Second,
		MaxConcurrent:       10,
		PreemptExpiredJobs:  true,
		EnablePersistence:   false,
		MaxRetries:          3,
		RetryDelay:          30 * time.Second,
		LogExecutionDetails: true,
		Name:                "default-scheduler",
		FailStrategy:        FailStrategyRetry,
		JobQueueCapacity:    100,
		Timeout:             30 * time.Minute, // 默认30分钟超时
	}
}

// NewJobQueueOptions 创建默认的任务队列选项
func NewJobQueueOptions() *JobQueueOptions {
	return &JobQueueOptions{
		Capacity:       0,
		SortByPriority: false,
	}
}
