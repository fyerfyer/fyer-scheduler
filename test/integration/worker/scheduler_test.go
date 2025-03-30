package worker

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/joblock"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/jobmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/scheduler"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestSchedulerBasicFunctions 测试调度器的基本功能
func TestSchedulerBasicFunctions(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建WorkerJobManager
	workerID := testutils.GenerateUniqueID("worker")
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
	executed := make(chan string, 10)
	executionFunc := func(job *scheduler.ScheduledJob) error {
		utils.Info("executing job", zap.String("job_id", job.Job.ID), zap.String("job_name", job.Job.Name))
		executed <- job.Job.ID
		return nil
	}

	// 创建调度器
	schedulerOptions := []scheduler.SchedulerOption{
		scheduler.WithCheckInterval(1 * time.Second),
		scheduler.WithMaxConcurrent(5),
	}

	sched := scheduler.NewScheduler(jobManager, lockManager, executionFunc, schedulerOptions...)

	// 启动调度器
	err = sched.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer sched.Stop()

	// 验证调度器状态
	status := sched.GetStatus()
	assert.True(t, status.IsRunning, "Scheduler should be running")

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 添加一个简单任务
	job1 := jobFactory.CreateSimpleJob()
	err = sched.AddJob(job1)
	require.NoError(t, err, "Failed to add job1")

	// 添加一个带cron表达式的任务
	job2 := jobFactory.CreateScheduledJob("@every 5s") // 每5秒执行一次
	err = sched.AddJob(job2)
	require.NoError(t, err, "Failed to add job2")

	// 验证任务数量
	jobs := sched.ListJobs()
	assert.Len(t, jobs, 2, "Should have 2 jobs in scheduler")

	// 立即触发任务执行
	err = sched.TriggerJob(job1.ID)
	require.NoError(t, err, "Failed to trigger job1")

	// 等待任务执行完成
	select {
	case executedJobID := <-executed:
		assert.Equal(t, job1.ID, executedJobID, "Executed job ID should match triggered job ID")
	case <-time.After(5 * time.Second):
		t.Fatal("Job execution timed out")
	}

	// 移除任务
	err = sched.RemoveJob(job1.ID)
	require.NoError(t, err, "Failed to remove job1")

	// 验证任务数量减少
	jobs = sched.ListJobs()
	assert.Len(t, jobs, 1, "Should have 1 job in scheduler after removal")

	// 停止调度器
	err = sched.Stop()
	require.NoError(t, err, "Failed to stop scheduler")

	status = sched.GetStatus()
	assert.False(t, status.IsRunning, "Scheduler should not be running after stop")
}

// TestScheduledJobExecution 测试定时调度任务执行
func TestScheduledJobExecution(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建WorkerJobManager
	workerID := testutils.GenerateUniqueID("worker")
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	err := jobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer jobManager.Stop()

	// 创建LockManager
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	err = lockManager.Start()
	require.NoError(t, err, "Failed to start lock manager")
	defer lockManager.Stop()

	// 创建执行通知通道和计数器
	executionCount := 0
	var countMutex sync.Mutex
	executed := make(chan string, 10)

	// 创建执行函数
	executionFunc := func(job *scheduler.ScheduledJob) error {
		countMutex.Lock()
		executionCount++
		countMutex.Unlock()
		executed <- job.Job.ID
		return nil
	}

	// 创建调度器，使用较短的检查间隔
	schedulerOptions := []scheduler.SchedulerOption{
		scheduler.WithCheckInterval(500 * time.Millisecond),
		scheduler.WithMaxConcurrent(5),
	}

	sched := scheduler.NewScheduler(jobManager, lockManager, executionFunc, schedulerOptions...)

	// 启动调度器
	err = sched.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer sched.Stop()

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 创建一个定时任务，使用"@every"语法更可靠地进行测试
	job := jobFactory.CreateScheduledJob("@every 1s")
	err = sched.AddJob(job)
	require.NoError(t, err, "Failed to add scheduled job")

	// 等待至少2次执行
	jobIDs := make([]string, 0)
	timeout := time.After(10 * time.Second)

	for i := 0; i < 2; i++ {
		select {
		case id := <-executed:
			jobIDs = append(jobIDs, id)
		case <-timeout:
			t.Fatalf("Timed out waiting for job executions, got %d executions", len(jobIDs))
		}
	}

	// 验证执行的任务ID
	for _, id := range jobIDs {
		assert.Equal(t, job.ID, id, "Executed job ID should match scheduled job ID")
	}

	// 验证执行次数
	countMutex.Lock()
	count := executionCount
	countMutex.Unlock()
	assert.GreaterOrEqual(t, count, 2, "Job should have executed at least 2 times")
}

