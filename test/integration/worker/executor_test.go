package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/mocks"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/executor"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 定义执行状态常量，与executor包中的常量保持一致
const (
	ExecutionStateSuccess = executor.ExecutionStateSuccess
	ExecutionStateFailed  = executor.ExecutionStateFailed
)

func TestExecutorBasicExecution(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	log.Println("current os: ", os.Getenv("GOOS"))

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportCompletion(testutils.MockAny(), testutils.MockAny()).Return(nil)

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(2),
		executor.WithDefaultTimeout(10*time.Second),
	)

	// 创建任务工厂和简单任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 创建执行上下文
	execCtx := &executor.ExecutionContext{
		ExecutionID:   "test-execution-1",
		Job:           job,
		Command:       "echo",
		Args:          []string{"hello world"},
		WorkDir:       "",
		Environment:   nil,
		Timeout:       5 * time.Second,
		Reporter:      mockReporter,
		MaxOutputSize: 10 * 1024 * 1024, // 10MB
	}

	// 执行命令
	result, err := exec.Execute(context.Background(), execCtx)

	// 验证结果
	require.NoError(t, err, "Execute should not return an error")
	assert.Equal(t, 0, result.ExitCode, "Exit code should be 0")
	assert.Equal(t, ExecutionStateSuccess, result.State, "Execution state should be success")
	assert.Contains(t, result.Output, "hello world", "Output should contain expected text")
}

func TestExecutorFailedCommand(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportError(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportCompletion(testutils.MockAny(), testutils.MockAny()).Return(nil)

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(2),
		executor.WithDefaultTimeout(5*time.Second),
	)

	// 创建任务工厂和简单任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 创建执行上下文，使用不存在的命令
	execCtx := &executor.ExecutionContext{
		ExecutionID:   "test-execution-2",
		Job:           job,
		Command:       "nonexistentcommand",
		Args:          []string{},
		WorkDir:       "",
		Environment:   nil,
		Timeout:       2 * time.Second,
		Reporter:      mockReporter,
		MaxOutputSize: 10 * 1024 * 1024, // 10MB
	}

	// 执行命令
	result, err := exec.Execute(context.Background(), execCtx)

	// 验证结果
	require.NoError(t, err, "Execute method should not return an error, but the command execution should fail")
	assert.NotEqual(t, 0, result.ExitCode, "Exit code should not be 0 for failed command")
	assert.Equal(t, ExecutionStateFailed, result.State, "Execution state should be failed")
}

func TestExecutorWithWorkDir(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportCompletion(testutils.MockAny(), testutils.MockAny()).Return(nil)

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(2),
		executor.WithDefaultTimeout(10*time.Second),
	)

	// 获取当前目录作为工作目录
	currentDir, err := os.Getwd()
	require.NoError(t, err, "Failed to get current directory")

	// 创建任务工厂和简单任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 使用适合当前操作系统的命令
	var command string
	var args []string
	if os.Getenv("GOOS") == "windows" {
		command = "cmd"
		args = []string{"/c", "echo %CD%"}
	} else {
		command = "pwd"
		args = []string{}
	}

	// 创建执行上下文，使用指定的工作目录
	execCtx := &executor.ExecutionContext{
		ExecutionID:   "test-execution-3",
		Job:           job,
		Command:       command,
		Args:          args,
		WorkDir:       currentDir,
		Environment:   nil,
		Timeout:       5 * time.Second,
		Reporter:      mockReporter,
		MaxOutputSize: 10 * 1024 * 1024, // 10MB
	}

	// 执行命令
	result, err := exec.Execute(context.Background(), execCtx)

	// 验证结果
	require.NoError(t, err, "Execute should not return an error")
	assert.Equal(t, 0, result.ExitCode, "Exit code should be 0")
	assert.Equal(t, ExecutionStateSuccess, result.State, "Execution state should be success")

	// 检查输出是否包含当前目录（考虑路径格式可能不同）
	normalizedOutput := filepath.ToSlash(result.Output)
	normalizedCurrentDir := filepath.ToSlash(currentDir)
	assert.Contains(t, normalizedOutput, normalizedCurrentDir, "Output should contain the current directory")
}

