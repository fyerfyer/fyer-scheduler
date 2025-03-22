package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Executor 实现IExecutor接口，负责管理命令执行
type Executor struct {
	options        *ExecutionOptions           // 执行器选项
	runningJobs    map[string]*ExecutorContext // 正在运行的任务 executionID -> context
	execStatusMap  map[string]*ExecutionStatus // 执行状态映射 executionID -> status
	processMutex   sync.RWMutex                // 保护进程映射的互斥锁
	statusMutex    sync.RWMutex                // 保护状态映射的互斥锁
	reporter       IExecutionReporter          // 执行状态报告器
	isRunning      bool                        // 执行器是否正在运行
	runningCount   int                         // 当前运行任务数
	runningCond    *sync.Cond                  // 用于等待可用执行槽的条件变量
	shutdownChan   chan struct{}               // 关闭信号
	executionCount int64                       // 总执行次数
	retentionTime  time.Duration               // 保留已完成执行信息的时间
	cleanupTicker  *time.Ticker                // 清理已完成执行的定时器
}

// ExecutorContext 包含与执行上下文相关的所有内容
type ExecutorContext struct {
	ExecCtx      *ExecutionContext     // 执行上下文
	Process      *Process              // 进程
	OutputMgr    *OutputManager        // 输出管理器
	CancelFunc   context.CancelFunc    // 用于取消执行
	StartTime    time.Time             // 开始时间
	EndTime      time.Time             // 结束时间
	State        ExecutionState        // 执行状态
	StdoutWriter *OutputWriter         // 标准输出写入器
	StderrWriter *OutputWriter         // 标准错误写入器
	ResultChan   chan *ExecutionResult // 结果通道
	Mutex        sync.RWMutex          // 保护上下文的互斥锁
}

// NewExecutor 创建一个新的执行器
func NewExecutor(reporter IExecutionReporter, options ...ExecutorOption) *Executor {
	// 创建基础配置
	opts := NewExecutionOptions()

	// 应用用户选项
	for _, option := range options {
		option(opts)
	}

	executor := &Executor{
		options:       opts,
		runningJobs:   make(map[string]*ExecutorContext),
		execStatusMap: make(map[string]*ExecutionStatus),
		reporter:      reporter,
		isRunning:     true,
		runningCount:  0,
		retentionTime: 24 * time.Hour, // 默认保留24小时
		shutdownChan:  make(chan struct{}),
	}

	// 初始化条件变量
	executor.runningCond = sync.NewCond(&executor.processMutex)

	// 启动清理过期执行状态的协程
	executor.cleanupTicker = time.NewTicker(30 * time.Minute)
	go executor.cleanupLoop()

	utils.Info("executor initialized",
		zap.Int("max_concurrent", opts.MaxConcurrentExecutions),
		zap.Int("buffer_size", opts.BufferSize),
		zap.Duration("default_timeout", opts.DefaultTimeout))

	return executor
}

