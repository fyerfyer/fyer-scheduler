package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/shirou/gopsutil/v3/process"
	"go.uber.org/zap"
)

// Process 封装了执行命令的系统进程
type Process struct {
	cmd           *exec.Cmd
	executionID   string
	pid           int
	startTime     time.Time
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	outBuffer     string
	done          chan struct{}
	cancelFunc    context.CancelFunc
	exitCode      int
	exitErr       error
	mutex         sync.RWMutex
	gracePeriod   time.Duration
	outputHandler func(string)
	maxOutputSize int
	currentSize   int

	isTimedOut bool
}

// ProcessOptions 定义进程选项
type ProcessOptions struct {
	ExecutionID   string
	Command       string
	Args          []string
	WorkDir       string
	Env           map[string]string
	GracePeriod   time.Duration
	OutputHandler func(string)
	MaxOutputSize int
}

// NewProcess 创建一个新的进程实例
func NewProcess(ctx context.Context, options ProcessOptions) (*Process, error) {
	// 创建可取消的上下文
	processCtx, cancelFunc := context.WithCancel(ctx)

	// 设置默认的优雅终止期
	gracePeriod := options.GracePeriod
	if gracePeriod <= 0 {
		gracePeriod = 5 * time.Second // 默认5秒
	}

	// 设置最大输出大小
	maxOutputSize := options.MaxOutputSize
	if maxOutputSize <= 0 {
		maxOutputSize = 10 * 1024 * 1024 // 默认10MB
	}

	// 创建命令
	cmd := exec.CommandContext(processCtx, options.Command, options.Args...)

	// 设置工作目录
	if options.WorkDir != "" {
		cmd.Dir = options.WorkDir
	}

	// 设置环境变量
	if len(options.Env) > 0 {
		cmd.Env = os.Environ() // 首先包含当前环境
		for k, v := range options.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// 获取命令输出
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancelFunc()
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancelFunc()
		stdout.Close()
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	return &Process{
		cmd:           cmd,
		executionID:   options.ExecutionID,
		done:          make(chan struct{}),
		cancelFunc:    cancelFunc,
		stdout:        stdout,
		stderr:        stderr,
		gracePeriod:   gracePeriod,
		outputHandler: options.OutputHandler,
		maxOutputSize: maxOutputSize,
	}, nil
}

// Start 启动进程
func (p *Process) Start() error {
	// 启动命令
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	p.mutex.Lock()
	p.startTime = time.Now()
	p.pid = p.cmd.Process.Pid
	p.mutex.Unlock()

	utils.Info("process started",
		zap.String("execution_id", p.executionID),
		zap.Int("pid", p.pid),
		zap.String("command", p.cmd.Path),
		zap.Strings("args", p.cmd.Args[1:]))

	// 启动输出捕获
	go p.captureOutput()

	// 监控进程完成
	go p.monitorCompletion()

	return nil
}

// Kill 终止进程
func (p *Process) Kill() error {
	p.mutex.RLock()
	pid := p.pid
	p.mutex.RUnlock()

	if pid <= 0 {
		return fmt.Errorf("no active process to kill")
	}

	utils.Info("killing process",
		zap.String("execution_id", p.executionID),
		zap.Int("pid", pid),
		zap.Duration("grace_period", p.gracePeriod))

	// 取消上下文，这会触发exec.CommandContext终止进程
	p.cancelFunc()

	// 在Windows和Unix上使用不同的终止方法
	var err error
	if runtime.GOOS == "windows" {
		err = p.killWindows()
	} else {
		err = p.killUnix()
	}

	if err != nil {
		utils.Error("failed to kill process",
			zap.String("execution_id", p.executionID),
			zap.Int("pid", pid),
			zap.Error(err))
		return fmt.Errorf("failed to kill process: %w", err)
	}

	// 等待进程完成
	select {
	case <-p.done:
		return nil
	case <-time.After(p.gracePeriod + 1*time.Second):
		return fmt.Errorf("process kill timed out")
	}
}

// killWindows 在Windows平台上终止进程
func (p *Process) killWindows() error {
	// Windows上，使用taskkill命令强制终止进程树
	killCmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", p.pid))
	return killCmd.Run()
}

// killUnix 在Unix平台上终止进程
func (p *Process) killUnix() error {
	proc := p.cmd.Process

	// 首先尝试发送SIGTERM信号进行优雅终止
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// 如果SIGTERM失败，直接使用SIGKILL
		return proc.Kill()
	}

	// 等待优雅终止的时间
	gracefulShutdown := make(chan struct{})
	go func() {
		p.cmd.Wait()
		close(gracefulShutdown)
	}()

	// 如果在优雅期内完成，则返回
	select {
	case <-gracefulShutdown:
		return nil
	case <-time.After(p.gracePeriod):
		// 超时后使用SIGKILL强制终止
		return proc.Kill()
	}
}

// Wait 等待进程完成并返回结果
func (p *Process) Wait() (int, error) {
	<-p.done

	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.exitCode, p.exitErr
}

// WaitWithTimeout 带超时的等待进程完成
func (p *Process) WaitWithTimeout(timeout time.Duration) (int, error, bool) {
	if timeout <= 0 {
		res, err := p.Wait()
		return res, err, false
	}

	fmt.Printf("DEBUG: Waiting for process %s with timeout %v\n", p.executionID, timeout)
	timeoutCh := time.After(timeout)

	select {
	case <-timeoutCh:
		fmt.Printf("DEBUG: Process %s timed out after %v\n", p.executionID, timeout)

		// 设置超时标志
		p.mutex.Lock()
		p.isTimedOut = true
		p.mutex.Unlock()

		// 终止进程
		p.Kill()

		// 等待进程实际结束
		<-p.done

		p.mutex.RLock()
		defer p.mutex.RUnlock()
		// 即使进程有退出码，也将其视为超时
		return -1, fmt.Errorf("process execution timed out after %v", timeout), true

	case <-p.done:
		// 进程在超时前完成
		fmt.Printf("DEBUG: Process %s completed before timeout, exit code: %d, error: %v\n",
			p.executionID, p.exitCode, p.exitErr)
		p.mutex.RLock()
		defer p.mutex.RUnlock()
		return p.exitCode, p.exitErr, false
	}
}