func TestExecutorWithEnvironment(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportCompletion(testutils.MockAny(), testutils.MockAny()).Return(nil)

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(2),
		executor.WithDefaultTimeout(10*time.Second),
	)

	// 创建任务工厂和简单任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 设置环境变量
	env := map[string]string{
		"TEST_VAR": "test_value",
	}

	// 使用适合当前操作系统的命令来打印环境变量
	var command string
	var args []string
	if os.Getenv("GOOS") == "windows" {
		command = "cmd"
		args = []string{"/c", "echo %TEST_VAR%"}
	} else {
		command = "echo"
		args = []string{"$TEST_VAR"}
	}

	// 创建执行上下文，使用环境变量
	execCtx := &executor.ExecutionContext{
		ExecutionID:   "test-execution-4",
		Job:           job,
		Command:       command,
		Args:          args,
		WorkDir:       "",
		Environment:   env,
		Timeout:       5 * time.Second,
		Reporter:      mockReporter,
		MaxOutputSize: 10 * 1024 * 1024, // 10MB
	}

	// 执行命令
	result, err := exec.Execute(context.Background(), execCtx)

	// 验证结果
	require.NoError(t, err, "Execute should not return an error")
	assert.Equal(t, 0, result.ExitCode, "Exit code should be 0")
	assert.Equal(t, ExecutionStateSuccess, result.State, "Execution state should be success")
	assert.Contains(t, result.Output, "test_value", "Output should contain the environment variable value")
}

func TestExecutorKill(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportProgress(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportCompletion(testutils.MockAny(), testutils.MockAny()).Return(nil)

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(2),
		executor.WithDefaultTimeout(30*time.Second),
		executor.WithKillGracePeriod(1*time.Second),
	)

	// 创建任务工厂和简单任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 创建一个长时间运行的命令
	var command string
	var args []string
	if os.Getenv("GOOS") == "windows" {
		command = "ping"
		args = []string{"-t", "localhost"}
	} else {
		command = "sleep"
		args = []string{"30"}
	}

	// 创建执行上下文
	execID := "test-kill-execution"
	execCtx := &executor.ExecutionContext{
		ExecutionID:   execID,
		Job:           job,
		Command:       command,
		Args:          args,
		WorkDir:       "",
		Environment:   nil,
		Timeout:       30 * time.Second,
		Reporter:      mockReporter,
		MaxOutputSize: 10 * 1024 * 1024, // 10MB
	}

	// 在goroutine中执行命令，因为它会运行很长时间
	resultChan := make(chan *executor.ExecutionResult, 1)
	errChan := make(chan error, 1)
	go func() {
		result, err := exec.Execute(context.Background(), execCtx)
		resultChan <- result
		errChan <- err
	}()

	// 等待一段时间让命令开始运行
	time.Sleep(2 * time.Second)

	// 获取执行状态，确认命令正在运行
	status, exists := exec.GetExecutionStatus(execID)
	require.True(t, exists, "Execution status should exist")
	assert.Equal(t, executor.ExecutionStateRunning, status.State, "Execution should be running")

	// 终止命令
	err := exec.Kill(execID)
	require.NoError(t, err, "Kill should not return an error")

	// 等待结果
	select {
	case result := <-resultChan:
		// 验证结果
		assert.NotEqual(t, 0, result.ExitCode, "Exit code should not be 0 for killed process")
		assert.NotEqual(t, ExecutionStateSuccess, result.State, "Execution state should not be success")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for execution result after kill")
	}
}

func TestExecutorWithConcurrency(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置较宽松的模拟期望，因为我们将执行多个命令
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportCompletion(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()

	// 创建执行器，设置并发数为2
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(2),
		executor.WithDefaultTimeout(10*time.Second),
	)

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 创建3个任务，使用sleep命令
	jobCount := 3
	results := make(chan bool, jobCount)

	// 使用适合当前操作系统的sleep命令
	var command string
	var args []string
	if os.Getenv("GOOS") == "windows" {
		command = "timeout"
		args = []string{"/t", "3", "/nobreak"}
	} else {
		command = "sleep"
		args = []string{"3"}
	}

	// 并发启动3个任务
	for i := 0; i < jobCount; i++ {
		go func(index int) {
			job := jobFactory.CreateSimpleJob()
			execCtx := &executor.ExecutionContext{
				ExecutionID:   fmt.Sprintf("test-concurrent-%d", index),
				Job:           job,
				Command:       command,
				Args:          args,
				WorkDir:       "",
				Environment:   nil,
				Timeout:       5 * time.Second,
				Reporter:      mockReporter,
				MaxOutputSize: 10 * 1024 * 1024, // 10MB
			}

			result, err := exec.Execute(context.Background(), execCtx)
			if err == nil && result.ExitCode == 0 {
				results <- true
			} else {
				results <- false
			}
		}(i)
	}

	// 检查并发执行
	execStatus := exec.GetRunningExecutions()
	time.Sleep(500 * time.Millisecond) // 给执行一些启动时间

	// 由于我们设置了最大并发数为2，应该只有2个任务在运行
	t.Logf("Running executions: %d", len(execStatus))
	assert.LessOrEqual(t, len(execStatus), 2, "Should have at most 2 concurrent executions")

	// 等待所有任务完成
	successCount := 0
	for i := 0; i < jobCount; i++ {
		success := <-results
		if success {
			successCount++
		}
	}

	assert.Equal(t, jobCount, successCount, "All jobs should have succeeded")
}

