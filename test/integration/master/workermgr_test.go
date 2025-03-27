package master

import (
	"go.mongodb.org/mongo-driver/bson"
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/workermgr"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkerManagerCRUD 测试Worker管理器的基本CRUD操作
func TestWorkerManagerCRUD(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建Worker仓库
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建Worker管理器
	healthCheckInterval := 5 * time.Second
	manager := workermgr.NewWorkerManager(workerRepo, healthCheckInterval, etcdClient)

	// 创建测试Worker
	worker1 := testutils.CreateWorker("test-worker-1", nil)
	worker1.Labels = map[string]string{"env": "test", "region": "us-east"}

	worker2 := testutils.CreateWorker("test-worker-2", nil)
	worker2.Labels = map[string]string{"env": "test", "region": "us-west"}

	// 注册Worker
	leaseID1, err := workerRepo.Register(worker1, 30)
	require.NoError(t, err, "Failed to register worker 1")
	defer etcdClient.ReleaseLock(leaseID1)

	leaseID2, err := workerRepo.Register(worker2, 30)
	require.NoError(t, err, "Failed to register worker 2")
	defer etcdClient.ReleaseLock(leaseID2)

	// 启动Worker管理器
	err = manager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer manager.Stop()

	// 等待Worker管理器处理注册
	time.Sleep(500 * time.Millisecond)

	// 测试获取单个Worker
	retrievedWorker, err := manager.GetWorker(worker1.ID)
	require.NoError(t, err, "Failed to get worker")
	assert.Equal(t, worker1.ID, retrievedWorker.ID, "Worker IDs should match")
	assert.Equal(t, worker1.Status, retrievedWorker.Status, "Worker status should match")
	assert.Equal(t, worker1.Labels["env"], retrievedWorker.Labels["env"], "Worker env label should match")

	// 测试获取所有Worker
	workers, total, err := manager.ListWorkers(1, 10)
	require.NoError(t, err, "Failed to list workers")
	assert.Equal(t, int64(2), total, "Total worker count should be 2")
	assert.Len(t, workers, 2, "Should return 2 workers")

	// 测试禁用Worker
	err = manager.DisableWorker(worker1.ID)
	require.NoError(t, err, "Failed to disable worker")

	// 验证Worker已禁用
	disabledWorker, err := manager.GetWorker(worker1.ID)
	require.NoError(t, err, "Failed to get disabled worker")
	assert.Equal(t, constants.WorkerStatusDisabled, disabledWorker.Status, "Worker should be disabled")

	// 测试启用Worker
	err = manager.EnableWorker(worker1.ID)
	require.NoError(t, err, "Failed to enable worker")

	// 验证Worker已启用
	enabledWorker, err := manager.GetWorker(worker1.ID)
	require.NoError(t, err, "Failed to get enabled worker")
	assert.Equal(t, constants.WorkerStatusOnline, enabledWorker.Status, "Worker should be enabled and online")

	// 测试更新Worker标签
	newLabels := map[string]string{
		"env":    "prod",
		"region": "us-east",
		"tier":   "application",
	}
	err = manager.UpdateWorkerLabels(worker1.ID, newLabels)
	require.NoError(t, err, "Failed to update worker labels")

	// 验证标签已更新
	updatedWorker, err := manager.GetWorker(worker1.ID)
	require.NoError(t, err, "Failed to get worker with updated labels")
	assert.Equal(t, "prod", updatedWorker.Labels["env"], "Worker env label should be updated")
	assert.Equal(t, "application", updatedWorker.Labels["tier"], "Worker tier label should be added")
}

// TestGetActiveWorkers 测试获取活跃的Worker
func TestGetActiveWorkers(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建Worker仓库
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建Worker管理器
	healthCheckInterval := 5 * time.Second
	manager := workermgr.NewWorkerManager(workerRepo, healthCheckInterval, etcdClient)

	// 创建并注册Worker
	// Worker 1 - 在线
	worker1 := testutils.CreateWorker("test-worker-1", nil)
	worker1.Status = constants.WorkerStatusOnline

	// Worker 2 - 在线
	worker2 := testutils.CreateWorker("test-worker-2", nil)
	worker2.Status = constants.WorkerStatusOnline

	// Worker 3 - 已禁用
	worker3 := testutils.CreateWorker("test-worker-3", nil)
	worker3.Status = constants.WorkerStatusDisabled

	// 注册所有Worker
	leaseID1, err := workerRepo.Register(worker1, 30)
	require.NoError(t, err, "Failed to register worker 1")
	defer etcdClient.ReleaseLock(leaseID1)

	leaseID2, err := workerRepo.Register(worker2, 30)
	require.NoError(t, err, "Failed to register worker 2")
	defer etcdClient.ReleaseLock(leaseID2)

	leaseID3, err := workerRepo.Register(worker3, 30)
	require.NoError(t, err, "Failed to register worker 3")
	defer etcdClient.ReleaseLock(leaseID3)

	// 启动Worker管理器
	err = manager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer manager.Stop()

	// 等待Worker管理器处理注册
	time.Sleep(500 * time.Millisecond)

	// 测试获取活跃Worker
	activeWorkers, err := manager.GetActiveWorkers()
	require.NoError(t, err, "Failed to get active workers")

	// 应该只返回两个在线的Worker
	assert.Len(t, activeWorkers, 2, "Should return 2 active workers")

	// 验证返回的Worker是正确的
	workerIDs := make(map[string]bool)
	for _, worker := range activeWorkers {
		workerIDs[worker.ID] = true
		assert.Equal(t, constants.WorkerStatusOnline, worker.Status, "Active worker should be online")
	}
	assert.True(t, workerIDs[worker1.ID], "Worker 1 should be in active workers")
	assert.True(t, workerIDs[worker2.ID], "Worker 2 should be in active workers")
	assert.False(t, workerIDs[worker3.ID], "Worker 3 should not be in active workers")
}

// TestWorkerHealthCheck 测试Worker健康检查功能
func TestWorkerHealthCheck(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建worker仓库
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建具有较短健康检查间隔的worker管理器
	healthCheckInterval := 500 * time.Millisecond
	manager := workermgr.NewWorkerManager(workerRepo, healthCheckInterval, etcdClient)

	// 创建具有当前心跳（不是过去时间）的测试worker
	worker := testutils.CreateWorker("test-worker-health", nil)
	worker.Status = constants.WorkerStatusOnline

	// 使用非常短的TTL注册worker
	_, err := workerRepo.Register(worker, 2) // 2秒TTL
	require.NoError(t, err, "Failed to register worker")

	// 启动worker管理器
	err = manager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer manager.Stop()

	// 验证worker最初是健康的
	isHealthy, err := manager.CheckWorkerHealth(worker.ID)
	require.NoError(t, err, "Failed to check worker health")
	assert.True(t, isHealthy, "Worker should be healthy initially")

	// 现在直接更新MongoDB记录以具有旧的心跳
	// 这模拟了一个长时间未发送心跳的worker
	oldHeartbeat := time.Now().Add(-60 * time.Second)
	filter := bson.M{"id": worker.ID}
	update := bson.M{"$set": bson.M{"last_heartbeat": oldHeartbeat}}
	_, err = mongoClient.UpdateOne("workers", filter, update, nil)
	require.NoError(t, err, "Failed to update worker heartbeat")

	// 同时更新etcd记录以具有相同的旧心跳
	worker.LastHeartbeat = oldHeartbeat
	workerJSON, err := worker.ToJSON()
	require.NoError(t, err, "Failed to serialize worker")
	err = etcdClient.Put(worker.Key(), workerJSON)
	require.NoError(t, err, "Failed to update worker in etcd")

	// 等待健康检查检测到过期的心跳
	time.Sleep(3 * healthCheckInterval)

	// 验证worker现在是不健康的
	isHealthy, err = manager.CheckWorkerHealth(worker.ID)
	require.NoError(t, err, "Failed to check worker health")
	assert.False(t, isHealthy, "Worker should be unhealthy after heartbeat expiration")

	// 获取worker以检查其状态
	updatedWorker, err := manager.GetWorker(worker.ID)
	require.NoError(t, err, "Failed to get worker")
	assert.Equal(t, constants.WorkerStatusOffline, updatedWorker.Status, "Worker status should be offline")
}

// TestWorkerEventHandling 测试Worker事件处理
func TestWorkerEventHandling(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建Worker仓库
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建Worker管理器
	healthCheckInterval := 5 * time.Second
	manager := workermgr.NewWorkerManager(workerRepo, healthCheckInterval, etcdClient)

	// 创建一个通道用于接收事件
	eventCh := make(chan string, 10)

	// 注册事件处理器
	manager.WatchWorkerChanges(func(eventType string, worker *models.Worker) {
		if worker != nil {
			t.Logf("Received event: %s for worker: %s", eventType, worker.ID)
			eventCh <- eventType
		}
	})

	// 启动Worker管理器
	err := manager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer manager.Stop()

	// 创建测试Worker
	worker := testutils.CreateWorker("test-worker-events", nil)

	// 注册Worker - 应该触发ADD事件
	leaseID, err := workerRepo.Register(worker, 30)
	require.NoError(t, err, "Failed to register worker")
	defer etcdClient.ReleaseLock(leaseID)

	// 等待事件处理
	select {
	case eventType := <-eventCh:
		assert.Equal(t, "PUT", eventType, "Should receive ADD event")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for worker ADD event")
	}

	// 更新Worker资源信息 - 应该触发更新事件
	worker.MemoryFree = 2048
	worker.LoadAvg = 0.5
	err = workerRepo.UpdateWorkerResources(worker)
	require.NoError(t, err, "Failed to update worker resources")

	// 等待事件处理
	select {
	case eventType := <-eventCh:
		assert.Equal(t, "PUT", eventType, "Should receive UPDATE event")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for worker UPDATE event")
	}
}

// TestWorkerDiscovery 测试Worker发现功能
func TestWorkerDiscovery(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建Worker仓库
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建Worker管理器
	healthCheckInterval := 5 * time.Second
	manager := workermgr.NewWorkerManager(workerRepo, healthCheckInterval, etcdClient)

	// 创建Worker发现器
	discovery := workermgr.NewWorkerDiscovery(manager, workerRepo, etcdClient, mongoClient)

	// 启动Worker发现器
	discovery.Start()

	// 等待Worker发现器初始化
	time.Sleep(500 * time.Millisecond)

	// 创建测试Worker
	worker1 := testutils.CreateWorker("discovery-worker-1", nil)
	worker2 := testutils.CreateWorker("discovery-worker-2", nil)

	// 注册Worker
	leaseID1, err := workerRepo.Register(worker1, 30)
	require.NoError(t, err, "Failed to register worker 1")
	defer etcdClient.ReleaseLock(leaseID1)

	// 等待Worker发现
	time.Sleep(500 * time.Millisecond)

	// 验证Worker已被发现
	discoveredWorkers := discovery.GetDiscoveredWorkers()
	require.GreaterOrEqual(t, len(discoveredWorkers), 1, "Should discover at least 1 worker")

	found := false
	for _, w := range discoveredWorkers {
		if w.ID == worker1.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "Worker 1 should be discovered")

	// 注册第二个Worker
	leaseID2, err := workerRepo.Register(worker2, 30)
	require.NoError(t, err, "Failed to register worker 2")
	defer etcdClient.ReleaseLock(leaseID2)

	// 等待Worker发现
	time.Sleep(500 * time.Millisecond)

	// 验证第二个Worker也被发现
	discoveredWorkers = discovery.GetDiscoveredWorkers()
	require.GreaterOrEqual(t, len(discoveredWorkers), 2, "Should discover at least 2 workers")

	// 验证可以通过ID获取Worker
	worker, err := discovery.GetWorkerByID(worker2.ID)
	require.NoError(t, err, "Failed to get worker by ID")
	assert.Equal(t, worker2.ID, worker.ID, "Worker IDs should match")

	// 验证可以通过状态获取Worker
	onlineWorkers := discovery.GetWorkersByStatus(constants.WorkerStatusOnline)
	assert.GreaterOrEqual(t, len(onlineWorkers), 2, "Should have at least 2 online workers")
}

// TestHealthChecker 测试健康检查器
func TestHealthChecker(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建Worker仓库
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建Worker管理器，使用非常短的健康检查间隔
	healthCheckInterval := 500 * time.Millisecond
	manager := workermgr.NewWorkerManager(workerRepo, healthCheckInterval, etcdClient)

	// 创建测试Worker，使用旧的心跳时间（60秒前）
	worker := testutils.CreateWorker("health-check-worker", nil)
	oldHeartbeat := time.Now().Add(-60 * time.Second)
	worker.LastHeartbeat = oldHeartbeat
	worker.Status = constants.WorkerStatusOnline

	// 注册Worker，使用短TTL
	leaseID, err := workerRepo.Register(worker, 10) // 10秒TTL
	require.NoError(t, err, "Failed to register worker")
	defer etcdClient.ReleaseLock(leaseID)

	// 启动Worker管理器
	err = manager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer manager.Stop()

	// 等待健康检查运行并将worker标记为离线
	time.Sleep(3 * healthCheckInterval)

	// 验证worker已被标记为offline
	updatedWorker, err := manager.GetWorker(worker.ID)
	require.NoError(t, err, "Failed to get worker")
	assert.Equal(t, constants.WorkerStatusOffline, updatedWorker.Status,
		"Worker should be marked as offline due to stale heartbeat")

	// 验证etcd和MongoDB中的数据一致 - 这是新增的一致性检测
	err = workerRepo.CheckConsistency(worker.ID)
	require.NoError(t, err, "Consistency check failed")

	// 查询不健康的workers
	workers, err := workerRepo.ListAll()
	require.NoError(t, err, "Failed to list all workers")

	var unhealthyWorkers []*models.Worker
	for _, w := range workers {
		if w.Status == constants.WorkerStatusOffline {
			unhealthyWorkers = append(unhealthyWorkers, w)
		}
	}

	assert.NotEmpty(t, unhealthyWorkers, "Should have our unhealthy worker listed")

	// 验证不健康的worker就是我们的测试worker
	if len(unhealthyWorkers) > 0 {
		assert.Equal(t, worker.ID, unhealthyWorkers[0].ID,
			"The unhealthy worker should be our test worker")
	}

	// 现在模拟一个心跳以使Worker恢复在线
	worker.LastHeartbeat = time.Now() // 更新心跳时间
	err = workerRepo.Heartbeat(worker)
	require.NoError(t, err, "Failed to update worker heartbeat")

	// 等待健康检查检测到worker重新上线
	time.Sleep(3 * healthCheckInterval)

	// 验证worker现在已恢复在线
	updatedWorker, err = manager.GetWorker(worker.ID)
	require.NoError(t, err, "Failed to get worker")
	assert.Equal(t, constants.WorkerStatusOnline, updatedWorker.Status,
		"Worker should be back online after heartbeat")

	// 再次验证一致性
	err = workerRepo.CheckConsistency(worker.ID)
	require.NoError(t, err, "Consistency check failed after status change")
}
