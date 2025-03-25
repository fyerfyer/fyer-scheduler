package master

import (
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/jobmgr"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJobManagerCRUD 测试任务管理器的基本CRUD操作
func TestJobManagerCRUD(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建任务管理器
	jm := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 测试创建任务
	err := jm.CreateJob(job)
	require.NoError(t, err, "Failed to create job")

	// 测试获取任务
	retrievedJob, err := jm.GetJob(job.ID)
	require.NoError(t, err, "Failed to get job")
	assert.Equal(t, job.ID, retrievedJob.ID, "Job IDs should match")
	assert.Equal(t, job.Name, retrievedJob.Name, "Job names should match")
	assert.Equal(t, job.Command, retrievedJob.Command, "Job commands should match")

	// 测试更新任务
	retrievedJob.Description = "Updated description"
	err = jm.UpdateJob(retrievedJob)
	require.NoError(t, err, "Failed to update job")

	// 验证更新
	updatedJob, err := jm.GetJob(job.ID)
	require.NoError(t, err, "Failed to get updated job")
	assert.Equal(t, "Updated description", updatedJob.Description, "Job description should be updated")

	// 测试删除任务
	err = jm.DeleteJob(job.ID)
	require.NoError(t, err, "Failed to delete job")

	// 验证删除
	_, err = jm.GetJob(job.ID)
	assert.Error(t, err, "Job should be deleted")
}

// TestJobStatusOperations 测试任务状态操作
func TestJobStatusOperations(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建任务管理器
	jm := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	job.Enabled = true

	// 保存任务
	err := jm.CreateJob(job)
	require.NoError(t, err, "Failed to create job")

	// 测试禁用任务
	err = jm.DisableJob(job.ID)
	require.NoError(t, err, "Failed to disable job")

	// 验证任务已禁用
	disabledJob, err := jm.GetJob(job.ID)
	require.NoError(t, err, "Failed to get job")
	assert.False(t, disabledJob.Enabled, "Job should be disabled")

	// 测试启用任务
	err = jm.EnableJob(job.ID)
	require.NoError(t, err, "Failed to enable job")

	// 验证任务已启用
	enabledJob, err := jm.GetJob(job.ID)
	require.NoError(t, err, "Failed to get job")
	assert.True(t, enabledJob.Enabled, "Job should be enabled")

	// 测试触发任务
	err = jm.TriggerJob(job.ID)
	require.NoError(t, err, "Failed to trigger job")

	// 验证任务状态为运行中
	runningJob, err := jm.GetJob(job.ID)
	require.NoError(t, err, "Failed to get job")
	assert.Equal(t, constants.JobStatusRunning, runningJob.Status, "Job status should be running")

	// 测试取消任务
	err = jm.CancelJob(job.ID, "Test cancellation")
	require.NoError(t, err, "Failed to cancel job")

	// 验证任务状态为已取消
	cancelledJob, err := jm.GetJob(job.ID)
	require.NoError(t, err, "Failed to get job")
	assert.Equal(t, constants.JobStatusCancelled, cancelledJob.Status, "Job status should be cancelled")
}

// TestJobListing 测试任务列表功能
func TestJobListing(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建任务管理器
	jm := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()

	// 创建多个不同状态的任务
	pendingJob := jobFactory.CreatePendingJob()
	runningJob := jobFactory.CreateRunningJob()
	completedJob := jobFactory.CreateCompletedJob(true)

	// 保存任务
	require.NoError(t, jm.CreateJob(pendingJob), "Failed to create pending job")
	require.NoError(t, jm.CreateJob(runningJob), "Failed to create running job")
	require.NoError(t, jm.CreateJob(completedJob), "Failed to create completed job")

	// 测试列出所有任务
	jobs, total, err := jm.ListJobs(1, 10)
	require.NoError(t, err, "Failed to list jobs")
	assert.Equal(t, int64(3), total, "Total job count should be 3")
	assert.Len(t, jobs, 3, "Should return 3 jobs")

	// 测试按状态列出任务
	runningJobs, runningTotal, err := jm.ListJobsByStatus(constants.JobStatusRunning, 1, 10)
	require.NoError(t, err, "Failed to list running jobs")
	assert.Equal(t, int64(1), runningTotal, "Total running job count should be 1")
	assert.Len(t, runningJobs, 1, "Should return 1 running job")
	assert.Equal(t, constants.JobStatusRunning, runningJobs[0].Status, "Job status should be running")
}

// TestJobDueJobs 测试获取到期任务
func TestJobDueJobs(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建任务管理器
	jm := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()

	// 创建一个已到期的任务
	dueJob := jobFactory.CreateScheduledJob("*/5 * * * *")
	dueJob.NextRunTime = time.Now().Add(-1 * time.Minute)
	dueJob.Enabled = true

	// 创建一个未到期的任务
	futureJob := jobFactory.CreateScheduledJob("*/5 * * * *")
	futureJob.NextRunTime = time.Now().Add(5 * time.Minute)
	futureJob.Enabled = true

	// 创建一个禁用的任务
	disabledJob := jobFactory.CreateScheduledJob("*/5 * * * *")
	disabledJob.NextRunTime = time.Now().Add(-1 * time.Minute)
	disabledJob.Enabled = false

	// 保存任务
	require.NoError(t, jm.CreateJob(dueJob), "Failed to create due job")
	require.NoError(t, jm.CreateJob(futureJob), "Failed to create future job")
	require.NoError(t, jm.CreateJob(disabledJob), "Failed to create disabled job")

	// 获取到期任务
	dueJobs, err := jm.GetDueJobs()
	require.NoError(t, err, "Failed to get due jobs")

	// 应该只有一个到期且已启用的任务
	assert.Len(t, dueJobs, 1, "Should return 1 due job")
	assert.Equal(t, dueJob.ID, dueJobs[0].ID, "Due job ID should match")
}

// TestJobEventHandling 测试任务事件处理
func TestJobEventHandling(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建任务管理器
	jm := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)

	// 启动任务管理器
	err := jm.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer jm.Stop()

	// 创建事件处理器
	eventHandler := jobmgr.NewEventHandler(jm)
	eventHandler.RegisterWithJobManager()

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateScheduledJob("*/1 * * * *") // 每分钟执行一次
	job.Status = constants.JobStatusPending

	// 保存任务
	err = jm.CreateJob(job)
	require.NoError(t, err, "Failed to create job")

	// 检查是否设置了下次运行时间
	retrievedJob, err := jm.GetJob(job.ID)
	require.NoError(t, err, "Failed to get job")
	assert.False(t, retrievedJob.NextRunTime.IsZero(), "Next run time should be set")

	// 模拟任务完成事件
	retrievedJob.Status = constants.JobStatusSucceeded
	retrievedJob.LastRunTime = time.Now()

	// 更新任务以触发事件
	err = jm.UpdateJob(retrievedJob)
	require.NoError(t, err, "Failed to update job")

	// 给事件处理一些时间
	time.Sleep(500 * time.Millisecond)

	// 验证下次运行时间已更新
	updatedJob, err := jm.GetJob(job.ID)
	require.NoError(t, err, "Failed to get updated job")
	assert.False(t, updatedJob.NextRunTime.IsZero(), "Next run time should still be set")
	assert.True(t, updatedJob.NextRunTime.After(retrievedJob.NextRunTime), "Next run time should be updated to a later time")
}

