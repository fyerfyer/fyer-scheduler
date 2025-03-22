package api

import (
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/jobmgr"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

// JobHandler 处理任务相关的API请求
type JobHandler struct {
	jobManager jobmgr.JobManager     // 任务管理器
	jobOps     *jobmgr.JobOperations // 任务操作
	scheduler  jobmgr.IScheduler     // 调度器
}

// NewJobHandler 创建一个新的任务处理器
func NewJobHandler(jobManager jobmgr.JobManager, jobOps *jobmgr.JobOperations, scheduler jobmgr.IScheduler) *JobHandler {
	return &JobHandler{
		jobManager: jobManager,
		jobOps:     jobOps,
		scheduler:  scheduler,
	}
}

// CreateJob 处理创建任务的请求
// POST /api/v1/jobs
func (h *JobHandler) CreateJob(c *gin.Context) {
	// 获取请求ID
	requestID := c.GetString(RequestIDKey)

	// 解析请求数据
	var req CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Warn("failed to parse create job request",
			zap.String("request_id", requestID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, NewErrorResponse(NewValidationError(err.Error()), requestID))
		return
	}

	// 验证请求数据
	if err := ValidateRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(NewValidationError(err.Error()), requestID))
		return
	}

	// 创建任务模型
	job, err := CreateJobFromRequest(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(WrapError(err, ErrBadRequest), requestID))
		return
	}

	// 保存任务
	err = h.jobManager.CreateJob(job)
	if err != nil {
		utils.Error("failed to create job",
			zap.String("request_id", requestID),
			zap.String("job_name", job.Name),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		return
	}

	utils.Info("job created successfully",
		zap.String("request_id", requestID),
		zap.String("job_id", job.ID),
		zap.String("job_name", job.Name))

	// 返回成功响应
	c.JSON(http.StatusCreated, NewResponse(ConvertToJobResponse(job), requestID))
}

// GetJob 处理获取单个任务的请求
// GET /api/v1/jobs/:id
func (h *JobHandler) GetJob(c *gin.Context) {
	// 获取请求ID和任务ID
	requestID := c.GetString(RequestIDKey)
	jobID := c.Param("id")

	if jobID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("job ID is required"), requestID))
		return
	}

	// 获取任务
	job, err := h.jobManager.GetJob(jobID)
	if err != nil {
		utils.Warn("failed to get job",
			zap.String("request_id", requestID),
			zap.String("job_id", jobID),
			zap.Error(err))

		c.JSON(http.StatusNotFound, NewErrorResponse(NewNotFoundError("Job", jobID), requestID))
		return
	}

	// 返回任务信息
	c.JSON(http.StatusOK, NewResponse(ConvertToJobResponse(job), requestID))
}