// Execute 执行一个命令并返回结果
func (e *Executor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	// 检查执行器是否正在运行
	if !e.isRunning {
		return nil, fmt.Errorf("executor is not running")
	}

	// 检查执行ID是否已存在
	e.processMutex.RLock()
	_, exists := e.runningJobs[execCtx.ExecutionID]
	e.processMutex.RUnlock()

	if exists {
		return nil, fmt.Errorf("execution %s is already running", execCtx.ExecutionID)
	}

	// 等待可用的执行槽
	e.waitForAvailableSlot(ctx)

	// 创建可取消的上下文
	execContext, cancelFunc := context.WithCancel(ctx)
	if execCtx.Timeout > 0 {
		execContext, cancelFunc = context.WithTimeout(ctx, execCtx.Timeout)
	}

	// 创建输出管理器
	outputMgr := NewOutputManager(execCtx.ExecutionID, execCtx.MaxOutputSize, execCtx)

	// 创建输出写入器
	stdoutWriter := NewOutputWriter(outputMgr)
	stderrWriter := NewOutputWriter(outputMgr)

	// 创建执行上下文
	executorCtx := &ExecutorContext{
		ExecCtx:      execCtx,
		CancelFunc:   cancelFunc,
		StartTime:    time.Now(),
		State:        ExecutionStatePending,
		OutputMgr:    outputMgr,
		StdoutWriter: stdoutWriter,
		StderrWriter: stderrWriter,
		ResultChan:   make(chan *ExecutionResult, 1),
	}

	// 添加到运行中的任务映射
	e.processMutex.Lock()
	e.runningJobs[execCtx.ExecutionID] = executorCtx
	e.runningCount++
	e.executionCount++
	e.processMutex.Unlock()

	// 创建初始状态
	status := &ExecutionStatus{
		ExecutionID: execCtx.ExecutionID,
		State:       ExecutionStatePending,
		StartTime:   executorCtx.StartTime,
	}

	// 更新状态映射
	e.statusMutex.Lock()
	e.execStatusMap[execCtx.ExecutionID] = status
	e.statusMutex.Unlock()

	// 报告开始执行
	if execCtx.Reporter != nil {
		execCtx.Reporter.ReportStart(execCtx.ExecutionID, 0)
	}

	// 异步执行命令
	go e.executeCommand(execContext, executorCtx)

	// 等待结果或取消
	select {
	case result := <-executorCtx.ResultChan:
		return result, nil
	case <-ctx.Done():
		// 上下文被取消，尝试终止任务
		e.Kill(execCtx.ExecutionID)
		return nil, ctx.Err()
	}
}

// executeCommand 执行命令（在协程中调用）
func (e *Executor) executeCommand(ctx context.Context, executorCtx *ExecutorContext) {
	execCtx := executorCtx.ExecCtx
	result := &ExecutionResult{
		ExecutionID: execCtx.ExecutionID,
		StartTime:   executorCtx.StartTime,
		State:       ExecutionStateRunning,
	}

	// 更新状态为运行中
	executorCtx.State = ExecutionStateRunning
	e.updateExecutionStatus(execCtx.ExecutionID, ExecutionStateRunning, 0)

	// 设置命令执行目录
	workDir := execCtx.WorkDir
	if workDir == "" && e.options.TempDirectory != "" {
		// 创建临时目录
		tempDir := filepath.Join(e.options.TempDirectory, execCtx.ExecutionID)
		if err := os.MkdirAll(tempDir, 0755); err == nil {
			workDir = tempDir
			defer os.RemoveAll(tempDir) // 执行结束后清理
		}
	}

	// 准备进程选项
	processOptions := ProcessOptions{
		ExecutionID: execCtx.ExecutionID,
		Command:     execCtx.Command,
		Args:        execCtx.Args,
		WorkDir:     workDir,
		Env:         execCtx.Environment,
		GracePeriod: e.options.KillGracePeriod,
		OutputHandler: func(output string) {
			_ = executorCtx.OutputMgr.AddOutput
		},
		MaxOutputSize: execCtx.MaxOutputSize,
	}

	// 创建进程
	process, err := NewProcess(ctx, processOptions)
	if err != nil {
		e.handleExecutionError(executorCtx, err, "failed to create process")
		return
	}

	// 保存进程引用
	executorCtx.Process = process

	// 启动进程
	utils.Info("starting process execution",
		zap.String("execution_id", execCtx.ExecutionID),
		zap.String("command", execCtx.Command),
		zap.Strings("args", execCtx.Args))

	if err := process.Start(); err != nil {
		e.handleExecutionError(executorCtx, err, "failed to start process")
		return
	}

	// 更新状态中的PID
	e.updateExecutionStatus(execCtx.ExecutionID, ExecutionStateRunning, process.GetPid())

	// 如果有报告器，报告进程启动
	if execCtx.Reporter != nil {
		execCtx.Reporter.ReportStart(execCtx.ExecutionID, process.GetPid())
	}

	// 启动资源监控
	monitorCtx, monitorCancel := context.WithCancel(ctx)
	defer monitorCancel()
	go e.monitorResources(monitorCtx, executorCtx)

	// 等待进程完成
	exitCode, exitErr, isTimeout := process.WaitWithTimeout(execCtx.Timeout)

	// 处理执行结果
	executorCtx.EndTime = time.Now()

	// 获取输出
	output := process.GetOutput()

	if isTimeout {
		// 处理超时情况
		e.handleExecutionTimeout(executorCtx)
	} else if exitErr != nil {
		// 处理执行错误
		result.ExitCode = exitCode
		result.Error = exitErr.Error()
		result.Output = output
		result.EndTime = executorCtx.EndTime
		result.State = ExecutionStateFailed

		executorCtx.State = ExecutionStateFailed
		e.updateExecutionStatus(execCtx.ExecutionID, ExecutionStateFailed, process.GetPid())

		if execCtx.Reporter != nil {
			execCtx.Reporter.ReportCompletion(execCtx.ExecutionID, result)
		}
	} else {
		// 处理执行成功
		result.ExitCode = exitCode
		result.Output = output
		result.EndTime = executorCtx.EndTime
		result.State = ExecutionStateSuccess

		executorCtx.State = ExecutionStateSuccess
		e.updateExecutionStatus(execCtx.ExecutionID, ExecutionStateSuccess, process.GetPid())

		if execCtx.Reporter != nil {
			execCtx.Reporter.ReportCompletion(execCtx.ExecutionID, result)
		}
	}

	// 发送结果
	select {
	case executorCtx.ResultChan <- result:
	default:
		utils.Warn("could not send result, channel might be closed",
			zap.String("execution_id", execCtx.ExecutionID))
	}

	// 清理资源
	e.processMutex.Lock()
	delete(e.runningJobs, execCtx.ExecutionID)
	e.runningCount--
	e.runningCond.Signal() // 通知等待的执行
	e.processMutex.Unlock()

	utils.Info("execution completed",
		zap.String("execution_id", execCtx.ExecutionID),
		zap.Int("exit_code", exitCode),
		zap.String("state", string(result.State)),
		zap.Duration("duration", executorCtx.EndTime.Sub(executorCtx.StartTime)))
}