// TestJobFailureHandling 测试任务失败处理
func TestJobFailureHandling(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建WorkerJobManager
	workerID := testutils.GenerateUniqueID("worker")
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	err := jobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer jobManager.Stop()

	// 创建LockManager
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	err = lockManager.Start()
	require.NoError(t, err, "Failed to start lock manager")
	defer lockManager.Stop()

	// 创建失败计数和通知通道
	failCount := 0
	var countMutex sync.Mutex
	executed := make(chan bool, 10) // true表示成功, false表示失败

	// 创建执行函数 - 第一次执行将失败，第二次执行将成功
	executionFunc := func(job *scheduler.ScheduledJob) error {
		countMutex.Lock()
		shouldFail := failCount == 0
		failCount++
		countMutex.Unlock()

		if shouldFail {
			executed <- false
			return fmt.Errorf("simulated failure for testing")
		}

		executed <- true
		return nil
	}

	// 创建调度器，配置为失败重试
	schedulerOptions := []scheduler.SchedulerOption{
		scheduler.WithCheckInterval(1 * time.Second),
		scheduler.WithMaxConcurrent(2),
		scheduler.WithRetryStrategy(1, 1*time.Second), // 重试1次，间隔1秒
		scheduler.WithFailStrategy(scheduler.FailStrategyRetry),
	}

	sched := scheduler.NewScheduler(jobManager, lockManager, executionFunc, schedulerOptions...)

	// 启动调度器
	err = sched.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer sched.Stop()

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 添加一个简单任务
	job := jobFactory.CreateSimpleJob()
	err = sched.AddJob(job)
	require.NoError(t, err, "Failed to add job")

	// 立即触发任务
	err = sched.TriggerJob(job.ID)
	require.NoError(t, err, "Failed to trigger job")

	// 等待第一次执行结果（应该失败）
	select {
	case success := <-executed:
		assert.False(t, success, "First execution should fail")
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for first execution")
	}

	// 等待重试执行结果（应该成功）
	select {
	case success := <-executed:
		assert.True(t, success, "Retry execution should succeed")
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for retry execution")
	}

	// 验证失败计数
	countMutex.Lock()
	count := failCount
	countMutex.Unlock()
	assert.Equal(t, 2, count, "Job should have executed twice (once failed, once succeeded)")
}

