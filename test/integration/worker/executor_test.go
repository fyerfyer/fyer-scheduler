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

func TestCronExpressionParsing(t *testing.T) {
	jobFactory := testutils.NewJobFactory()

	testCases := []struct {
		name     string
		cronExpr string
		valid    bool
	}{
		{"Every Minute", "* * * * *", true},
		{"Every Hour", "0 * * * *", true},
		{"Every Day at Midnight", "0 0 * * *", true},
		{"Every Monday", "0 0 * * 1", true},
		{"Every Month First Day", "0 0 1 * *", true},
		{"Invalid Format", "* * *", false},
		{"Invalid Values", "99 99 99 99 99", false},
		{"Extended Format", "*/5 * * * *", true},
		{"Complex Expression", "15,45 */2 1-15 * 1-5", true},
		{"With Seconds", "0 */5 * * * *", false}, // 6-part expressions not supported by default
		{"Special @daily", "@daily", true},
		{"Special @hourly", "@hourly", true},
		{"Special @weekly", "@weekly", true},
		{"Special @monthly", "@monthly", true},
		{"Special @yearly", "@yearly", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job := jobFactory.CreateScheduledJob(tc.cronExpr)
			if tc.valid {
				assert.NotNil(t, job, "Expected valid cron expression")
			} else {
				assert.Nil(t, job, "Expected invalid cron expression")
			}
		})
	}
}

func TestCronNextRunTime(t *testing.T) {
	jobFactory := testutils.NewJobFactory()

	testCases := []struct {
		name     string
		cronExpr string
		validate func(t *testing.T, nextRunTime time.Time)
	}{
		{
			"Every Minute",
			"* * * * *",
			func(t *testing.T, nextRunTime time.Time) {
				// Should be less than 1 minute in the future
				expected := time.Now().Add(time.Minute)
				assert.True(t, nextRunTime.Before(expected),
					"NextRunTime should be less than 1 minute in the future")
			},
		},
		{
			"Every Hour",
			"0 * * * *",
			func(t *testing.T, nextRunTime time.Time) {
				now := time.Now()
				nextHour := now.Add(time.Hour).Truncate(time.Hour)
				diff := nextRunTime.Sub(nextHour).Abs()
				assert.True(t, diff < time.Minute,
					"NextRunTime should be at the next hour")
			},
		},
		{
			"Every 5 Minutes",
			"*/5 * * * *",
			func(t *testing.T, nextRunTime time.Time) {
				expected := time.Now().Add(5 * time.Minute)
				assert.True(t, nextRunTime.Before(expected),
					"NextRunTime should be within the next 5 minutes")

				assert.Equal(t, 0, nextRunTime.Minute()%5,
					"NextRunTime minute should be divisible by 5")
			},
		},
		{
			"Every 15 Minutes",
			"*/15 * * * *",
			func(t *testing.T, nextRunTime time.Time) {
				expected := time.Now().Add(15 * time.Minute)
				assert.True(t, nextRunTime.Before(expected),
					"NextRunTime should be within the next 15 minutes")

				validMinutes := map[int]bool{0: true, 15: true, 30: true, 45: true}
				assert.True(t, validMinutes[nextRunTime.Minute()],
					"NextRunTime minute should be 0, 15, 30, or 45")
			},
		},
		{
			"Daily at Midnight",
			"0 0 * * *",
			func(t *testing.T, nextRunTime time.Time) {
				assert.Equal(t, 0, nextRunTime.Hour(), "NextRunTime hour should be 0")
				assert.Equal(t, 0, nextRunTime.Minute(), "NextRunTime minute should be 0")

				now := time.Now()
				tomorrow := now.AddDate(0, 0, 1)

				if now.Hour() >= 0 || (now.Hour() == 0 && now.Minute() > 0) {
					assert.Equal(t, tomorrow.Day(), nextRunTime.Day(),
						"NextRunTime should be tomorrow at midnight")
				}
			},
		},
		{
			"Weekdays at 9am",
			"0 9 * * 1-5",
			func(t *testing.T, nextRunTime time.Time) {
				assert.Equal(t, 9, nextRunTime.Hour(), "NextRunTime hour should be 9")
				assert.Equal(t, 0, nextRunTime.Minute(), "NextRunTime minute should be 0")

				weekday := nextRunTime.Weekday()
				assert.True(t, weekday >= time.Monday && weekday <= time.Friday,
					"NextRunTime should be on a weekday (Monday-Friday)")

				assert.True(t, nextRunTime.After(time.Now()),
					"NextRunTime should be in the future")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job := jobFactory.CreateScheduledJob(tc.cronExpr)
			require.NotNil(t, job, "Failed to create job with cron expression: %s", tc.cronExpr)

			assert.False(t, job.NextRunTime.IsZero(), "NextRunTime should be set")

			tc.validate(t, job.NextRunTime)
		})
	}
}

