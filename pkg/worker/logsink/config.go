package logsink

import (
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// LogSinkOption 定义日志采集器配置选项的函数类型
type LogSinkOption func(*LogSinkOptions)

// LogSinkOptions 包含日志采集器的配置选项
type LogSinkOptions struct {
	// 基础配置
	BufferSize    int           // 缓冲区大小
	BatchInterval time.Duration // 批处理间隔
	RetryCount    int           // 重试次数
	RetryInterval time.Duration // 重试间隔

	// 内存管理
	MaxBufferMemory int64  // 最大内存使用(字节)
	LocalCacheDir   string // 本地缓存目录

	// 日志控制
	FlushTimeout time.Duration  // 刷新超时时间
	SendMode     LogSendMode    // 发送模式
	StorageType  LogStorageType // 存储类型

	// Master节点配置
	MasterURL string // Master节点URL
	WorkerID  string // Worker节点ID

	// MongoDB相关
	MongoDBURI        string // MongoDB连接URI
	MongoDBDatabase   string // MongoDB数据库名
	MongoDBCollection string // MongoDB集合名
}

// NewLogSinkOptions 创建一个默认的日志采集器配置选项
func NewLogSinkOptions() *LogSinkOptions {
	return &LogSinkOptions{
		BufferSize:        DefaultBufferSize,
		BatchInterval:     DefaultBatchInterval,
		RetryCount:        DefaultRetryCount,
		RetryInterval:     DefaultRetryInterval,
		MaxBufferMemory:   DefaultMaxBufferMemory,
		LocalCacheDir:     DefaultLocalCacheDir,
		FlushTimeout:      DefaultFlushTimeout,
		SendMode:          SendModeBatch,
		StorageType:       StorageTypeMaster,
		MongoDBCollection: "job_logs",
	}
}

// WithBufferSize 设置缓冲区大小
func WithBufferSize(size int) LogSinkOption {
	return func(opts *LogSinkOptions) {
		if size > 0 {
			opts.BufferSize = size
			utils.Info("log sink buffer size set", zap.Int("size", size))
		}
	}
}

// WithBatchInterval 设置批处理间隔
func WithBatchInterval(interval time.Duration) LogSinkOption {
	return func(opts *LogSinkOptions) {
		if interval > 0 {
			opts.BatchInterval = interval
			utils.Info("log sink batch interval set", zap.Duration("interval", interval))
		}
	}
}

// WithRetrySettings 设置重试配置
func WithRetrySettings(count int, interval time.Duration) LogSinkOption {
	return func(opts *LogSinkOptions) {
		if count > 0 {
			opts.RetryCount = count
		}
		if interval > 0 {
			opts.RetryInterval = interval
		}
		utils.Info("log sink retry settings set",
			zap.Int("count", count),
			zap.Duration("interval", interval))
	}
}

// WithMaxBufferMemory 设置最大内存使用量
func WithMaxBufferMemory(maxMemory int64) LogSinkOption {
	return func(opts *LogSinkOptions) {
		if maxMemory > 0 {
			opts.MaxBufferMemory = maxMemory
			utils.Info("log sink max buffer memory set", zap.Int64("max_memory", maxMemory))
		}
	}
}

// WithLocalCacheDir 设置本地缓存目录
func WithLocalCacheDir(dir string) LogSinkOption {
	return func(opts *LogSinkOptions) {
		if dir != "" {
			opts.LocalCacheDir = dir
			utils.Info("log sink local cache directory set", zap.String("directory", dir))
		}
	}
}

// WithFlushTimeout 设置刷新超时时间
func WithFlushTimeout(timeout time.Duration) LogSinkOption {
	return func(opts *LogSinkOptions) {
		if timeout > 0 {
			opts.FlushTimeout = timeout
			utils.Info("log sink flush timeout set", zap.Duration("timeout", timeout))
		}
	}
}

// WithSendMode 设置发送模式
func WithSendMode(mode LogSendMode) LogSinkOption {
	return func(opts *LogSinkOptions) {
		opts.SendMode = mode
		utils.Info("log sink send mode set", zap.Int("mode", int(mode)))
	}
}

// WithStorageType 设置存储类型
func WithStorageType(storageType LogStorageType) LogSinkOption {
	return func(opts *LogSinkOptions) {
		opts.StorageType = storageType
		utils.Info("log sink storage type set", zap.Int("type", int(storageType)))
	}
}

// WithMasterConfig 设置Master节点配置
func WithMasterConfig(masterURL, workerID string) LogSinkOption {
	return func(opts *LogSinkOptions) {
		if masterURL != "" {
			opts.MasterURL = masterURL
		}
		if workerID != "" {
			opts.WorkerID = workerID
		}
		utils.Info("log sink master config set",
			zap.String("master_url", masterURL),
			zap.String("worker_id", workerID))
	}
}

// WithMongoDBConfig 设置MongoDB配置
func WithMongoDBConfig(uri, database, collection string) LogSinkOption {
	return func(opts *LogSinkOptions) {
		if uri != "" {
			opts.MongoDBURI = uri
		}
		if database != "" {
			opts.MongoDBDatabase = database
		}
		if collection != "" {
			opts.MongoDBCollection = collection
		}
		utils.Info("log sink mongodb config set",
			zap.String("database", database),
			zap.String("collection", collection))
	}
}

// ApplyOptions 应用选项到配置
func ApplyOptions(opts *LogSinkOptions, options ...LogSinkOption) *LogSinkOptions {
	for _, option := range options {
		option(opts)
	}
	return opts
}

// CreateLogSinkOptions 创建并配置日志采集器选项
func CreateLogSinkOptions(options ...LogSinkOption) *LogSinkOptions {
	opts := NewLogSinkOptions()
	return ApplyOptions(opts, options...)
}

// Validate 验证配置选项
func (opts *LogSinkOptions) Validate() error {
	// 确保必要的配置已设置
	if opts.BufferSize <= 0 {
		opts.BufferSize = DefaultBufferSize
	}

	if opts.BatchInterval <= 0 {
		opts.BatchInterval = DefaultBatchInterval
	}

	if opts.RetryCount < 0 {
		opts.RetryCount = DefaultRetryCount
	}

	if opts.RetryInterval <= 0 {
		opts.RetryInterval = DefaultRetryInterval
	}

	if opts.MaxBufferMemory <= 0 {
		opts.MaxBufferMemory = DefaultMaxBufferMemory
	}

	if opts.FlushTimeout <= 0 {
		opts.FlushTimeout = DefaultFlushTimeout
	}

	return nil
}