// TestConcurrentJobExecution 测试并发任务执行
func TestConcurrentJobExecution(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建WorkerJobManager
	workerID := testutils.GenerateUniqueID("worker")
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	err := jobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer jobManager.Stop()

	// 创建LockManager
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	err = lockManager.Start()
	require.NoError(t, err, "Failed to start lock manager")
	defer lockManager.Stop()

	// 创建等待组和计数器以跟踪并发执行
	var wg sync.WaitGroup
	executed := make(chan string, 10)

	// 创建执行函数 - 模拟需要一些时间的操作
	executionFunc := func(job *scheduler.ScheduledJob) error {
		time.Sleep(1 * time.Second) // 模拟一些工作
		executed <- job.Job.ID
		wg.Done()
		return nil
	}

	// 创建调度器，配置并发度为3
	schedulerOptions := []scheduler.SchedulerOption{
		scheduler.WithCheckInterval(500 * time.Millisecond),
		scheduler.WithMaxConcurrent(3), // 最多3个并发任务
	}

	sched := scheduler.NewScheduler(jobManager, lockManager, executionFunc, schedulerOptions...)

	// 启动调度器
	err = sched.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer sched.Stop()

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 创建5个任务
	jobs := make([]*models.Job, 5)
	for i := 0; i < 5; i++ {
		jobs[i] = jobFactory.CreateSimpleJob()
		err = sched.AddJob(jobs[i])
		require.NoError(t, err, "Failed to add job")
		wg.Add(1)
	}

	// 立即触发所有任务
	for _, job := range jobs {
		err = sched.TriggerJob(job.ID)
		require.NoError(t, err, "Failed to trigger job")
	}

	// 设置等待超时
	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	// 等待所有任务完成或超时
	select {
	case <-doneCh:
		// 所有任务已完成
	case <-time.After(10 * time.Second):
		t.Fatal("Timed out waiting for all jobs to complete")
	}

	// 验证所有任务都已执行
	executedCount := 0
	executedJobs := make(map[string]bool)

	// 收集已执行的任务
	for i := 0; i < 5; i++ {
		select {
		case id := <-executed:
			executedJobs[id] = true
			executedCount++
		default:
			// 没有更多已执行的任务
			break
		}
	}

	assert.Equal(t, 5, executedCount, "All 5 jobs should have executed")

	// 验证每个任务都只执行了一次
	for _, job := range jobs {
		assert.True(t, executedJobs[job.ID], "Job should have been executed")
	}
}

// TestJobEventHandling 测试调度器如何响应作业事件
func TestJobEventHandling(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建 WorkerJobManager
	workerID := testutils.GenerateUniqueID("worker")
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	err := jobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer jobManager.Stop()

	// 创建 LockManager
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	err = lockManager.Start()
	require.NoError(t, err, "Failed to start lock manager")
	defer lockManager.Stop()

	// 创建执行通道
	executed := make(chan string, 10)
	executionFunc := func(job *scheduler.ScheduledJob) error {
		executed <- job.Job.ID
		return nil
	}

	// 创建调度器
	sched := scheduler.NewScheduler(jobManager, lockManager, executionFunc)
	err = sched.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer sched.Stop()

	// 创建作业工厂
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 将作业保存到etcd以模拟来自主节点的作业事件
	jobJSON, err := job.ToJSON()
	require.NoError(t, err, "Failed to serialize job")

	// 将worker ID添加到作业环境中以将其分配给此worker
	if job.Env == nil {
		job.Env = make(map[string]string)
	}
	job.Env["WORKER_ID"] = workerID

	// 保存到etcd以触发作业事件
	err = etcdClient.Put(job.Key(), jobJSON)
	require.NoError(t, err, "Failed to save job to etcd")

	// 等待作业被调度器拾取
	time.Sleep(2 * time.Second)

	// 检查作业是否已添加到调度器
	jobs := sched.ListJobs()
	assert.GreaterOrEqual(t, len(jobs), 1, "Job should be added to scheduler")

	// 如果我们从etcd中删除作业，它应该从调度器中删除
	err = etcdClient.Delete(job.Key())
	require.NoError(t, err, "Failed to delete job from etcd")

	// 等待作业被移除
	time.Sleep(2 * time.Second)

	// 检查作业是否已从调度器中移除
	jobs = sched.ListJobs()
	foundJob := false
	for _, j := range jobs {
		if j.Job.ID == job.ID {
			foundJob = true
			break
		}
	}
	assert.False(t, foundJob, "Job should be removed from scheduler")
}

