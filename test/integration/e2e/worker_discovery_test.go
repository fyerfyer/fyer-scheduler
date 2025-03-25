package e2e

import (
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/workermgr"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试Worker发现功能
func TestWorkerDiscovery(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 清理可能存在的测试数据
	err := testutils.CleanEtcdPrefix(etcdClient, constants.WorkerPrefix)
	require.NoError(t, err, "Failed to clean etcd prefix")

	// 创建WorkerRepo
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建WorkerManager
	workerManager := workermgr.NewWorkerManager(workerRepo, 5*time.Second, etcdClient)
	err = workerManager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer workerManager.Stop()

	// 注册Worker监听
	workerEvents := make([]string, 0)
	workerManager.WatchWorkerChanges(func(eventType string, worker *models.Worker) {
		workerEvents = append(workerEvents, eventType)
	})

	// 创建测试Worker
	worker1ID := testutils.GenerateUniqueID("worker")
	worker2ID := testutils.GenerateUniqueID("worker")

	// 创建并注册第一个Worker
	testWorker1, err := testutils.CreateTestWorker(etcdClient, worker1ID, map[string]string{"env": "test", "role": "backend"})
	require.NoError(t, err, "Failed to create test worker 1")

	// 等待Worker被发现
	found := testutils.WaitForCondition(func() bool {
		activeWorkers, err := workerManager.GetActiveWorkers()
		if err != nil {
			return false
		}

		for _, w := range activeWorkers {
			if w.ID == worker1ID {
				return true
			}
		}
		return false
	}, 5*time.Second)

	require.True(t, found, "Worker 1 was not discovered")

	// 检查Worker1被正确发现
	worker1, err := workerManager.GetWorker(worker1ID)
	require.NoError(t, err, "Failed to get worker 1")
	assert.Equal(t, worker1ID, worker1.ID, "Worker ID mismatch")
	assert.Equal(t, constants.WorkerStatusOnline, worker1.Status, "Worker status should be online")
	assert.Equal(t, "test", worker1.Labels["env"], "Worker label mismatch")

	// 创建并注册第二个Worker
	testWorker2, err := testutils.CreateTestWorker(etcdClient, worker2ID, map[string]string{"env": "test", "role": "frontend"})
	require.NoError(t, err, "Failed to create test worker 2")

	// 等待第二个Worker被发现
	found = testutils.WaitForCondition(func() bool {
		activeWorkers, err := workerManager.GetActiveWorkers()
		if err != nil {
			return false
		}

		foundCount := 0
		for _, w := range activeWorkers {
			if w.ID == worker1ID || w.ID == worker2ID {
				foundCount++
			}
		}
		return foundCount == 2
	}, 5*time.Second)

	require.True(t, found, "Worker 2 was not discovered")

	// 获取所有活跃Worker
	activeWorkers, err := workerManager.GetActiveWorkers()
	require.NoError(t, err, "Failed to get active workers")
	assert.GreaterOrEqual(t, len(activeWorkers), 2, "Should have at least 2 active workers")

	// 测试禁用Worker功能
	err = workerManager.DisableWorker(worker1ID)
	require.NoError(t, err, "Failed to disable worker")

	// 等待Worker状态更新
	updated := testutils.WaitForCondition(func() bool {
		w, err := workerManager.GetWorker(worker1ID)
		if err != nil {
			return false
		}
		return w.Status == constants.WorkerStatusDisabled
	}, 5*time.Second)

	require.True(t, updated, "Worker status was not updated to disabled")

	// 检查Worker状态已更新
	disabledWorker, err := workerManager.GetWorker(worker1ID)
	require.NoError(t, err, "Failed to get disabled worker")
	assert.Equal(t, constants.WorkerStatusDisabled, disabledWorker.Status, "Worker status should be disabled")

	// 测试更新Worker标签
	err = workerManager.UpdateWorkerLabels(worker2ID, map[string]string{
		"env":  "production",
		"role": "frontend",
		"tier": "web",
	})
	require.NoError(t, err, "Failed to update worker labels")

	// 等待标签更新
	labelsUpdated := testutils.WaitForCondition(func() bool {
		w, err := workerManager.GetWorker(worker2ID)
		if err != nil {
			return false
		}
		return w.Labels["env"] == "production" && w.Labels["tier"] == "web"
	}, 5*time.Second)

	require.True(t, labelsUpdated, "Worker labels were not updated")

	// 验证标签已更新
	updatedWorker, err := workerManager.GetWorker(worker2ID)
	require.NoError(t, err, "Failed to get updated worker")
	assert.Equal(t, "production", updatedWorker.Labels["env"], "Worker env label mismatch")
	assert.Equal(t, "web", updatedWorker.Labels["tier"], "Worker tier label mismatch")

	// 测试重新启用Worker
	err = workerManager.EnableWorker(worker1ID)
	require.NoError(t, err, "Failed to enable worker")

	// 等待Worker状态更新为启用
	reEnabled := testutils.WaitForCondition(func() bool {
		w, err := workerManager.GetWorker(worker1ID)
		if err != nil {
			return false
		}
		return w.Status == constants.WorkerStatusOnline
	}, 5*time.Second)

	require.True(t, reEnabled, "Worker status was not updated to online")

	// 测试检查Worker健康状态
	healthy, err := workerManager.CheckWorkerHealth(worker2ID)
	require.NoError(t, err, "Failed to check worker health")
	assert.True(t, healthy, "Worker should be healthy")

	// 停止Worker并验证状态变化
	err = testWorker1.Stop()
	require.NoError(t, err, "Failed to stop worker 1")

	// 等待Worker状态更新为离线
	offline := testutils.WaitForCondition(func() bool {
		w, err := workerManager.GetWorker(worker1ID)
		if err != nil {
			return false
		}
		return w.Status == constants.WorkerStatusOffline
	}, 10*time.Second)

	require.True(t, offline, "Worker status was not updated to offline")

	// 获取所有Worker并检查数量
	allWorkers, total, err := workerManager.ListWorkers(1, 10)
	require.NoError(t, err, "Failed to list workers")
	assert.GreaterOrEqual(t, total, int64(2), "Should have at least 2 workers")
	assert.GreaterOrEqual(t, len(allWorkers), 2, "Should have at least 2 workers in result")

	// 清理
	err = testWorker2.Stop()
	require.NoError(t, err, "Failed to stop worker 2")
}

// 测试Worker资源收集和报告功能
func TestWorkerResourceReporting(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 清理可能存在的测试数据
	err := testutils.CleanEtcdPrefix(etcdClient, constants.WorkerPrefix)
	require.NoError(t, err, "Failed to clean etcd prefix")

	// 创建WorkerRepo
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建WorkerManager
	workerManager := workermgr.NewWorkerManager(workerRepo, 5*time.Second, etcdClient)
	err = workerManager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer workerManager.Stop()

	// 创建带资源收集的测试Worker
	workerID := testutils.GenerateUniqueID("worker")
	testWorker, err := testutils.CreateTestWorker(etcdClient, workerID, map[string]string{"env": "test"})
	require.NoError(t, err, "Failed to create test worker")

	// 等待Worker被发现
	found := testutils.WaitForCondition(func() bool {
		w, err := workerManager.GetWorker(workerID)
		return err == nil && w != nil
	}, 5*time.Second)

	require.True(t, found, "Worker was not discovered")

	// 验证Worker资源信息已收集
	worker, err := workerManager.GetWorker(workerID)
	require.NoError(t, err, "Failed to get worker")

	// 检查基本资源信息是否存在
	assert.Greater(t, worker.CPUCores, 0, "CPU cores should be reported")
	assert.Greater(t, worker.MemoryTotal, int64(0), "Memory total should be reported")
	assert.Greater(t, worker.DiskTotal, int64(0), "Disk total should be reported")

	// 等待资源信息更新
	time.Sleep(3 * time.Second)

	// 再次检查资源信息是否更新
	updatedWorker, err := workerManager.GetWorker(workerID)
	require.NoError(t, err, "Failed to get updated worker")

	assert.Greater(t, updatedWorker.CPUCores, 0, "CPU cores should be reported")
	assert.Greater(t, updatedWorker.MemoryTotal, int64(0), "Memory total should be reported")
	assert.GreaterOrEqual(t, updatedWorker.MemoryTotal, updatedWorker.MemoryFree, "Memory free should be less than or equal to total memory")
	assert.Greater(t, updatedWorker.DiskTotal, int64(0), "Disk total should be reported")
	assert.GreaterOrEqual(t, updatedWorker.DiskTotal, updatedWorker.DiskFree, "Disk free should be less than or equal to total disk")

	// 清理
	err = testWorker.Stop()
	require.NoError(t, err, "Failed to stop worker")
}