// TestStartStop 测试任务管理器的启动和停止
func TestStartStop(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建任务管理器
	jm := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)

	// 测试启动
	err := jm.Start()
	require.NoError(t, err, "Failed to start job manager")

	// 测试重复启动
	err = jm.Start()
	assert.NoError(t, err, "Starting an already running job manager should not error")

	// 测试停止
	err = jm.Stop()
	require.NoError(t, err, "Failed to stop job manager")

	// 测试重复停止
	err = jm.Stop()
	assert.NoError(t, err, "Stopping an already stopped job manager should not error")
}

// TestWorkerSelection 测试Worker选择策略
func TestWorkerSelection(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建并注册测试Worker
	worker1 := testutils.CreateWorker("test-worker-1", nil)
	worker1.Status = constants.WorkerStatusOnline
	worker1.RunningJobs = []string{"job-1", "job-2"}
	worker1.LoadAvg = 0.8
	worker1.MemoryFree = 2048

	worker2 := testutils.CreateWorker("test-worker-2", nil)
	worker2.Status = constants.WorkerStatusOnline
	worker2.RunningJobs = []string{"job-3"}
	worker2.LoadAvg = 0.4
	worker2.MemoryFree = 4096

	worker3 := testutils.CreateWorker("test-worker-3", nil)
	worker3.Status = constants.WorkerStatusOnline
	worker3.RunningJobs = []string{}
	worker3.LoadAvg = 0.1
	worker3.MemoryFree = 1024

	_, err := workerRepo.Register(worker1, 30)
	require.NoError(t, err, "Failed to register worker 1")
	_, err = workerRepo.Register(worker2, 30)
	require.NoError(t, err, "Failed to register worker 2")
	_, err = workerRepo.Register(worker3, 30)
	require.NoError(t, err, "Failed to register worker 3")

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 测试最少任务策略
	leastJobsSelector := jobmgr.NewWorkerSelector(workerRepo, jobmgr.LeastJobsStrategy)
	selectedWorker, err := leastJobsSelector.SelectWorker(job)
	require.NoError(t, err, "Failed to select worker using least jobs strategy")
	assert.Equal(t, "test-worker-3", selectedWorker.ID, "Worker with least jobs should be selected")

	// 测试最低负载策略
	loadBalanceSelector := jobmgr.NewWorkerSelector(workerRepo, jobmgr.LoadBalanceStrategy)
	selectedWorker, err = loadBalanceSelector.SelectWorker(job)
	require.NoError(t, err, "Failed to select worker using load balance strategy")
	assert.Equal(t, "test-worker-3", selectedWorker.ID, "Worker with lowest load should be selected")

	// 测试资源优先策略
	resourceSelector := jobmgr.NewWorkerSelector(workerRepo, jobmgr.ResourceStrategy)
	selectedWorker, err = resourceSelector.SelectWorker(job)
	require.NoError(t, err, "Failed to select worker using resource strategy")
	assert.Equal(t, "test-worker-2", selectedWorker.ID, "Worker with most resources should be selected")

	// 测试策略变更
	assert.Equal(t, jobmgr.ResourceStrategy, resourceSelector.GetStrategy(), "Strategy should match what was set")
	resourceSelector.SetStrategy(jobmgr.RandomStrategy)
	assert.Equal(t, jobmgr.RandomStrategy, resourceSelector.GetStrategy(), "Strategy should be updated")
}