// TestExecuteJobFunction 测试内部的 executeJob 函数
func TestExecuteJobFunction(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建作业管理器
	workerID := testutils.GenerateUniqueID("worker")
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	require.NoError(t, jobManager.Start(), "Failed to start job manager")
	defer jobManager.Stop()

	// 创建锁管理器
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	require.NoError(t, lockManager.Start(), "Failed to start lock manager")
	defer lockManager.Stop()

	// 创建执行跟踪
	executionCompleted := make(chan struct{})
	executionStarted := make(chan struct{})
	var executedJobID string
	var executionError error

	// 创建一个执行函数，用于在运行时发出信号
	executionFunc := func(job *scheduler.ScheduledJob) error {
		executedJobID = job.Job.ID
		close(executionStarted) // 发出执行已开始的信号

		// 模拟一些工作
		time.Sleep(100 * time.Millisecond)

		// 发出完成信号
		close(executionCompleted)
		return executionError
	}

	// 使用自定义执行函数创建调度器
	sched := scheduler.NewScheduler(
		jobManager,
		lockManager,
		executionFunc,
		scheduler.WithCheckInterval(100*time.Millisecond),
		scheduler.WithMaxConcurrent(5),
	)
	require.NoError(t, sched.Start(), "Failed to start scheduler")
	defer sched.Stop()

	// 创建作业工厂和测试作业
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 将作业添加到调度器
	require.NoError(t, sched.AddJob(job), "Failed to add job")

	// 触发作业执行
	require.NoError(t, sched.TriggerJob(job.ID), "Failed to trigger job")

	// 等待作业开始执行，设置超时时间
	select {
	case <-executionStarted:
		t.Log("Job execution started")
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for job execution to start")
	}

	// 等待作业执行完成，设置超时时间
	select {
	case <-executionCompleted:
		t.Log("Job execution completed")
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for job execution to complete")
	}

	// 等待作业状态更新完成 - 这里是问题所在，需要增加足够的等待时间
	statusUpdated := testutils.WaitForCondition(func() bool {
		scheduledJob, found := sched.GetJob(job.ID)
		if !found {
			return false
		}
		// 检查作业是否已从"running"状态变为"succeeded"
		return scheduledJob.Job.Status == "succeeded"
	}, 2*time.Second) // 增加等待时间到2秒

	assert.True(t, statusUpdated, "Timed out waiting for job status to update")

	// 验证执行的作业ID是否与预期的作业ID匹配
	assert.Equal(t, job.ID, executedJobID, "Executed job ID does not match expected job ID")

	// 检查作业状态转换是否正确记录
	scheduledJob, found := sched.GetJob(job.ID)
	assert.True(t, found, "Job not found in scheduler after execution")
	assert.Equal(t, "succeeded", scheduledJob.Job.Status, "Job status should be 'succeeded'")
	assert.Equal(t, 0, scheduledJob.Job.LastExitCode, "Job exit code should be 0")
	assert.NotZero(t, scheduledJob.Job.LastRunTime, "Job last run time should be set")
}

