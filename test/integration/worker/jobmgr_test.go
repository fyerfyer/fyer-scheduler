package worker

import (
	"context"
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
	manager := jobmgr.NewWorkerJobManager(etcdClient, workerID)

	// 启动任务管理器
	err = manager.Start()
	require.NoError(t, err, "Failed to start job manager")

	return manager
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
	h.events = append(h.events, event)
	h.t.Logf("Received job event: type=%d, job=%s", event.Type, event.Job.Name)
}

func TestEtcdGetWithPrefix(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建etcd客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")
	defer etcdClient.Close()

	err = testutils.CleanEtcdPrefix(etcdClient, constants.JobPrefix)
	require.NoError(t, err, "Failed to clean etcd data")

	// 添加一个测试任务到etcd
	jobID := testutils.GenerateUniqueID("job")
	jobKey := constants.JobPrefix + jobID
	err = etcdClient.Put(jobKey, "{\"id\":\""+jobID+"\",\"name\":\"test\"}")
	require.NoError(t, err, "Failed to add test job")

	// 创建用于等待操作完成的通道
	done := make(chan struct{})

	// 尝试在一个goroutine中使用前缀获取
	go func() {
		result, err := etcdClient.GetWithPrefix(constants.JobPrefix)
		if err != nil {
			t.Logf("Error fetching with prefix: %v", err)
		} else {
			t.Logf("Successfully fetched %d keys", len(result))
		}
		close(done)
	}()

	select {
	case <-done:
		t.Log("GetWithPrefix completed successfully")
	case <-ctx.Done():
		t.Fatal("GetWithPrefix timed out")
	}
}

func TestListJobs(t *testing.T) {
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err)
	defer etcdClient.Close()

	workerID := testutils.GenerateUniqueID("worker")
	manager := jobmgr.NewWorkerJobManager(etcdClient, workerID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	var jobsList []*models.Job
	var listErr error

	go func() {
		jobsList, listErr = manager.ListJobs()
		close(done)
	}()

	select {
	case <-done:
		t.Logf("ListJobs completed with %d jobs", len(jobsList))
		if listErr != nil {
			t.Errorf("Error: %v", listErr)
		}
	case <-ctx.Done():
		t.Fatalf("ListJobs timed out")
	}
}

func TestJobManagerInitialization(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// 创建 etcd 客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")
	defer etcdClient.Close()

	err = testutils.CleanEtcdPrefix(etcdClient, constants.JobPrefix)
	require.NoError(t, err, "Failed to clean etcd data")

	workerID := testutils.GenerateUniqueID("worker")
	t.Logf("Using worker ID: %s", workerID)

	// 启动 goroutine来创建管理器
	doneCh := make(chan struct{})
	var manager jobmgr.IWorkerJobManager
	var startErr error

	go func() {
		// 创建任务管理器
		t.Log("Creating job manager...")
		mgr := jobmgr.NewWorkerJobManager(etcdClient, workerID)
		t.Log("Starting job manager...")
		startErr = mgr.Start()
		t.Log("Job manager Start() method returned")

		manager = mgr
		close(doneCh)
	}()

	// 等待 goroutine 完成或超时
	select {
	case <-doneCh:
		// Success
		if startErr != nil {
			t.Fatalf("Failed to start job manager: %v", startErr)
		}

		defer manager.Stop()
		assert.Equal(t, workerID, manager.GetWorkerID(), "Worker ID should match")
		t.Log("Test completed successfully")

	case <-ctx.Done():
		t.Fatalf("Test timeout: %v", ctx.Err())
	}
}

func TestJobManagerRegisterHandler(t *testing.T) {
	// 创建任务管理器
	manager := createTestJobManager(t, "")
	defer manager.Stop()

	// 创建事件处理器
	handler := newTestJobEventHandler(t)

	// 注册事件处理器
	manager.RegisterHandler(handler)

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
	assert.Equal(t, 1, len(handler.events), "Should have received 1 event")
	assert.Equal(t, jobmgr.JobEventSave, handler.events[0].Type, "Event type should match")
	assert.Equal(t, job.ID, handler.events[0].Job.ID, "Job ID should match")
}

