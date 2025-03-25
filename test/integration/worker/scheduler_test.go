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
	job2 := jobFactory.CreateScheduledJob("*/5 * * * * *") // 每5秒执行一次
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