// ListJobs 处理获取任务列表的请求
// GET /api/v1/jobs
func (h *JobHandler) ListJobs(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)

	// 获取分页参数
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "10")

	page, err := strconv.ParseInt(pageStr, 10, 64)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.ParseInt(pageSizeStr, 10, 64)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	// 获取状态过滤参数
	status := c.Query("status")

	var jobs []*JobResponse
	var total int64

	// 根据是否有状态过滤决定调用哪个方法
	if status != "" {
		jobList, count, err := h.jobManager.ListJobsByStatus(status, page, pageSize)
		if err != nil {
			utils.Error("failed to list jobs by status",
				zap.String("request_id", requestID),
				zap.String("status", status),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
			return
		}

		total = count
		jobs = make([]*JobResponse, len(jobList))
		for i, job := range jobList {
			jobs[i] = ConvertToJobResponse(job)
		}
	} else {
		jobList, count, err := h.jobManager.ListJobs(page, pageSize)
		if err != nil {
			utils.Error("failed to list jobs",
				zap.String("request_id", requestID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
			return
		}

		total = count
		jobs = make([]*JobResponse, len(jobList))
		for i, job := range jobList {
			jobs[i] = ConvertToJobResponse(job)
		}
	}

	// 构建分页响应
	response := &PaginationResponse{
		Total:    total,
		Page:     int(page),
		PageSize: int(pageSize),
		Items:    jobs,
	}

	c.JSON(http.StatusOK, NewResponse(response, requestID))
}

// UpdateJob 处理更新任务的请求
// PUT /api/v1/jobs/:id
func (h *JobHandler) UpdateJob(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	jobID := c.Param("id")

	if jobID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("job ID is required"), requestID))
		return
	}

	// 解析请求数据
	var req UpdateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Warn("failed to parse update job request",
			zap.String("request_id", requestID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, NewErrorResponse(NewValidationError(err.Error()), requestID))
		return
	}

	// 验证请求数据
	if err := ValidateRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(NewValidationError(err.Error()), requestID))
		return
	}

	// 获取现有任务
	job, err := h.jobManager.GetJob(jobID)
	if err != nil {
		utils.Warn("job not found for update",
			zap.String("request_id", requestID),
			zap.String("job_id", jobID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, NewErrorResponse(NewNotFoundError("Job", jobID), requestID))
		return
	}

	// 更新任务模型
	err = UpdateJobFromRequest(job, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(WrapError(err, ErrBadRequest), requestID))
		return
	}

	// 保存更新
	err = h.jobManager.UpdateJob(job)
	if err != nil {
		utils.Error("failed to update job",
			zap.String("request_id", requestID),
			zap.String("job_id", jobID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		return
	}

	utils.Info("job updated successfully",
		zap.String("request_id", requestID),
		zap.String("job_id", jobID))

	// 返回更新后的任务
	c.JSON(http.StatusOK, NewResponse(ConvertToJobResponse(job), requestID))
}

// DeleteJob 处理删除任务的请求
// DELETE /api/v1/jobs/:id
func (h *JobHandler) DeleteJob(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	jobID := c.Param("id")

	if jobID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("job ID is required"), requestID))
		return
	}

	// 删除任务
	err := h.jobManager.DeleteJob(jobID)
	if err != nil {
		utils.Error("failed to delete job",
			zap.String("request_id", requestID),
			zap.String("job_id", jobID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		return
	}

	utils.Info("job deleted successfully",
		zap.String("request_id", requestID),
		zap.String("job_id", jobID))

	// 返回成功响应
	c.JSON(http.StatusOK, NewResponse(map[string]string{"message": "Job deleted successfully"}, requestID))
}

// TriggerJob 处理立即执行任务的请求
// POST /api/v1/jobs/:id/run
func (h *JobHandler) TriggerJob(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	jobID := c.Param("id")

	if jobID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("job ID is required"), requestID))
		return
	}

	// 解析可选的Worker指定请求
	var req TriggerJobRequest
	if err := c.ShouldBindJSON(&req); err == nil {
		// 如果指定了特定的Worker，使用JobOperations执行
		if req.WorkerID != "" {
			executionID, err := h.jobOps.RunOnceOnSpecificWorker(jobID, req.WorkerID)
			if err != nil {
				utils.Error("failed to trigger job on specific worker",
					zap.String("request_id", requestID),
					zap.String("job_id", jobID),
					zap.String("worker_id", req.WorkerID),
					zap.Error(err))

				if IsNotFoundError(err) {
					c.JSON(http.StatusNotFound, NewErrorResponse(ErrJobNotFound, requestID))
				} else {
					c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
				}
				return
			}

			utils.Info("job triggered on specific worker",
				zap.String("request_id", requestID),
				zap.String("job_id", jobID),
				zap.String("worker_id", req.WorkerID),
				zap.String("execution_id", executionID))

			response := &TriggerJobResponse{
				ExecutionID: executionID,
				JobID:       jobID,
				Status:      "pending",
			}

			c.JSON(http.StatusOK, NewResponse(response, requestID))
			return
		}
	}

	// 使用JobOperations立即触发任务
	executionID, err := h.jobOps.TriggerJobExecution(jobID)
	if err != nil {
		utils.Error("failed to trigger job execution",
			zap.String("request_id", requestID),
			zap.String("job_id", jobID),
			zap.Error(err))

		if IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, NewErrorResponse(ErrJobNotFound, requestID))
		} else {
			c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		}
		return
	}

	utils.Info("job triggered successfully",
		zap.String("request_id", requestID),
		zap.String("job_id", jobID),
		zap.String("execution_id", executionID))

	// 返回执行ID
	response := &TriggerJobResponse{
		ExecutionID: executionID,
		JobID:       jobID,
		Status:      "pending",
	}

	c.JSON(http.StatusOK, NewResponse(response, requestID))
}