// GetPid 获取进程ID
func (p *Process) GetPid() int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.pid
}

// GetOutput 获取已捕获的输出
func (p *Process) GetOutput() string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.outBuffer
}

// GetResourceUsage 获取进程资源使用情况
func (p *Process) GetResourceUsage() (float64, float64, error) {
	p.mutex.RLock()
	pid := p.pid
	p.mutex.RUnlock()

	if pid <= 0 {
		return 0, 0, fmt.Errorf("no active process")
	}

	// 使用gopsutil获取进程信息
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get process info: %w", err)
	}

	// 获取CPU使用率
	cpuPercent, err := proc.CPUPercent()
	if err != nil {
		cpuPercent = 0
	}

	// 获取内存使用
	memInfo, err := proc.MemoryInfo()
	if err != nil {
		return cpuPercent, 0, fmt.Errorf("failed to get memory info: %w", err)
	}

	// 转换为MB
	memMB := float64(memInfo.RSS) / 1024 / 1024

	return cpuPercent, memMB, nil
}

// IsRunning 检查进程是否仍在运行
func (p *Process) IsRunning() bool {
	p.mutex.RLock()
	pid := p.pid
	p.mutex.RUnlock()

	if pid <= 0 {
		return false
	}

	// 尝试获取进程信息，如果无法获取则认为进程已终止
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return false
	}

	// 检查进程状态
	statuses, err := proc.Status()
	if err != nil {
		return false
	}

	// 不同操作系统下的运行状态可能不同
	// Linux: "R" (running), "S" (sleeping), "D" (disk sleep), "Z" (zombie), "T" (stopped), "t" (tracing stop)
	// Windows: "running", "sleeping", "idle"
	runningStates := map[string]bool{
		"R": true, "S": true, "D": true,
		"running": true, "sleeping": true, "idle": true,
	}

	for _, status := range statuses {
		if runningStates[status] {
			return true
		}
	}

	return false
}

// GetRuntime 获取进程已运行时间
func (p *Process) GetRuntime() time.Duration {
	p.mutex.RLock()
	startTime := p.startTime
	p.mutex.RUnlock()

	if startTime.IsZero() {
		return 0
	}

	return time.Since(startTime)
}

// captureOutput 捕获进程输出并处理
func (p *Process) captureOutput() {
	var wg sync.WaitGroup
	wg.Add(2)

	// 处理标准输出
	go func() {
		defer wg.Done()
		p.captureStream(p.stdout)
	}()

	// 处理标准错误
	go func() {
		defer wg.Done()
		p.captureStream(p.stderr)
	}()

	// 等待两个goroutine完成
	wg.Wait()
}

// captureStream 捕获单个流并处理
func (p *Process) captureStream(reader io.ReadCloser) {
	scanner := bufio.NewScanner(reader)

	// 增大缓冲区以处理长行
	const maxCapacity = 512 * 1024 // 512KB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Text()

		p.mutex.Lock()
		// 检查是否超过最大输出大小
		if p.currentSize+len(line)+1 <= p.maxOutputSize {
			if p.outBuffer != "" {
				p.outBuffer += "\n"
			}
			p.outBuffer += line
			p.currentSize += len(line) + 1 // +1 for newline
		} else if p.currentSize < p.maxOutputSize {
			// 如果还有一些空间，添加截断消息
			p.outBuffer += "\n... output truncated ..."
			p.currentSize = p.maxOutputSize
		}
		p.mutex.Unlock()

		// 如果设置了输出处理器，调用它
		if p.outputHandler != nil {
			p.outputHandler(line)
		}
	}

	if err := scanner.Err(); err != nil {
		// 只有在不是因为进程终止而产生的IO错误才记录
		if err != io.EOF && !isProcessTerminated(err) {
			utils.Warn("error reading process output",
				zap.String("execution_id", p.executionID),
				zap.Error(err))
		}
	}
}

// monitorCompletion 监控进程完成状态
func (p *Process) monitorCompletion() {
	// 等待命令完成
	err := p.cmd.Wait()

	p.mutex.Lock()
	// 设置退出码和错误
	if err != nil {
		p.exitErr = err
		if exitErr, ok := err.(*exec.ExitError); ok {
			// 从ExitError中获取退出码
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				p.exitCode = status.ExitStatus()
			} else {
				p.exitCode = 1 // 默认非零退出码
			}
		} else {
			p.exitCode = 1 // 默认非零退出码
		}
	} else {
		p.exitCode = 0
		p.exitErr = nil
	}
	p.mutex.Unlock()

	utils.Info("process completed",
		zap.String("execution_id", p.executionID),
		zap.Int("pid", p.pid),
		zap.Int("exit_code", p.exitCode),
		zap.Error(p.exitErr))

	// 发出完成信号
	close(p.done)
}

// isProcessTerminated 检查错误是否是因为进程终止导致的
func isProcessTerminated(err error) bool {
	// 常见的进程终止错误信息
	terminationErrors := []string{
		"process already finished",
		"broken pipe",
		"closed pipe",
		"file already closed",
	}

	errMsg := err.Error()
	for _, termErr := range terminationErrors {
		if strings.Contains(strings.ToLower(errMsg), strings.ToLower(termErr)) {
			return true
		}
	}
	return false
}