// TestScheduler 测试调度器功能
func TestScheduler(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建任务管理器和调度器
	jobManager := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)
	scheduler := jobmgr.NewScheduler(jobManager, workerRepo, 1*time.Second)

	// 启动调度器
	err := scheduler.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer scheduler.Stop()

	// 创建并注册测试Worker
	worker := testutils.CreateWorker("test-worker-1", nil)
	worker.Status = constants.WorkerStatusOnline
	_, err = workerRepo.Register(worker, 30)
	require.NoError(t, err, "Failed to register worker")

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	job.NextRunTime = time.Now().Add(-1 * time.Second) // 设置为已到期
	job.Enabled = true

	// 保存任务
	err = jobManager.CreateJob(job)
	require.NoError(t, err, "Failed to create job")

	// 手动触发立即执行
	err = scheduler.TriggerImmediateJob(job.ID)
	require.NoError(t, err, "Failed to trigger immediate job execution")

	// 验证调度器状态
	status := scheduler.GetSchedulerStatus()
	assert.NotNil(t, status, "Scheduler status should not be nil")
	assert.True(t, status["is_running"].(bool), "Scheduler should be running")
}

// TestJobOperations 测试高级任务操作
func TestJobOperations(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建任务管理器
	jobManager := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)

	// 创建Worker选择器
	workerSelector := jobmgr.NewWorkerSelector(workerRepo, jobmgr.LeastJobsStrategy)

	// 创建调度器
	scheduler := jobmgr.NewScheduler(jobManager, workerRepo, 1*time.Second)

	// 创建任务操作
	jobOps := jobmgr.NewJobOperations(jobManager, workerRepo, logRepo, workerSelector, scheduler)

	// 创建并注册测试Worker
	worker := testutils.CreateWorker("test-worker-1", nil)
	worker.Status = constants.WorkerStatusOnline
	_, err := workerRepo.Register(worker, 30)
	require.NoError(t, err, "Failed to register worker")

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	job.Enabled = true

	// 保存任务
	err = jobManager.CreateJob(job)
	require.NoError(t, err, "Failed to create job")

	// 测试批量触发任务
	results, errors := jobOps.BatchTriggerJobs([]string{job.ID})
	require.Len(t, errors, 0, "Should not have errors when triggering jobs")
	require.Len(t, results, 1, "Should have one result when triggering one job")
	assert.NotEmpty(t, results[job.ID], "Execution ID should not be empty")

	// 测试任务统计
	stats, err := jobOps.GetJobStatistics(job.ID)
	require.NoError(t, err, "Failed to get job statistics")
	assert.NotNil(t, stats, "Job statistics should not be nil")
}

// waitForJobStatus 等待任务达到指定状态，用于测试异步操作
func waitForJobStatus(t *testing.T, jm jobmgr.JobManager, jobID string, expectedStatus string, timeout time.Duration) bool {
	return testutils.WaitForCondition(func() bool {
		job, err := jm.GetJob(jobID)
		if err != nil {
			t.Logf("Error getting job: %v", err)
			return false
		}
		return job.Status == expectedStatus
	}, timeout)
}