func TestExecutorWithTimeout(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportProgress(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportCompletion(testutils.MockAny(), testutils.MockAny()).Return(nil)

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(2),
		executor.WithDefaultTimeout(10*time.Second),
	)

	// 创建任务工厂和简单任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 创建一个长时间运行的命令
	var command string
	var args []string
	if os.Getenv("GOOS") == "windows" {
		command = "ping"
		args = []string{"-t", "localhost"}
	} else {
		command = "sleep"
		args = []string{"30"}
	}

	// 创建执行上下文，设置非常短的超时
	execCtx := &executor.ExecutionContext{
		ExecutionID:   "test-timeout-execution",
		Job:           job,
		Command:       command,
		Args:          args,
		WorkDir:       "",
		Environment:   nil,
		Timeout:       2 * time.Second, // 非常短的超时
		Reporter:      mockReporter,
		MaxOutputSize: 10 * 1024 * 1024, // 10MB
	}

	// 执行命令
	result, err := exec.Execute(context.Background(), execCtx)

	// 验证结果
	require.NoError(t, err, "Execute should not return an error")
	assert.NotEqual(t, 0, result.ExitCode, "Exit code should not be 0 for timed out process")
	assert.Equal(t, executor.ExecutionStateTimeout, result.State, "Execution state should be timeout")
}

func TestExecutorMultipleCommands(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportCompletion(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(5),
		executor.WithDefaultTimeout(10*time.Second),
	)

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 准备一组要执行的命令
	commandPairs := []struct {
		cmd  string
		args []string
	}{
		{"echo", []string{"Hello World"}},
		{"echo", []string{"Test Multiple Commands"}},
		{"echo", []string{"Success Expected"}},
	}

	// 依次执行每个命令
	for i, cmdPair := range commandPairs {
		// 创建执行上下文
		execID := fmt.Sprintf("test-multi-%d", i)
		execCtx := &executor.ExecutionContext{
			ExecutionID:   execID,
			Job:           job,
			Command:       cmdPair.cmd,
			Args:          cmdPair.args,
			WorkDir:       "",
			Environment:   nil,
			Timeout:       5 * time.Second,
			Reporter:      mockReporter,
			MaxOutputSize: 10 * 1024 * 1024, // 10MB
		}

		// 执行命令
		result, err := exec.Execute(context.Background(), execCtx)

		// 验证结果
		require.NoError(t, err, "Execute should not return an error")
		assert.Equal(t, 0, result.ExitCode, "Exit code should be 0")
		assert.Equal(t, ExecutionStateSuccess, result.State, "Execution state should be success")

		// 验证输出包含命令参数
		for _, arg := range cmdPair.args {
			assert.Contains(t, result.Output, arg, "Output should contain the command argument")
		}
	}
}

func TestExecuteJobWithContextCancellation(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportProgress(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportCompletion(testutils.MockAny(), testutils.MockAny()).Return(nil)

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(2),
		executor.WithDefaultTimeout(30*time.Second),
	)

	// 创建任务工厂和简单任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 创建一个长时间运行的命令
	var command string
	var args []string
	if os.Getenv("GOOS") == "windows" {
		command = "ping"
		args = []string{"-t", "localhost"}
	} else {
		command = "sleep"
		args = []string{"10"}
	}

	// 创建一个可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 创建执行上下文
	execCtx := &executor.ExecutionContext{
		ExecutionID:   "test-ctx-cancel",
		Job:           job,
		Command:       command,
		Args:          args,
		WorkDir:       "",
		Environment:   nil,
		Timeout:       20 * time.Second,
		Reporter:      mockReporter,
		MaxOutputSize: 10 * 1024 * 1024, // 10MB
	}

	// 在goroutine中执行命令
	resultChan := make(chan *executor.ExecutionResult, 1)
	errChan := make(chan error, 1)
	go func() {
		result, err := exec.Execute(ctx, execCtx)
		resultChan <- result
		errChan <- err
	}()

	// 等待命令开始执行
	time.Sleep(1 * time.Second)

	// 取消上下文
	cancel()

	// 等待结果
	var result *executor.ExecutionResult
	var err error
	select {
	case result = <-resultChan:
		err = <-errChan
		// 正常情况，已收到结果
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for execution result after context cancellation")
	}

	// 验证执行已被取消
	require.NoError(t, err, "Execute should not return an error")
	assert.NotEqual(t, 0, result.ExitCode, "Exit code should not be 0 for cancelled process")
	assert.Equal(t, executor.ExecutionStateCancelled, result.State, "Execution state should be cancelled")
}
