package joblock

import (
	"time"
)

// JobLockOption 提供函数选项模式配置锁
type JobLockOption func(*JobLock)

// JobLockOptions 任务锁的配置选项
type JobLockOptions struct {
	// TTL 锁超时时间(秒)
	TTL int64

	// MaxRetries 最大重试次数
	MaxRetries int

	// RetryDelay 重试间隔(毫秒)
	RetryDelay time.Duration
}

// WithTTL 设置锁超时时间
func WithTTL(ttl int64) JobLockOption {
	return func(lock *JobLock) {
		if ttl > 0 {
			lock.ttl = ttl
		}
	}
}

// WithMaxRetries 设置最大重试次数
func WithMaxRetries(maxRetries int) JobLockOption {
	return func(lock *JobLock) {
		if maxRetries > 0 {
			lock.maxRetries = maxRetries
		}
	}
}

// WithRetryDelay 设置重试间隔
func WithRetryDelay(retryDelay time.Duration) JobLockOption {
	return func(lock *JobLock) {
		if retryDelay > 0 {
			lock.retryDelay = retryDelay
		}
	}
}

// FromLockOptions 从选项结构体创建选项
func FromLockOptions(options *JobLockOptions) []JobLockOption {
	var opts []JobLockOption

	if options == nil {
		return opts
	}

	if options.TTL > 0 {
		opts = append(opts, WithTTL(options.TTL))
	}

	if options.MaxRetries > 0 {
		opts = append(opts, WithMaxRetries(options.MaxRetries))
	}

	if options.RetryDelay > 0 {
		opts = append(opts, WithRetryDelay(options.RetryDelay))
	}

	return opts
}

// DefaultLockOptions 返回默认锁配置选项
func DefaultLockOptions() *JobLockOptions {
	return &JobLockOptions{
		TTL:        5,                      // 默认5秒
		MaxRetries: 3,                      // 默认重试3次
		RetryDelay: 200 * time.Millisecond, // 默认200毫秒间隔
	}
}

// Validate 验证锁配置选项
func (o *JobLockOptions) Validate() error {
	// 设置默认值
	if o.TTL <= 0 {
		o.TTL = 5
	}

	if o.MaxRetries <= 0 {
		o.MaxRetries = 3
	}

	if o.RetryDelay <= 0 {
		o.RetryDelay = 200 * time.Millisecond
	}

	return nil
}
