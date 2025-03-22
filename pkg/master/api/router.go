package api

import (
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/gin-gonic/gin"
)

// Router 处理API路由的设置和管理
type Router struct {
	jobHandler    *JobHandler    // 任务处理器
	logHandler    *LogHandler    // 日志处理器
	workerHandler *WorkerHandler // Worker处理器
	systemHandler *SystemHandler // 系统处理器
	apiKeys       []string       // API密钥列表，用于认证
}

// NewRouter 创建新的路由管理器
func NewRouter(
	jobHandler *JobHandler,
	logHandler *LogHandler,
	workerHandler *WorkerHandler,
	systemHandler *SystemHandler,
	apiKeys []string,
) *Router {
	return &Router{
		jobHandler:    jobHandler,
		logHandler:    logHandler,
		workerHandler: workerHandler,
		systemHandler: systemHandler,
		apiKeys:       apiKeys,
	}
}

// Setup 设置API路由和中间件
func (r *Router) Setup(router *gin.Engine) {
	// 应用全局中间件
	router.Use(Recovery())
	router.Use(RequestID())
	router.Use(RequestLogger())
	router.Use(CORS())

	// 健康检查路由，不需要认证
	router.GET("/health", r.systemHandler.GetHealthCheck)

	// API路由组，应用API密钥认证
	apiRoutes := router.Group("/api/v1")
	if len(r.apiKeys) > 0 {
		utils.Info("api authentication enabled")
		apiRoutes.Use(APIKeyAuth(r.apiKeys))
	} else {
		utils.Warn("api authentication disabled - no API keys configured")
	}

	// 系统相关路由
	systemRoutes := apiRoutes.Group("/system")
	{
		systemRoutes.GET("/status", r.systemHandler.GetSystemStatus)
		systemRoutes.GET("/stats", r.systemHandler.GetSystemStats)
	}

	// 任务相关路由
	jobRoutes := apiRoutes.Group("/jobs")
	{
		jobRoutes.POST("", r.jobHandler.CreateJob)                 // 创建任务
		jobRoutes.GET("", r.jobHandler.ListJobs)                   // 获取任务列表
		jobRoutes.GET("/:id", r.jobHandler.GetJob)                 // 获取单个任务
		jobRoutes.PUT("/:id", r.jobHandler.UpdateJob)              // 更新任务
		jobRoutes.DELETE("/:id", r.jobHandler.DeleteJob)           // 删除任务
		jobRoutes.POST("/:id/run", r.jobHandler.TriggerJob)        // 立即执行任务
		jobRoutes.POST("/:id/kill", r.jobHandler.KillJob)          // 终止任务
		jobRoutes.PUT("/:id/enable", r.jobHandler.EnableJob)       // 启用任务
		jobRoutes.PUT("/:id/disable", r.jobHandler.DisableJob)     // 禁用任务
		jobRoutes.GET("/:id/stats", r.jobHandler.GetJobStatistics) // 获取任务统计
	}

	// 日志相关路由
	logRoutes := apiRoutes.Group("/logs")
	{
		logRoutes.GET("/jobs/:jobID", r.logHandler.GetJobLogs)                            // 获取任务日志
		logRoutes.GET("/executions/:executionID", r.logHandler.GetExecutionLog)           // 获取执行日志
		logRoutes.GET("/executions/:executionID/stream", r.logHandler.StreamExecutionLog) // 日志流
		logRoutes.GET("/jobs/:jobID/stats", r.logHandler.GetJobLogStats)                  // 任务日志统计
		logRoutes.GET("/system", r.logHandler.GetSystemLogs)                              // 系统日志
	}

	// Worker相关路由
	workerRoutes := apiRoutes.Group("/workers")
	{
		workerRoutes.GET("", r.workerHandler.ListWorkers)                   // 获取Worker列表
		workerRoutes.GET("/active", r.workerHandler.GetActiveWorkers)       // 获取活跃Worker
		workerRoutes.GET("/:id", r.workerHandler.GetWorker)                 // 获取单个Worker
		workerRoutes.PUT("/:id/enable", r.workerHandler.EnableWorker)       // 启用Worker
		workerRoutes.PUT("/:id/disable", r.workerHandler.DisableWorker)     // 禁用Worker
		workerRoutes.GET("/:id/health", r.workerHandler.CheckWorkerHealth)  // 检查Worker健康
		workerRoutes.PUT("/:id/labels", r.workerHandler.UpdateWorkerLabels) // 更新Worker标签
	}

	utils.Info("api routes initialized")
}