// handleExecutionError 处理执行错误
func (e *Executor) handleExecutionError(executorCtx *ExecutorContext, err error, msg string) {
	execCtx := executorCtx.ExecCtx
	executorCtx.EndTime = time.Now()
	executorCtx.State = ExecutionStateFailed

	errMsg := fmt.Sprintf("%s: %v", msg, err)

	result := &ExecutionResult{
		ExecutionID: execCtx.ExecutionID,
		ExitCode:    1,
		Error:       errMsg,
		StartTime:   executorCtx.StartTime,
		EndTime:     executorCtx.EndTime,
		State:       ExecutionStateFailed,
	}

	e.updateExecutionStatus(execCtx.ExecutionID, ExecutionStateFailed, 0)

	if execCtx.Reporter != nil {
		execCtx.Reporter.ReportError(execCtx.ExecutionID, err)
		execCtx.Reporter.ReportCompletion(execCtx.ExecutionID, result)
	}

	executorCtx.ResultChan <- result

	// 清理资源
	e.processMutex.Lock()
	delete(e.runningJobs, execCtx.ExecutionID)
	e.runningCount--
	e.runningCond.Signal() // 通知等待的执行
	e.processMutex.Unlock()

	utils.Error("execution failed",
		zap.String("execution_id", execCtx.ExecutionID),
		zap.Error(err),
		zap.String("message", msg))
}

