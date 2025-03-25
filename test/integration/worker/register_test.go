package worker

import (
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/register"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkerRegistration 测试Worker注册基本功能
func TestWorkerRegistration(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建Worker注册选项
	workerID := testutils.GenerateUniqueID("worker")
	hostname := "test-host"
	ip := "127.0.0.1"

	options := register.RegisterOptions{
		NodeID:           workerID,
		Hostname:         hostname,
		IP:               ip,
		HeartbeatTimeout: 10, // 10秒超时
		HeartbeatTTL:     60, // 60秒TTL
		ResourceInterval: 5 * time.Second,
	}

	// 创建Worker注册器
	workerRegister, err := register.NewWorkerRegister(etcdClient, options)
	require.NoError(t, err, "Failed to create worker register")

	// 启动注册服务
	err = workerRegister.Start()
	require.NoError(t, err, "Failed to start worker register")
	defer workerRegister.Stop()

	// 验证Worker信息
	worker := workerRegister.GetWorker()
	assert.Equal(t, workerID, worker.ID, "Worker ID should match")
	assert.Equal(t, hostname, worker.Hostname, "Hostname should match")
	assert.Equal(t, ip, worker.IP, "IP should match")
	assert.Equal(t, constants.WorkerStatusOnline, worker.Status, "Worker status should be online")

	// 验证注册状态
	status := workerRegister.GetStatus()
	assert.True(t, status.IsRegistered, "Worker should be registered")
	assert.Equal(t, constants.WorkerStatusOnline, status.State, "Worker state should be online")
	assert.NotZero(t, status.LeaseID, "Lease ID should be non-zero")

	// 测试设置状态
	err = workerRegister.SetStatus(constants.WorkerStatusDisabled)
	require.NoError(t, err, "Failed to set worker status")

	worker = workerRegister.GetWorker()
	assert.Equal(t, constants.WorkerStatusDisabled, worker.Status, "Worker status should be disabled")
}

// TestWorkerHeartbeat 测试Worker心跳功能
func TestWorkerHeartbeat(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建Worker注册选项
	workerID := testutils.GenerateUniqueID("worker")
	options := register.RegisterOptions{
		NodeID:           workerID,
		Hostname:         "test-host",
		IP:               "127.0.0.1",
		HeartbeatTimeout: 10,
		HeartbeatTTL:     60,
		ResourceInterval: 5 * time.Second,
	}

	// 创建Worker注册器
	workerRegister, err := register.NewWorkerRegister(etcdClient, options)
	require.NoError(t, err, "Failed to create worker register")

	// 启动注册服务
	err = workerRegister.Start()
	require.NoError(t, err, "Failed to start worker register")
	defer workerRegister.Stop()

	// 记录初始心跳时间
	initialHeartbeat := workerRegister.GetWorker().LastHeartbeat

	// 等待一段时间
	time.Sleep(2 * time.Second)

	// 手动发送心跳
	err = workerRegister.Heartbeat()
	require.NoError(t, err, "Failed to send heartbeat")

	// 验证心跳时间已更新
	updatedHeartbeat := workerRegister.GetWorker().LastHeartbeat
	assert.True(t, updatedHeartbeat.After(initialHeartbeat), "Heartbeat time should be updated")
}

// TestWorkerResourceUpdate 测试Worker资源更新
func TestWorkerResourceUpdate(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建Worker注册选项
	workerID := testutils.GenerateUniqueID("worker")
	options := register.RegisterOptions{
		NodeID:           workerID,
		Hostname:         "test-host",
		IP:               "127.0.0.1",
		HeartbeatTimeout: 10,
		HeartbeatTTL:     60,
		ResourceInterval: 1 * time.Second, // 短间隔以便测试
	}

	// 创建Worker注册器
	workerRegister, err := register.NewWorkerRegister(etcdClient, options)
	require.NoError(t, err, "Failed to create worker register")

	// 启动注册服务
	err = workerRegister.Start()
	require.NoError(t, err, "Failed to start worker register")
	defer workerRegister.Stop()

	// 手动更新资源
	err = workerRegister.UpdateResources()
	require.NoError(t, err, "Failed to update resources")

	// 验证资源信息
	worker := workerRegister.GetWorker()
	assert.NotZero(t, worker.CPUCores, "CPU cores should be set")
	assert.NotZero(t, worker.MemoryTotal, "Total memory should be set")
	assert.NotZero(t, worker.DiskTotal, "Total disk should be set")

	// 等待资源自动更新
	time.Sleep(1500 * time.Millisecond)

	// 验证资源信息仍然有效
	updatedWorker := workerRegister.GetWorker()
	assert.Equal(t, worker.CPUCores, updatedWorker.CPUCores, "CPU cores should remain consistent")
}

// TestWorkerRunningJobsManagement 测试运行中任务管理
func TestWorkerRunningJobsManagement(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建Worker注册选项
	workerID := testutils.GenerateUniqueID("worker")
	options := register.RegisterOptions{
		NodeID:           workerID,
		Hostname:         "test-host",
		IP:               "127.0.0.1",
		HeartbeatTimeout: 10,
		HeartbeatTTL:     60,
		ResourceInterval: 5 * time.Second,
	}

	// 创建Worker注册器
	workerRegister, err := register.NewWorkerRegister(etcdClient, options)
	require.NoError(t, err, "Failed to create worker register")

	// 启动注册服务
	err = workerRegister.Start()
	require.NoError(t, err, "Failed to start worker register")
	defer workerRegister.Stop()

	// 创建一些测试任务
	jobFactory := testutils.NewJobFactory()
	job1 := jobFactory.CreateSimpleJob()
	job2 := jobFactory.CreateSimpleJob()

	// 添加运行中的任务
	err = workerRegister.AddRunningJob(job1.ID)
	require.NoError(t, err, "Failed to add running job 1")

	err = workerRegister.AddRunningJob(job2.ID)
	require.NoError(t, err, "Failed to add running job 2")

	// 验证运行中的任务列表
	status := workerRegister.GetStatus()
	assert.Equal(t, 2, len(status.RunningJobs), "Should have 2 running jobs")
	assert.Contains(t, status.RunningJobs, job1.ID, "Running jobs should contain job 1")
	assert.Contains(t, status.RunningJobs, job2.ID, "Running jobs should contain job 2")

	// 移除一个任务
	err = workerRegister.RemoveRunningJob(job1.ID)
	require.NoError(t, err, "Failed to remove running job 1")

	// 验证任务已移除
	updatedStatus := workerRegister.GetStatus()
	assert.Equal(t, 1, len(updatedStatus.RunningJobs), "Should have 1 running job")
	assert.NotContains(t, updatedStatus.RunningJobs, job1.ID, "Running jobs should not contain job 1")
	assert.Contains(t, updatedStatus.RunningJobs, job2.ID, "Running jobs should still contain job 2")
}

// TestHealthCheck 测试健康检查功能
func TestHealthCheck(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建Worker注册选项
	workerID := testutils.GenerateUniqueID("worker")
	options := register.RegisterOptions{
		NodeID:           workerID,
		Hostname:         "test-host",
		IP:               "127.0.0.1",
		HeartbeatTimeout: 10,
		HeartbeatTTL:     60,
		ResourceInterval: 5 * time.Second,
	}

	// 创建Worker注册器
	workerRegister, err := register.NewWorkerRegister(etcdClient, options)
	require.NoError(t, err, "Failed to create worker register")

	// 启动注册服务
	err = workerRegister.Start()
	require.NoError(t, err, "Failed to start worker register")
	defer workerRegister.Stop()

	// 创建资源收集器
	resourceCollector := register.NewResourceCollector(5 * time.Second)
	resourceCollector.Start()
	defer resourceCollector.Stop()

	// 创建健康检查器
	healthChecker := register.NewHealthChecker(workerRegister, resourceCollector, 1*time.Second)

	// 启动健康检查
	err = healthChecker.Start()
	require.NoError(t, err, "Failed to start health checker")
	defer healthChecker.Stop()

	// 等待健康检查执行
	time.Sleep(2 * time.Second)

	// 验证健康状态
	assert.True(t, healthChecker.IsHealthy(), "Worker should be healthy")

	// 获取健康状态信息
	healthStatus := healthChecker.GetHealthStatus()
	assert.NotNil(t, healthStatus, "Health status should not be nil")

	// 检查资源信息是否包含在健康状态中
	resourceInfo, exists := healthStatus["resources"]
	assert.True(t, exists, "Health status should include resource information")
	assert.NotNil(t, resourceInfo, "Resource information should not be nil")
}

// TestSignalHandler 测试信号处理
func TestSignalHandler(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建Worker注册选项
	workerID := testutils.GenerateUniqueID("worker")
	options := register.RegisterOptions{
		NodeID:           workerID,
		Hostname:         "test-host",
		IP:               "127.0.0.1",
		HeartbeatTimeout: 10,
		HeartbeatTTL:     60,
		ResourceInterval: 5 * time.Second,
	}

	// 创建Worker注册器
	workerRegister, err := register.NewWorkerRegister(etcdClient, options)
	require.NoError(t, err, "Failed to create worker register")

	// 启动注册服务
	err = workerRegister.Start()
	require.NoError(t, err, "Failed to start worker register")
	defer workerRegister.Stop()

	// 创建资源收集器
	resourceCollector := register.NewResourceCollector(5 * time.Second)
	resourceCollector.Start()
	defer resourceCollector.Stop()

	// 创建健康检查器
	healthChecker := register.NewHealthChecker(workerRegister, resourceCollector, 1*time.Second)
	err = healthChecker.Start()
	require.NoError(t, err, "Failed to start health checker")
	defer healthChecker.Stop()

	// 创建信号处理器
	signalHandler := register.NewSignalHandler(workerRegister, healthChecker, resourceCollector)

	// 启动信号处理器
	signalHandler.Start()
	defer signalHandler.Stop()

	// 模拟手动触发关闭
	// 注意：在实际测试中，我们不会真正调用os.Exit()，所以这里只测试部分功能
	shutdownChannel := make(chan struct{})
	go func() {
		// 只是触发一下，但不实际完成关闭流程避免测试提前退出
		signalHandler.Start() // 再次启动应该无效
		close(shutdownChannel)
	}()

	// 等待异步操作完成
	select {
	case <-shutdownChannel:
		// 成功
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for shutdown handling")
	}
}

// TestIntegratedWorkerLifecycle 测试Worker完整生命周期
func TestIntegratedWorkerLifecycle(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建和启动所有组件
	workerID := testutils.GenerateUniqueID("worker")
	options := register.RegisterOptions{
		NodeID:           workerID,
		Hostname:         "test-host",
		IP:               "127.0.0.1",
		HeartbeatTimeout: 10,
		HeartbeatTTL:     60,
		ResourceInterval: 5 * time.Second,
		Labels: map[string]string{
			"env":    "test",
			"region": "local",
		},
	}

	// 创建Worker注册器
	workerRegister, err := register.NewWorkerRegister(etcdClient, options)
	require.NoError(t, err, "Failed to create worker register")

	// 启动注册服务
	err = workerRegister.Start()
	require.NoError(t, err, "Failed to start worker register")

	// 创建资源收集器
	resourceCollector := register.NewResourceCollector(2 * time.Second)
	resourceCollector.Start()

	// 创建健康检查器
	healthChecker := register.NewHealthChecker(workerRegister, resourceCollector, 1*time.Second)
	err = healthChecker.Start()
	require.NoError(t, err, "Failed to start health checker")

	// 添加一个运行中的任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	err = workerRegister.AddRunningJob(job.ID)
	require.NoError(t, err, "Failed to add running job")

	// 运行一段时间让组件相互交互
	time.Sleep(3 * time.Second)

	// 验证Worker状态
	worker := workerRegister.GetWorker()
	assert.Equal(t, options.NodeID, worker.ID, "Worker ID should match")
	assert.Equal(t, constants.WorkerStatusOnline, worker.Status, "Worker should be online")
	assert.Equal(t, 1, len(worker.RunningJobs), "Worker should have 1 running job")
	assert.NotEmpty(t, worker.Labels, "Worker should have labels")
	assert.Equal(t, "test", worker.Labels["env"], "Worker should have correct labels")

	// 检查健康状态
	assert.True(t, healthChecker.IsHealthy(), "Worker should be healthy")

	// 模拟任务完成
	err = workerRegister.RemoveRunningJob(job.ID)
	require.NoError(t, err, "Failed to remove running job")

	// 验证任务已移除
	worker = workerRegister.GetWorker()
	assert.Equal(t, 0, len(worker.RunningJobs), "Worker should have no running jobs")

	// 正常关闭所有组件
	healthChecker.Stop()
	resourceCollector.Stop()
	workerRegister.Stop()

	// 验证Worker状态已更新为离线
	status := workerRegister.GetStatus()
	assert.Equal(t, constants.WorkerStatusOffline, status.State, "Worker should be offline")
}
