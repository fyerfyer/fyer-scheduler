package worker

import (
	"context"
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
	ExecutionStateCompleted = "completed"
	ExecutionStateFailed    = "failed"
	ExecutionStateTimeout   = "timeout"
)

func TestExecutorBasicExecution(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil)
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
	assert.Contains(t, result.Output, "hello world", "Output should contain 'hello world'")
	assert.Equal(t, ExecutionStateCompleted, result.State, "State should be completed")
}

func TestExecutorFailedCommand(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportError(testutils.MockAny(), testutils.MockAny()).Return(nil)

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(2),
		executor.WithDefaultTimeout(10*time.Second),
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
		Timeout:       5 * time.Second,
		Reporter:      mockReporter,
		MaxOutputSize: 10 * 1024 * 1024, // 10MB
	}

	// 执行命令
	result, err := exec.Execute(context.Background(), execCtx)

	// 验证结果
	assert.Error(t, err, "Execute should return an error for non-existent command")
	assert.NotEqual(t, 0, result.ExitCode, "Exit code should not be 0")
	assert.Equal(t, ExecutionStateFailed, result.State, "State should be failed")
}

func TestExecutorTimeout(t *testing.T) {
	// 跳过Windows平台测试，因为Windows下的sleep命令行为不同
	if os.Getenv("GOOS") == "windows" {
		t.Skip("Skipping test on Windows platform")
	}

	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportError(testutils.MockAny(), testutils.MockAny()).Return(nil)

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(2),
		executor.WithDefaultTimeout(10*time.Second),
	)

	// 创建任务工厂和简单任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 创建执行上下文，设置超时命令
	execCtx := &executor.ExecutionContext{
		ExecutionID:   "test-execution-3",
		Job:           job,
		Command:       "sleep",
		Args:          []string{"5"}, // 运行5秒
		WorkDir:       "",
		Environment:   nil,
		Timeout:       1 * time.Second, // 但我们只允许1秒超时
		Reporter:      mockReporter,
		MaxOutputSize: 10 * 1024 * 1024, // 10MB
	}

	// 执行命令
	result, err := exec.Execute(context.Background(), execCtx)

	// 验证结果
	assert.Error(t, err, "Execute should return an error for timeout")
	assert.Equal(t, ExecutionStateTimeout, result.State, "State should be timeout")
}

func TestExecutorWithWorkDir(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportCompletion(testutils.MockAny(), testutils.MockAny()).Return(nil)

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(2),
		executor.WithDefaultTimeout(10*time.Second),
	)

	// 获取当前目录作为工作目录
	workDir, err := os.Getwd()
	require.NoError(t, err, "Failed to get current directory")

	// 创建任务工厂和简单任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 创建执行上下文，使用指定的工作目录
	execCtx := &executor.ExecutionContext{
		ExecutionID:   "test-execution-4",
		Job:           job,
		Command:       "pwd",
		Args:          []string{},
		WorkDir:       workDir,
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
	assert.Contains(t, result.Output, filepath.Base(workDir), "Output should contain the working directory")
	assert.Equal(t, ExecutionStateCompleted, result.State, "State should be completed")
}

func TestExecutorWithEnvironment(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil)
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

	// 创建执行上下文，使用环境变量
	execCtx := &executor.ExecutionContext{
		ExecutionID:   "test-execution-5",
		Job:           job,
		Command:       "env", // 在Linux上打印环境变量
		Args:          []string{},
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
	assert.Contains(t, result.Output, "TEST_VAR=test_value", "Output should contain the environment variable")
	assert.Equal(t, ExecutionStateCompleted, result.State, "State should be completed")
}
