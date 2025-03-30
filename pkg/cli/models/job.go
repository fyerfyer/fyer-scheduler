package models

import (
	"fmt"
	"time"
)

// Job 表示任务数据模型，用于CLI工具中展示和操作任务
type Job struct {
	// 基本信息
	ID          string    `json:"id"`          // 任务ID
	Name        string    `json:"name"`        // 任务名称
	Description string    `json:"description"` // 任务描述
	CreateTime  time.Time `json:"create_time"` // 创建时间
	UpdateTime  time.Time `json:"update_time"` // 更新时间

	// 执行信息
	Command    string            `json:"command"`     // 执行命令
	Args       []string          `json:"args"`        // 命令参数
	WorkDir    string            `json:"work_dir"`    // 工作目录
	Env        map[string]string `json:"env"`         // 环境变量
	Timeout    int               `json:"timeout"`     // 超时时间（秒）
	MaxRetry   int               `json:"max_retry"`   // 最大重试次数
	RetryDelay int               `json:"retry_delay"` // 重试间隔（秒）

	// 调度信息
	CronExpr    string     `json:"cron_expr"`     // Cron表达式
	Enabled     bool       `json:"enabled"`       // 是否启用
	LastRunTime *time.Time `json:"last_run_time"` // 上次运行时间
	NextRunTime *time.Time `json:"next_run_time"` // 下次运行时间

	// 状态信息
	Status       string `json:"status"`         // 任务状态
	LastResult   string `json:"last_result"`    // 上次执行结果
	LastExitCode int    `json:"last_exit_code"` // 上次退出码
}

// JobList 表示任务列表，包含分页信息
type JobList struct {
	Total    int64 `json:"total"`     // 总任务数
	Page     int   `json:"page"`      // 当前页码
	PageSize int   `json:"page_size"` // 每页大小
	Items    []Job `json:"items"`     // 任务列表
}

// CreateJobRequest 创建任务的请求模型
type CreateJobRequest struct {
	Name        string            `json:"name"`        // 任务名称
	Description string            `json:"description"` // 任务描述
	Command     string            `json:"command"`     // 执行命令
	Args        []string          `json:"args"`        // 命令参数
	CronExpr    string            `json:"cron_expr"`   // Cron表达式
	WorkDir     string            `json:"work_dir"`    // 工作目录
	Env         map[string]string `json:"env"`         // 环境变量
	Timeout     int               `json:"timeout"`     // 超时时间（秒）
	MaxRetry    int               `json:"max_retry"`   // 最大重试次数
	RetryDelay  int               `json:"retry_delay"` // 重试间隔（秒）
	Enabled     bool              `json:"enabled"`     // 是否启用
}

// UpdateJobRequest 更新任务的请求模型
type UpdateJobRequest struct {
	Name        string            `json:"name"`        // 任务名称
	Description string            `json:"description"` // 任务描述
	Command     string            `json:"command"`     // 执行命令
	Args        []string          `json:"args"`        // 命令参数
	CronExpr    string            `json:"cron_expr"`   // Cron表达式
	WorkDir     string            `json:"work_dir"`    // 工作目录
	Env         map[string]string `json:"env"`         // 环境变量
	Timeout     int               `json:"timeout"`     // 超时时间（秒）
	MaxRetry    int               `json:"max_retry"`   // 最大重试次数
	RetryDelay  int               `json:"retry_delay"` // 重试间隔（秒）
	Enabled     bool              `json:"enabled"`     // 是否启用
}

// JobLog 任务日志模型
type JobLog struct {
	ExecutionID  string    `json:"execution_id"`  // 执行ID
	JobID        string    `json:"job_id"`        // 任务ID
	JobName      string    `json:"job_name"`      // 任务名称
	WorkerID     string    `json:"worker_id"`     // 执行Worker的ID
	WorkerIP     string    `json:"worker_ip"`     // 执行Worker的IP
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

// JobLogList 任务日志列表，包含分页信息
type JobLogList struct {
	Total    int64    `json:"total"`     // 总日志数
	Page     int      `json:"page"`      // 当前页码
	PageSize int      `json:"page_size"` // 每页大小
	Items    []JobLog `json:"items"`     // 日志列表
}

// JobStats 任务统计信息
type JobStats struct {
	JobID           string  `json:"job_id"`            // 任务ID
	JobName         string  `json:"job_name"`          // 任务名称
	TotalRuns       int     `json:"total_runs"`        // 总执行次数
	SuccessRuns     int     `json:"success_runs"`      // 成功执行次数
	FailureRuns     int     `json:"failure_runs"`      // 失败执行次数
	SuccessRate     float64 `json:"success_rate"`      // 成功率
	AvgDuration     float64 `json:"avg_duration"`      // 平均执行时长
	LastExecution   string  `json:"last_execution"`    // 最近执行时间
	NextExecution   string  `json:"next_execution"`    // 下次执行时间
	AverageExitCode float64 `json:"average_exit_code"` // 平均退出码
}

// NewCreateJobRequest 创建一个默认的任务创建请求
func NewCreateJobRequest() *CreateJobRequest {
	return &CreateJobRequest{
		Args:       []string{},
		Env:        make(map[string]string),
		Timeout:    3600, // 默认1小时超时
		MaxRetry:   0,    // 默认不重试
		RetryDelay: 60,   // 默认重试间隔60秒
		Enabled:    true, // 默认启用
	}
}

// NewUpdateJobRequest 从现有任务创建一个更新请求
func NewUpdateJobRequest(job *Job) *UpdateJobRequest {
	return &UpdateJobRequest{
		Name:        job.Name,
		Description: job.Description,
		Command:     job.Command,
		Args:        job.Args,
		CronExpr:    job.CronExpr,
		WorkDir:     job.WorkDir,
		Env:         job.Env,
		Timeout:     job.Timeout,
		MaxRetry:    job.MaxRetry,
		RetryDelay:  job.RetryDelay,
		Enabled:     job.Enabled,
	}
}

// FormatTimeAgo 将时间格式化为"多久之前"的形式
func FormatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	}

	if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}

	if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}

	days := int(duration.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}
