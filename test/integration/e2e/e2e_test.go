package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/jobmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/workermgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/register"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJobCreationAndExecution 测试任务创建和执行的端到端流程
func TestJobCreationAndExecution(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建Master组件
	workerManager := workermgr.NewWorkerManager(workerRepo, 5*time.Second, etcdClient)
	err := workerManager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer workerManager.Stop()

	// 创建任务管理器
	masterJobManager := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)
	err = masterJobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer masterJobManager.Stop()

	// 创建Worker选择器
	//workerSelector := jobmgr.NewWorkerSelector(workerRepo, jobmgr.LeastJobsStrategy)

	// 创建调度器
	scheduler := jobmgr.NewScheduler(masterJobManager, workerRepo, 5*time.Second)
	err = scheduler.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer scheduler.Stop()

	// 创建和注册Worker
	workerID := testutils.GenerateUniqueID("worker")
	workerRegister, err := testutils.CreateTestWorker(etcdClient, workerID, map[string]string{"env": "test"})
	require.NoError(t, err, "Failed to create test worker")
	defer workerRegister.Stop()

	// 等待Worker被发现
	found := testutils.WaitForCondition(func() bool {
		workers, err := workerManager.GetActiveWorkers()
		if err != nil {
			return false
		}
		for _, w := range workers {
			if w.ID == workerID {
				return true
			}
		}
		return false
	}, 10*time.Second)

	require.True(t, found, "Worker was not discovered")

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	job.Status = constants.JobStatusPending
	job.Enabled = true

	// 保存任务
	err = masterJobManager.CreateJob(job)
	require.NoError(t, err, "Failed to create job")

	// 立即触发任务
	err = masterJobManager.TriggerJob(job.ID)
	require.NoError(t, err, "Failed to trigger job")

	// 等待任务被调度和执行
	executed := testutils.WaitForCondition(func() bool {
		updatedJob, err := masterJobManager.GetJob(job.ID)
		if err != nil {
			return false
		}
		// 检查任务是否已被执行（状态为成功或失败）
		return updatedJob.Status == constants.JobStatusSucceeded || updatedJob.Status == constants.JobStatusFailed
	}, 15*time.Second)

	require.True(t, executed, "Job was not executed")

	// 获取任务日志
	logs, count, err := logRepo.GetByJobID(job.ID, 1, 10)
	require.NoError(t, err, "Failed to get job logs")
	assert.True(t, count > 0, "No job logs found")
	if count > 0 {
		assert.Contains(t, logs[0].Output, "hello world", "Job output does not contain expected content")
	}
}

// TestScheduledJob 测试定时任务调度功能
func TestScheduledJob(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建Master组件
	workerManager := workermgr.NewWorkerManager(workerRepo, 5*time.Second, etcdClient)
	err := workerManager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer workerManager.Stop()

	// 创建任务管理器
	masterJobManager := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)
	err = masterJobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer masterJobManager.Stop()

	// 创建调度器
	scheduler := jobmgr.NewScheduler(masterJobManager, workerRepo, 2*time.Second)
	err = scheduler.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer scheduler.Stop()

	// 创建和注册Worker
	workerID := testutils.GenerateUniqueID("worker")
	workerRegister, err := testutils.CreateTestWorker(etcdClient, workerID, map[string]string{"env": "test"})
	require.NoError(t, err, "Failed to create test worker")
	defer workerRegister.Stop()

	// 等待Worker被发现
	found := testutils.WaitForCondition(func() bool {
		workers, err := workerManager.GetActiveWorkers()
		if err != nil {
			return false
		}
		for _, w := range workers {
			if w.ID == workerID {
				return true
			}
		}
		return false
	}, 10*time.Second)
	require.True(t, found, "Worker was not discovered")

	// 创建定时任务，配置为每3秒运行一次
	jobFactory := testutils.NewJobFactory()
	scheduledJob := jobFactory.CreateScheduledJob("@every 3s")
	scheduledJob.Enabled = true

	// 保存任务
	err = masterJobManager.CreateJob(scheduledJob)
	require.NoError(t, err, "Failed to create scheduled job")

	// 等待任务被调度执行至少一次
	executed := testutils.WaitForCondition(func() bool {
		_, count, err := logRepo.GetByJobID(scheduledJob.ID, 1, 10)
		if err != nil || count == 0 {
			return false
		}
		return true
	}, 15*time.Second)

	require.True(t, executed, "Scheduled job was not executed")

	// 验证任务的下次执行时间已更新
	updatedJob, err := masterJobManager.GetJob(scheduledJob.ID)
	require.NoError(t, err, "Failed to get updated job")
	assert.False(t, updatedJob.NextRunTime.IsZero(), "Next run time was not set")
	assert.True(t, updatedJob.NextRunTime.After(time.Now()), "Next run time should be in the future")
}