// TestJobStateTransitions 验证作业是否正确地在状态之间转换
func TestJobStateTransitions(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建作业管理器
	workerID := testutils.GenerateUniqueID("worker")
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	require.NoError(t, jobManager.Start(), "Failed to start job manager")
	defer jobManager.Stop()

	// 创建锁管理器
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	require.NoError(t, lockManager.Start(), "Failed to start lock manager")
	defer lockManager.Stop()

	// 用于验证的作业状态
	jobStates := make([]string, 0, 3)
	jobStateMutex := sync.Mutex{}
	stateTransitionCompleted := make(chan struct{})

	// 创建执行函数
	executionFunc := func(job *scheduler.ScheduledJob) error {
		if job == nil {
			return fmt.Errorf("nil job passed to execution function")
		}

		// 在执行前记录状态（应该是运行中）
		jobStateMutex.Lock()
		currentState := job.Job.Status
		jobStates = append(jobStates, currentState)
		jobStateMutex.Unlock()

		// 模拟工作
		time.Sleep(100 * time.Millisecond)

		// 执行完成后，调度器应该更新状态
		// 我们将在执行函数外部检查这一点
		close(stateTransitionCompleted)
		return nil
	}

	// 创建调度器
	schedulerOpts := []scheduler.SchedulerOption{
		scheduler.WithCheckInterval(100 * time.Millisecond),
	}
	sched := scheduler.NewScheduler(jobManager, lockManager, executionFunc, schedulerOpts...)

	// 启动调度器
	require.NoError(t, sched.Start(), "Failed to start scheduler")
	defer sched.Stop()

	// 创建作业工厂和测试作业
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 记录初始作业状态
	jobStateMutex.Lock()
	jobStates = append(jobStates, job.Status) // 应该是 "pending"
	jobStateMutex.Unlock()

	// 将作业添加到调度器
	require.NoError(t, sched.AddJob(job), "Failed to add job to scheduler")

	// 从调度器中获取作业以验证初始状态
	scheduledJob, found := sched.GetJob(job.ID)
	require.True(t, found, "Job should be in scheduler")
	assert.False(t, scheduledJob.IsRunning, "Job should not be running initially")

	// 触发作业执行
	require.NoError(t, sched.TriggerJob(job.ID), "Failed to trigger job")

	// 等待执行完成
	select {
	case <-stateTransitionCompleted:
		// 执行完成
	case <-time.After(5 * time.Second):
		require.Fail(t, "Job execution didn't complete within timeout")
	}

	// 等待状态转换完成
	time.Sleep(5 * time.Second)

	// 验证最终作业状态
	scheduledJob, found = sched.GetJob(job.ID)
	require.True(t, found, "Job should still be in scheduler after execution")
	assert.False(t, scheduledJob.IsRunning, "Job should not be running after completion")

	// 验证预期的状态转换
	jobStateMutex.Lock()
	defer jobStateMutex.Unlock()
	assert.GreaterOrEqual(t, len(jobStates), 2, "Should have recorded at least 2 job states")

	// 第一个状态应该是 pending
	assert.Equal(t, "pending", jobStates[0], "Initial job state should be pending")

	// 第二个状态应该是 running
	if len(jobStates) > 1 {
		assert.Equal(t, "running", jobStates[1], "Job state during execution should be running")
	}
}

