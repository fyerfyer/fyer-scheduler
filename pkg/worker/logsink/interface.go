package logsink

import (
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
)

// LogLevel 定义日志级别
type LogLevel int

const (
	// 日志级别常量
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// LogSendMode 定义日志发送模式
type LogSendMode int

const (
	// 日志发送模式
	SendModeImmediate  LogSendMode = iota // 立即发送
	SendModeBatch                         // 批量发送
	SendModeBufferFull                    // 缓冲区满时发送
)

// LogStorageType 定义日志存储类型
type LogStorageType int

const (
	// 日志存储类型
	StorageTypeMaster  LogStorageType = iota // 发送到Master节点
	StorageTypeMongoDB                       // 直接存储到MongoDB
	StorageTypeBoth                          // 同时存储到Master和MongoDB
)

// 默认配置常量
const (
	DefaultBufferSize      = 1000              // 默认缓冲区大小
	DefaultBatchInterval   = 5 * time.Second   // 默认批处理间隔
	DefaultRetryCount      = 3                 // 默认重试次数
	DefaultRetryInterval   = 2 * time.Second   // 默认重试间隔
	DefaultMaxBufferMemory = 100 * 1024 * 1024 // 默认最大内存使用 (100MB)
	DefaultLocalCacheDir   = "./logs/cache"    // 默认本地缓存目录
	DefaultFlushTimeout    = 10 * time.Second  // 默认刷新超时时间
)

// LogEntry 表示单条日志条目
type LogEntry struct {
	ExecutionID string                 `json:"execution_id"` // 执行ID
	JobID       string                 `json:"job_id"`       // 任务ID
	Content     string                 `json:"content"`      // 日志内容
	Timestamp   time.Time              `json:"timestamp"`    // 时间戳
	Level       LogLevel               `json:"level"`        // 日志级别
	WorkerID    string                 `json:"worker_id"`    // Worker ID
	Metadata    map[string]interface{} `json:"metadata"`     // 元数据
}

// ILogSink 定义日志采集器接口
type ILogSink interface {
	// Start 启动日志采集服务
	Start() error

	// Stop 停止日志采集服务
	Stop() error

	// AddLog 添加单条日志
	AddLog(executionID string, content string) error

	// AddLogWithLevel 添加带级别的日志
	AddLogWithLevel(executionID string, content string, level LogLevel) error

	// AddLogEntry 添加完整的日志条目
	AddLogEntry(entry *LogEntry) error

	// Flush 立即将当前缓冲区的日志发送出去
	Flush() error

	// FlushExecutionLogs 立即发送指定执行ID的所有日志
	FlushExecutionLogs(executionID string) error

	// SetSendMode 设置发送模式
	SetSendMode(mode LogSendMode)

	// SetStorageType 设置存储类型
	SetStorageType(storageType LogStorageType)

	// GetStats 获取日志采集器统计信息
	GetStats() map[string]interface{}
}

// LogSinkStatus 表示日志采集器的状态
type LogSinkStatus struct {
	IsRunning          bool           `json:"is_running"`           // 是否运行中
	BufferedLogCount   int            `json:"buffered_log_count"`   // 缓冲区日志数量
	SentLogCount       int64          `json:"sent_log_count"`       // 已发送日志数量
	FailedLogCount     int64          `json:"failed_log_count"`     // 发送失败日志数量
	ExecutionLogCounts map[string]int `json:"execution_log_counts"` // 各执行ID的日志数量
	LastSendTime       time.Time      `json:"last_send_time"`       // 最后一次发送时间
	LastError          string         `json:"last_error"`           // 最后一次错误
	CurrentMemoryUsage int64          `json:"current_memory_usage"` // 当前内存使用量
	SendMode           LogSendMode    `json:"send_mode"`            // 当前发送模式
	StorageType        LogStorageType `json:"storage_type"`         // 当前存储类型
}

// LogSinkEventType 定义日志采集器事件类型
type LogSinkEventType int

const (
	// 事件类型
	EventTypeSendSuccess LogSinkEventType = iota // 发送成功
	EventTypeSendFailure                         // 发送失败
	EventTypeBufferFull                          // 缓冲区满
	EventTypeFlush                               // 手动刷新
	EventTypeRetry                               // 重试发送
)

// LogSinkEventHandler 定义事件处理函数
type LogSinkEventHandler func(eventType LogSinkEventType, data interface{})

// ILogSinkFactory 定义日志采集器工厂接口
type ILogSinkFactory interface {
	// CreateLogSink 创建日志采集器
	CreateLogSink(options ...LogSinkOption) (ILogSink, error)
}

// ResultCallback 定义发送结果回调函数类型
type ResultCallback func(success bool, executionID string, logCount int, err error)

// CreateJobLogFromEntry 从日志条目创建作业日志
func CreateJobLogFromEntry(entry *LogEntry) *models.JobLog {
	log := &models.JobLog{
		ExecutionID: entry.ExecutionID,
		JobID:       entry.JobID,
		Output:      entry.Content,
	}
	return log
}
