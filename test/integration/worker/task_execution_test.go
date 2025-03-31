package worker

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/executor"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/joblock"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/jobmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/scheduler"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestEndToEndTaskExecution 测试从分配到执行的完整流程
func TestEndToEndTaskExecution(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建唯一的worker ID
	workerID := testutils.GenerateUniqueID("worker")

	// 创建执行结果追踪器
	executionResults := make(map[string]*executor.ExecutionResult)
	executionLogs := make(map[string]string)
	executionMutex := sync.Mutex{}

	// 创建执行报告器
	mockReporter := &testIntegrationExecutionReporter{
		executionResults: executionResults,
		executionLogs:    executionLogs,
		mutex:            &executionMutex,
	}

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(5),
		executor.WithDefaultTimeout(30*time.Second),
	)

	// 创建WorkerJobManager
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	err := jobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer jobManager.Stop()

	// 创建LockManager
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	err = lockManager.Start()
	require.NoError(t, err, "Failed to start lock manager")
	defer lockManager.Stop()

	// 创建执行函数
	executeJobFunc := func(job *scheduler.ScheduledJob) error {
		t.Logf("executing job %s", job.Job.ID)

		// 创建执行上下文
		executionID := "exec-" + job.Job.ID

		// 处理命令和参数
		var command string
		var args []string

		if runtime.GOOS == "windows" {
			// Windows环境下使用cmd /c作为命令，后面是参数
			command = "cmd"
			args = []string{"/c", job.Job.Command}
		} else {
			// Linux环境下，拆分命令和参数
			parts := strings.Split(job.Job.Command, " ")
			command = parts[0]
			if len(parts) > 1 {
				args = parts[1:]
			}
		}

		execCtx := &executor.ExecutionContext{
			ExecutionID:   executionID,
			Job:           job.Job,
			Command:       command,
			Args:          args,
			WorkDir:       job.Job.WorkDir,
			Environment:   job.Job.Env,
			Timeout:       time.Duration(job.Job.Timeout) * time.Second,
			Reporter:      mockReporter,
			MaxOutputSize: 1024 * 1024, // 1MB
		}

		// 执行命令
		ctx := context.Background()
		result, err := exec.Execute(ctx, execCtx)
		if err != nil {
			t.Logf("Error executing job: %v", err)
			return err
		}

		// 保存执行结果
		executionMutex.Lock()
		executionResults[executionID] = result
		executionMutex.Unlock()

		return nil
	}

	// 创建调度器
	sched := scheduler.NewScheduler(
		jobManager,
		lockManager,
		executeJobFunc,
		scheduler.WithCheckInterval(1*time.Second),
		scheduler.WithMaxConcurrent(3),
	)

	// 启动调度器
	err = sched.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer sched.Stop()

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 创建一个简单任务 - 使用适合当前操作系统的命令
	var job *models.Job
	if runtime.GOOS == "windows" {
		job = jobFactory.CreateJobWithCommand("echo hello world")
	} else {
		job = jobFactory.CreateJobWithCommand("echo hello world")
	}

	// 添加 Worker ID 到任务环境变量，将其分配给当前 Worker
	if job.Env == nil {
		job.Env = make(map[string]string)
	}
	job.Env["WORKER_ID"] = workerID

	// 转换为 JSON 并保存到 etcd
	jobJSON, err := job.ToJSON()
	require.NoError(t, err, "Failed to convert job to JSON")

	// 将任务保存到 etcd
	err = etcdClient.Put(constants.JobPrefix+job.ID, jobJSON)
	require.NoError(t, err, "Failed to put job to etcd")

	// 等待 JobManager 检测到任务
	time.Sleep(2 * time.Second)

	// 立即触发任务执行
	err = sched.TriggerJob(job.ID)
	require.NoError(t, err, "Failed to trigger job")

	// 等待任务执行完成
	var result *executor.ExecutionResult

	// 等待最多10秒钟
	success := testutils.WaitForCondition(func() bool {
		executionMutex.Lock()
		defer executionMutex.Unlock()

		executionID := "exec-" + job.ID
		result = executionResults[executionID]
		return result != nil
	}, 10*time.Second)

	// 验证任务执行结果
	require.True(t, success, "Job execution did not complete within timeout")
	assert.NotNil(t, result, "Execution result should not be nil")
	assert.Equal(t, executor.ExecutionStateSuccess, result.State, "Job should have executed successfully")
	assert.Equal(t, 0, result.ExitCode, "Exit code should be 0")
	assert.Contains(t, result.Output, "hello world", "Output should contain 'hello world'")
}

