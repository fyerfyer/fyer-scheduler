package api

import (
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/jobmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/logmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/workermgr"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SystemHandler 处理系统相关的API请求
type SystemHandler struct {
	jobManager    jobmgr.JobManager       // 任务管理器
	workerManager workermgr.WorkerManager // Worker管理器
	logManager    logmgr.LogManager       // 日志管理器
	startTime     time.Time               // 系统启动时间
	version       string                  // 系统版本
}

// NewSystemHandler 创建一个新的系统处理器
func NewSystemHandler(
	jobManager jobmgr.JobManager,
	workerManager workermgr.WorkerManager,
	logManager logmgr.LogManager,
	version string,
) *SystemHandler {
	return &SystemHandler{
		jobManager:    jobManager,
		workerManager: workerManager,
		logManager:    logManager,
		startTime:     time.Now(),
		version:       version,
	}
}

// GetSystemStatus 获取系统状态信息
// GET /api/v1/system/status
func (h *SystemHandler) GetSystemStatus(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)

	// 获取任务统计
	runningJobs := 0
	totalJobs := 0

	// 获取任务列表，分页获取，假设系统中不会有太多任务
	jobs, count, err := h.jobManager.ListJobs(1, 1000)
	if err != nil {
		utils.Error("failed to get jobs for status",
			zap.String("request_id", requestID),
			zap.Error(err))
	} else {
		totalJobs = int(count)
		// 统计运行中的任务
		for _, job := range jobs {
			if job.Status == "running" {
				runningJobs++
			}
		}
	}

	// 获取Worker统计
	activeWorkers := 0
	workers, err := h.workerManager.GetActiveWorkers()
	if err != nil {
		utils.Error("failed to get active workers",
			zap.String("request_id", requestID),
			zap.Error(err))
	} else {
		activeWorkers = len(workers)
	}

	// 获取系统状态摘要
	summary, err := h.logManager.GetSystemStatusSummary()
	if err != nil {
		utils.Error("failed to get system status summary",
			zap.String("request_id", requestID),
			zap.Error(err))
		summary = make(map[string]interface{})
	}

	// 获取成功率和平均执行时间
	successRate := 0.0
	avgExecutionTime := 0.0

	if val, ok := summary["success_rate"]; ok {
		if sr, ok := val.(float64); ok {
			successRate = sr
		}
	}

	if val, ok := summary["avg_duration"]; ok {
		if ad, ok := val.(float64); ok {
			avgExecutionTime = ad
		}
	}

	// 构建响应
	status := &SystemStatusResponse{
		TotalJobs:        totalJobs,
		RunningJobs:      runningJobs,
		ActiveWorkers:    activeWorkers,
		SuccessRate:      successRate,
		AvgExecutionTime: avgExecutionTime,
		Version:          h.version,
		StartTime:        h.startTime.Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, NewResponse(status, requestID))
}

// GetSystemStats 获取系统统计信息
// GET /api/v1/system/stats
func (h *SystemHandler) GetSystemStats(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)

	// 获取每日执行统计
	days := 7 // 默认获取7天的数据
	if daysParam := c.Query("days"); daysParam != "" {
		if d, err := strconv.Atoi(daysParam); err == nil {
			days = d
			if days <= 0 || days > 90 {
				days = 7
			}
		}
	}

	dailyStats, err := h.logManager.GetDailyJobExecutionStats(days)
	if err != nil {
		utils.Warn("failed to get daily execution stats",
			zap.String("request_id", requestID),
			zap.Error(err))
		dailyStats = make([]map[string]interface{}, 0)
	}

	// 获取Worker统计
	workerStats := make([]map[string]interface{}, 0)
	workers, _, err := h.workerManager.ListWorkers(1, 100)
	if err == nil && len(workers) > 0 {
		for _, worker := range workers {
			workerStat := map[string]interface{}{
				"id":             worker.ID,
				"hostname":       worker.Hostname,
				"ip":             worker.IP,
				"status":         worker.Status,
				"running_jobs":   len(worker.RunningJobs),
				"last_heartbeat": worker.LastHeartbeat,
			}
			workerStats = append(workerStats, workerStat)
		}
	}

	// 获取系统资源信息
	systemResources := map[string]interface{}{
		"cpu_cores":     runtime.NumCPU(),
		"goroutines":    runtime.NumGoroutine(),
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"server_uptime": time.Since(h.startTime).String(),
		"api_version":   h.version,
	}

	// 构建完整统计响应
	stats := map[string]interface{}{
		"daily_execution": dailyStats,
		"workers":         workerStats,
		"system":          systemResources,
	}

	c.JSON(http.StatusOK, NewResponse(stats, requestID))
}

// GetHealthCheck 系统健康检查
// GET /api/v1/health
func (h *SystemHandler) GetHealthCheck(c *gin.Context) {
	requestID := c.GetString(RequestIDKey)

	// 简单的健康检查响应
	health := map[string]interface{}{
		"status":  "ok",
		"version": h.version,
		"uptime":  time.Since(h.startTime).String(),
	}

	c.JSON(http.StatusOK, NewResponse(health, requestID))
}
