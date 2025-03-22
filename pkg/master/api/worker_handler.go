package api

import (
	"net/http"
	"strconv"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/workermgr"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// WorkerHandler 处理Worker相关的API请求
type WorkerHandler struct {
	workerManager workermgr.WorkerManager // Worker管理器
}

// NewWorkerHandler 创建一个新的Worker处理器
func NewWorkerHandler(workerManager workermgr.WorkerManager) *WorkerHandler {
	return &WorkerHandler{
		workerManager: workerManager,
	}
}

// ListWorkers 获取Worker列表
// GET /api/v1/workers
func (h *WorkerHandler) ListWorkers(c *gin.Context) {
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

	// 获取Worker列表
	workers, total, err := h.workerManager.ListWorkers(page, pageSize)
	if err != nil {
		utils.Error("failed to list workers",
			zap.String("request_id", requestID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		return
	}

	// 转换为响应格式
	workerResponses := make([]*WorkerResponse, len(workers))
	for i, worker := range workers {
		workerResponses[i] = ConvertToWorkerResponse(worker)
	}

	// 构建分页响应
	response := &PaginationResponse{
		Total:    total,
		Page:     int(page),
		PageSize: int(pageSize),
		Items:    workerResponses,
	}

	c.JSON(http.StatusOK, NewResponse(response, requestID))
}

// GetWorker 获取特定Worker的详情
// GET /api/v1/workers/:id
func (h *WorkerHandler) GetWorker(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	workerID := c.Param("id")

	if workerID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("worker ID is required"), requestID))
		return
	}

	// 获取Worker详情
	worker, err := h.workerManager.GetWorker(workerID)
	if err != nil {
		utils.Warn("failed to get worker",
			zap.String("request_id", requestID),
			zap.String("worker_id", workerID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, NewErrorResponse(NewNotFoundError("Worker", workerID), requestID))
		return
	}

	// 转换为响应格式
	response := ConvertToWorkerResponse(worker)

	c.JSON(http.StatusOK, NewResponse(response, requestID))
}

// EnableWorker 启用Worker
// PUT /api/v1/workers/:id/enable
func (h *WorkerHandler) EnableWorker(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	workerID := c.Param("id")

	if workerID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("worker ID is required"), requestID))
		return
	}

	// 启用Worker
	err := h.workerManager.EnableWorker(workerID)
	if err != nil {
		utils.Error("failed to enable worker",
			zap.String("request_id", requestID),
			zap.String("worker_id", workerID),
			zap.Error(err))

		if IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, NewErrorResponse(ErrWorkerNotFound, requestID))
		} else {
			c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		}
		return
	}

	utils.Info("worker enabled successfully",
		zap.String("request_id", requestID),
		zap.String("worker_id", workerID))

	// 返回成功响应
	c.JSON(http.StatusOK, NewResponse(map[string]string{"message": "Worker enabled successfully"}, requestID))
}

// DisableWorker 禁用Worker
// PUT /api/v1/workers/:id/disable
func (h *WorkerHandler) DisableWorker(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	workerID := c.Param("id")

	if workerID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("worker ID is required"), requestID))
		return
	}

	// 禁用Worker
	err := h.workerManager.DisableWorker(workerID)
	if err != nil {
		utils.Error("failed to disable worker",
			zap.String("request_id", requestID),
			zap.String("worker_id", workerID),
			zap.Error(err))

		if IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, NewErrorResponse(ErrWorkerNotFound, requestID))
		} else {
			c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		}
		return
	}

	utils.Info("worker disabled successfully",
		zap.String("request_id", requestID),
		zap.String("worker_id", workerID))

	// 返回成功响应
	c.JSON(http.StatusOK, NewResponse(map[string]string{"message": "Worker disabled successfully"}, requestID))
}

// GetActiveWorkers 获取所有活跃的Worker
// GET /api/v1/workers/active
func (h *WorkerHandler) GetActiveWorkers(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)

	// 获取活跃Worker列表
	workers, err := h.workerManager.GetActiveWorkers()
	if err != nil {
		utils.Error("failed to get active workers",
			zap.String("request_id", requestID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		return
	}

	// 转换为响应格式
	workerResponses := make([]*WorkerResponse, len(workers))
	for i, worker := range workers {
		workerResponses[i] = ConvertToWorkerResponse(worker)
	}

	c.JSON(http.StatusOK, NewResponse(workerResponses, requestID))
}

// CheckWorkerHealth 检查Worker的健康状态
// GET /api/v1/workers/:id/health
func (h *WorkerHandler) CheckWorkerHealth(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	workerID := c.Param("id")

	if workerID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("worker ID is required"), requestID))
		return
	}

	// 检查Worker健康状态
	isHealthy, err := h.workerManager.CheckWorkerHealth(workerID)
	if err != nil {
		utils.Warn("failed to check worker health",
			zap.String("request_id", requestID),
			zap.String("worker_id", workerID),
			zap.Error(err))

		if IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, NewErrorResponse(ErrWorkerNotFound, requestID))
		} else {
			c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		}
		return
	}

	// 返回健康状态
	c.JSON(http.StatusOK, NewResponse(map[string]bool{"healthy": isHealthy}, requestID))
}

// UpdateWorkerLabels 更新Worker的标签
// PUT /api/v1/workers/:id/labels
func (h *WorkerHandler) UpdateWorkerLabels(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	workerID := c.Param("id")

	if workerID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("worker ID is required"), requestID))
		return
	}

	// 解析请求数据
	var labels map[string]string
	if err := c.ShouldBindJSON(&labels); err != nil {
		utils.Warn("failed to parse labels request",
			zap.String("request_id", requestID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, NewErrorResponse(NewValidationError(err.Error()), requestID))
		return
	}

	// 更新Worker标签
	err := h.workerManager.UpdateWorkerLabels(workerID, labels)
	if err != nil {
		utils.Error("failed to update worker labels",
			zap.String("request_id", requestID),
			zap.String("worker_id", workerID),
			zap.Error(err))

		if IsNotFoundError(err) {
			c.JSON(http.StatusNotFound, NewErrorResponse(ErrWorkerNotFound, requestID))
		} else {
			c.JSON(http.StatusInternalServerError, NewErrorResponse(WrapError(err, ErrInternalServer), requestID))
		}
		return
	}

	utils.Info("worker labels updated successfully",
		zap.String("request_id", requestID),
		zap.String("worker_id", workerID))

	// 返回成功响应
	c.JSON(http.StatusOK, NewResponse(map[string]string{"message": "Worker labels updated successfully"}, requestID))
}