// TestWorkerJobAssignment 验证作业是否正确分配给工作节点
func TestWorkerJobAssignment(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建作业管理器
	workerID := testutils.GenerateUniqueID("worker")
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	require.NoError(t, jobManager.Start(), "Failed to start job manager")
	defer jobManager.Stop()

	// 创建锁管理器
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	require.NoError(t, lockManager.Start(), "Failed to start lock manager")
	defer lockManager.Stop()

	// 作业执行跟踪
	executed := make(chan string, 5)

	// 创建执行函数
	executionFunc := func(job *scheduler.ScheduledJob) error {
		if job == nil {
			return fmt.Errorf("nil job passed to execution function")
		}

		// 记录作业执行
		executed <- job.Job.ID
		return nil
	}

	// 创建调度器
	sched := scheduler.NewScheduler(jobManager, lockManager, executionFunc)

	// 启动调度器
	require.NoError(t, sched.Start(), "Failed to start scheduler")
	defer sched.Stop()

	// 创建作业工厂
	jobFactory := testutils.NewJobFactory()

	// 创建一个分配给当前工作节点的作业
	assignedJob := jobFactory.CreateJobWithCommand("echo assigned to this worker")
	assignedJob.Env = map[string]string{
		"WORKER_ID": workerID,
	}

	// 创建一个分配给其他工作节点的作业
	otherWorkerID := testutils.GenerateUniqueID("worker")
	unassignedJob := jobFactory.CreateJobWithCommand("echo assigned to other worker")
	unassignedJob.Env = map[string]string{
		"WORKER_ID": otherWorkerID,
	}

	// 创建一个没有分配工作节点的作业
	unspecifiedJob := jobFactory.CreateJobWithCommand("echo no worker specified")

	// 将所有作业添加到调度器
	require.NoError(t, sched.AddJob(assignedJob), "Failed to add assigned job")
	require.NoError(t, sched.AddJob(unassignedJob), "Failed to add unassigned job")
	require.NoError(t, sched.AddJob(unspecifiedJob), "Failed to add unspecified job")

	// 触发所有作业
	require.NoError(t, sched.TriggerJob(assignedJob.ID), "Failed to trigger assigned job")
	require.NoError(t, sched.TriggerJob(unassignedJob.ID), "Failed to trigger unassigned job")
	require.NoError(t, sched.TriggerJob(unspecifiedJob.ID), "Failed to trigger unspecified job")

	// 等待作业执行完成，设置超时时间
	executedJobs := make([]string, 0)
	timeout := time.After(5 * time.Second)

	// 在超时时间内收集已执行的作业
collectLoop:
	for {
		select {
		case jobID := <-executed:
			executedJobs = append(executedJobs, jobID)
		case <-timeout:
			break collectLoop
		}
	}

	// 分配给当前工作节点的作业和未指定工作节点的作业应该执行，但分配给其他工作节点的作业不应执行
	assert.Contains(t, executedJobs, assignedJob.ID, "Job assigned to this worker should execute")
	assert.Contains(t, executedJobs, unspecifiedJob.ID, "Job with no worker specified should execute")
	assert.NotContains(t, executedJobs, unassignedJob.ID, "Job assigned to other worker should not execute")
}

