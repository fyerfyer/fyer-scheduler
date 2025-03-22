package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// 通用响应结构
type Response struct {
	Success bool        `json:"success"`         // 请求是否成功
	Data    interface{} `json:"data,omitempty"`  // 响应数据
	Error   *ErrorInfo  `json:"error,omitempty"` // 错误信息
	TraceID string      `json:"trace_id"`        // 请求跟踪ID
	Time    int64       `json:"time"`            // 响应时间戳
}

// ErrorInfo 用于响应中传递错误信息
type ErrorInfo struct {
	Code    int    `json:"code"`              // 错误码
	Message string `json:"message"`           // 错误消息
	Details string `json:"details,omitempty"` // 详细错误信息，用于调试
}

// NewResponse 创建一个成功的响应
func NewResponse(data interface{}, traceID string) *Response {
	return &Response{
		Success: true,
		Data:    data,
		TraceID: traceID,
		Time:    time.Now().Unix(),
	}
}

// NewErrorResponse 创建一个错误响应
func NewErrorResponse(err *APIError, traceID string) *Response {
	return &Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    err.Code,
			Message: err.Message,
			Details: err.Details,
		},
		TraceID: traceID,
		Time:    time.Now().Unix(),
	}
}

// 分页请求基础结构
type PaginationRequest struct {
	Page     int `json:"page" form:"page" validate:"min=1"`                   // 当前页码
	PageSize int `json:"page_size" form:"page_size" validate:"min=1,max=100"` // 每页数量
}

// 分页响应基础结构
type PaginationResponse struct {
	Total    int64       `json:"total"`     // 总记录数
	Page     int         `json:"page"`      // 当前页码
	PageSize int         `json:"page_size"` // 每页数量
	Items    interface{} `json:"items"`     // 数据项列表
}

// CreateJobRequest 创建任务请求
type CreateJobRequest struct {
	Name        string            `json:"name" validate:"required"`       // 任务名称
	Description string            `json:"description"`                    // 任务描述
	Command     string            `json:"command" validate:"required"`    // 执行命令
	Args        []string          `json:"args"`                           // 命令参数
	CronExpr    string            `json:"cron_expr" validate:"omitempty"` // Cron表达式
	WorkDir     string            `json:"work_dir"`                       // 工作目录
	Env         map[string]string `json:"env"`                            // 环境变量
	Timeout     int               `json:"timeout" validate:"min=0"`       // 超时时间(秒)
	MaxRetry    int               `json:"max_retry" validate:"min=0"`     // 最大重试次数
	RetryDelay  int               `json:"retry_delay" validate:"min=0"`   // 重试间隔(秒)
	Enabled     bool              `json:"enabled"`                        // 是否启用
}

// UpdateJobRequest 更新任务请求
type UpdateJobRequest struct {
	Name        string            `json:"name" validate:"required"`       // 任务名称
	Description string            `json:"description"`                    // 任务描述
	Command     string            `json:"command" validate:"required"`    // 执行命令
	Args        []string          `json:"args"`                           // 命令参数
	CronExpr    string            `json:"cron_expr" validate:"omitempty"` // Cron表达式
	WorkDir     string            `json:"work_dir"`                       // 工作目录
	Env         map[string]string `json:"env"`                            // 环境变量
	Timeout     int               `json:"timeout" validate:"min=0"`       // 超时时间(秒)
	MaxRetry    int               `json:"max_retry" validate:"min=0"`     // 最大重试次数
	RetryDelay  int               `json:"retry_delay" validate:"min=0"`   // 重试间隔(秒)
	Enabled     bool              `json:"enabled"`                        // 是否启用
}

// JobResponse 任务响应
type JobResponse struct {
	ID           string            `json:"id"`             // 任务ID
	Name         string            `json:"name"`           // 任务名称
	Description  string            `json:"description"`    // 任务描述
	Command      string            `json:"command"`        // 执行命令
	Args         []string          `json:"args"`           // 命令参数
	CronExpr     string            `json:"cron_expr"`      // Cron表达式
	WorkDir      string            `json:"work_dir"`       // 工作目录
	Env          map[string]string `json:"env"`            // 环境变量
	Timeout      int               `json:"timeout"`        // 超时时间(秒)
	MaxRetry     int               `json:"max_retry"`      // 最大重试次数
	RetryDelay   int               `json:"retry_delay"`    // 重试间隔(秒)
	Enabled      bool              `json:"enabled"`        // 是否启用
	Status       string            `json:"status"`         // 任务状态
	LastRunTime  *time.Time        `json:"last_run_time"`  // 上次运行时间
	NextRunTime  *time.Time        `json:"next_run_time"`  // 下次运行时间
	LastResult   string            `json:"last_result"`    // 上次执行结果
	LastExitCode int               `json:"last_exit_code"` // 上次退出码
	CreateTime   time.Time         `json:"create_time"`    // 创建时间
	UpdateTime   time.Time         `json:"update_time"`    // 更新时间
}

