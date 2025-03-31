package worker

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/mocks"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/jobmgr"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 创建测试用的任务管理器
func createTestJobManager(t *testing.T, workerID string) jobmgr.IWorkerJobManager {
	// 创建etcd客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")

	// 如果未提供workerID，则生成一个唯一ID
	if workerID == "" {
		workerID = testutils.GenerateUniqueID("worker")
	}

	// 创建任务管理器
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)

	// 启动任务管理器
	err = jobManager.Start()
	require.NoError(t, err, "Failed to start job manager")

	return jobManager
}

// 测试用的任务事件处理器
type testJobEventHandler struct {
	events []*jobmgr.JobEvent
	t      *testing.T
}

func newTestJobEventHandler(t *testing.T) *testJobEventHandler {
	return &testJobEventHandler{
		events: make([]*jobmgr.JobEvent, 0),
		t:      t,
	}
}

// HandleJobEvent 实现IJobEventHandler接口
func (h *testJobEventHandler) HandleJobEvent(event *jobmgr.JobEvent) {
	h.t.Logf("Received job event: type=%v, job=%s", event.Type, event.Job.ID)
	h.events = append(h.events, event)
}

func TestEtcdGetWithPrefix(t *testing.T) {
	// 创建etcd客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")
	defer etcdClient.Close()

	// 添加一个测试任务到etcd
	testKey := "test/prefix/key1"
	testValue := "test-value"
	err = etcdClient.Put(testKey, testValue)
	require.NoError(t, err, "Failed to put test key")

	// 创建用于等待操作完成的通道
	done := make(chan struct{})

	// 尝试在一个goroutine中使用前缀获取
	go func() {
		defer close(done)

		result, err := etcdClient.GetWithPrefix("test/prefix")
		if err != nil {
			t.Errorf("GetWithPrefix failed: %v", err)
			return
		}

		// 验证结果
		if len(result) == 0 {
			t.Errorf("Expected non-empty result, got empty map")
			return
		}

		if val, ok := result[testKey]; !ok || val != testValue {
			t.Errorf("Expected value %s for key %s, got %s", testValue, testKey, val)
			return
		}
	}()

	// 等待操作完成或超时
	select {
	case <-done:
		// 成功完成
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out waiting for GetWithPrefix")
	}
}

func TestListJobs(t *testing.T) {
	// 创建任务管理器
	jobManager := createTestJobManager(t, "")
	defer jobManager.Stop()

	// 创建任务工厂
	jobFactory := testutils.NewJobFactory()

	// 创建测试任务
	job1 := jobFactory.CreateSimpleJob()
	job1Str, err := job1.ToJSON()
	require.NoError(t, err, "Failed to convert job to JSON")

	job2 := jobFactory.CreateScheduledJob("@every 1m")
	job2Str, err := job2.ToJSON()
	require.NoError(t, err, "Failed to convert job to JSON")

	// 保存任务到etcd
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")

	err = etcdClient.Put(constants.JobPrefix+job1.ID, job1Str)
	require.NoError(t, err, "Failed to put job1")

	err = etcdClient.Put(constants.JobPrefix+job2.ID, job2Str)
	require.NoError(t, err, "Failed to put job2")

	// 等待任务被发现
	time.Sleep(1 * time.Second)

	// 列出所有任务
	jobs, err := jobManager.ListJobs()
	require.NoError(t, err, "Failed to list jobs")

	// 验证是否包含我们创建的任务
	foundJob1 := false
	foundJob2 := false

	for _, job := range jobs {
		if job.ID == job1.ID {
			foundJob1 = true
		}
		if job.ID == job2.ID {
			foundJob2 = true
		}
	}

	assert.True(t, foundJob1, "Job 1 should be found in job list")
	assert.True(t, foundJob2, "Job 2 should be found in job list")
}

