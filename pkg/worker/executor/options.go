package executor

import (
	"runtime"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// ExecutorOption 定义执行器选项的函数类型
type ExecutorOption func(*ExecutionOptions)

// WithMaxConcurrentExecutions 设置最大并发执行任务数
func WithMaxConcurrentExecutions(max int) ExecutorOption {
	return func(opts *ExecutionOptions) {
		if max > 0 {
			opts.MaxConcurrentExecutions = max
			utils.Info("executor max concurrent executions set", zap.Int("max", max))
		}
	}
}

// WithDefaultTimeout 设置默认任务超时时间
func WithDefaultTimeout(timeout time.Duration) ExecutorOption {
	return func(opts *ExecutionOptions) {
		if timeout > 0 {
			opts.DefaultTimeout = timeout
			utils.Info("executor default timeout set", zap.Duration("timeout", timeout))
		}
	}
}

// WithBufferSize 设置输出缓冲区大小
func WithBufferSize(size int) ExecutorOption {
	return func(opts *ExecutionOptions) {
		if size > 0 {
			opts.BufferSize = size
			utils.Info("executor buffer size set", zap.Int("size", size))
		}
	}
}

// WithShellExecutable 设置Shell可执行文件路径
func WithShellExecutable(shell string) ExecutorOption {
	return func(opts *ExecutionOptions) {
		opts.ShellExecutable = shell
		utils.Info("executor shell executable set", zap.String("shell", shell))
	}
}

// WithKillGracePeriod 设置终止任务的优雅期
func WithKillGracePeriod(period time.Duration) ExecutorOption {
	return func(opts *ExecutionOptions) {
		if period > 0 {
			opts.KillGracePeriod = period
			utils.Info("executor kill grace period set", zap.Duration("period", period))
		}
	}
}

// WithMaxOutputSize 设置最大输出大小
func WithMaxOutputSize(size int) ExecutorOption {
	return func(opts *ExecutionOptions) {
		if size > 0 {
			opts.MaxOutputSize = size
			utils.Info("executor max output size set", zap.Int("size", size))
		}
	}
}

// WithSystemDefault 根据系统自动设置最佳选项
func WithSystemDefault() ExecutorOption {
	return func(opts *ExecutionOptions) {
		// 根据CPU核心数设置并发数
		numCPU := runtime.NumCPU()
		opts.MaxConcurrentExecutions = numCPU * 2

		// 根据系统类型设置shell
		if runtime.GOOS == "windows" {
			opts.ShellExecutable = "cmd.exe"
		} else {
			opts.ShellExecutable = "/bin/sh"
		}

		utils.Info("executor system defaults applied",
			zap.Int("max_concurrent", opts.MaxConcurrentExecutions),
			zap.String("shell", opts.ShellExecutable))
	}
}

// WithTempDirectory 设置临时文件目录
func WithTempDirectory(dir string) ExecutorOption {
	return func(opts *ExecutionOptions) {
		opts.TempDirectory = dir
		utils.Info("executor temp directory set", zap.String("directory", dir))
	}
}

// ApplyOptions 应用选项到执行器选项实例
func ApplyOptions(opts *ExecutionOptions, options ...ExecutorOption) *ExecutionOptions {
	for _, option := range options {
		option(opts)
	}
	return opts
}

// CreateExecutionOptions 使用选项创建一个新的执行器选项
func CreateExecutionOptions(options ...ExecutorOption) *ExecutionOptions {
	opts := NewExecutionOptions()
	return ApplyOptions(opts, options...)
}
