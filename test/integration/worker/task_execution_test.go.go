package worker

import (
	"context"
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

	// 创建执行通道和执行跟踪器
	executedJobs := make(map[string]bool)
	executionResults := make(map[string]*executor.ExecutionResult)
	executionMutex := sync.Mutex{}

	// 创建执行报告器
	mockReporter := &testExecutionReporter{
		executionResults: executionResults,
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
		executionMutex.Lock()
		defer executionMutex.Unlock()

		utils.Info("executing job", zap.String("job_id", job.Job.ID))

		// 创建执行上下文
		execCtx := &executor.ExecutionContext{
			ExecutionID:   "exec-" + job.Job.ID,
			Job:           job.Job,
			Command:       job.Job.Command,
			Args:          job.Job.Args,
			WorkDir:       job.Job.WorkDir,
			Environment:   job.Job.Env,
			Timeout:       time.Duration(job.Job.Timeout) * time.Second,
			Reporter:      mockReporter,
			MaxOutputSize: 10 * 1024 * 1024, // 10MB
		}

		// 执行命令
		result, err := exec.Execute(context.Background(), execCtx)
		if err != nil {
			utils.Error("job execution failed", zap.String("job_id", job.Job.ID), zap.Error(err))
			return err
		}

		executedJobs[job.Job.ID] = true
		executionResults[job.Job.ID] = result

		utils.Info("job executed successfully",
			zap.String("job_id", job.Job.ID),
			zap.Int("exit_code", result.ExitCode))

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

	// 创建一个简单任务
	simpleJob := jobFactory.CreateSimpleJob()
	simpleJob.Status = constants.JobStatusPending
	simpleJob.Enabled = true

	// 添加 Worker ID 到任务环境变量，将其分配给当前 Worker
	if simpleJob.Env == nil {
		simpleJob.Env = make(map[string]string)
	}
	simpleJob.Env["WORKER_ID"] = workerID

	// 转换为 JSON 并保存到 etcd
	jobJSON, err := simpleJob.ToJSON()
	require.NoError(t, err, "Failed to convert job to JSON")

	// 将任务保存到 etcd
	jobKey := constants.JobPrefix + simpleJob.ID
	err = etcdClient.Put(jobKey, jobJSON)
	require.NoError(t, err, "Failed to put job in etcd")

	// 等待 JobManager 检测到任务
	time.Sleep(2 * time.Second)

	// 立即触发任务执行
	err = sched.TriggerJob(simpleJob.ID)
	require.NoError(t, err, "Failed to trigger job")

	// 等待任务执行完成
	waitForExecution := func() bool {
		executionMutex.Lock()
		defer executionMutex.Unlock()
		return executedJobs[simpleJob.ID]
	}

	// 等待最多10秒钟
	success := testutils.WaitForCondition(waitForExecution, 10*time.Second)
	assert.True(t, success, "Job should have been executed within timeout")

	// 验证任务执行结果
	executionMutex.Lock()
	result, exists := executionResults[simpleJob.ID]
	executionMutex.Unlock()

	assert.True(t, exists, "Execution result should exist")
	if exists {
		assert.Equal(t, 0, result.ExitCode, "Exit code should be 0")
		assert.Equal(t, executor.ExecutionStateSuccess, result.State, "Execution state should be success")
		assert.Contains(t, result.Output, "hello world", "Output should contain expected text")
	}
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

	// 创建唯一的 worker ID
	workerID := testutils.GenerateUniqueID("worker")

	// 创建执行结果追踪
	executionResults := make(map[string]*executor.ExecutionResult)
	executionMutex := sync.Mutex{}

	// 创建执行报告器
	mockReporter := &testExecutionReporter{
		executionResults: executionResults,
		mutex:            &executionMutex,
	}

	// 创建执行器
	exec := executor.NewExecutor(
		mockReporter,
		executor.WithMaxConcurrentExecutions(5),
		executor.WithDefaultTimeout(30*time.Second),
	)

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
		utils.Info("executing job with executor", zap.String("job_id", job.Job.ID))

		// 创建执行上下文
		execCtx := &executor.ExecutionContext{
			ExecutionID:   "exec-" + job.Job.ID,
			Job:           job.Job,
			Command:       job.Job.Command,
			Args:          job.Job.Args,
			WorkDir:       job.Job.WorkDir,
			Environment:   job.Job.Env,
			Timeout:       time.Duration(job.Job.Timeout) * time.Second,
			Reporter:      mockReporter,
			MaxOutputSize: 10 * 1024 * 1024, // 10MB
		}

		// 执行命令
		result, err := exec.Execute(context.Background(), execCtx)
		if err != nil {
			utils.Error("job execution failed", zap.String("job_id", job.Job.ID), zap.Error(err))
			return err
		}

		executionMutex.Lock()
		executionResults[job.Job.ID] = result
		executionMutex.Unlock()

		utils.Info("job executed successfully",
			zap.String("job_id", job.Job.ID),
			zap.Int("exit_code", result.ExitCode))

		return nil
	}

	// 创建调度器
	sched := scheduler.NewScheduler(
		jobManager,
		lockManager,
		executeJobFunc,
		scheduler.WithCheckInterval(1*time.Second),
	)

	// 启动调度器
	err = sched.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer sched.Stop()

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 创建一个命令任务
	commandJob := jobFactory.CreateJobWithCommand("echo test integration")
	commandJob.Status = constants.JobStatusPending
	commandJob.Enabled = true

	// 添加 Worker ID 到任务环境变量
	if commandJob.Env == nil {
		commandJob.Env = make(map[string]string)
	}
	commandJob.Env["WORKER_ID"] = workerID

	// 添加任务到调度器
	err = sched.AddJob(commandJob)
	require.NoError(t, err, "Failed to add job to scheduler")

	// 立即触发任务执行
	err = sched.TriggerJob(commandJob.ID)
	require.NoError(t, err, "Failed to trigger job")

	// 等待任务执行完成
	waitForExecution := func() bool {
		executionMutex.Lock()
		defer executionMutex.Unlock()
		_, exists := executionResults[commandJob.ID]
		return exists
	}

	// 等待最多10秒钟
	success := testutils.WaitForCondition(waitForExecution, 10*time.Second)
	assert.True(t, success, "Job should have been executed within timeout")

	// 验证任务执行结果
	executionMutex.Lock()
	result, exists := executionResults[commandJob.ID]
	executionMutex.Unlock()

	assert.True(t, exists, "Execution result should exist")
	if exists {
		assert.Equal(t, 0, result.ExitCode, "Exit code should be 0")
		assert.Equal(t, executor.ExecutionStateSuccess, result.State, "Execution state should be success")
		assert.Contains(t, result.Output, "test integration", "Output should contain expected text")
	}
}

