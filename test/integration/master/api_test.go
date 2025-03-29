package master

import (
	"bytes"
	"encoding/json"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/workermgr"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/api"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/logmgr"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupRouter 创建一个带有API处理程序的测试路由器
func setupRouter(t *testing.T) (*gin.Engine, *api.JobHandler, *api.LogHandler, *api.WorkerHandler, *api.SystemHandler) {
	// 将Gin设置为测试模式
	gin.SetMode(gin.TestMode)

	// 创建仓库客户端
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)

	// 创建作业仓库和管理器
	jobRepo := testutils.CreateTestJobRepo(t, etcdClient, mongoClient)
	workerRepo := testutils.CreateTestWorkerRepo(t, etcdClient, mongoClient)
	logRepo := testutils.CreateTestLogRepo(t, mongoClient)
	logManager := logmgr.NewLogManager(logRepo)
	workerManager := workermgr.NewWorkerManager(workerRepo, 5*time.Second, etcdClient)

	// 创建作业管理器和操作
	jobManager := testutils.CreateTestJobManager(t, jobRepo, logRepo, workerRepo, etcdClient)
	jobOps := testutils.CreateTestJobOperations(t, jobManager, workerRepo, logRepo)

	// 创建API处理程序
	jobHandler := api.NewJobHandler(jobManager, jobOps, nil)
	logHandler := api.NewLogHandler(logManager)
	workerHandler := api.NewWorkerHandler(testutils.CreateTestWorkerManager(t, workerRepo, etcdClient))
	systemHandler := api.NewSystemHandler(jobManager, workerManager, logManager, "test-version")

	// 创建路由器
	router := gin.New()
	router.Use(gin.Recovery())

	// 设置API路由
	apiGroup := router.Group("/api/v1")

	// 作业路由
	jobRoutes := apiGroup.Group("/jobs")
	jobRoutes.POST("", jobHandler.CreateJob)
	jobRoutes.GET("", jobHandler.ListJobs)
	jobRoutes.GET("/:id", jobHandler.GetJob)
	jobRoutes.PUT("/:id", jobHandler.UpdateJob)
	jobRoutes.DELETE("/:id", jobHandler.DeleteJob)
	jobRoutes.POST("/:id/run", jobHandler.TriggerJob)
	jobRoutes.POST("/:id/kill", jobHandler.KillJob)
	jobRoutes.PUT("/:id/enable", jobHandler.EnableJob)
	jobRoutes.PUT("/:id/disable", jobHandler.DisableJob)
	jobRoutes.GET("/:id/stats", jobHandler.GetJobStatistics)

	// 日志路由
	logRoutes := apiGroup.Group("/logs")
	logRoutes.GET("/jobs/:jobID", logHandler.GetJobLogs)
	logRoutes.GET("/executions/:executionID", logHandler.GetExecutionLog)

	// Worker路由
	workerRoutes := apiGroup.Group("/workers")
	workerRoutes.GET("", workerHandler.ListWorkers)
	workerRoutes.GET("/active", workerHandler.GetActiveWorkers)
	workerRoutes.GET("/:id", workerHandler.GetWorker)
	workerRoutes.PUT("/:id/enable", workerHandler.EnableWorker)
	workerRoutes.PUT("/:id/disable", workerHandler.DisableWorker)
	workerRoutes.GET("/:id/health", workerHandler.CheckWorkerHealth)

	// System路由
	systemRoutes := apiGroup.Group("/system")
	systemRoutes.GET("/status", systemHandler.GetSystemStatus)
	router.GET("/health", systemHandler.GetHealthCheck)

	return router, jobHandler, logHandler, workerHandler, systemHandler
}

// TestListJobs 测试ListJobs API端点
func TestListJobs(t *testing.T) {
	router, _, _, _, _ := setupRouter(t)

	// 创建一些测试作业
	jobs := []api.CreateJobRequest{
		{
			Name:        "test-list-job-1",
			Description: "First job for list test",
			Command:     "echo list1",
			CronExpr:    "@every 1h",
			Enabled:     true,
		},
		{
			Name:        "test-list-job-2",
			Description: "Second job for list test",
			Command:     "echo list2",
			Enabled:     false,
		},
	}

	// 创建测试作业
	for _, jobReq := range jobs {
		w := performCreateJobRequest(t, router, jobReq)
		assert.Equal(t, http.StatusCreated, w.Code, "Should create job successfully")
	}

	// 测试列出所有作业
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 解析响应
	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	// 验证作业列表
	paginationResp, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Expected pagination response")

	items, ok := paginationResp["items"].([]interface{})
	require.True(t, ok, "Expected items array")

	// 应该有至少两个作业
	assert.GreaterOrEqual(t, len(items), 2, "Should have at least 2 jobs")
}

