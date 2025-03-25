package master

import (
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

	// 创建Worker仓库
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建Worker管理器 - 使用较短的健康检查间隔以加快测试
	healthCheckInterval := 500 * time.Millisecond
	manager := workermgr.NewWorkerManager(workerRepo, healthCheckInterval, etcdClient)

	// 创建测试Worker
	worker := testutils.CreateWorker("test-worker-health", nil)
	worker.Status = constants.WorkerStatusOnline

	// 注册Worker
	_, err := workerRepo.Register(worker, 2) // 使用较短的TTL以便测试超时
	require.NoError(t, err, "Failed to register worker")

	// 启动Worker管理器
	err = manager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer manager.Stop()

	// 等待Worker管理器处理注册
	time.Sleep(500 * time.Millisecond)

	// 测试健康检查 - 此时应该是健康的
	isHealthy, err := manager.CheckWorkerHealth(worker.ID)
	require.NoError(t, err, "Failed to check worker health")
	assert.True(t, isHealthy, "Worker should be healthy")

	// 等待租约过期 - 这会导致Worker变为离线状态
	time.Sleep(3 * time.Second)

	// 再次测试健康检查 - 此时应该不健康
	isHealthy, err = manager.CheckWorkerHealth(worker.ID)
	require.NoError(t, err, "Failed to check worker health")
	assert.False(t, isHealthy, "Worker should be unhealthy after lease expiration")

	// 验证Worker状态已变为离线
	offlineWorker, err := manager.GetWorker(worker.ID)
	require.NoError(t, err, "Failed to get worker")
	assert.Equal(t, constants.WorkerStatusOffline, offlineWorker.Status, "Worker status should be offline")
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

// TestWorkerMetrics 测试Worker指标收集
func TestWorkerMetrics(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建Worker仓库
	workerRepo := repo.NewWorkerRepo(etcdClient, mongoClient)

	// 创建Worker管理器
	healthCheckInterval := 5 * time.Second
	manager := workermgr.NewWorkerManager(workerRepo, healthCheckInterval, etcdClient)

	// 启动Worker管理器
	err := manager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer manager.Stop()

	// 创建指标管理器
	metricsManager := workermgr.NewMetricsManager(manager, 1*time.Second)

	// 启动指标收集
	metricsManager.Start()
	defer metricsManager.Stop()

	// 创建测试Worker，设置一些资源信息
	worker1 := testutils.CreateWorker("metrics-worker-1", nil)
	worker1.CPUCores = 4
	worker1.MemoryTotal = 8192
	worker1.MemoryFree = 4096
	worker1.DiskTotal = 500
	worker1.DiskFree = 250
	worker1.LoadAvg = 0.2

	worker2 := testutils.CreateWorker("metrics-worker-2", nil)
	worker2.CPUCores = 8
	worker2.MemoryTotal = 16384
	worker2.MemoryFree = 8192
	worker2.DiskTotal = 1000
	worker2.DiskFree = 500
	worker2.LoadAvg = 0.5

	// 注册Worker
	leaseID1, err := workerRepo.Register(worker1, 30)
	require.NoError(t, err, "Failed to register worker 1")
	defer etcdClient.ReleaseLock(leaseID1)

	leaseID2, err := workerRepo.Register(worker2, 30)
	require.NoError(t, err, "Failed to register worker 2")
	defer etcdClient.ReleaseLock(leaseID2)

	// 等待指标收集
	time.Sleep(2 * time.Second)

	// 获取Worker指标
	metrics, err := metricsManager.GetAllWorkerMetrics()
	require.NoError(t, err, "Failed to get worker metrics")
	require.NotNil(t, metrics, "Metrics should not be nil")
	assert.GreaterOrEqual(t, len(metrics), 2, "Should have metrics for at least 2 workers")

	// 验证指标是否正确
	for workerID, metric := range metrics {
		switch workerID {
		case worker1.ID:
			assert.InDelta(t, 50.0, metric.DiskUsageRatio, 1.0, "Disk usage ratio should be around 50%")
			assert.InDelta(t, 50.0, metric.MemoryUsageRatio, 1.0, "Memory usage ratio should be around 50%")
			assert.Equal(t, worker1.LoadAvg, metric.LoadAverage, "Load average should match")
		case worker2.ID:
			assert.InDelta(t, 50.0, metric.DiskUsageRatio, 1.0, "Disk usage ratio should be around 50%")
			assert.InDelta(t, 50.0, metric.MemoryUsageRatio, 1.0, "Memory usage ratio should be around 50%")
			assert.Equal(t, worker2.LoadAvg, metric.LoadAverage, "Load average should match")
		}
	}

	// 测试集群统计信息
	clusterStats, err := metricsManager.GetClusterStats()
	require.NoError(t, err, "Failed to get cluster stats")
	require.NotNil(t, clusterStats, "Cluster stats should not be nil")
	assert.GreaterOrEqual(t, clusterStats["worker_count"], float64(2), "Should have at least 2 workers")
	assert.GreaterOrEqual(t, clusterStats["cpu_cores_total"], float64(12), "Total CPU cores should be at least 12")
	assert.GreaterOrEqual(t, clusterStats["memory_total_mb"], float64(24576), "Total memory should be at least 24576MB")
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

	// 创建Worker管理器
	healthCheckInterval := 5 * time.Second
	manager := workermgr.NewWorkerManager(workerRepo, healthCheckInterval, etcdClient)

	// 创建健康检查器
	healthChecker := workermgr.NewHealthChecker(manager, workerRepo, 500*time.Millisecond, *etcdClient)

	// 创建测试Worker
	worker := testutils.CreateWorker("health-check-worker", nil)
	worker.Status = constants.WorkerStatusOnline

	// 注册Worker
	_, err := workerRepo.Register(worker, 2) // 使用较短的TTL以便测试超时
	require.NoError(t, err, "Failed to register worker")

	// 启动Worker管理器和健康检查器
	err = manager.Start()
	require.NoError(t, err, "Failed to start worker manager")
	defer manager.Stop()

	healthChecker.Start()
	defer healthChecker.Stop()

	// 等待健康检查执行
	time.Sleep(1 * time.Second)

	// 测试获取不健康的Worker列表 - 此时应该为空
	unhealthyWorkers, err := healthChecker.GetUnhealthyWorkers()
	require.NoError(t, err, "Failed to get unhealthy workers")
	assert.Empty(t, unhealthyWorkers, "Should have no unhealthy workers")

	// 等待租约过期，导致Worker变为不健康
	time.Sleep(3 * time.Second)

	// 再次获取不健康的Worker列表 - 应该包含我们的Worker
	unhealthyWorkers, err = healthChecker.GetUnhealthyWorkers()
	require.NoError(t, err, "Failed to get unhealthy workers")
	assert.NotEmpty(t, unhealthyWorkers, "Should have unhealthy workers")

	found := false
	for _, w := range unhealthyWorkers {
		if w.ID == worker.ID {
			found = true
			assert.Equal(t, constants.WorkerStatusOffline, w.Status, "Worker status should be offline")
			break
		}
	}
	assert.True(t, found, "Our worker should be in the unhealthy list")

	// 测试心跳处理
	err = healthChecker.ProcessHeartbeat(worker.ID, map[string]interface{}{
		"memory_free": float64(4096),
		"disk_free":   float64(200),
		"load_avg":    float64(0.3),
	})
	require.NoError(t, err, "Failed to process heartbeat")

	// 等待心跳处理
	time.Sleep(500 * time.Millisecond)

	// 验证Worker状态已恢复
	updatedWorker, err := manager.GetWorker(worker.ID)
	require.NoError(t, err, "Failed to get worker")
	assert.Equal(t, constants.WorkerStatusOnline, updatedWorker.Status, "Worker status should be online after heartbeat")
	assert.Equal(t, int64(4096), updatedWorker.MemoryFree, "Worker memory free should be updated")
	assert.Equal(t, float64(0.3), updatedWorker.LoadAvg, "Worker load average should be updated")
}