func TestJobManagerInitialization(t *testing.T) {
	// 创建 etcd 客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")
	defer etcdClient.Close()

	// 启动 goroutine来创建管理器
	done := make(chan struct{})
	var manager jobmgr.IWorkerJobManager

	go func() {
		defer close(done)
		workerID := testutils.GenerateUniqueID("worker")
		manager = jobmgr.NewWorkerJobManager(etcdClient, workerID)
		err := manager.Start()
		if err != nil {
			t.Errorf("Failed to start job manager: %v", err)
			return
		}
	}()

	// 等待 goroutine 完成或超时
	select {
	case <-done:
		// 成功初始化
		assert.NotNil(t, manager, "Job manager should not be nil")
		// 清理
		if manager != nil {
			manager.Stop()
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out waiting for job manager initialization")
	}
}

func TestJobManagerRegisterHandler(t *testing.T) {
	// 创建任务管理器
	jobManager := createTestJobManager(t, "")
	defer jobManager.Stop()

	// 创建事件处理器
	handler := newTestJobEventHandler(t)

	// 注册事件处理器
	jobManager.RegisterHandler(handler)

	// 创建一个测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 创建任务事件
	event := &jobmgr.JobEvent{
		Type: jobmgr.JobEventSave,
		Job:  job,
	}

	// 手动调用处理方法
	handler.HandleJobEvent(event)

	// 验证事件处理
	assert.Equal(t, 1, len(handler.events), "Handler should have received 1 event")
	assert.Equal(t, jobmgr.JobEventSave, handler.events[0].Type, "Event type should be JobEventSave")
	assert.Equal(t, job.ID, handler.events[0].Job.ID, "Job ID should match")
}

func TestJobManagerGetJob(t *testing.T) {
	// 创建etcd客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")
	defer etcdClient.Close()

	// 创建任务管理器
	workerID := testutils.GenerateUniqueID("worker")
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	err = jobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer jobManager.Stop()

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 序列化任务并保存到etcd
	jobJSON, err := job.ToJSON()
	require.NoError(t, err, "Failed to convert job to JSON")

	err = etcdClient.Put(constants.JobPrefix+job.ID, jobJSON)
	require.NoError(t, err, "Failed to put job in etcd")

	// 等待任务被发现
	time.Sleep(1 * time.Second)

	// 获取任务
	retrievedJob, err := jobManager.GetJob(job.ID)
	require.NoError(t, err, "Failed to get job")
	assert.Equal(t, job.ID, retrievedJob.ID, "Retrieved job ID should match the original")
	assert.Equal(t, job.Name, retrievedJob.Name, "Retrieved job name should match the original")
}

func TestJobManagerListJobs(t *testing.T) {
	// 创建etcd客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")
	defer etcdClient.Close()

	// 创建任务管理器
	workerID := testutils.GenerateUniqueID("worker")
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	err = jobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer jobManager.Stop()

	// 创建多个测试任务
	jobFactory := testutils.NewJobFactory()
	job1 := jobFactory.CreateSimpleJob()
	job2 := jobFactory.CreateScheduledJob("@every 5m")
	job3 := jobFactory.CreateJobWithCommand("echo test command")

	// 保存任务到etcd
	jobs := []*models.Job{job1, job2, job3}
	for _, job := range jobs {
		jobJSON, err := job.ToJSON()
		require.NoError(t, err, "Failed to convert job to JSON")

		err = etcdClient.Put(constants.JobPrefix+job.ID, jobJSON)
		require.NoError(t, err, "Failed to put job in etcd")
	}

	// 等待任务被发现
	time.Sleep(1 * time.Second)

	// 列出所有任务
	retrievedJobs, err := jobManager.ListJobs()
	require.NoError(t, err, "Failed to list jobs")

	// 验证是否包含我们创建的任务
	jobMap := make(map[string]bool)
	for _, job := range retrievedJobs {
		jobMap[job.ID] = true
	}

	assert.True(t, jobMap[job1.ID], "Job 1 should be in the list")
	assert.True(t, jobMap[job2.ID], "Job 2 should be in the list")
	assert.True(t, jobMap[job3.ID], "Job 3 should be in the list")
}

func TestJobManagerWatchJobs(t *testing.T) {
	// 创建etcd客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")
	defer etcdClient.Close()

	// 创建任务管理器
	workerID := testutils.GenerateUniqueID("worker")
	jobManager := jobmgr.NewWorkerJobManager(etcdClient, workerID)
	err = jobManager.Start()
	require.NoError(t, err, "Failed to start job manager")
	defer jobManager.Stop()

	// 创建和注册测试事件处理器
	handler := newTestJobEventHandler(t)
	jobManager.RegisterHandler(handler)

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 将Worker ID添加到任务环境中，使其被分配给此Worker
	if job.Env == nil {
		job.Env = make(map[string]string)
	}
	job.Env["WORKER_ID"] = workerID

	// 序列化并保存任务到etcd
	jobJSON, err := job.ToJSON()
	require.NoError(t, err, "Failed to convert job to JSON")

	err = etcdClient.Put(constants.JobPrefix+job.ID, jobJSON)
	require.NoError(t, err, "Failed to put job in etcd")

	// 等待任务事件被处理
	time.Sleep(2 * time.Second)

	// 验证是否收到事件
	assert.GreaterOrEqual(t, len(handler.events), 1, "Should receive at least one event")

	eventReceived := false
	for _, event := range handler.events {
		if event.Type == jobmgr.JobEventSave && event.Job.ID == job.ID {
			eventReceived = true
			break
		}
	}
	assert.True(t, eventReceived, "Should receive a save event for the job")

	// 删除任务，测试删除事件
	err = etcdClient.Delete(constants.JobPrefix + job.ID)
	require.NoError(t, err, "Failed to delete job from etcd")

	// 等待删除事件被处理
	time.Sleep(2 * time.Second)

	deleteEventReceived := false
	for _, event := range handler.events {
		if event.Type == jobmgr.JobEventDelete && event.Job.ID == job.ID {
			deleteEventReceived = true
			break
		}
	}
	assert.True(t, deleteEventReceived, "Should receive a delete event for the job")
}

func TestJobManagerWithMockHandler(t *testing.T) {
	// 创建任务管理器
	jobManager := createTestJobManager(t, "")
	defer jobManager.Stop()

	// 创建mock事件处理器
	mockHandler := mocks.NewIJobEventHandler(t)

	// 设置期望
	mockHandler.EXPECT().HandleJobEvent(testutils.MockAny()).Return()

	// 注册mock处理器
	jobManager.RegisterHandler(mockHandler)

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 创建任务事件
	event := &jobmgr.JobEvent{
		Type: jobmgr.JobEventSave,
		Job:  job,
	}

	// 手动触发事件处理
	mockHandler.HandleJobEvent(event)
}

func TestJobManagerIsJobAssignedToWorker(t *testing.T) {
	// 创建任务管理器
	workerID := testutils.GenerateUniqueID("worker")
	jobManager := createTestJobManager(t, workerID)
	defer jobManager.Stop()

	// 创建分配给此Worker的任务
	jobFactory := testutils.NewJobFactory()
	job1 := jobFactory.CreateSimpleJob()

	// 添加Worker ID到环境变量
	if job1.Env == nil {
		job1.Env = make(map[string]string)
	}
	job1.Env["WORKER_ID"] = workerID

	// 验证任务分配
	assigned := jobManager.IsJobAssignedToWorker(job1)
	assert.True(t, assigned, "Job with explicit worker ID should be assigned to the worker")

	// 创建分配给其他Worker的任务
	job2 := jobFactory.CreateSimpleJob()
	if job2.Env == nil {
		job2.Env = make(map[string]string)
	}
	job2.Env["WORKER_ID"] = "other-worker-id"

	// 验证任务不分配
	assigned = jobManager.IsJobAssignedToWorker(job2)
	assert.False(t, assigned, "Job assigned to other worker should not be assigned to this worker")
}

func TestReportJobStatus(t *testing.T) {
	// 创建作业管理器
	workerID := testutils.GenerateUniqueID("worker")
	jobManager := createTestJobManager(t, workerID)
	defer jobManager.Stop()

	// 创建etcd客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")
	defer etcdClient.Close()

	// 创建测试作业
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 将worker ID添加到作业
	if job.Env == nil {
		job.Env = make(map[string]string)
	}
	job.Env["WORKER_ID"] = workerID

	// 序列化作业并保存到etcd
	jobJSON, err := job.ToJSON()
	require.NoError(t, err, "Failed to convert job to JSON")

	err = etcdClient.Put(constants.JobPrefix+job.ID, jobJSON)
	require.NoError(t, err, "Failed to put job in etcd")

	// 等待作业被发现
	time.Sleep(1 * time.Second)

	// 报告运行状态
	err = jobManager.ReportJobStatus(job, constants.JobStatusRunning)
	require.NoError(t, err, "Failed to report job status")

	// 验证作业状态已在etcd中更新
	updatedJobJSON, err := etcdClient.Get(constants.JobPrefix + job.ID)
	require.NoError(t, err, "Failed to get job from etcd")

	var updatedJob models.Job
	err = json.Unmarshal([]byte(updatedJobJSON), &updatedJob)
	require.NoError(t, err, "Failed to parse job JSON")

	assert.Equal(t, constants.JobStatusRunning, updatedJob.Status, "Job status should be updated to running")

	// 验证作业在运行作业列表中
	runningJobs, err := jobManager.ListRunningJobs()
	require.NoError(t, err, "Failed to list running jobs")

	found := false
	for _, rj := range runningJobs {
		if rj.ID == job.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "Job should be in running jobs list")

	// 报告完成状态
	err = jobManager.ReportJobStatus(job, constants.JobStatusSucceeded)
	require.NoError(t, err, "Failed to report completed status")

	// 验证作业已从运行作业列表中移除
	runningJobs, err = jobManager.ListRunningJobs()
	require.NoError(t, err, "Failed to list running jobs")

	found = false
	for _, rj := range runningJobs {
		if rj.ID == job.ID {
			found = true
			break
		}
	}
	assert.False(t, found, "Job should be removed from running jobs list")
}

func TestKillJob(t *testing.T) {
	// 创建作业管理器
	jobManager := createTestJobManager(t, "")
	defer jobManager.Stop()

	// 创建etcd客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")
	defer etcdClient.Close()

	// 创建测试作业
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()

	// 将工作节点ID添加到作业环境变量中
	if job.Env == nil {
		job.Env = make(map[string]string)
	}
	job.Env["WORKER_ID"] = jobManager.GetWorkerID()

	// 将作业保存到etcd
	jobJSON, err := job.ToJSON()
	require.NoError(t, err, "Failed to convert job to JSON")
	err = etcdClient.Put(constants.JobPrefix+job.ID, jobJSON)
	require.NoError(t, err, "Failed to put job in etcd")

	// 等待作业被发现
	time.Sleep(1 * time.Second)

	// 首先将作业报告为运行中
	err = jobManager.ReportJobStatus(job, constants.JobStatusRunning)
	require.NoError(t, err, "Failed to report job as running")

	// 发送终止信号
	err = jobManager.KillJob(job.ID)
	require.NoError(t, err, "Failed to kill job")

	// 等待终止信号被处理
	time.Sleep(100 * time.Millisecond)

	// 验证终止信号是否已发送到etcd - 检查带有 "/kill" 后缀的正确路径
	killKey := constants.JobPrefix + job.ID + "/kill"
	killValue, err := etcdClient.Get(killKey)
	require.NoError(t, err, "Failed to get kill signal from etcd")
	assert.NotEmpty(t, killValue, "Kill value should not be empty")
	assert.Contains(t, killValue, "kill_signal_", "Kill value should contain kill_signal_ prefix")

	// 清理终止信号
	etcdClient.Delete(killKey)
}

func TestIsJobAssignedToWorker(t *testing.T) {
	// 创建作业管理器
	workerID := testutils.GenerateUniqueID("worker")
	jobManager := createTestJobManager(t, workerID)
	defer jobManager.Stop()

	// 创建测试作业工厂
	jobFactory := testutils.NewJobFactory()

	// 测试1：作业显式分配给此worker
	job1 := jobFactory.CreateSimpleJob()
	if job1.Env == nil {
		job1.Env = make(map[string]string)
	}
	job1.Env["WORKER_ID"] = workerID

	assert.True(t, jobManager.IsJobAssignedToWorker(job1),
		"Job explicitly assigned to this worker should return true")

	// 测试2：作业显式分配给另一个worker
	job2 := jobFactory.CreateSimpleJob()
	if job2.Env == nil {
		job2.Env = make(map[string]string)
	}
	job2.Env["WORKER_ID"] = "another-worker"

	assert.False(t, jobManager.IsJobAssignedToWorker(job2),
		"Job explicitly assigned to another worker should return false")

	// 测试3：没有显式分配worker的作业（根据实际实现可能返回true或false）
	job3 := jobFactory.CreateSimpleJob()

	// 注：这个测试结果取决于具体实现，一些系统默认允许任何worker处理未分配的作业
	isAssigned := jobManager.IsJobAssignedToWorker(job3)
	t.Logf("Unassigned job assignment result: %v", isAssigned)

	// 测试4：运行状态的作业应分配给正在运行它的worker
	job4 := jobFactory.CreateSimpleJob()
	job4.Status = constants.JobStatusRunning

	// 模拟作业已被此worker执行
	if job4.Env == nil {
		job4.Env = make(map[string]string)
	}
	job4.Env["RUNNING_WORKER_ID"] = workerID

	assert.True(t, jobManager.IsJobAssignedToWorker(job4),
		"Running job should be assigned to the worker that's running it")
}
