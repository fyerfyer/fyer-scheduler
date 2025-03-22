package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/logmgr"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LogHandler 处理日志相关的API请求
type LogHandler struct {
	logManager    logmgr.LogManager        // 日志管理器
	streamManager *logmgr.LogStreamManager // 日志流管理器
}

// NewLogHandler 创建一个新的日志处理器
func NewLogHandler(logManager logmgr.LogManager) *LogHandler {
	return &LogHandler{
		logManager:    logManager,
		streamManager: logmgr.NewLogStreamManager(logManager.(*logmgr.MasterLogManager)),
	}
}

// GetJobLogs 获取任务的执行日志
// GET /api/v1/logs/jobs/{jobID}
func (h *LogHandler) GetJobLogs(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	jobID := c.Param("jobID")

	if jobID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrBadRequest.WithDetails("job ID is required"), requestID))
		return
	}

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

	// 获取时间范围参数
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	var startTime, endTime time.Time
	var timeRangeQuery bool

	if startTimeStr != "" {
		startTime, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, NewErrorResponse(
				ErrBadRequest.WithDetails("invalid start_time format, expected RFC3339"), requestID))
			return
		}
		timeRangeQuery = true
	}

	if endTimeStr != "" {
		endTime, err = time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, NewErrorResponse(
				ErrBadRequest.WithDetails("invalid end_time format, expected RFC3339"), requestID))
			return
		}
		timeRangeQuery = true
	} else if timeRangeQuery {
		// 如果提供了开始时间但没有结束时间，使用当前时间作为结束时间
		endTime = time.Now()
	}

	// 根据是否有时间范围决定调用哪个方法
	var logs []*LogResponse
	var total int64

	if timeRangeQuery {
		// 首先获取该任务在时间范围内的所有日志
		jobLogs, count, err := h.logManager.GetLogsByTimeRange(startTime, endTime, page, pageSize)
		if err != nil {
			utils.Error("failed to get logs by time range",
				zap.String("request_id", requestID),
				zap.String("job_id", jobID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, NewErrorResponse(
				WrapError(err, ErrInternalServer), requestID))
			return
		}

		// 然后过滤出指定任务的日志
		filteredLogs := make([]*LogResponse, 0)
		for _, log := range jobLogs {
			if log.JobID == jobID {
				filteredLogs = append(filteredLogs, ConvertToLogResponse(log))
			}
		}

		logs = filteredLogs
		total = count
	} else {
		// 直接获取该任务的所有日志
		jobLogs, count, err := h.logManager.GetLogsByJobID(jobID, page, pageSize)
		if err != nil {
			utils.Error("failed to get job logs",
				zap.String("request_id", requestID),
				zap.String("job_id", jobID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, NewErrorResponse(
				WrapError(err, ErrInternalServer), requestID))
			return
		}

		logs = make([]*LogResponse, len(jobLogs))
		for i, log := range jobLogs {
			logs[i] = ConvertToLogResponse(log)
		}
		total = count
	}

	// 构建分页响应
	response := &PaginationResponse{
		Total:    total,
		Page:     int(page),
		PageSize: int(pageSize),
		Items:    logs,
	}

	c.JSON(http.StatusOK, NewResponse(response, requestID))
}

// GetExecutionLog 获取特定执行的日志详情
// GET /api/v1/logs/executions/{executionID}
func (h *LogHandler) GetExecutionLog(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	executionID := c.Param("executionID")

	if executionID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(
			ErrBadRequest.WithDetails("execution ID is required"), requestID))
		return
	}

	// 获取执行日志
	log, err := h.logManager.GetLogByExecutionID(executionID)
	if err != nil {
		utils.Warn("failed to get execution log",
			zap.String("request_id", requestID),
			zap.String("execution_id", executionID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, NewErrorResponse(
			NewNotFoundError("Execution log", executionID), requestID))
		return
	}

	// 返回日志详情
	c.JSON(http.StatusOK, NewResponse(ConvertToLogResponse(log), requestID))
}

