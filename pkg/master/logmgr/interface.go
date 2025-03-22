package logmgr

import (
	"io"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
)

// LogManager 定义日志管理器的接口
type LogManager interface {
	// 基础日志查询方法
	GetLogByExecutionID(executionID string) (*models.JobLog, error)
	GetLogsByJobID(jobID string, page, pageSize int64) ([]*models.JobLog, int64, error)
	GetLatestLogByJobID(jobID string) (*models.JobLog, error)
	GetLogsByTimeRange(startTime, endTime time.Time, page, pageSize int64) ([]*models.JobLog, int64, error)
	GetLogsByWorkerID(workerID string, page, pageSize int64) ([]*models.JobLog, int64, error)
	GetLogsByStatus(status string, page, pageSize int64) ([]*models.JobLog, int64, error)

	// 日志聚合和统计方法
	GetJobStatusCounts(jobID string) (map[string]int64, error)
	GetJobSuccessRate(jobID string, period time.Duration) (float64, error)
	GetSystemStatusSummary() (map[string]interface{}, error)
	GetDailyJobExecutionStats(days int) ([]map[string]interface{}, error)

	// 日志流操作
	StreamLogOutput(executionID string, writer io.Writer) error
	SubscribeToLogUpdates(executionID string) (<-chan string, error)
	UnsubscribeFromLogUpdates(executionID string) error

	// 日志管理操作
	CleanupOldLogs(beforeTime time.Time) (int64, error)
	UpdateLogStatus(executionID, status string, output string) error
	AppendOutput(executionID string, output string) error

	// 服务管理
	Start() error
	Stop() error
}

// LogSubscription 表示对日志更新的订阅
type LogSubscription struct {
	ExecutionID string
	Channel     chan string
	LastUpdated time.Time
}

// LogStreamOptions 定义日志流选项
type LogStreamOptions struct {
	// 是否包含历史日志
	IncludeHistory bool

	// 是否持续等待新日志
	Follow bool

	// 超时时间，为0表示不超时
	Timeout time.Duration
}

// LogQuery 定义日志查询参数
type LogQuery struct {
	JobID      string
	WorkerID   string
	Status     string
	StartTime  time.Time
	EndTime    time.Time
	SearchText string
	Page       int64
	PageSize   int64
	SortBy     string
	SortDir    string
}