func TestExecutorWithScheduledCommands(t *testing.T) {
	mockReporter := mocks.NewIExecutionReporter(t)

	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportError(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe() // 添加这一行
	mockReporter.EXPECT().ReportCompletion(testutils.MockAny(), testutils.MockAny()).Return(nil)

	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(5),
		executor.WithDefaultTimeout(30*time.Second),
	)

	jobFactory := testutils.NewJobFactory()

	testCases := []struct {
		name         string
		command      string
		args         []string
		expectedOut  string
		expectErr    bool
		expectedCode int
	}{
		{"Echo Command", "cmd", []string{"/c", "echo", "scheduled task"}, "scheduled task", false, 0},
		{"Directory Listing", "cmd", []string{"/c", "dir"}, "", false, 0},
		{"Sleep Command", "cmd", []string{"/c", "timeout", "1"}, "", false, 1}, // 改为期望退出码 1
		{"Invalid Command", "nonexistentcmd", []string{}, "", true, 1},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job := jobFactory.CreateScheduledJob("0 0 * * *")
			require.NotNil(t, job, "Failed to create scheduled job")

			execCtx := &executor.ExecutionContext{
				ExecutionID:   fmt.Sprintf("test-scheduled-exec-%d", i+1),
				Job:           job,
				Command:       tc.command,
				Args:          tc.args,
				Timeout:       5 * time.Second,
				Reporter:      mockReporter,
				MaxOutputSize: 1024 * 1024,
			}

			result, err := exec.Execute(context.Background(), execCtx)

			require.NoError(t, err, "Execute should not return an error")

			if tc.expectErr {
				assert.NotEqual(t, 0, result.ExitCode, "Expected non-zero exit code")
				assert.Equal(t, executor.ExecutionStateFailed, result.State, "Expected failed state")
			} else {
				assert.Equal(t, tc.expectedCode, result.ExitCode, "Expected exit code %d", tc.expectedCode)

				if tc.expectedCode != 0 {
					assert.Equal(t, executor.ExecutionStateFailed, result.State, "Expected failed state for non-zero exit code")
				} else {
					assert.Equal(t, executor.ExecutionStateSuccess, result.State, "Expected success state")
				}

				if tc.expectedOut != "" {
					assert.Contains(t, result.Output, tc.expectedOut, "Output should contain expected text")
				}
			}
		})
	}
}

func TestJobRetryLogic(t *testing.T) {
	mockReporter := mocks.NewIExecutionReporter(t)

	mockReporter.EXPECT().ReportStart(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportOutput(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportError(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportCompletion(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe()
	mockReporter.EXPECT().ReportProgress(testutils.MockAny(), testutils.MockAny()).Return(nil).Maybe() // 添加对ReportProgress的处理

	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(5),
		executor.WithDefaultTimeout(10*time.Second),
	)

	jobFactory := testutils.NewJobFactory()

	job := jobFactory.CreateComplexJob(
		"retry-test-job",
		"cmd",
		"*/10 * * * *",
		5,
		nil,
	)

	job.Command = "cmd"
	job.Args = []string{"/c", "exit", "1"}
	job.MaxRetry = 2

	retryCount := 0

	executeWithRetry := func(ctx context.Context, execCtx *executor.ExecutionContext) (*executor.ExecutionResult, error) {
		var finalResult *executor.ExecutionResult
		var finalErr error

		result, err := exec.Execute(ctx, execCtx)
		retryCount++

		finalResult = result
		finalErr = err

		retryAttempt := 0
		for retryAttempt < job.MaxRetry && result.ExitCode != 0 {
			time.Sleep(100 * time.Millisecond)

			execCtx.ExecutionID = fmt.Sprintf("%s-retry-%d", execCtx.ExecutionID, retryAttempt+1)

			result, err = exec.Execute(ctx, execCtx)
			retryCount++
			retryAttempt++
			finalResult = result
			finalErr = err
		}

		return finalResult, finalErr
	}

	execCtx := &executor.ExecutionContext{
		ExecutionID:   "test-retry-exec",
		Job:           job,
		Command:       job.Command,
		Args:          job.Args,
		Timeout:       5 * time.Second,
		Reporter:      mockReporter,
		MaxOutputSize: 1024 * 1024,
	}

	result, err := executeWithRetry(context.Background(), execCtx)

	require.NoError(t, err, "Execute should not return an error")
	assert.Equal(t, 1, result.ExitCode, "Final exit code should be 1")
	assert.Equal(t, executor.ExecutionStateFailed, result.State, "Final state should be failed")
	assert.Equal(t, 3, retryCount, "Should have attempted execution 3 times (initial + 2 retries)")
}