// handleExecutionTimeout 处理执行超时
func (e *Executor) handleExecutionTimeout(executorCtx *ExecutorContext) {
	execCtx := executorCtx.ExecCtx
	process := executorCtx.Process

	executorCtx.EndTime = time.Now()
	executorCtx.State = ExecutionStateTimeout

	// 尝试终止进程
	_ = process.Kill()

	result := &ExecutionResult{
		ExecutionID: execCtx.ExecutionID,
		ExitCode:    -1,
		Error:       "execution timed out",
		Output:      process.GetOutput(),
		StartTime:   executorCtx.StartTime,
		EndTime:     executorCtx.EndTime,
		State:       ExecutionStateTimeout,
	}

	e.updateExecutionStatus(execCtx.ExecutionID, ExecutionStateTimeout, process.GetPid())

	if execCtx.Reporter != nil {
		execCtx.Reporter.ReportError(execCtx.ExecutionID, fmt.Errorf("execution timed out after %v", execCtx.Timeout))
		execCtx.Reporter.ReportCompletion(execCtx.ExecutionID, result)
	}
}

// monitorResources 监控进程资源使用情况
func (e *Executor) monitorResources(ctx context.Context, executorCtx *ExecutorContext) {
	execCtx := executorCtx.ExecCtx
	process := executorCtx.Process
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if process == nil {
				return
			}

			// 检查进程是否还在运行
			if !process.IsRunning() {
				return
			}

			// 获取资源使用情况
			cpu, memory, err := process.GetResourceUsage()
			if err != nil {
				utils.Debug("failed to get resource usage",
					zap.String("execution_id", execCtx.ExecutionID),
					zap.Error(err))
				continue
			}

			// 更新状态
			status := &ExecutionStatus{
				ExecutionID:   execCtx.ExecutionID,
				State:         ExecutionStateRunning,
				StartTime:     executorCtx.StartTime,
				Pid:           process.GetPid(),
				CPU:           cpu,
				Memory:        memory,
				CurrentOutput: process.GetOutput(),
				OutputSize:    len(process.GetOutput()),
			}

			// 更新状态映射
			e.statusMutex.Lock()
			e.execStatusMap[execCtx.ExecutionID] = status
			e.statusMutex.Unlock()

			// 如果有报告器，报告进度
			if execCtx.Reporter != nil {
				execCtx.Reporter.ReportProgress(execCtx.ExecutionID, status)
			}
		}
	}
}

// updateExecutionStatus 更新执行状态
func (e *Executor) updateExecutionStatus(executionID string, state ExecutionState, pid int) {
	e.statusMutex.Lock()
	defer e.statusMutex.Unlock()

	status, exists := e.execStatusMap[executionID]
	if !exists {
		status = &ExecutionStatus{
			ExecutionID: executionID,
			StartTime:   time.Now(),
		}
	}

	status.State = state
	status.Pid = pid

	e.execStatusMap[executionID] = status
}

// Kill 终止正在执行的任务
func (e *Executor) Kill(executionID string) error {
	e.processMutex.RLock()
	executorCtx, exists := e.runningJobs[executionID]
	e.processMutex.RUnlock()

	if !exists {
		return fmt.Errorf("no running execution with ID %s", executionID)
	}

	utils.Info("killing execution", zap.String("execution_id", executionID))

	// 取消上下文
	if executorCtx.CancelFunc != nil {
		executorCtx.CancelFunc()
	}

	// 终止进程
	if executorCtx.Process != nil {
		if err := executorCtx.Process.Kill(); err != nil {
			utils.Warn("failed to kill process",
				zap.String("execution_id", executionID),
				zap.Error(err))
		}
	}

	// 更新状态
	executorCtx.State = ExecutionStateCancelled
	e.updateExecutionStatus(executionID, ExecutionStateCancelled, 0)

	// 报告完成
	if executorCtx.ExecCtx.Reporter != nil {
		result := &ExecutionResult{
			ExecutionID: executionID,
			ExitCode:    -1,
			Error:       "execution was cancelled",
			StartTime:   executorCtx.StartTime,
			EndTime:     time.Now(),
			State:       ExecutionStateCancelled,
		}

		executorCtx.ExecCtx.Reporter.ReportCompletion(executionID, result)
	}

	return nil
}

