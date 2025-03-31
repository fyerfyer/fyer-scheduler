package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/executor"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/joblock"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/jobmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/logsink"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/register"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/scheduler"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestFullWorkerIntegration 测试Worker所有组件的完整集成
func TestFullWorkerIntegration(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建一个唯一的Worker ID
	workerID := testutils.GenerateUniqueID("worker")

	// 创建一个通道来跟踪任务执行
	executedJobs := make(chan string, 10)

	// 1. 创建Worker注册组件
	workerRegisterOptions := register.RegisterOptions{
		NodeID:           workerID,
		Hostname:         "test-host",
		IP:               "127.0.0.1",
		HeartbeatTimeout: 30,
		HeartbeatTTL:     60,
		ResourceInterval: 5 * time.Second,
	}

	workerRegister, err := register.NewWorkerRegister(etcdClient, workerRegisterOptions)
	require.NoError(t, err, "Failed to create worker register")

	// 启动Worker注册服务
	err = workerRegister.Start()
	require.NoError(t, err, "Failed to start worker register")
	defer workerRegister.Stop()

	// 2. 创建资源收集器
	resourceCollector := register.NewResourceCollector(5 * time.Second)
	resourceCollector.Start()
	defer resourceCollector.Stop()

	// 3. 创建健康检查器
	healthChecker := register.NewHealthChecker(workerRegister, resourceCollector, 5*time.Second)
	err = healthChecker.Start()
	require.NoError(t, err, "Failed to start health checker")
	defer healthChecker.Stop()

	// 4. 创建任务管理器
	workerJobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	err = workerJobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer workerJobManager.Stop()

	// 5. 创建分布式锁管理器
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	err = lockManager.Start()
	require.NoError(t, err, "Failed to start lock manager")
	defer lockManager.Stop()

	// 6. 创建任务执行报告器
	testExecutionReporter := &testIntegrationExecutionReporter{
		executionResults: make(map[string]*executor.ExecutionResult),
		executionLogs:    make(map[string]string),
		mutex:            &sync.Mutex{},
	}

	// 7. 创建执行器
	exec := executor.NewExecutor(
		testExecutionReporter,
		executor.WithMaxConcurrentExecutions(5),
		executor.WithDefaultTimeout(30*time.Second),
	)

	// 8. 创建日志接收器选项
	sink, err := logsink.NewLogSink(
		logsink.WithBufferSize(100),
		logsink.WithBatchInterval(500*time.Millisecond),
		logsink.WithSendMode(logsink.SendModeBufferFull),
	)
	require.NoError(t, err, "Failed to create log sink")
	err = sink.Start()
	require.NoError(t, err, "Failed to start log sink")
	defer sink.Stop()

	// 9. 创建执行函数
	executeJobFunc := func(job *scheduler.ScheduledJob) error {
		executedJobs <- job.Job.ID

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
			Reporter:      testExecutionReporter,
			MaxOutputSize: 10 * 1024 * 1024, // 10MB
		}

		// 注册执行ID和任务的关联
		sink.RegisterExecutionJob(execCtx.ExecutionID, job.Job.ID)

		// 添加日志
		sink.AddLog(execCtx.ExecutionID, fmt.Sprintf("Starting execution of job %s", job.Job.ID))

		// 执行命令
		result, err := exec.Execute(context.Background(), execCtx)
		if err != nil {
			sink.AddLog(execCtx.ExecutionID, fmt.Sprintf("Execution error: %v", err))
			utils.Error("job execution failed", zap.String("job_id", job.Job.ID), zap.Error(err))
			return err
		}

		// 记录执行结果
		sink.AddLog(execCtx.ExecutionID, fmt.Sprintf("Execution completed with exit code %d", result.ExitCode))
		if result.ExitCode != 0 {
			sink.AddLog(execCtx.ExecutionID, fmt.Sprintf("Command output: %s", result.Output))
			return fmt.Errorf("command exited with non-zero code: %d", result.ExitCode)
		}

		utils.Info("job executed successfully",
			zap.String("job_id", job.Job.ID),
			zap.Int("exit_code", result.ExitCode))

		return nil
	}

	// 10. 创建调度器
	sched := scheduler.NewScheduler(
		workerJobManager,
		lockManager,
		executeJobFunc,
		scheduler.WithCheckInterval(1*time.Second),
		scheduler.WithMaxConcurrent(3),
		scheduler.WithRetryStrategy(2, 2*time.Second),
	)

	// 启动调度器
	err = sched.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer sched.Stop()

	// 等待所有服务启动
	time.Sleep(2 * time.Second)

	// 验证Worker状态
	workerStatus := workerRegister.GetStatus()
	assert.True(t, workerStatus.IsRegistered, "Worker should be registered")
	assert.Equal(t, constants.WorkerStatusOnline, workerStatus.State, "Worker should be online")

	// 验证健康状态
	healthStatus := healthChecker.GetHealthStatus()
	assert.NotNil(t, healthStatus, "Health status should not be nil")
	assert.True(t, healthChecker.IsHealthy(), "Worker should be healthy")

	// 验证调度器状态
	schedulerStatus := sched.GetStatus()
	assert.True(t, schedulerStatus.IsRunning, "Scheduler should be running")

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 创建测试任务
	job1 := jobFactory.CreateSimpleJob()
	job1.Status = constants.JobStatusPending
	job1.Enabled = true
	job1.Command = "echo"
	job1.Args = []string{"Hello from integration test"}

	// 添加Worker ID到任务环境，使其被分配给当前Worker
	if job1.Env == nil {
		job1.Env = make(map[string]string)
	}
	job1.Env["WORKER_ID"] = workerID

	// 转换任务为JSON
	jobJSON, err := job1.ToJSON()
	require.NoError(t, err, "Failed to convert job to JSON")

	// 将任务保存到etcd，模拟任务分配
	err = etcdClient.Put(constants.JobPrefix+job1.ID, jobJSON)
	require.NoError(t, err, "Failed to put job in etcd")

	// 等待任务被添加到调度器
	time.Sleep(2 * time.Second)

	// 验证任务已被添加
	scheduledJobs := sched.ListJobs()
	var foundJob bool
	for _, job := range scheduledJobs {
		if job.Job.ID == job1.ID {
			foundJob = true
			break
		}
	}
	assert.True(t, foundJob, "Job should have been added to scheduler")

	// 触发任务执行
	err = sched.TriggerJob(job1.ID)
	require.NoError(t, err, "Failed to trigger job execution")

	// 等待任务执行完成
	var executedJobID string
	select {
	case executedJobID = <-executedJobs:
		assert.Equal(t, job1.ID, executedJobID, "Executed job ID should match")
	case <-time.After(5 * time.Second):
		assert.Fail(t, "Timeout waiting for job execution")
	}

	// 测试定时任务
	job2 := jobFactory.CreateScheduledJob("@every 2s")
	job2.Status = constants.JobStatusPending
	job2.Enabled = true
	job2.Command = "echo"
	job2.Args = []string{"Scheduled task execution"}

	// 添加Worker ID
	if job2.Env == nil {
		job2.Env = make(map[string]string)
	}
	job2.Env["WORKER_ID"] = workerID

	// 将任务保存到etcd
	job2JSON, err := job2.ToJSON()
	require.NoError(t, err, "Failed to convert job to JSON")
	err = etcdClient.Put(constants.JobPrefix+job2.ID, job2JSON)
	require.NoError(t, err, "Failed to put job in etcd")

	// 等待任务被添加到调度器
	time.Sleep(2 * time.Second)

	// 等待定时任务自动执行
	select {
	case executedJobID = <-executedJobs:
		assert.Equal(t, job2.ID, executedJobID, "Executed job ID should match")
	case <-time.After(5 * time.Second):
		assert.Fail(t, "Timeout waiting for scheduled job execution")
	}

	// 测试任务状态报告
	jobStatusKey := constants.JobPrefix + job1.ID
	updatedJobJSON, err := etcdClient.Get(jobStatusKey)
	require.NoError(t, err, "Failed to get updated job from etcd")

	var updatedJob models.Job
	err = json.Unmarshal([]byte(updatedJobJSON), &updatedJob)
	require.NoError(t, err, "Failed to unmarshal updated job")

	// 验证任务状态已被更新（可能是running, completed或pending状态）
	assert.NotEqual(t, "", updatedJob.Status, "Job status should have been updated")

	// 测试终止任务
	err = workerJobManager.KillJob(job2.ID)
	require.NoError(t, err, "Failed to send kill signal")

	// 验证终止信号已发送到etcd
	killSignalKey := job2.ID
	killSignal, err := etcdClient.Get(killSignalKey)
	require.NoError(t, err, "Failed to get kill signal from etcd")
	assert.NotEmpty(t, killSignal, "Kill signal should be present in etcd")

	// 等待调度器处理终止信号
	time.Sleep(2 * time.Second)

	// 模拟任务完成后从Worker移除
	updatedWorkerStatus := workerRegister.GetStatus()
	t.Logf("Running jobs: %v", updatedWorkerStatus.RunningJobs)
}