// TestWorkerFailover 测试Worker故障转移
func TestWorkerFailover(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建Master组件
	workerManager := workermgr.NewWorkerManager(workerRepo, 5*time.Second, etcdClient)
	err := workerManager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer workerManager.Stop()

	// 创建任务管理器
	masterJobManager := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)
	err = masterJobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer masterJobManager.Stop()

	// 创建调度器
	scheduler := jobmgr.NewScheduler(masterJobManager, workerRepo, 2*time.Second)
	err = scheduler.Start()
	require.NoError(t, err, "Failed to start scheduler")
	defer scheduler.Stop()

	// 创建和注册两个Worker
	worker1ID := testutils.GenerateUniqueID("worker")
	worker1Register, err := testutils.CreateTestWorker(etcdClient, worker1ID, map[string]string{"env": "test"})
	require.NoError(t, err, "Failed to create test worker 1")

	worker2ID := testutils.GenerateUniqueID("worker")
	worker2Register, err := testutils.CreateTestWorker(etcdClient, worker2ID, map[string]string{"env": "test"})
	require.NoError(t, err, "Failed to create test worker 2")

	// 等待Worker被发现
	found := testutils.WaitForCondition(func() bool {
		workers, err := workerManager.GetActiveWorkers()
		if err != nil {
			return false
		}
		worker1Found := false
		worker2Found := false
		for _, w := range workers {
			if w.ID == worker1ID {
				worker1Found = true
			}
			if w.ID == worker2ID {
				worker2Found = true
			}
		}
		return worker1Found && worker2Found
	}, 10*time.Second)
	require.True(t, found, "Workers were not discovered")

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	job.Enabled = true

	// 保存任务
	err = masterJobManager.CreateJob(job)
	require.NoError(t, err, "Failed to create job")

	// 停止第一个Worker，模拟故障
	worker1Register.Stop()

	// 等待Master检测到Worker状态变化
	statusChanged := testutils.WaitForCondition(func() bool {
		worker, err := workerManager.GetWorker(worker1ID)
		if err != nil {
			return false
		}
		return worker.Status == constants.WorkerStatusOffline
	}, 15*time.Second)
	require.True(t, statusChanged, "Worker status did not change to offline")

	// 触发任务执行
	err = masterJobManager.TriggerJob(job.ID)
	require.NoError(t, err, "Failed to trigger job")

	// 等待任务被调度到第二个Worker并执行
	executed := testutils.WaitForCondition(func() bool {
		logs, count, err := logRepo.GetByJobID(job.ID, 1, 10)
		if err != nil || count == 0 {
			return false
		}
		// 检查是否由第二个Worker执行
		return logs[0].WorkerID == worker2ID
	}, 15*time.Second)

	require.True(t, executed, "Job was not executed by the second worker")

	// 清理
	worker2Register.Stop()
}

// TestWorkerStatusUpdate 测试Worker状态更新
func TestWorkerStatusUpdate(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建Worker管理器
	workerManager := workermgr.NewWorkerManager(workerRepo, 5*time.Second, etcdClient)
	err := workerManager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer workerManager.Stop()

	// 创建Worker
	workerID := testutils.GenerateUniqueID("worker")
	workerRegister, err := testutils.CreateTestWorker(etcdClient, workerID, map[string]string{"env": "test"})
	require.NoError(t, err, "Failed to create test worker")

	// 等待Worker被发现
	found := testutils.WaitForCondition(func() bool {
		worker, err := workerManager.GetWorker(workerID)
		return err == nil && worker != nil && worker.Status == constants.WorkerStatusOnline
	}, 10*time.Second)
	require.True(t, found, "Worker was not discovered or not online")

	// 禁用Worker
	err = workerManager.DisableWorker(workerID)
	require.NoError(t, err, "Failed to disable worker")

	// 验证Worker状态已更新
	disabled := testutils.WaitForCondition(func() bool {
		worker, err := workerManager.GetWorker(workerID)
		return err == nil && worker.Status == constants.WorkerStatusDisabled
	}, 10*time.Second)
	require.True(t, disabled, "Worker status was not updated to disabled")

	// 启用Worker
	err = workerManager.EnableWorker(workerID)
	require.NoError(t, err, "Failed to enable worker")

	// 验证Worker状态已更新
	enabled := testutils.WaitForCondition(func() bool {
		worker, err := workerManager.GetWorker(workerID)
		return err == nil && worker.Status == constants.WorkerStatusOnline
	}, 10*time.Second)
	require.True(t, enabled, "Worker status was not updated to online")

	// 停止Worker
	err = workerRegister.Stop()
	require.NoError(t, err, "Failed to stop worker")

	// 验证Worker状态变为离线
	offline := testutils.WaitForCondition(func() bool {
		worker, err := workerManager.GetWorker(workerID)
		return err == nil && worker.Status == constants.WorkerStatusOffline
	}, 15*time.Second)
	require.True(t, offline, "Worker status was not updated to offline")
}