// LogSearchRequest 日志搜索请求
type LogSearchRequest struct {
	PaginationRequest
	JobID     string    `json:"job_id" form:"job_id"`         // 任务ID
	WorkerID  string    `json:"worker_id" form:"worker_id"`   // Worker ID
	Status    string    `json:"status" form:"status"`         // 执行状态
	StartTime time.Time `json:"start_time" form:"start_time"` // 开始时间
	EndTime   time.Time `json:"end_time" form:"end_time"`     // 结束时间
}

// LogResponse 日志响应
type LogResponse struct {
	ExecutionID  string    `json:"execution_id"`  // 执行ID
	JobID        string    `json:"job_id"`        // 任务ID
	JobName      string    `json:"job_name"`      // 任务名称
	WorkerID     string    `json:"worker_id"`     // Worker ID
	WorkerIP     string    `json:"worker_ip"`     // Worker IP
	Command      string    `json:"command"`       // 执行命令
	Status       string    `json:"status"`        // 执行状态
	ExitCode     int       `json:"exit_code"`     // 退出码
	Error        string    `json:"error"`         // 错误信息
	Output       string    `json:"output"`        // 命令输出
	ScheduleTime time.Time `json:"schedule_time"` // 调度时间
	StartTime    time.Time `json:"start_time"`    // 开始时间
	EndTime      time.Time `json:"end_time"`      // 结束时间
	Duration     float64   `json:"duration"`      // 执行时长(秒)
	IsManual     bool      `json:"is_manual"`     // 是否手动触发
}

// TriggerJobRequest 触发任务请求
type TriggerJobRequest struct {
	WorkerID string `json:"worker_id"` // 指定Worker ID，可选
}

// TriggerJobResponse 触发任务响应
type TriggerJobResponse struct {
	ExecutionID string `json:"execution_id"` // 执行ID
	JobID       string `json:"job_id"`       // 任务ID
	Status      string `json:"status"`       // 初始状态
}

// WorkerResponse Worker响应
type WorkerResponse struct {
	ID            string            `json:"id"`             // Worker ID
	Hostname      string            `json:"hostname"`       // 主机名
	IP            string            `json:"ip"`             // IP地址
	Status        string            `json:"status"`         // 状态
	LastHeartbeat time.Time         `json:"last_heartbeat"` // 最后心跳时间
	RunningJobs   []string          `json:"running_jobs"`   // 运行中的任务
	CPUCores      int               `json:"cpu_cores"`      // CPU核心数
	MemoryTotal   int64             `json:"memory_total"`   // 总内存(MB)
	MemoryFree    int64             `json:"memory_free"`    // 可用内存(MB)
	DiskTotal     int64             `json:"disk_total"`     // 总磁盘(GB)
	DiskFree      int64             `json:"disk_free"`      // 可用磁盘(GB)
	LoadAvg       float64           `json:"load_avg"`       // 负载平均值
	Labels        map[string]string `json:"labels"`         // 标签
	CreateTime    time.Time         `json:"create_time"`    // 创建时间
}

// SystemStatusResponse 系统状态响应
type SystemStatusResponse struct {
	TotalJobs        int     `json:"total_jobs"`         // 总任务数
	RunningJobs      int     `json:"running_jobs"`       // 运行中的任务数
	ActiveWorkers    int     `json:"active_workers"`     // 活跃Worker数
	SuccessRate      float64 `json:"success_rate"`       // 成功率
	AvgExecutionTime float64 `json:"avg_execution_time"` // 平均执行时间
	Version          string  `json:"version"`            // 系统版本
	StartTime        string  `json:"start_time"`         // 启动时间
}