// TestWorkerJobManagerAndSchedulerIntegration 测试 WorkerJobManager 和调度器的集成
func TestWorkerJobManagerAndSchedulerIntegration(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建唯一的 worker ID
	workerID := testutils.GenerateUniqueID("worker")

	// 创建执行跟踪器
	executedJobs := make(map[string]bool)
	executionMutex := sync.Mutex{}

	// 创建 WorkerJobManager
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	err := jobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer jobManager.Stop()

	// 创建 LockManager
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	err = lockManager.Start()
	require.NoError(t, err, "Failed to start lock manager")
	defer lockManager.Stop()

	// 创建执行函数
	executeJobFunc := func(job *scheduler.ScheduledJob) error {
		executionMutex.Lock()
		defer executionMutex.Unlock()
		executedJobs[job.Job.ID] = true
		utils.Info("job executed", zap.String("job_id", job.Job.ID))
		return nil
	}

	// 创建调度器
	sched := scheduler.NewScheduler(
		jobManager,
		lockManager,
		executeJobFunc,
		scheduler.WithCheckInterval(500*time.Millisecond),
	)

	// 启动调度器
	err = sched.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer sched.Stop()

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 创建两个不同类型的任务
	simpleJob := jobFactory.CreateSimpleJob()
	simpleJob.Status = constants.JobStatusPending
	simpleJob.Enabled = true

	scheduledJob := jobFactory.CreateScheduledJob("@every 2s")
	scheduledJob.Status = constants.JobStatusPending
	scheduledJob.Enabled = true

	// 将任务分配给当前 Worker
	for _, job := range []*models.Job{simpleJob, scheduledJob} {
		if job.Env == nil {
			job.Env = make(map[string]string)
		}
		job.Env["WORKER_ID"] = workerID

		// 转换为 JSON 并保存到 etcd
		jobJSON, err := job.ToJSON()
		require.NoError(t, err, "Failed to convert job to JSON")

		// 将任务保存到 etcd
		jobKey := constants.JobPrefix + job.ID
		err = etcdClient.Put(jobKey, jobJSON)
		require.NoError(t, err, "Failed to put job in etcd")
	}

	// 等待 JobManager 检测到任务
	time.Sleep(2 * time.Second)

	// 立即触发简单任务执行
	err = sched.TriggerJob(simpleJob.ID)
	require.NoError(t, err, "Failed to trigger simple job")

	// 等待简单任务和定时任务都执行
	time.Sleep(3 * time.Second)

	// 验证任务执行
	executionMutex.Lock()
	assert.True(t, executedJobs[simpleJob.ID], "Simple job should have been executed")
	assert.True(t, executedJobs[scheduledJob.ID], "Scheduled job should have been executed at least once")
	executionMutex.Unlock()

	// 验证调度器状态
	status := sched.GetStatus()
	assert.True(t, status.IsRunning, "Scheduler should be running")
}