// TestJobStatusCancellation 测试任务取消功能
func TestJobStatusCancellation(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	jobRepo := repo.NewJobRepo(etcdClient, mongoClient)
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建Master组件
	workerManager := workermgr.NewWorkerManager(workerRepo, 5*time.Second, etcdClient)
	err := workerManager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer workerManager.Stop()

	// 创建任务管理器
	masterJobManager := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)
	err = masterJobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer masterJobManager.Stop()

	// 创建Worker
	workerID := testutils.GenerateUniqueID("worker")
	workerRegister, err := testutils.CreateTestWorker(etcdClient, workerID, map[string]string{"env": "test"})
	require.NoError(t, err, "Failed to create test worker")
	defer workerRegister.Stop()

	// 等待Worker被发现
	found := testutils.WaitForCondition(func() bool {
		workers, err := workerManager.GetActiveWorkers()
		if err != nil {
			return false
		}
		for _, w := range workers {
			if w.ID == workerID {
				return true
			}
		}
		return false
	}, 10*time.Second)
	require.True(t, found, "Worker was not discovered")

	// 创建一个长时间运行的任务
	jobFactory := testutils.NewJobFactory()
	// 使用sleep命令创建一个需要运行10秒的任务
	longRunningJob := jobFactory.CreateJobWithCommand("sleep 10")
	longRunningJob.Enabled = true

	// 保存任务
	err = masterJobManager.CreateJob(longRunningJob)
	require.NoError(t, err, "Failed to create long running job")

	// 触发任务执行
	err = masterJobManager.TriggerJob(longRunningJob.ID)
	require.NoError(t, err, "Failed to trigger job")

	// 等待任务进入运行状态
	running := testutils.WaitForCondition(func() bool {
		job, err := masterJobManager.GetJob(longRunningJob.ID)
		return err == nil && job.Status == constants.JobStatusRunning
	}, 10*time.Second)
	require.True(t, running, "Job did not enter running state")

	// 取消任务
	err = masterJobManager.CancelJob(longRunningJob.ID, "Manual test cancellation")
	require.NoError(t, err, "Failed to cancel job")

	// 验证任务状态变为已取消
	cancelled := testutils.WaitForCondition(func() bool {
		job, err := masterJobManager.GetJob(longRunningJob.ID)
		return err == nil && job.Status == constants.JobStatusCancelled
	}, 10*time.Second)
	require.True(t, cancelled, "Job was not cancelled")

	// 验证日志中记录了取消信息
	logUpdated := testutils.WaitForCondition(func() bool {
		logs, count, err := logRepo.GetByJobID(longRunningJob.ID, 1, 10)
		if err != nil || count == 0 {
			return false
		}
		return strings.Contains(logs[0].Output, "cancelled") ||
			logs[0].Status == constants.JobStatusCancelled
	}, 10*time.Second)
	assert.True(t, logUpdated, "Job log did not record cancellation")
}

// TestWorkerResourceUpdate 测试Worker资源更新
func TestWorkerResourceUpdate(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建仓库
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建Worker管理器
	workerManager := workermgr.NewWorkerManager(workerRepo, 5*time.Second, etcdClient)
	err := workerManager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer workerManager.Stop()

	// 创建Worker注册选项，配置较短的资源更新间隔
	workerID := testutils.GenerateUniqueID("worker")
	hostname := "test-host"
	ip := "127.0.0.1"

	options := register.RegisterOptions{
		NodeID:           workerID,
		Hostname:         hostname,
		IP:               ip,
		HeartbeatTimeout: 10,
		HeartbeatTTL:     60,
		ResourceInterval: 2 * time.Second, // 2秒更新一次资源
	}

	// 创建并启动Worker注册器
	worker, err := register.NewWorkerRegister(etcdClient, options)
	require.NoError(t, err, "Failed to create worker register")

	err = worker.Start()
	require.NoError(t, err, "Failed to start worker register")
	defer worker.Stop()

	// 等待Worker被发现
	found := testutils.WaitForCondition(func() bool {
		w, err := workerManager.GetWorker(workerID)
		return err == nil && w != nil
	}, 10*time.Second)
	require.True(t, found, "Worker was not discovered")

	// 记录初始资源信息
	//initialWorker, err := workerManager.GetWorker(workerID)
	//require.NoError(t, err, "Failed to get worker")

	//initialMemFree := initialWorker.MemoryFree
	//initialCPUCores := initialWorker.CPUCores

	// 等待资源更新
	time.Sleep(5 * time.Second)

	// 手动触发资源更新
	err = worker.UpdateResources()
	require.NoError(t, err, "Failed to update resources")

	// 验证资源信息已更新
	updated := testutils.WaitForCondition(func() bool {
		updatedWorker, err := workerManager.GetWorker(workerID)
		if err != nil {
			return false
		}

		// 验证资源信息存在且有合理值
		return updatedWorker.CPUCores > 0 &&
			updatedWorker.MemoryTotal > 0 &&
			updatedWorker.DiskTotal > 0
	}, 10*time.Second)

	require.True(t, updated, "Worker resources were not updated")

	// 获取最新Worker信息
	updatedWorker, err := workerManager.GetWorker(workerID)
	require.NoError(t, err, "Failed to get updated worker")

	// 验证资源值不为零
	assert.Greater(t, updatedWorker.CPUCores, 0, "CPU cores should be greater than 0")
	assert.Greater(t, updatedWorker.MemoryTotal, int64(0), "Total memory should be greater than 0")
	assert.Greater(t, updatedWorker.DiskTotal, int64(0), "Total disk should be greater than 0")
}
