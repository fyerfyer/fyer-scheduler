package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// Job 表示一个需要调度执行的任务
type Job struct {
	// 基本信息
	ID          string    `json:"id" bson:"id"`                   // 任务唯一标识
	Name        string    `json:"name" bson:"name"`               // 任务名称
	Description string    `json:"description" bson:"description"` // 任务描述
	CreateTime  time.Time `json:"create_time" bson:"create_time"` // 创建时间
	UpdateTime  time.Time `json:"update_time" bson:"update_time"` // 更新时间
	Creator     string    `json:"creator" bson:"creator"`         // 创建者

	// 执行信息
	Command    string            `json:"command"`     // 要执行的命令
	Args       []string          `json:"args"`        // 命令参数
	Env        map[string]string `json:"env"`         // 环境变量
	WorkDir    string            `json:"work_dir"`    // 工作目录
	Timeout    int               `json:"timeout"`     // 超时时间(秒)
	MaxRetry   int               `json:"max_retry"`   // 最大重试次数
	RetryDelay int               `json:"retry_delay"` // 重试间隔(秒)

	// 调度信息
	CronExpr    string    `json:"cron_expr" bson:"cron_expr"`         // Cron表达式
	Enabled     bool      `json:"enabled" bson:"enabled"`             // 是否启用
	LastRunTime time.Time `json:"last_run_time" bson:"last_run_time"` // 上次运行时间
	NextRunTime time.Time `json:"next_run_time" bson:"next_run_time"` // 下次运行时间

	// 状态信息
	Status       string `json:"status" bson:"status"`                 // 任务状态
	LastResult   string `json:"last_result" bson:"last_result"`       // 上次执行结果
	LastExitCode int    `json:"last_exit_code" bson:"last_exit_code"` // 上次退出码
}

// NewJob 创建一个新任务
func NewJob(name, command, cronExpr string) (*Job, error) {
	// 验证Cron表达式
	if cronExpr != "" {
		_, err := cron.ParseStandard(cronExpr)
		if err != nil {
			return nil, fmt.Errorf("invalid cron expression: %w", err)
		}
	}

	// 验证命令
	if command == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}

	// 创建任务
	now := time.Now()
	job := &Job{
		ID:         uuid.New().String(),
		Name:       name,
		Command:    command,
		CronExpr:   cronExpr,
		CreateTime: now,
		UpdateTime: now,
		Status:     constants.JobStatusPending,
		Enabled:    true,
		MaxRetry:   0,
		RetryDelay: 60,
		Timeout:    3600, // 默认1小时超时
	}

	// 如果有Cron表达式，计算下次运行时间
	if cronExpr != "" {
		if err := job.CalculateNextRunTime(); err != nil {
			return nil, fmt.Errorf("calculate next run time error: %w", err)
		}
	}

	return job, nil
}

// Validate 验证任务是否有效
func (j *Job) Validate() error {
	if j.Name == "" {
		return fmt.Errorf("job name cannot be empty")
	}

	if j.Command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	if j.CronExpr != "" {
		_, err := cron.ParseStandard(j.CronExpr)
		if err != nil {
			return fmt.Errorf("invalid cron expression: %w", err)
		}
	}

	return nil
}

// CalculateNextRunTime 计算下次运行时间
func (j *Job) CalculateNextRunTime() error {
	if j.CronExpr == "" {
		return fmt.Errorf("cron expression is empty")
	}

	schedule, err := cron.ParseStandard(j.CronExpr)
	if err != nil {
		return fmt.Errorf("parse cron expression error: %w", err)
	}

	baseTime := time.Now()
	if !j.LastRunTime.IsZero() {
		baseTime = j.LastRunTime
	}

	j.NextRunTime = schedule.Next(baseTime)
	return nil
}

// ToJSON 将任务转换为JSON字符串
func (j *Job) ToJSON() (string, error) {
	bytes, err := json.Marshal(j)
	if err != nil {
		return "", fmt.Errorf("marshal job error: %w", err)
	}
	return string(bytes), nil
}

// FromJSON 从JSON字符串解析任务
func FromJSON(jsonStr string) (*Job, error) {
	job := &Job{}
	err := json.Unmarshal([]byte(jsonStr), job)
	if err != nil {
		return nil, fmt.Errorf("unmarshal job error: %w", err)
	}
	return job, nil
}

// SetStatus 设置任务状态
func (j *Job) SetStatus(status string) {
	j.Status = status
	j.UpdateTime = time.Now()
}

// Key 获取任务在etcd中的存储键
func (j *Job) Key() string {
	return constants.JobPrefix + j.ID
}

// IsRunnable 检查任务是否可运行
func (j *Job) IsRunnable() bool {
	return j.Enabled && j.Status != constants.JobStatusRunning
}