// TestExecutorAndSchedulerIntegration 测试执行器与调度器的集成
func TestExecutorAndSchedulerIntegration(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建唯一的worker ID
	workerID := testutils.GenerateUniqueID("worker")

	// 创建执行结果追踪器
	executionResults := make(map[string]*executor.ExecutionResult)
	executionLogs := make(map[string]string)
	executionMutex := sync.Mutex{}

	// 创建执行报告器
	mockReporter := &testIntegrationExecutionReporter{
		executionResults: executionResults,
		executionLogs:    executionLogs,
		mutex:            &executionMutex,
	}

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(5),
		executor.WithDefaultTimeout(30*time.Second),
	)

	// 创建WorkerJobManager
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	err := jobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer jobManager.Stop()

	// 创建LockManager
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	err = lockManager.Start()
	require.NoError(t, err, "Failed to start lock manager")
	defer lockManager.Stop()

	// 创建执行函数
	executeJobFunc := func(job *scheduler.ScheduledJob) error {
		utils.Info("executing job with executor", zap.String("job_id", job.Job.ID))

		// 创建执行上下文
		executionID := "exec-" + job.Job.ID
		execCtx := &executor.ExecutionContext{
			ExecutionID:   executionID,
			Job:           job.Job,
			Command:       job.Job.Command,
			Args:          job.Job.Args,
			WorkDir:       job.Job.WorkDir,
			Environment:   job.Job.Env,
			Timeout:       time.Duration(job.Job.Timeout) * time.Second,
			Reporter:      mockReporter,
			MaxOutputSize: 10 * 1024 * 1024, // 10MB
		}

		if execCtx.Environment == nil {
			execCtx.Environment = make(map[string]string)
		}
		execCtx.Environment["EXECUTOR_WORKER_ID"] = workerID

		// 使用 executor 执行命令
		result, err := exec.Execute(context.Background(), execCtx)
		if err != nil {
			utils.Error("job execution failed", zap.String("job_id", job.Job.ID), zap.Error(err))
			return err
		}

		utils.Info("job executed successfully", zap.String("job_id", job.Job.ID), zap.Int("exit_code", result.ExitCode))

		// 保存执行结果
		mockReporter.ReportCompletion(executionID, result)
		executionMutex.Lock()
		executionLogs[executionID] = "Process started with PID " + result.ExecutionID + "\n" + result.Output
		executionMutex.Unlock()

		return nil
	}

	// 创建调度器
	sched := scheduler.NewScheduler(
		jobManager,
		lockManager,
		executeJobFunc,
		scheduler.WithCheckInterval(1*time.Second),
		scheduler.WithMaxConcurrent(3),
	)

	// 启动调度器
	err = sched.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer sched.Stop()

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 创建一个命令任务 - 使用适合当前操作系统的命令，正确分割命令和参数
	var job *models.Job
	if runtime.GOOS == "windows" {
		// 在Windows上，正确分离命令和参数
		job = jobFactory.CreateJobWithCommand("cmd.exe")
		job.Args = []string{"/c", "echo test integration"}
	} else {
		// 在Unix/Linux上
		job = jobFactory.CreateJobWithCommand("echo")
		job.Args = []string{"test integration"}
	}

	// 添加 Worker ID 到任务环境变量，将其分配给当前 Worker
	if job.Env == nil {
		job.Env = make(map[string]string)
	}
	job.Env["WORKER_ID"] = workerID

	// 添加任务到调度器
	err = sched.AddJob(job)
	require.NoError(t, err, "Failed to add job to scheduler")

	// 立即触发任务执行
	err = sched.TriggerJob(job.ID)
	require.NoError(t, err, "Failed to trigger job")

	// 等待任务执行完成
	var result *executor.ExecutionResult
	executionID := "exec-" + job.ID

	// 等待最多10秒钟
	jobExecuted := false
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		executionMutex.Lock()
		if r, exists := executionResults[executionID]; exists {
			result = r
			jobExecuted = true
			executionMutex.Unlock()
			break
		}
		executionMutex.Unlock()
	}

	// 验证任务执行结果
	assert.True(t, jobExecuted, "Job should have been executed within timeout")
	assert.True(t, result != nil, "Execution result should exist")
	if result != nil {
		assert.Equal(t, 0, result.ExitCode, "Job should exit with code 0")
		assert.Equal(t, executor.ExecutionStateSuccess, result.State, "Job should succeed")
		assert.Contains(t, executionLogs[executionID], "test integration", "Output should contain expected text")
	}
}