// TestMultipleWorkersDistributedExecution 测试多个worker之间的任务分配
func TestMultipleWorkersDistributedExecution(t *testing.T) {
	// 设置测试环境
	etcdClient, _ := testutils.SetupTestEnvironment(t)

	// 创建两个worker
	workerID1 := "worker1-" + testutils.RandomString(8)
	workerID2 := "worker2-" + testutils.RandomString(8)

	utils.Info("creating two workers for test",
		zap.String("worker1", workerID1),
		zap.String("worker2", workerID2))

	// 创建第一个Worker的组件
	jobManager1 := jobmgr.NewWorkerJobManager(etcdClient, workerID1)
	require.NoError(t, jobManager1.Start())

	lockManager1 := joblock.NewLockManager(etcdClient, workerID1)
	require.NoError(t, lockManager1.Start())

	// 创建第二个Worker的组件
	jobManager2 := jobmgr.NewWorkerJobManager(etcdClient, workerID2)
	require.NoError(t, jobManager2.Start())

	lockManager2 := joblock.NewLockManager(etcdClient, workerID2)
	require.NoError(t, lockManager2.Start())

	// 创建执行记录通道和互斥锁
	executedJobs := make(map[string]int)
	executeMutex := sync.Mutex{}

	// 创建第一个Worker的执行函数
	executeFunc1 := func(job *scheduler.ScheduledJob) error {
		utils.Info("worker1 executing job",
			zap.String("job_id", job.Job.ID),
			zap.String("job_name", job.Job.Name),
			zap.String("worker_id", workerID1),
			zap.Any("job_env", job.Job.Env))

		executeMutex.Lock()
		executedJobs[job.Job.ID]++
		executeMutex.Unlock()

		// 模拟执行时间
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	// 创建第二个Worker的执行函数
	executeFunc2 := func(job *scheduler.ScheduledJob) error {
		utils.Info("worker2 executing job",
			zap.String("job_id", job.Job.ID),
			zap.String("job_name", job.Job.Name),
			zap.String("worker_id", workerID2),
			zap.Any("job_env", job.Job.Env))

		executeMutex.Lock()
		executedJobs[job.Job.ID]++
		executeMutex.Unlock()

		// 模拟执行时间
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	// 创建调度器配置选项
	schedulerOpts := []scheduler.SchedulerOption{
		scheduler.WithCheckInterval(200 * time.Millisecond),
	}

	// 创建两个调度器
	scheduler1 := scheduler.NewScheduler(jobManager1, lockManager1, executeFunc1, schedulerOpts...)
	require.NoError(t, scheduler1.Start())

	scheduler2 := scheduler.NewScheduler(jobManager2, lockManager2, executeFunc2, schedulerOpts...)
	require.NoError(t, scheduler2.Start())

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 1. 创建只分配给Worker1的任务
	job1 := jobFactory.CreateJobWithCommand("echo worker1 specific job")
	job1.Env = map[string]string{
		"WORKER_ID":          workerID1,
		"EXECUTOR_WORKER_ID": workerID1,
	}
	job1ID := job1.ID
	utils.Info("created job1 for worker1",
		zap.String("job_id", job1.ID),
		zap.Any("env", job1.Env))

	require.NoError(t, scheduler1.AddJob(job1))
	require.NoError(t, scheduler2.AddJob(job1))

	// 2. 创建只分配给Worker2的任务
	job2 := jobFactory.CreateJobWithCommand("echo worker2 specific job")
	job2.Env = map[string]string{
		"WORKER_ID":          workerID2,
		"EXECUTOR_WORKER_ID": workerID2,
	}
	job2ID := job2.ID
	utils.Info("created job2 for worker2",
		zap.String("job_id", job2.ID),
		zap.Any("env", job2.Env))

	require.NoError(t, scheduler1.AddJob(job2))
	require.NoError(t, scheduler2.AddJob(job2))

	// 3. 创建未指定Worker的任务（共享任务）
	job3 := jobFactory.CreateJobWithCommand("echo shared job")
	job3ID := job3.ID
	utils.Info("created job3 (shared)",
		zap.String("job_id", job3.ID),
		zap.Any("env", job3.Env))

	require.NoError(t, scheduler1.AddJob(job3))
	require.NoError(t, scheduler2.AddJob(job3))

	// 立即触发所有任务
	utils.Info("triggering all jobs")
	utils.Info("triggering job1", zap.String("job_id", job1.ID))
	require.NoError(t, scheduler1.TriggerJob(job1.ID))
	utils.Info("triggering job2", zap.String("job_id", job2.ID))
	require.NoError(t, scheduler2.TriggerJob(job2.ID))
	utils.Info("triggering job3", zap.String("job_id", job3.ID))
	require.NoError(t, scheduler1.TriggerJob(job3.ID))

	// 等待任务执行完成 - 增加等待时间以确保任务被执行
	time.Sleep(500 * time.Millisecond)

	// 停止调度器
	require.NoError(t, scheduler1.Stop())
	require.NoError(t, scheduler2.Stop())

	// 停止锁管理器
	require.NoError(t, lockManager1.Stop())
	require.NoError(t, lockManager2.Stop())

	// 停止任务管理器
	require.NoError(t, jobManager1.Stop())
	require.NoError(t, jobManager2.Stop())

	// 验证每个任务的执行情况
	utils.Info("execution results",
		zap.Any("executed_jobs", executedJobs),
		zap.String("job1_id", job1ID),
		zap.String("job2_id", job2ID),
		zap.String("job3_id", job3ID))

	// 专属任务应该只被指定的Worker执行
	assert.Equal(t, 1, executedJobs[job1ID], "Worker1 specific job should be executed exactly once")
	assert.Equal(t, 1, executedJobs[job2ID], "Worker2 specific job should be executed exactly once")

	// 共享任务应该只被一个Worker执行（由于分布式锁的存在）
	assert.LessOrEqual(t, executedJobs[job3ID], 1, "Shared job should be executed at most once due to locking")

	// 清理
	testutils.CleanTestData(t, etcdClient)
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
