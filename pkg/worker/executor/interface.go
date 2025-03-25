package executor

import (
	"context"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
)

// IExecutor 定义了任务执行器的接口
type IExecutor interface {
	// Execute 执行一个命令并返回结果
	Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error)

	// Kill 终止正在执行的任务
	Kill(executionID string) error

	// GetRunningExecutions 获取当前正在运行的任务列表
	GetRunningExecutions() map[string]*ExecutionStatus

	// GetExecutionStatus 获取指定任务的执行状态
	GetExecutionStatus(executionID string) (*ExecutionStatus, bool)
}

// ExecutionContext 包含任务执行的上下文信息
type ExecutionContext struct {
	// ExecutionID 执行ID
	ExecutionID string

	// Job 任务信息
	Job *models.Job

	// Command 要执行的命令
	Command string

	// Args 命令参数
	Args []string

	// WorkDir 工作目录
	WorkDir string

	// Environment 环境变量
	Environment map[string]string

	// Timeout 执行超时时间
	Timeout time.Duration

	// Reporter 执行状态报告器
	Reporter IExecutionReporter

	// MaxOutputSize 最大输出大小（字节）
	MaxOutputSize int
}

// ExecutionResult 表示命令执行的结果
type ExecutionResult struct {
	// ExecutionID 执行ID
	ExecutionID string

	// ExitCode 退出码
	ExitCode int

	// Output 命令输出
	Output string

	// Error 执行错误
	Error string

	// StartTime 开始时间
	StartTime time.Time

	// EndTime 结束时间
	EndTime time.Time

	// State 执行状态
	State ExecutionState
}

// ExecutionState 定义任务执行状态
type ExecutionState string

const (
	// ExecutionStatePending 任务等待执行
	ExecutionStatePending ExecutionState = "pending"

	// ExecutionStateRunning 任务正在运行
	ExecutionStateRunning ExecutionState = "running"

	// ExecutionStateSuccess 任务执行成功
	ExecutionStateSuccess ExecutionState = "success"

	// ExecutionStateFailed 任务执行失败
	ExecutionStateFailed ExecutionState = "failed"

	// ExecutionStateCancelled 任务被取消
	ExecutionStateCancelled ExecutionState = "cancelled"

	// ExecutionStateTimeout 任务执行超时
	ExecutionStateTimeout ExecutionState = "timeout"
)

// ExecutionStatus 表示任务的执行状态
type ExecutionStatus struct {
	// ExecutionID 执行ID
	ExecutionID string

	// State 执行状态
	State ExecutionState

	// StartTime 开始时间
	StartTime time.Time

	// Pid 进程ID
	Pid int

	// CPU CPU使用率
	CPU float64

	// Memory 内存使用（MB）
	Memory float64

	// CurrentOutput 当前已收集的输出
	CurrentOutput string

	// OutputSize 输出大小
	OutputSize int
}

// IExecutionReporter 定义状态报告接口
type IExecutionReporter interface {
	// ReportStart 报告任务开始执行
	ReportStart(executionID string, pid int) error

	// ReportOutput 报告命令输出
	ReportOutput(executionID string, output string) error

	// ReportCompletion 报告任务完成
	ReportCompletion(executionID string, result *ExecutionResult) error

	// ReportError 报告执行错误
	ReportError(executionID string, err error) error

	// ReportProgress 报告执行进度
	ReportProgress(executionID string, status *ExecutionStatus) error
}

// ExecutionOptions 执行器选项
type ExecutionOptions struct {
	// MaxConcurrentExecutions 最大并发执行数
	MaxConcurrentExecutions int

	// DefaultTimeout 默认超时时间
	DefaultTimeout time.Duration

	// BufferSize 输出缓冲区大小
	BufferSize int

	// ShellExecutable shell可执行文件路径
	ShellExecutable string

	// KillGracePeriod 终止任务的优雅期
	KillGracePeriod time.Duration

	// MaxOutputSize 最大输出大小（字节）
	MaxOutputSize int

	// TempDirectory 临时目录
	TempDirectory string
}

// NewExecutionOptions 创建默认的执行器选项
func NewExecutionOptions() *ExecutionOptions {
	return &ExecutionOptions{
		MaxConcurrentExecutions: 10,
		DefaultTimeout:          30 * time.Minute,
		BufferSize:              1024 * 1024, // 1MB
		ShellExecutable:         "",          // 将根据系统自动检测
		KillGracePeriod:         5 * time.Second,
		MaxOutputSize:           10 * 1024 * 1024, // 10MB
		TempDirectory:           "",               // 默认使用系统临时目录
	}
}

// NewExecutionContext 创建执行上下文
func NewExecutionContext(executionID string, job *models.Job, command string, args []string, workDir string, env map[string]string, timeout time.Duration) *ExecutionContext {
	return &ExecutionContext{
		ExecutionID:   executionID,
		Job:           job,
		Command:       command,
		Args:          args,
		WorkDir:       workDir,
		Environment:   env,
		Timeout:       timeout,
		MaxOutputSize: 1024 * 1024, // 默认1MB
	}
}

// ExecutionResultFromJobLog 从JobLog创建ExecutionResult
func ExecutionResultFromJobLog(log *models.JobLog) *ExecutionResult {
	var state ExecutionState
	switch log.Status {
	case "succeeded":
		state = ExecutionStateSuccess
	case "failed":
		state = ExecutionStateFailed
	case "cancelled":
		state = ExecutionStateCancelled
	default:
		state = ExecutionStatePending
	}

	return &ExecutionResult{
		ExecutionID: log.ExecutionID,
		ExitCode:    log.ExitCode,
		Output:      log.Output,
		Error:       log.Error,
		StartTime:   log.StartTime,
		EndTime:     log.EndTime,
		State:       state,
	}
}

// CreateContext 创建可取消的执行上下文
func (c *ExecutionContext) CreateContext() context.Context {
	// 创建带超时的上下文
	ctx, _ := context.WithTimeout(context.Background(), c.Timeout)
	return ctx
}