// GetRunningExecutions 获取当前正在运行的任务列表
func (e *Executor) GetRunningExecutions() map[string]*ExecutionStatus {
	e.statusMutex.RLock()
	defer e.statusMutex.RUnlock()

	result := make(map[string]*ExecutionStatus)

	// 复制正在运行的执行状态
	for id, status := range e.execStatusMap {
		if status.State == ExecutionStateRunning {
			statusCopy := *status
			result[id] = &statusCopy
		}
	}

	return result
}

// GetExecutionStatus 获取指定任务的执行状态
func (e *Executor) GetExecutionStatus(executionID string) (*ExecutionStatus, bool) {
	e.statusMutex.RLock()
	defer e.statusMutex.RUnlock()

	status, exists := e.execStatusMap[executionID]
	if !exists {
		return nil, false
	}

	// 返回副本以避免并发修改问题
	statusCopy := *status
	return &statusCopy, true
}

// waitForAvailableSlot 等待可用的执行槽
func (e *Executor) waitForAvailableSlot(ctx context.Context) {
	if e.options.MaxConcurrentExecutions <= 0 {
		return // 无限制
	}

	e.processMutex.Lock()
	defer e.processMutex.Unlock()

	// 等待直到运行中的任务数小于最大并发执行数
	for e.runningCount >= e.options.MaxConcurrentExecutions {
		// 创建一个通道用于监听上下文取消
		done := make(chan struct{})

		// 在协程中监听上下文取消
		go func() {
			select {
			case <-ctx.Done():
				e.runningCond.Signal() // 唤醒等待的协程
				close(done)
			case <-done:
				// 等待函数结束则退出
			}
		}()

		// 等待条件满足
		e.runningCond.Wait()

		close(done) // 通知监听协程退出

		// 检查上下文是否已取消
		if ctx.Err() != nil {
			return
		}
	}
}

// cleanupLoop 定期清理过期的执行状态
func (e *Executor) cleanupLoop() {
	for {
		select {
		case <-e.cleanupTicker.C:
			e.cleanupExpiredExecutions()
		case <-e.shutdownChan:
			e.cleanupTicker.Stop()
			return
		}
	}
}

// cleanupExpiredExecutions 清理过期的执行状态
func (e *Executor) cleanupExpiredExecutions() {
	now := time.Now()
	threshold := now.Add(-e.retentionTime)

	e.statusMutex.Lock()
	defer e.statusMutex.Unlock()

	for id, status := range e.execStatusMap {
		// 跳过正在运行的执行
		if status.State == ExecutionStateRunning || status.State == ExecutionStatePending {
			continue
		}

		// 如果开始时间在阈值之前，则删除
		if status.StartTime.Before(threshold) {
			delete(e.execStatusMap, id)
		}
	}
}

// GenerateExecutionID 生成一个唯一的执行ID
func GenerateExecutionID() string {
	return uuid.New().String()
}

// Stop 停止执行器
func (e *Executor) Stop() {
	if !e.isRunning {
		return
	}

	e.isRunning = false
	close(e.shutdownChan)

	// 终止所有运行中的任务
	e.processMutex.Lock()
	runningJobs := make(map[string]*ExecutorContext)
	for id, ctx := range e.runningJobs {
		runningJobs[id] = ctx
	}
	e.processMutex.Unlock()

	for id := range runningJobs {
		e.Kill(id)
	}

	utils.Info("executor stopped")
}

// GetExecutionsCount 获取已执行的总任务数
func (e *Executor) GetExecutionsCount() int64 {
	e.processMutex.RLock()
	defer e.processMutex.RUnlock()
	return e.executionCount
}

// SetRetentionTime 设置执行状态保留时间
func (e *Executor) SetRetentionTime(retention time.Duration) {
	if retention <= 0 {
		return
	}

	e.retentionTime = retention
}