// KillJob 处理终止任务的请求
// POST /api/v1/jobs/:id/kill
func (h *JobHandler) KillJob(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	jobID := c.Param("id")

	if jobID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("job ID is required"), requestID))
		return
	}

	// 获取终止原因
	reason := "Manually terminated"
	if c.Query("reason") != "" {
		reason = c.Query("reason")
	}

	// 使用JobOperations取消任务
	err := h.jobOps.CancelJobExecution(jobID, reason)
	if err != nil {
		utils.Error("failed to kill job",
			zap.String("request_id", requestID),
			zap.String("job_id", jobID),
			zap.Error(err))

		if IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, NewErrorResponse(ErrJobNotFound, requestID))
		} else if err.Error() == "job is not running" {
			c.JSON(http.StatusBadRequest, NewErrorResponse(ErrJobNotRunning, requestID))
		} else {
			c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		}
		return
	}

	utils.Info("job killed successfully",
		zap.String("request_id", requestID),
		zap.String("job_id", jobID),
		zap.String("reason", reason))

	// 返回成功响应
	c.JSON(http.StatusOK, NewResponse(map[string]string{"message": "Job terminated successfully"}, requestID))
}

// EnableJob 处理启用任务的请求
// PUT /api/v1/jobs/:id/enable
func (h *JobHandler) EnableJob(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	jobID := c.Param("id")

	if jobID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("job ID is required"), requestID))
		return
	}

	// 启用任务
	err := h.jobManager.EnableJob(jobID)
	if err != nil {
		utils.Error("failed to enable job",
			zap.String("request_id", requestID),
			zap.String("job_id", jobID),
			zap.Error(err))

		if IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, NewErrorResponse(ErrJobNotFound, requestID))
		} else {
			c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		}
		return
	}

	utils.Info("job enabled successfully",
		zap.String("request_id", requestID),
		zap.String("job_id", jobID))

	// 返回成功响应
	c.JSON(http.StatusOK, NewResponse(map[string]string{"message": "Job enabled successfully"}, requestID))
}

// DisableJob 处理禁用任务的请求
// PUT /api/v1/jobs/:id/disable
func (h *JobHandler) DisableJob(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	jobID := c.Param("id")

	if jobID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("job ID is required"), requestID))
		return
	}

	// 禁用任务
	err := h.jobManager.DisableJob(jobID)
	if err != nil {
		utils.Error("failed to disable job",
			zap.String("request_id", requestID),
			zap.String("job_id", jobID),
			zap.Error(err))

		if IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, NewErrorResponse(ErrJobNotFound, requestID))
		} else {
			c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		}
		return
	}

	utils.Info("job disabled successfully",
		zap.String("request_id", requestID),
		zap.String("job_id", jobID))

	// 返回成功响应
	c.JSON(http.StatusOK, NewResponse(map[string]string{"message": "Job disabled successfully"}, requestID))
}

// GetJobStatistics 获取任务统计信息
// GET /api/v1/jobs/:id/stats
func (h *JobHandler) GetJobStatistics(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	jobID := c.Param("id")

	if jobID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("job ID is required"), requestID))
		return
	}

	// 获取任务统计信息
	stats, err := h.jobOps.GetJobStatistics(jobID)
	if err != nil {
		utils.Error("failed to get job statistics",
			zap.String("request_id", requestID),
			zap.String("job_id", jobID),
			zap.Error(err))

		if IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, NewErrorResponse(ErrJobNotFound, requestID))
		} else {
			c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		}
		return
	}

	c.JSON(http.StatusOK, NewResponse(stats, requestID))
}