// TestNetworkOrConnectionIssues 测试网络或连接问题
func TestNetworkOrConnectionIssues(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建唯一的 worker ID
	workerID := testutils.GenerateUniqueID("worker")

	// 创建执行通道和执行跟踪器
	jobEvents := make(map[string]jobmgr.JobEventType)
	eventMutex := sync.Mutex{}

	// 创建任务事件处理器
	eventHandler := &testTaskJobEventHandler{
		events:     jobEvents,
		eventMutex: &eventMutex,
	}

	// 创建 WorkerJobManager
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	err := jobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer jobManager.Stop()

	// 注册事件处理器
	jobManager.RegisterHandler(eventHandler)

	// 创建 LockManager
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	err = lockManager.Start()
	require.NoError(t, err, "Failed to start lock manager")
	defer lockManager.Stop()

	// 创建执行函数
	var jobsExecuted int
	jobsExecutedMutex := sync.Mutex{}

	executeJobFunc := func(job *scheduler.ScheduledJob) error {
		jobsExecutedMutex.Lock()
		jobsExecuted++
		jobsExecutedMutex.Unlock()

		utils.Info("job executed", zap.String("job_id", job.Job.ID))
		return nil
	}

	// 创建调度器
	sched := scheduler.NewScheduler(
		jobManager,
		lockManager,
		executeJobFunc,
		scheduler.WithCheckInterval(500*time.Millisecond),
		scheduler.WithRetryStrategy(3, 1*time.Second),
	)

	// 启动调度器
	err = sched.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer sched.Stop()

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 创建一个简单任务
	job := jobFactory.CreateSimpleJob()
	job.Status = constants.JobStatusPending
	job.Enabled = true

	// 添加 Worker ID 到任务环境变量
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

	// 等待 JobManager 检测到任务
	time.Sleep(2 * time.Second)

	// 验证任务已被添加到调度器
	eventMutex.Lock()
	_, eventReceived := jobEvents[job.ID]
	eventMutex.Unlock()
	assert.True(t, eventReceived, "Job event should have been received")

	// 模拟网络断开：关闭 etcd 客户端并重新创建一个
	err = etcdClient.Close()
	require.NoError(t, err, "Failed to close etcd client")

	// 立即触发任务执行，应该可以成功因为任务已在本地添加
	err = sched.TriggerJob(job.ID)
	require.NoError(t, err, "Failed to trigger job")

	// 等待任务执行
	time.Sleep(2 * time.Second)

	// 验证任务已执行
	jobsExecutedMutex.Lock()
	executed := jobsExecuted > 0
	jobsExecutedMutex.Unlock()
	assert.True(t, executed, "Job should have been executed despite network issues")

	// 重新创建 etcd 连接
	newEtcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create new etcd client")
	defer newEtcdClient.Close()

	// 更新任务状态（模拟重新连接后的操作）
	job.Status = constants.JobStatusSucceeded
	jobJSON, err = job.ToJSON()
	require.NoError(t, err, "Failed to convert job to JSON")

	err = newEtcdClient.Put(jobKey, jobJSON)
	require.NoError(t, err, "Failed to update job in etcd")

	// 等待 etcd 变更传播
	time.Sleep(2 * time.Second)
}

// testExecutionReporter 是一个用于测试的执行报告器
type testExecutionReporter struct {
	executionResults map[string]*executor.ExecutionResult
	mutex            *sync.Mutex
}

func (r *testExecutionReporter) ReportStart(executionID string, pid int) error {
	return nil
}

func (r *testExecutionReporter) ReportOutput(executionID string, output string) error {
	return nil
}

func (r *testExecutionReporter) ReportCompletion(executionID string, result *executor.ExecutionResult) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.executionResults[executionID] = result
	return nil
}

func (r *testExecutionReporter) ReportError(executionID string, err error) error {
	return nil
}

func (r *testExecutionReporter) ReportProgress(executionID string, status *executor.ExecutionStatus) error {
	return nil
}

// testTaskJobEventHandler 是一个用于测试的任务事件处理器
type testTaskJobEventHandler struct {
	events     map[string]jobmgr.JobEventType
	eventMutex *sync.Mutex
}

func (h *testTaskJobEventHandler) HandleJobEvent(event *jobmgr.JobEvent) {
	h.eventMutex.Lock()
	defer h.eventMutex.Unlock()
	h.events[event.Job.ID] = event.Type
}
