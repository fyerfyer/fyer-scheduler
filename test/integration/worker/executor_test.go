package worker

import (
	"context"
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
	assert.Contains(t, result.Output, "hello world", "Output should contain 'hello world'")
	assert.Equal(t, ExecutionStateSuccess, result.State, "State should be completed")
}

func TestExecutorFailedCommand(t *testing.T) {
	// 创建模拟的执行报告器
	mockReporter := mocks.NewIExecutionReporter(t)

	// 设置模拟期望
	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportError(testutils.MockAny(), testutils.MockAny()).Return(nil)
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
	require.NoError(t, err, "Execute should not return an error")
	assert.NotEqual(t, 0, result.ExitCode, "Exit code should not be 0")
	assert.Equal(t, ExecutionStateFailed, result.State, "State should be failed")
	assert.Contains(t, result.Error, "executable file not found", "Error should indicate command not found")
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
	workDir, err := os.Getwd()
	require.NoError(t, err, "Failed to get current directory")

	// 创建任务工厂和简单任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 创建执行上下文，使用指定的工作目录
	execCtx := &executor.ExecutionContext{
		ExecutionID:   "test-execution-4",
		Job:           job,
		Command:       "cmd",                // 改为Windows命令
		Args:          []string{"/c", "cd"}, // 在Windows上使用cd命令代替pwd
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
	assert.Equal(t, ExecutionStateSuccess, result.State, "State should be completed")
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

	// 创建执行上下文，使用环境变量
	execCtx := &executor.ExecutionContext{
		ExecutionID:   "test-execution-5",
		Job:           job,
		Command:       "cmd",                 // 改为Windows的命令
		Args:          []string{"/c", "set"}, // 在Windows上使用set命令代替env
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
	// Windows可能会返回格式为 "TEST_VAR=test_value" 的输出
	assert.Equal(t, ExecutionStateSuccess, result.State, "State should be completed")
}