// TestDeleteJob 测试DeleteJob API端点
func TestDeleteJob(t *testing.T) {
	router, _, _, _, _ := setupRouter(t)

	// 首先创建一个作业
	jobReq := api.CreateJobRequest{
		Name:        "test-delete-job",
		Description: "Job for delete test",
		Command:     "echo delete",
	}

	// 创建作业
	w := performCreateJobRequest(t, router, jobReq)
	jobID := extractJobIDFromResponse(t, w)

	// 现在删除作业
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/jobs/"+jobID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 验证作业已被删除
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, "Job should be deleted")
}

// TestEnableDisableJob 测试EnableJob和DisableJob API端点
func TestEnableDisableJob(t *testing.T) {
	router, _, _, _, _ := setupRouter(t)

	// 首先创建一个禁用的作业
	jobReq := api.CreateJobRequest{
		Name:        "test-enable-disable",
		Description: "Job for enable/disable test",
		Command:     "echo toggle",
		Enabled:     false,
	}

	// 创建作业
	w := performCreateJobRequest(t, router, jobReq)
	jobID := extractJobIDFromResponse(t, w)

	// 启用作业
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/jobs/"+jobID+"/enable", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 验证作业已启用
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	jobMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Failed to extract job from response")
	assert.Equal(t, true, jobMap["enabled"], "Job should be enabled")

	// 禁用作业
	req, _ = http.NewRequest(http.MethodPut, "/api/v1/jobs/"+jobID+"/disable", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 验证作业已禁用
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	jobMap, ok = resp.Data.(map[string]interface{})
	require.True(t, ok, "Failed to extract job from response")
	assert.Equal(t, false, jobMap["enabled"], "Job should be disabled")
}

// TestGetJobStats 测试GetJobStatistics API端点
func TestGetJobStats(t *testing.T) {
	router, _, _, _, _ := setupRouter(t)
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")

	// 注册一个测试工作节点
	workerID := "test-worker-stats"
	workerRegister, err := testutils.CreateTestWorker(etcdClient, workerID, map[string]string{"env": "test"})
	require.NoError(t, err, "Failed to create test worker")
	defer workerRegister.Stop()

	// 等待工作节点注册传播
	time.Sleep(500 * time.Millisecond)

	// 创建测试作业
	jobReq := api.CreateJobRequest{
		Name:        "test-stats-job",
		Description: "Job for stats test",
		Command:     "echo stats",
		Enabled:     true,
	}

	// 创建作业
	w := performCreateJobRequest(t, router, jobReq)
	jobID := extractJobIDFromResponse(t, w)

	// 触发作业执行
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/jobs/"+jobID+"/run", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "Should trigger job successfully")

	// 等待作业执行
	time.Sleep(500 * time.Millisecond)

	// 获取作业统计信息
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID+"/stats", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 验证统计信息
	var resp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	// 仅验证存在统计信息，不验证具体内容
	_, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok, "Response should contain stats data")
}

// TestKillJob 测试KillJob API端点
func TestKillJob(t *testing.T) {
	router, _, _, _, _ := setupRouter(t)
	etcdClient, _ := testutils.SetupTestEnvironment(t)

	// 注册一个测试工作节点
	workerID := "test-worker-kill"
	workerRegister, err := testutils.CreateTestWorker(etcdClient, workerID, map[string]string{"env": "test"})
	require.NoError(t, err, "Failed to create test worker")
	defer workerRegister.Stop()

	// 等待工作节点注册传播
	time.Sleep(500 * time.Millisecond)

	// 创建一个长时间运行的作业
	jobReq := api.CreateJobRequest{
		Name:    "test-kill-job",
		Command: "sleep 30", // 长时间运行的命令
		Enabled: true,
	}

	// 创建作业
	w := performCreateJobRequest(t, router, jobReq)
	jobID := extractJobIDFromResponse(t, w)

	// 触发作业执行
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/jobs/"+jobID+"/run", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "Should trigger job successfully")

	// 等待作业开始运行
	time.Sleep(500 * time.Millisecond)

	// 终止作业
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/jobs/"+jobID+"/kill", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 验证作业被终止
	// 注意：这可能不总是可靠的，因为终止是异步的
	time.Sleep(500 * time.Millisecond)

	// 获取作业状态
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	jobMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Failed to extract job from response")

	// 作业可能处于不同状态，但不应该是正在运行
	assert.NotEqual(t, constants.JobStatusRunning, jobMap["status"], "Job should not be running")
}