// StreamExecutionLog 流式获取执行日志（SSE方式）
// GET /api/v1/logs/executions/{executionID}/stream
func (h *LogHandler) StreamExecutionLog(c *gin.Context) {
	executionID := c.Param("executionID")

	if executionID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(
			ErrBadRequest.WithDetails("execution ID is required"), c.GetString(RequestIDKey)))
		return
	}

	// 检查是否支持SSE
	if _, ok := c.GetQuery("websocket"); ok {
		// 使用WebSocket方式处理
		h.handleWebSocketStream(c, executionID)
	} else {
		// 默认使用SSE方式处理
		h.handleSSEStream(c, executionID)
	}
}

// handleSSEStream 处理SSE方式的日志流
func (h *LogHandler) handleSSEStream(c *gin.Context, executionID string) {
	requestID := c.GetString(RequestIDKey)

	// 首先检查执行ID是否存在
	_, err := h.logManager.GetLogByExecutionID(executionID)
	if err != nil {
		utils.Warn("execution ID not found for streaming",
			zap.String("request_id", requestID),
			zap.String("execution_id", executionID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, NewErrorResponse(
			NewNotFoundError("Execution log", executionID), requestID))
		return
	}

	// 使用流管理器处理SSE连接
	h.streamManager.HandleSSE(c.Writer, c.Request, executionID)
}

// handleWebSocketStream 处理WebSocket方式的日志流
func (h *LogHandler) handleWebSocketStream(c *gin.Context, executionID string) {
	requestID := c.GetString(RequestIDKey)

	// 首先检查执行ID是否存在
	_, err := h.logManager.GetLogByExecutionID(executionID)
	if err != nil {
		utils.Warn("execution ID not found for streaming",
			zap.String("request_id", requestID),
			zap.String("execution_id", executionID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, NewErrorResponse(
			NewNotFoundError("Execution log", executionID), requestID))
		return
	}

	// 使用流管理器处理WebSocket连接
	h.streamManager.HandleWebSocket(c.Writer, c.Request, executionID)
}

// GetJobLogStats 获取任务的日志统计信息
// GET /api/v1/logs/jobs/{jobID}/stats
func (h *LogHandler) GetJobLogStats(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)
	jobID := c.Param("jobID")

	if jobID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(
			ErrBadRequest.WithDetails("job ID is required"), requestID))
		return
	}

	// 获取任务状态统计
	statusCounts, err := h.logManager.GetJobStatusCounts(jobID)
	if err != nil {
		utils.Error("failed to get job status counts",
			zap.String("request_id", requestID),
			zap.String("job_id", jobID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, NewErrorResponse(
			WrapError(err, ErrInternalServer), requestID))
		return
	}

	// 获取任务成功率
	successRate, err := h.logManager.GetJobSuccessRate(jobID, 30*24*time.Hour) // 30天数据
	if err != nil {
		utils.Warn("failed to get job success rate",
			zap.String("request_id", requestID),
			zap.String("job_id", jobID),
			zap.Error(err))
		// 不返回错误，保持前面的数据
		successRate = 0
	}

	// 构建响应数据
	stats := map[string]interface{}{
		"job_id":        jobID,
		"status_counts": statusCounts,
		"success_rate":  successRate,
	}

	c.JSON(http.StatusOK, NewResponse(stats, requestID))
}

// GetSystemLogs 获取系统日志统计信息
// GET /api/v1/logs/system
func (h *LogHandler) GetSystemLogs(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)

	// 获取系统状态摘要
	summary, err := h.logManager.GetSystemStatusSummary()
	if err != nil {
		utils.Error("failed to get system status summary",
			zap.String("request_id", requestID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, NewErrorResponse(
			WrapError(err, ErrInternalServer), requestID))
		return
	}

	// 获取日常执行统计
	// 默认获取最近7天的数据
	daysStr := c.DefaultQuery("days", "7")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 || days > 90 {
		days = 7
	}

	dailyStats, err := h.logManager.GetDailyJobExecutionStats(days)
	if err != nil {
		utils.Warn("failed to get daily execution stats",
			zap.String("request_id", requestID),
			zap.Error(err))
		// 不返回错误，保持前面的数据
	}

	// 构建响应数据
	data := map[string]interface{}{
		"summary":     summary,
		"daily_stats": dailyStats,
	}

	c.JSON(http.StatusOK, NewResponse(data, requestID))
}