// 将Job模型转换为JobResponse
func ConvertToJobResponse(job *models.Job) *JobResponse {
	var lastRunTime, nextRunTime *time.Time

	if !job.LastRunTime.IsZero() {
		t := job.LastRunTime
		lastRunTime = &t
	}

	if !job.NextRunTime.IsZero() {
		t := job.NextRunTime
		nextRunTime = &t
	}

	return &JobResponse{
		ID:           job.ID,
		Name:         job.Name,
		Description:  job.Description,
		Command:      job.Command,
		Args:         job.Args,
		CronExpr:     job.CronExpr,
		WorkDir:      job.WorkDir,
		Env:          job.Env,
		Timeout:      job.Timeout,
		MaxRetry:     job.MaxRetry,
		RetryDelay:   job.RetryDelay,
		Enabled:      job.Enabled,
		Status:       job.Status,
		LastRunTime:  lastRunTime,
		NextRunTime:  nextRunTime,
		LastResult:   job.LastResult,
		LastExitCode: job.LastExitCode,
		CreateTime:   job.CreateTime,
		UpdateTime:   job.UpdateTime,
	}
}

// 将JobLog模型转换为LogResponse
func ConvertToLogResponse(log *models.JobLog) *LogResponse {
	return &LogResponse{
		ExecutionID:  log.ExecutionID,
		JobID:        log.JobID,
		JobName:      log.JobName,
		WorkerID:     log.WorkerID,
		WorkerIP:     log.WorkerIP,
		Command:      log.Command,
		Status:       log.Status,
		ExitCode:     log.ExitCode,
		Error:        log.Error,
		Output:       log.Output,
		ScheduleTime: log.ScheduleTime,
		StartTime:    log.StartTime,
		EndTime:      log.EndTime,
		Duration:     log.Duration(),
		IsManual:     log.IsManual,
	}
}

// 将Worker模型转换为WorkerResponse
func ConvertToWorkerResponse(worker *models.Worker) *WorkerResponse {
	return &WorkerResponse{
		ID:            worker.ID,
		Hostname:      worker.Hostname,
		IP:            worker.IP,
		Status:        worker.Status,
		LastHeartbeat: worker.LastHeartbeat,
		RunningJobs:   worker.RunningJobs,
		CPUCores:      worker.CPUCores,
		MemoryTotal:   worker.MemoryTotal,
		MemoryFree:    worker.MemoryFree,
		DiskTotal:     worker.DiskTotal,
		DiskFree:      worker.DiskFree,
		LoadAvg:       worker.LoadAvg,
		Labels:        worker.Labels,
		CreateTime:    worker.CreateTime,
	}
}

// CreateJobFromRequest 从请求创建任务模型
func CreateJobFromRequest(req *CreateJobRequest) (*models.Job, error) {
	// 验证请求数据
	if err := validate.Struct(req); err != nil {
		return nil, fmt.Errorf("invalid request data: %w", err)
	}

	// 创建新的任务模型
	job, err := models.NewJob(req.Name, req.Command, req.CronExpr)
	if err != nil {
		return nil, err
	}

	// 设置附加字段
	job.Description = req.Description
	job.Args = req.Args
	job.WorkDir = req.WorkDir
	job.Env = req.Env
	job.Timeout = req.Timeout
	job.MaxRetry = req.MaxRetry
	job.RetryDelay = req.RetryDelay
	job.Enabled = req.Enabled

	return job, nil
}

// UpdateJobFromRequest 从请求更新任务模型
func UpdateJobFromRequest(job *models.Job, req *UpdateJobRequest) error {
	// 验证请求数据
	if err := validate.Struct(req); err != nil {
		return fmt.Errorf("invalid request data: %w", err)
	}

	// 更新字段
	job.Name = req.Name
	job.Description = req.Description
	job.Command = req.Command
	job.Args = req.Args
	job.CronExpr = req.CronExpr
	job.WorkDir = req.WorkDir
	job.Env = req.Env
	job.Timeout = req.Timeout
	job.MaxRetry = req.MaxRetry
	job.RetryDelay = req.RetryDelay
	job.Enabled = req.Enabled
	job.UpdateTime = time.Now()

	// 如果有Cron表达式，重新计算下次运行时间
	if job.CronExpr != "" {
		if err := job.CalculateNextRunTime(); err != nil {
			return fmt.Errorf("failed to calculate next run time: %w", err)
		}
	}

	return nil
}

// ValidateRequest 通用请求验证函数
func ValidateRequest(req interface{}) error {
	if err := validate.Struct(req); err != nil {
		// 格式化验证错误
		var errMsgs []string
		for _, err := range err.(validator.ValidationErrors) {
			errMsgs = append(errMsgs, fmt.Sprintf("field '%s' validation failed: %s", err.Field(), err.Tag()))
		}
		return fmt.Errorf("invalid request: %s", strings.Join(errMsgs, "; "))
	}
	return nil
}
