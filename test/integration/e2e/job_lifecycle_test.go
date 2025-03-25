package e2e

import (
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/jobmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/logmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/workermgr"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试任务的完整生命周期
func TestJobLifecycle(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 清理可能存在的测试数据
	err := testutils.CleanEtcdPrefix(etcdClient, constants.JobPrefix)
	require.NoError(t, err, "Failed to clean job prefix in etcd")
	err = testutils.CleanEtcdPrefix(etcdClient, constants.WorkerPrefix)
	require.NoError(t, err, "Failed to clean worker prefix in etcd")

	// 创建仓库实例
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建Job Manager
	masterJobManager := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)
	err = masterJobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer masterJobManager.Stop()

	// 创建Worker Manager
	masterWorkerManager := workermgr.NewWorkerManager(workerRepo, 5*time.Second, etcdClient)
	err = masterWorkerManager.(workermgr.WorkerManager).Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer masterWorkerManager.(workermgr.WorkerManager).Stop()

	// 创建Log Manager
	masterLogManager := logmgr.NewLogManager(logRepo)

	// 创建调度器
	scheduler := jobmgr.NewScheduler(masterJobManager, workerRepo, 5*time.Second)
	err = scheduler.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer scheduler.Stop()

	// 创建测试Worker并等待发现
	workerID := testutils.GenerateUniqueID("worker")
	testWorker, err := testutils.CreateTestWorker(etcdClient, workerID, map[string]string{"env": "test"})
	require.NoError(t, err, "Failed to create test worker")
	defer testWorker.Stop()

	// 等待Worker被发现
	found := testutils.WaitForCondition(func() bool {
		workers, err := masterWorkerManager.(workermgr.WorkerManager).GetActiveWorkers()
		return err == nil && len(workers) > 0
	}, 10*time.Second)

	require.True(t, found, "Worker was not discovered")

	// 创建JobFactory
	jobFactory := testutils.NewJobFactory()

	// 测试一：创建并运行简单任务
	t.Run("Simple Job Execution", func(t *testing.T) {
		// 创建一个简单的测试任务 - 使用echo命令快速完成
		job := jobFactory.CreateJobWithCommand("echo 'simple job test'")
		job.Name = "test-simple-job"

		// 保存任务
		err := masterJobManager.CreateJob(job)
		require.NoError(t, err, "Failed to create job")

		// 立即触发任务
		err = masterJobManager.TriggerJob(job.ID)
		require.NoError(t, err, "Failed to trigger job")

		// 等待任务完成
		completed := testutils.WaitForCondition(func() bool {
			currentJob, err := masterJobManager.GetJob(job.ID)
			if err != nil {
				return false
			}
			return currentJob.Status == constants.JobStatusSucceeded ||
				currentJob.Status == constants.JobStatusFailed
		}, 15*time.Second)

		require.True(t, completed, "Job did not complete in time")

		// 验证任务状态
		updatedJob, err := masterJobManager.GetJob(job.ID)
		require.NoError(t, err, "Failed to get updated job")
		assert.Equal(t, constants.JobStatusSucceeded, updatedJob.Status, "Job status should be completed")

		// 获取执行日志
		logs, count, err := masterLogManager.GetLogsByJobID(job.ID, 1, 10)
		require.NoError(t, err, "Failed to get job logs")
		assert.Equal(t, int64(1), count, "Should have one log entry")
		assert.Len(t, logs, 1, "Should have one log entry")
		assert.Equal(t, 0, logs[0].ExitCode, "Exit code should be 0")
		assert.Contains(t, logs[0].Output, "simple job test", "Output should contain the echo text")
	})

	// 测试二：运行带超时的长时间任务
	t.Run("Job With Timeout", func(t *testing.T) {
		// 创建一个会超时的任务 - 睡眠30秒，但超时设为2秒
		job := jobFactory.CreateJobWithCommand("sleep 30")
		job.Name = "test-timeout-job"
		job.Timeout = 2 // 2秒超时

		// 保存任务
		err := masterJobManager.CreateJob(job)
		require.NoError(t, err, "Failed to create timeout job")

		// 立即触发任务
		err = masterJobManager.TriggerJob(job.ID)
		require.NoError(t, err, "Failed to trigger timeout job")

		// 等待任务超时
		timedOut := testutils.WaitForCondition(func() bool {
			currentJob, err := masterJobManager.GetJob(job.ID)
			if err != nil {
				return false
			}
			return currentJob.Status == constants.JobStatusFailed
		}, 10*time.Second)

		require.True(t, timedOut, "Job did not timeout as expected")

		// 验证任务状态
		updatedJob, err := masterJobManager.GetJob(job.ID)
		require.NoError(t, err, "Failed to get updated timeout job")
		assert.Equal(t, constants.JobStatusFailed, updatedJob.Status, "Job status should be failed")

		// 获取执行日志
		logs, _, err := masterLogManager.GetLogsByJobID(job.ID, 1, 10)
		require.NoError(t, err, "Failed to get timeout job logs")
		assert.Len(t, logs, 1, "Should have one log entry")
		assert.Contains(t, logs[0].Error, "timeout", "Error should mention timeout")
	})

	// 测试三：测试Cron调度任务
	t.Run("Cron Scheduled Job", func(t *testing.T) {
		// 创建一个每分钟执行一次的Cron任务
		job := jobFactory.CreateScheduledJob("* * * * *") // 每分钟执行
		job.Name = "test-cron-job"
		job.Command = "echo 'cron job test'"

		// 保存任务
		err := masterJobManager.CreateJob(job)
		require.NoError(t, err, "Failed to create cron job")

		// 获取原始任务
		originalJob, err := masterJobManager.GetJob(job.ID)
		require.NoError(t, err, "Failed to get original cron job")

		// 确保有下一次执行时间
		assert.False(t, originalJob.NextRunTime.IsZero(), "NextRunTime should be set for cron job")

		// 立即触发一次任务
		err = masterJobManager.TriggerJob(job.ID)
		require.NoError(t, err, "Failed to trigger cron job")

		// 等待任务完成
		completed := testutils.WaitForCondition(func() bool {
			currentJob, err := masterJobManager.GetJob(job.ID)
			if err != nil {
				return false
			}
			// 注意：对于Cron任务，执行完成后状态会恢复到pending
			return currentJob.Status == constants.JobStatusPending &&
				!currentJob.LastRunTime.IsZero()
		}, 15*time.Second)

		require.True(t, completed, "Cron job did not complete and return to pending")

		// 验证任务状态和下次执行时间
		updatedJob, err := masterJobManager.GetJob(job.ID)
		require.NoError(t, err, "Failed to get updated cron job")
		assert.Equal(t, constants.JobStatusPending, updatedJob.Status, "Cron job status should be pending after execution")
		assert.NotEqual(t, originalJob.NextRunTime, updatedJob.NextRunTime, "NextRunTime should be updated")
		assert.False(t, updatedJob.LastRunTime.IsZero(), "LastRunTime should be set")

		// 禁用任务，避免影响后续测试
		err = masterJobManager.DisableJob(job.ID)
		require.NoError(t, err, "Failed to disable cron job")
	})

	// 测试四：任务取消
	t.Run("Job Cancellation", func(t *testing.T) {
		// 创建一个长时间运行的任务
		job := jobFactory.CreateJobWithCommand("sleep 60")
		job.Name = "test-cancel-job"

		// 保存任务
		err := masterJobManager.CreateJob(job)
		require.NoError(t, err, "Failed to create cancelable job")

		// 立即触发任务
		err = masterJobManager.TriggerJob(job.ID)
		require.NoError(t, err, "Failed to trigger cancelable job")

		// 等待任务开始运行
		started := testutils.WaitForCondition(func() bool {
			currentJob, err := masterJobManager.GetJob(job.ID)
			if err != nil {
				return false
			}
			return currentJob.Status == constants.JobStatusRunning
		}, 10*time.Second)

		require.True(t, started, "Job did not start running")

		// 取消任务
		err = masterJobManager.CancelJob(job.ID, "Manual cancellation for testing")
		require.NoError(t, err, "Failed to cancel job")

		// 等待任务被取消
		cancelled := testutils.WaitForCondition(func() bool {
			currentJob, err := masterJobManager.GetJob(job.ID)
			if err != nil {
				return false
			}
			return currentJob.Status == constants.JobStatusCancelled
		}, 10*time.Second)

		require.True(t, cancelled, "Job was not cancelled")

		// 验证任务状态
		updatedJob, err := masterJobManager.GetJob(job.ID)
		require.NoError(t, err, "Failed to get updated cancelled job")
		assert.Equal(t, constants.JobStatusCancelled, updatedJob.Status, "Job status should be cancelled")

		// 获取执行日志
		logs, _, err := masterLogManager.GetLogsByJobID(job.ID, 1, 10)
		require.NoError(t, err, "Failed to get cancelled job logs")
		assert.Len(t, logs, 1, "Should have one log entry")
	})

	// 测试五：任务重试
	t.Run("Job Retry", func(t *testing.T) {
		// 创建一个会失败的任务，设置自动重试
		job := jobFactory.CreateJobWithCommand("exit 1") // 返回错误码1，表示失败
		job.Name = "test-retry-job"
		job.MaxRetry = 2 // 最多重试2次

		// 保存任务
		err := masterJobManager.CreateJob(job)
		require.NoError(t, err, "Failed to create retry job")

		// 立即触发任务
		err = masterJobManager.TriggerJob(job.ID)
		require.NoError(t, err, "Failed to trigger retry job")

		// 等待最终失败(包括重试)，这可能需要更长时间
		failed := testutils.WaitForCondition(func() bool {
			currentJob, err := masterJobManager.GetJob(job.ID)
			if err != nil {
				return false
			}
			return currentJob.Status == constants.JobStatusFailed
		}, 30*time.Second)

		require.True(t, failed, "Job did not complete retries and fail")

		// 验证任务状态
		updatedJob, err := masterJobManager.GetJob(job.ID)
		require.NoError(t, err, "Failed to get updated retry job")
		assert.Equal(t, constants.JobStatusFailed, updatedJob.Status, "Job status should be failed after retries")

		// 获取执行日志，应该有多条(原始执行 + 重试次数)
		logs, _, err := masterLogManager.GetLogsByJobID(job.ID, 1, 10)
		require.NoError(t, err, "Failed to get retry job logs")

		// 应该有至少一条日志记录
		assert.NotEmpty(t, logs, "Should have at least one log entry")

		// 最后一条日志应该是失败状态
		lastLog := logs[0]
		assert.Equal(t, 1, lastLog.ExitCode, "Exit code should be 1")
	})
}