// TestSchedulerJobManagerIntegration 测试调度器和任务管理器的集成
func TestSchedulerJobManagerIntegration(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建Worker ID
	workerID := testutils.GenerateUniqueID("worker")

	// 创建任务执行通道
	executedJobs := make(chan string, 10)

	// 创建任务管理器
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	err := jobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer jobManager.Stop()

	// 创建锁管理器
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	err = lockManager.Start()
	require.NoError(t, err, "Failed to start lock manager")
	defer lockManager.Stop()

	// 创建执行函数
	executeJobFunc := func(job *scheduler.ScheduledJob) error {
		executedJobs <- job.Job.ID
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

	// 创建一个测试任务
	job := jobFactory.CreateSimpleJob()
	job.Status = constants.JobStatusPending
	job.Enabled = true

	// 添加Worker ID到任务环境
	if job.Env == nil {
		job.Env = make(map[string]string)
	}
	job.Env["WORKER_ID"] = workerID

	// 转换为JSON并保存到etcd
	jobJSON, err := job.ToJSON()
	require.NoError(t, err, "Failed to convert job to JSON")
	err = etcdClient.Put(constants.JobPrefix+job.ID, jobJSON)
	require.NoError(t, err, "Failed to put job in etcd")

	// 等待任务管理器检测到任务
	time.Sleep(2 * time.Second)

	// 验证任务已被添加到调度器
	scheduledJobs := sched.ListJobs()
	assert.GreaterOrEqual(t, len(scheduledJobs), 1, "At least one job should be in scheduler")

	foundJob := false
	for _, j := range scheduledJobs {
		if j.Job.ID == job.ID {
			foundJob = true
			break
		}
	}
	assert.True(t, foundJob, "Job should be found in scheduler")

	// 触发任务执行
	err = sched.TriggerJob(job.ID)
	require.NoError(t, err, "Failed to trigger job")

	// 等待任务执行
	select {
	case executedJobID := <-executedJobs:
		assert.Equal(t, job.ID, executedJobID, "Executed job ID should match")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for job execution")
	}

	// 删除任务
	err = etcdClient.Delete(constants.JobPrefix + job.ID)
	require.NoError(t, err, "Failed to delete job from etcd")

	// 等待任务被从调度器中移除
	time.Sleep(2 * time.Second)

	// 验证任务已从调度器中移除
	scheduledJobs = sched.ListJobs()
	foundJob = false
	for _, j := range scheduledJobs {
		if j.Job.ID == job.ID {
			foundJob = true
			break
		}
	}
	assert.False(t, foundJob, "Job should be removed from scheduler")
}

// TestLockManagerJobExecutionIntegration 测试分布式锁与任务执行的集成
func TestLockManagerJobExecutionIntegration(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建两个Worker模拟并发争用
	workerID1 := testutils.GenerateUniqueID("worker1")
	workerID2 := testutils.GenerateUniqueID("worker2")

	// 创建任务执行通道
	worker1ExecutedJobs := make(chan string, 10)
	worker2ExecutedJobs := make(chan string, 10)

	// 创建Worker 1的组件
	lockManager1 := joblock.NewLockManager(etcdClient, workerID1)
	err := lockManager1.Start()
	require.NoError(t, err, "Failed to start lock manager 1")
	defer lockManager1.Stop()

	jobManager1 := jobmgr.NewWorkerJobManager(etcdClient, workerID1)
	err = jobManager1.Start()
	require.NoError(t, err, "Failed to start job manager 1")
	defer jobManager1.Stop()

	// 创建Worker 2的组件
	lockManager2 := joblock.NewLockManager(etcdClient, workerID2)
	err = lockManager2.Start()
	require.NoError(t, err, "Failed to start lock manager 2")
	defer lockManager2.Stop()

	jobManager2 := jobmgr.NewWorkerJobManager(etcdClient, workerID2)
	err = jobManager2.Start()
	require.NoError(t, err, "Failed to start job manager 2")
	defer jobManager2.Stop()

	// 创建Worker 1的执行函数
	executeJobFunc1 := func(job *scheduler.ScheduledJob) error {
		// 获取任务锁
		lock := lockManager1.CreateLock(job.Job.ID, joblock.WithTTL(30))
		acquired, err := lock.TryLock()
		if err != nil {
			return fmt.Errorf("failed to acquire lock: %v", err)
		}
		if !acquired {
			return fmt.Errorf("could not acquire lock for job %s", job.Job.ID)
		}
		defer lock.Unlock()

		// 任务执行
		worker1ExecutedJobs <- job.Job.ID
		utils.Info("job executed by worker 1", zap.String("job_id", job.Job.ID))
		time.Sleep(2 * time.Second) // 模拟任务执行时间
		return nil
	}

	// 创建Worker 2的执行函数
	executeJobFunc2 := func(job *scheduler.ScheduledJob) error {
		// 获取任务锁
		lock := lockManager2.CreateLock(job.Job.ID, joblock.WithTTL(30))
		acquired, err := lock.TryLock()
		if err != nil {
			return fmt.Errorf("failed to acquire lock: %v", err)
		}
		if !acquired {
			return fmt.Errorf("could not acquire lock for job %s", job.Job.ID)
		}
		defer lock.Unlock()

		// 任务执行
		worker2ExecutedJobs <- job.Job.ID
		utils.Info("job executed by worker 2", zap.String("job_id", job.Job.ID))
		time.Sleep(2 * time.Second) // 模拟任务执行时间
		return nil
	}

	// 创建Worker 1的调度器
	sched1 := scheduler.NewScheduler(
		jobManager1,
		lockManager1,
		executeJobFunc1,
		scheduler.WithCheckInterval(500*time.Millisecond),
	)
	err = sched1.Start()
	require.NoError(t, err, "Failed to start scheduler 1")
	defer sched1.Stop()

	// 创建Worker 2的调度器
	sched2 := scheduler.NewScheduler(
		jobManager2,
		lockManager2,
		executeJobFunc2,
		scheduler.WithCheckInterval(500*time.Millisecond),
	)
	err = sched2.Start()
	require.NoError(t, err, "Failed to start scheduler 2")
	defer sched2.Stop()

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 创建一个共享任务，允许两个Worker都执行
	sharedJob := jobFactory.CreateSimpleJob()
	sharedJob.Status = constants.JobStatusPending
	sharedJob.Enabled = true

	// 将任务保存到etcd
	jobJSON, err := sharedJob.ToJSON()
	require.NoError(t, err, "Failed to convert job to JSON")
	err = etcdClient.Put(constants.JobPrefix+sharedJob.ID, jobJSON)
	require.NoError(t, err, "Failed to put job in etcd")

	// 等待任务被添加到两个调度器
	time.Sleep(2 * time.Second)

	// 同时触发两个调度器的任务
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := sched1.TriggerJob(sharedJob.ID)
		if err != nil {
			t.Logf("Worker 1 trigger error: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		err := sched2.TriggerJob(sharedJob.ID)
		if err != nil {
			t.Logf("Worker 2 trigger error: %v", err)
		}
	}()

	wg.Wait()

	// 等待任务执行
	var executedWorker string
	select {
	case <-worker1ExecutedJobs:
		executedWorker = "worker1"
		t.Log("Job executed by worker 1")
	case <-worker2ExecutedJobs:
		executedWorker = "worker2"
		t.Log("Job executed by worker 2")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for job execution")
	}

	// 验证只有一个Worker执行了任务
	assertNoMoreExecutions := func() {
		select {
		case <-worker1ExecutedJobs:
			if executedWorker != "worker1" {
				t.Fatal("Job was executed by both workers")
			}
		case <-worker2ExecutedJobs:
			if executedWorker != "worker2" {
				t.Fatal("Job was executed by both workers")
			}
		case <-time.After(3 * time.Second):
			// 这是预期行为，没有更多执行
			t.Log("No more executions detected, which is expected")
		}
	}

	assertNoMoreExecutions()

	// 等待锁释放
	time.Sleep(4 * time.Second)

	// 再次触发任务执行
	go func() {
		err := sched1.TriggerJob(sharedJob.ID)
		if err != nil {
			t.Logf("Worker 1 second trigger error: %v", err)
		}
	}()

	go func() {
		err := sched2.TriggerJob(sharedJob.ID)
		if err != nil {
			t.Logf("Worker 2 second trigger error: %v", err)
		}
	}()

	// 等待第二次执行
	select {
	case <-worker1ExecutedJobs:
		t.Log("Job executed by worker 1 in second round")
	case <-worker2ExecutedJobs:
		t.Log("Job executed by worker 2 in second round")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for second job execution")
	}

	// 再次验证只有一个Worker执行了任务
	assertNoMoreExecutions()
}
