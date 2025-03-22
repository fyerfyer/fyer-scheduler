package scheduler

import (
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// SchedulerOption 调度器配置选项的函数类型
type SchedulerOption func(*SchedulerOptions)

// WithCheckInterval 设置调度检查间隔
func WithCheckInterval(interval time.Duration) SchedulerOption {
	return func(opts *SchedulerOptions) {
		if interval > 0 {
			opts.CheckInterval = interval
			utils.Info("scheduler check interval set", zap.Duration("interval", interval))
		}
	}
}

// WithMaxConcurrent 设置最大并发执行任务数
func WithMaxConcurrent(max int) SchedulerOption {
	return func(opts *SchedulerOptions) {
		if max >= 0 {
			opts.MaxConcurrent = max
			utils.Info("scheduler max concurrent jobs set", zap.Int("max", max))
		}
	}
}

// WithPreemptExpiredJobs 设置是否抢占过期任务
func WithPreemptExpiredJobs(preempt bool) SchedulerOption {
	return func(opts *SchedulerOptions) {
		opts.PreemptExpiredJobs = preempt
		utils.Info("scheduler preempt expired jobs set", zap.Bool("preempt", preempt))
	}
}

// WithPersistence 设置是否启用持久化
func WithPersistence(enable bool) SchedulerOption {
	return func(opts *SchedulerOptions) {
		opts.EnablePersistence = enable
		utils.Info("scheduler persistence set", zap.Bool("enable", enable))
	}
}

// WithJobQueueCapacity 设置任务队列容量
func WithJobQueueCapacity(capacity int) SchedulerOption {
	return func(opts *SchedulerOptions) {
		if capacity > 0 {
			opts.JobQueueCapacity = capacity
			utils.Info("scheduler job queue capacity set", zap.Int("capacity", capacity))
		}
	}
}

// WithRetryStrategy 设置任务重试策略
func WithRetryStrategy(maxRetries int, retryDelay time.Duration) SchedulerOption {
	return func(opts *SchedulerOptions) {
		if maxRetries >= 0 {
			opts.MaxRetries = maxRetries
			opts.RetryDelay = retryDelay
			utils.Info("scheduler retry strategy set",
				zap.Int("max_retries", maxRetries),
				zap.Duration("retry_delay", retryDelay))
		}
	}
}

// WithLogExecutionDetails 设置是否记录执行详情
func WithLogExecutionDetails(enable bool) SchedulerOption {
	return func(opts *SchedulerOptions) {
		opts.LogExecutionDetails = enable
		utils.Info("scheduler log execution details set", zap.Bool("enable", enable))
	}
}

// WithSchedulerName 设置调度器名称
func WithSchedulerName(name string) SchedulerOption {
	return func(opts *SchedulerOptions) {
		if name != "" {
			opts.Name = name
			utils.Info("scheduler name set", zap.String("name", name))
		}
	}
}

// WithFailStrategy 设置任务失败策略
func WithFailStrategy(strategy FailStrategy) SchedulerOption {
	return func(opts *SchedulerOptions) {
		opts.FailStrategy = strategy
		utils.Info("scheduler fail strategy set", zap.Int("strategy", int(strategy)))
	}
}

// ApplyOptions 应用一系列选项到配置
func ApplyOptions(opts *SchedulerOptions, options ...SchedulerOption) *SchedulerOptions {
	for _, option := range options {
		option(opts)
	}
	return opts
}

// CreateSchedulerOptions 使用选项创建一个新的调度器配置
func CreateSchedulerOptions(options ...SchedulerOption) *SchedulerOptions {
	opts := NewSchedulerOptions()
	return ApplyOptions(opts, options...)
}

// FailStrategy 定义任务失败后的处理策略
type FailStrategy int

const (
	// FailStrategyStop 停止重试，将任务标记为失败
	FailStrategyStop FailStrategy = iota

	// FailStrategyRetry 根据重试设置重试任务
	FailStrategyRetry

	// FailStrategyIgnore 忽略失败，仍然计算下次执行时间
	FailStrategyIgnore
)