// 测试任务依赖和流程
func TestJobWorkflow(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 清理可能存在的测试数据
	err := testutils.CleanEtcdPrefix(etcdClient, constants.JobPrefix)
	require.NoError(t, err, "Failed to clean job prefix in etcd")

	// 创建仓库实例
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建managers
	masterJobManager := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)
	err = masterJobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer masterJobManager.Stop()

	// 创建Worker Manager
	masterWorkerManager := workermgr.NewWorkerManager(workerRepo, 5*time.Second, etcdClient)
	err = masterWorkerManager.(workermgr.WorkerManager).Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer masterWorkerManager.(workermgr.WorkerManager).Stop()

	// 创建测试Worker
	workerID := testutils.GenerateUniqueID("worker")
	testWorker, err := testutils.CreateTestWorker(etcdClient, workerID, map[string]string{"env": "test"})
	require.NoError(t, err, "Failed to create test worker")
	defer testWorker.Stop()

	// 等待Worker被发现
	found := testutils.WaitForCondition(func() bool {
		workers, err := masterWorkerManager.(workermgr.WorkerManager).GetActiveWorkers()
		return err == nil && len(workers) > 0
	}, 10*time.Second)

	require.True(t, found, "Worker was not discovered")

	// 创建一系列相关任务作为工作流
	t.Run("Sequential Job Workflow", func(t *testing.T) {
		// 创建三个任务：准备数据 -> 处理数据 -> 清理

		// 第一个任务：准备数据
		prepareJob, err := models.NewJob("workflow-prepare", "echo 'Creating test data' > /tmp/workflow_test.txt", "")
		require.NoError(t, err, "Failed to create prepare job")

		err = masterJobManager.CreateJob(prepareJob)
		require.NoError(t, err, "Failed to save prepare job")

		// 第二个任务：处理数据
		processJob, err := models.NewJob("workflow-process", "echo 'Processing data' >> /tmp/workflow_test.txt", "")
		require.NoError(t, err, "Failed to create process job")

		err = masterJobManager.CreateJob(processJob)
		require.NoError(t, err, "Failed to save process job")

		// 第三个任务：清理
		cleanupJob, err := models.NewJob("workflow-cleanup", "echo 'Cleanup complete' >> /tmp/workflow_test.txt && cat /tmp/workflow_test.txt", "")
		require.NoError(t, err, "Failed to create cleanup job")

		err = masterJobManager.CreateJob(cleanupJob)
		require.NoError(t, err, "Failed to save cleanup job")

		// 按顺序执行任务

		// 步骤1: 执行准备任务
		err = masterJobManager.TriggerJob(prepareJob.ID)
		require.NoError(t, err, "Failed to trigger prepare job")

		// 等待准备任务完成
		completed := testutils.WaitForCondition(func() bool {
			job, err := masterJobManager.GetJob(prepareJob.ID)
			if err != nil {
				return false
			}
			return job.Status == constants.JobStatusSucceeded
		}, 15*time.Second)

		require.True(t, completed, "Prepare job did not complete")

		// 步骤2: 执行处理任务
		err = masterJobManager.TriggerJob(processJob.ID)
		require.NoError(t, err, "Failed to trigger process job")

		// 等待处理任务完成
		completed = testutils.WaitForCondition(func() bool {
			job, err := masterJobManager.GetJob(processJob.ID)
			if err != nil {
				return false
			}
			return job.Status == constants.JobStatusSucceeded
		}, 15*time.Second)

		require.True(t, completed, "Process job did not complete")

		// 步骤3: 执行清理任务
		err = masterJobManager.TriggerJob(cleanupJob.ID)
		require.NoError(t, err, "Failed to trigger cleanup job")

		// 等待清理任务完成
		completed = testutils.WaitForCondition(func() bool {
			job, err := masterJobManager.GetJob(cleanupJob.ID)
			if err != nil {
				return false
			}
			return job.Status == constants.JobStatusSucceeded
		}, 15*time.Second)

		require.True(t, completed, "Cleanup job did not complete")

		// 验证最终结果
		logs, _, err := logRepo.GetByJobID(cleanupJob.ID, 1, 10)
		require.NoError(t, err, "Failed to get cleanup job logs")
		assert.NotEmpty(t, logs, "Should have cleanup job logs")

		// 检查输出包含所有任务的结果
		if len(logs) > 0 {
			assert.Contains(t, logs[0].Output, "Creating test data", "Output should contain prepare job result")
			assert.Contains(t, logs[0].Output, "Processing data", "Output should contain process job result")
			assert.Contains(t, logs[0].Output, "Cleanup complete", "Output should contain cleanup job result")
		}
	})
}