// TestGetJobLogs 测试GetJobLogs API端点
func TestGetJobLogs(t *testing.T) {
	router, _, _, _, _ := setupRouter(t)
	etcdClient, _ := testutils.SetupTestEnvironment(t)

	// 注册一个测试工作节点
	workerID := "test-worker-logs"
	workerRegister, err := testutils.CreateTestWorker(etcdClient, workerID, map[string]string{"env": "test"})
	require.NoError(t, err, "Failed to create test worker")
	defer workerRegister.Stop()

	// 等待工作节点注册传播
	time.Sleep(500 * time.Millisecond)

	// 创建测试作业
	jobReq := api.CreateJobRequest{
		Name:    "test-logs-job",
		Command: "echo 'test logs'",
		Enabled: true,
	}

	// 创建作业
	w := performCreateJobRequest(t, router, jobReq)
	jobID := extractJobIDFromResponse(t, w)

	// 触发作业执行
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/jobs/"+jobID+"/run", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "Should trigger job successfully")

	// 解析响应以获取执行ID
	var triggerResp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &triggerResp)
	require.NoError(t, err, "Failed to unmarshal response")

	execData, ok := triggerResp.Data.(map[string]interface{})
	require.True(t, ok, "Failed to extract execution data")

	executionID, ok := execData["execution_id"].(string)
	require.True(t, ok && executionID != "", "Should have valid execution ID")

	// 等待作业执行完成
	time.Sleep(1 * time.Second)

	// 获取作业日志
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/logs/jobs/"+jobID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 验证日志
	var logsResp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &logsResp)
	require.NoError(t, err, "Failed to unmarshal response")

	// 检查分页响应结构
	paginationData, ok := logsResp.Data.(map[string]interface{})
	require.True(t, ok, "Expected pagination data")

	// 获取日志项
	items, ok := paginationData["items"].([]interface{})
	require.True(t, ok, "Expected items array")

	// 应该至少有一个日志
	assert.GreaterOrEqual(t, len(items), 1, "Should have at least one log entry")

	// 获取特定执行日志
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/logs/executions/"+executionID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")
}

// TestWorkerOperations 测试Worker相关API端点
func TestWorkerOperations(t *testing.T) {
	router, _, _, _, _ := setupRouter(t)
	etcdClient, _ := testutils.SetupTestEnvironment(t)

	// 注册一个测试工作节点
	workerID := "test-worker-api"
	workerRegister, err := testutils.CreateTestWorker(etcdClient, workerID, map[string]string{"env": "test"})
	require.NoError(t, err, "Failed to create test worker")
	defer workerRegister.Stop()

	// 等待工作节点注册传播
	time.Sleep(500 * time.Millisecond)

	// 测试列出Worker
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/workers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 验证Worker列表
	var listResp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &listResp)
	require.NoError(t, err, "Failed to unmarshal response")

	paginationData, ok := listResp.Data.(map[string]interface{})
	require.True(t, ok, "Expected pagination data")

	items, ok := paginationData["items"].([]interface{})
	require.True(t, ok, "Expected items array")

	// 应该至少有一个Worker
	assert.GreaterOrEqual(t, len(items), 1, "Should have at least one worker")

	// 测试获取单个Worker
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/workers/"+workerID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 测试禁用Worker
	req, _ = http.NewRequest(http.MethodPut, "/api/v1/workers/"+workerID+"/disable", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 验证Worker已禁用
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/workers/"+workerID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var workerResp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &workerResp)
	require.NoError(t, err, "Failed to unmarshal response")

	workerData, ok := workerResp.Data.(map[string]interface{})
	require.True(t, ok, "Failed to extract worker data")

	assert.Equal(t, constants.WorkerStatusDisabled, workerData["status"], "Worker should be disabled")

	// 测试启用Worker
	req, _ = http.NewRequest(http.MethodPut, "/api/v1/workers/"+workerID+"/enable", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 测试获取活跃Worker
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/workers/active", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")
}

// TestSystemAPI 测试系统相关API端点
func TestSystemAPI(t *testing.T) {
	router, _, _, _, _ := setupRouter(t)

	// 测试健康检查
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 测试系统状态
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/system/status", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	var statusResp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &statusResp)
	require.NoError(t, err, "Failed to unmarshal response")

	statusData, ok := statusResp.Data.(map[string]interface{})
	require.True(t, ok, "Expected status data")

	// 验证关键字段存在
	_, ok = statusData["version"]
	assert.True(t, ok, "Response should contain version")

	_, ok = statusData["total_jobs"]
	assert.True(t, ok, "Response should contain total_jobs")

	_, ok = statusData["active_workers"]
	assert.True(t, ok, "Response should contain active_workers")
}

// 辅助函数
func performCreateJobRequest(t *testing.T, router *gin.Engine, jobReq api.CreateJobRequest) *httptest.ResponseRecorder {
	reqBody, err := json.Marshal(jobReq)
	require.NoError(t, err, "Failed to marshal job request")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	return w
}

func extractJobIDFromResponse(t *testing.T, w *httptest.ResponseRecorder) string {
	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	jobRespMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Failed to extract job from response")

	jobID, ok := jobRespMap["id"].(string)
	require.True(t, ok, "Failed to extract job ID")

	return jobID
}