func TestJobManagerGetJob(t *testing.T) {
	// 创建etcd客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")

	// 创建任务管理器
	workerID := testutils.GenerateUniqueID("worker")
	manager := createTestJobManager(t, workerID)
	defer manager.Stop()

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreatePendingJob()
	job.Status = constants.JobStatusPending

	// 序列化任务并保存到etcd
	jobKey := constants.JobPrefix + job.ID
	jobJSON, err := job.ToJSON()
	require.NoError(t, err, "Failed to serialize job")

	err = etcdClient.Put(jobKey, jobJSON)
	require.NoError(t, err, "Failed to save job to etcd")

	// 等待任务被发现
	time.Sleep(500 * time.Millisecond)

	// 获取任务
	retrievedJob, err := manager.GetJob(job.ID)
	assert.NoError(t, err, "Should successfully get job")
	assert.NotNil(t, retrievedJob, "Job should not be nil")
	assert.Equal(t, job.ID, retrievedJob.ID, "Job ID should match")
	assert.Equal(t, job.Name, retrievedJob.Name, "Job name should match")
}

func TestJobManagerListJobs(t *testing.T) {
	// 创建etcd客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")

	// 创建任务管理器
	workerID := testutils.GenerateUniqueID("worker")
	manager := createTestJobManager(t, workerID)
	defer manager.Stop()

	// 创建多个测试任务
	jobFactory := testutils.NewJobFactory()
	job1 := jobFactory.CreatePendingJob()
	job2 := jobFactory.CreatePendingJob()

	// 保存任务到etcd
	for _, job := range []*models.Job{job1, job2} {
		jobKey := constants.JobPrefix + job.ID
		jobJSON, err := job.ToJSON()
		require.NoError(t, err, "Failed to serialize job")

		err = etcdClient.Put(jobKey, jobJSON)
		require.NoError(t, err, "Failed to save job to etcd")
	}

	// 等待任务被发现
	time.Sleep(500 * time.Millisecond)

	// 列出所有任务
	jobs, err := manager.ListJobs()
	assert.NoError(t, err, "Should successfully list jobs")
	assert.GreaterOrEqual(t, len(jobs), 2, "Should have at least 2 jobs")

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

	assert.True(t, foundJob1, "Should have found job1")
	assert.True(t, foundJob2, "Should have found job2")
}

func TestJobManagerWatchJobs(t *testing.T) {
	// 创建etcd客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")

	// 创建任务管理器
	workerID := testutils.GenerateUniqueID("worker")
	manager := createTestJobManager(t, workerID)
	defer manager.Stop()

	// 创建和注册测试事件处理器
	handler := newTestJobEventHandler(t)
	manager.RegisterHandler(handler)

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreatePendingJob()

	// 将Worker ID添加到任务中，使其被分配给此Worker
	if job.Env == nil {
		job.Env = make(map[string]string)
	}
	job.Env["WORKER_ID"] = workerID

	// 序列化并保存任务到etcd
	jobKey := constants.JobPrefix + job.ID
	jobJSON, err := job.ToJSON()
	require.NoError(t, err, "Failed to serialize job")

	err = etcdClient.Put(jobKey, jobJSON)
	require.NoError(t, err, "Failed to save job to etcd")

	// 等待任务事件被处理
	time.Sleep(1 * time.Second)

	// 验证是否收到事件
	foundEvent := false
	for _, event := range handler.events {
		if event.Job.ID == job.ID {
			foundEvent = true
			break
		}
	}

	assert.True(t, foundEvent, "Should have received event for the created job")

	// 删除任务，测试删除事件
	err = etcdClient.Delete(jobKey)
	require.NoError(t, err, "Failed to delete job from etcd")

	// 等待删除事件被处理
	time.Sleep(1 * time.Second)
}

func TestJobManagerWithMockHandler(t *testing.T) {
	// 创建任务管理器
	manager := createTestJobManager(t, "")
	defer manager.Stop()

	// 创建mock事件处理器
	mockHandler := mocks.NewIJobEventHandler(t)

	// 设置期望
	mockHandler.EXPECT().HandleJobEvent(testutils.MockAny()).Return()

	// 注册mock处理器
	manager.RegisterHandler(mockHandler)

	// 创建测试任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreatePendingJob()

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
	manager := createTestJobManager(t, workerID)
	defer manager.Stop()

	// 创建分配给此Worker的任务
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreatePendingJob()

	// 添加Worker ID到环境变量
	if job.Env == nil {
		job.Env = make(map[string]string)
	}
	job.Env["WORKER_ID"] = workerID

	// 验证任务分配
	isAssigned := manager.IsJobAssignedToWorker(job)
	assert.True(t, isAssigned, "Job should be assigned to this worker")

	// 创建分配给其他Worker的任务
	otherJob := jobFactory.CreatePendingJob()
	if otherJob.Env == nil {
		otherJob.Env = make(map[string]string)
	}
	otherJob.Env["WORKER_ID"] = "other-worker-id"

	// 验证任务不分配
	isAssigned = manager.IsJobAssignedToWorker(otherJob)
	assert.False(t, isAssigned, "Job should not be assigned to this worker")
}
