package master

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/api"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

// setupRouter 创建一个带有API处理程序的测试路由器
func setupRouter(t *testing.T) (*gin.Engine, *api.JobHandler, *api.LogHandler) {
	// 将Gin设置为测试模式
	gin.SetMode(gin.TestMode)

	// 创建仓库客户端
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)

	// 创建作业仓库和管理器
	jobRepo := testutils.CreateTestJobRepo(t, etcdClient, mongoClient)
	workerRepo := testutils.CreateTestWorkerRepo(t, etcdClient, mongoClient)
	logRepo := testutils.CreateTestLogRepo(t, mongoClient)

	// 创建作业管理器和操作
	jobManager := testutils.CreateTestJobManager(t, jobRepo, logRepo, workerRepo, etcdClient)
	jobOps := testutils.CreateTestJobOperations(t, jobManager, workerRepo, logRepo)

	// 创建API处理程序
	jobHandler := api.NewJobHandler(jobManager, jobOps, nil)
	logHandler := api.NewLogHandler(logRepo)

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

	// 日志路由
	logRoutes := apiGroup.Group("/logs")
	logRoutes.GET("/jobs/:jobID", logHandler.GetJobLogs)
	logRoutes.GET("/executions/:executionID", logHandler.GetExecutionLog)

	return router, jobHandler, logHandler
}

// TestCreateJob 测试CreateJob API端点
func TestCreateJob(t *testing.T) {
	router, _, _ := setupRouter(t)

	// 创建测试作业请求
	jobReq := api.CreateJobRequest{
		Name:        "test-api-job",
		Description: "Job created via API test",
		Command:     "echo hello",
		CronExpr:    "@every 1h",
		Enabled:     true,
	}

	reqBody, err := json.Marshal(jobReq)
	require.NoError(t, err, "Failed to marshal job request")

	// 创建HTTP请求
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusCreated, w.Code, "Expected status 201")

	// 解析响应
	var resp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	assert.True(t, resp.Success, "Expected success response")

	// 提取作业响应
	jobRespMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Failed to extract job from response")

	assert.Equal(t, jobReq.Name, jobRespMap["name"], "Job name mismatch")
	assert.Equal(t, jobReq.Command, jobRespMap["command"], "Job command mismatch")

	// 保存作业ID以供其他测试使用
	jobID := jobRespMap["id"].(string)
	assert.NotEmpty(t, jobID, "Job ID should not be empty")
}

// TestGetJob 测试GetJob API端点
func TestGetJob(t *testing.T) {
	router, _, _ := setupRouter(t)

	// 首先创建一个作业
	jobReq := api.CreateJobRequest{
		Name:        "test-get-job",
		Description: "Job for get test",
		Command:     "echo hello",
	}

	// 创建作业
	w := performCreateJobRequest(t, router, jobReq)
	jobID := extractJobIDFromResponse(t, w)

	// 现在测试获取作业
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 解析响应
	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	// 验证作业详情
	jobMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Failed to extract job from response")
	assert.Equal(t, jobID, jobMap["id"], "Job ID mismatch")
	assert.Equal(t, jobReq.Name, jobMap["name"], "Job name mismatch")
}

// TestUpdateJob 测试UpdateJob API端点
func TestUpdateJob(t *testing.T) {
	router, _, _ := setupRouter(t)

	// 首先创建一个作业
	jobReq := api.CreateJobRequest{
		Name:        "test-update-job",
		Description: "Original description",
		Command:     "echo original",
	}

	// 创建作业
	w := performCreateJobRequest(t, router, jobReq)
	jobID := extractJobIDFromResponse(t, w)

	// 现在更新作业
	updateReq := api.UpdateJobRequest{
		Name:        "test-update-job",
		Description: "Updated description",
		Command:     "echo updated",
	}

	reqBody, err := json.Marshal(updateReq)
	require.NoError(t, err, "Failed to marshal update request")

	req, _ := http.NewRequest(http.MethodPut, "/api/v1/jobs/"+jobID, bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 验证作业是否已通过获取更新
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	jobMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Failed to extract job from response")
	assert.Equal(t, updateReq.Description, jobMap["description"], "Job description not updated")
	assert.Equal(t, updateReq.Command, jobMap["command"], "Job command not updated")
}

// TestTriggerJob 测试TriggerJob API端点
func TestTriggerJob(t *testing.T) {
	router, _, _ := setupRouter(t)

	// 首先创建一个作业
	jobReq := api.CreateJobRequest{
		Name:    "test-trigger-job",
		Command: "echo triggered",
	}

	// 创建作业
	w := performCreateJobRequest(t, router, jobReq)
	jobID := extractJobIDFromResponse(t, w)

	// 现在触发作业
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/jobs/"+jobID+"/run", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 解析响应以获取执行ID
	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	resultMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Failed to extract result from response")

	// 响应中应存在执行ID
	executionID, exists := resultMap["execution_id"]
	assert.True(t, exists, "Execution ID should be present in response")
	assert.NotEmpty(t, executionID, "Execution ID should not be empty")
}

// TestListJobs 测试ListJobs API端点
func TestListJobs(t *testing.T) {
	router, _, _ := setupRouter(t)

	// 首先创建一些作业
	for i := 1; i <= 3; i++ {
		jobReq := api.CreateJobRequest{
			Name:    fmt.Sprintf("test-list-job-%d", i),
			Command: "echo list test",
		}
		performCreateJobRequest(t, router, jobReq)
	}

	// 现在测试列出作业
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/jobs?page=1&page_size=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 断言响应
	assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

	// 解析响应
	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	// 验证作业列表
	paginationMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Failed to extract pagination from response")

	items, exists := paginationMap["items"]
	assert.True(t, exists, "Items should be present in response")

	jobList, ok := items.([]interface{})
	require.True(t, ok, "Failed to extract job list from response")
	assert.GreaterOrEqual(t, len(jobList), 3, "Should have at least 3 jobs")
}

// 辅助函数
func performCreateJobRequest(t *testing.T, router *gin.Engine, jobReq api.CreateJobRequest) *httptest.ResponseRecorder {
	reqBody, err := json.Marshal(jobReq)
	require.NoError(t, err, "Failed to marshal job request")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "Expected status 201")
	return w
}

func extractJobIDFromResponse(t *testing.T, w *httptest.ResponseRecorder) string {
	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	jobMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Failed to extract job from response")

	jobID, ok := jobMap["id"].(string)
	require.True(t, ok, "Failed to extract job ID from response")
	require.NotEmpty(t, jobID, "Job ID should not be empty")

	return jobID
}
