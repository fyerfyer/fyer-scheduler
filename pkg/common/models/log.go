package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// JobLog 表示一个任务执行日志
type JobLog struct {
	// 数据库标识
	ID primitive.ObjectID `json:"id" bson:"_id,omitempty"` // MongoDB ObjectID

	// 基本信息
	ExecutionID string `json:"execution_id" bson:"execution_id"` // 执行ID
	JobID       string `json:"job_id" bson:"job_id"`             // 任务ID
	JobName     string `json:"job_name" bson:"job_name"`         // 任务名称
	WorkerID    string `json:"worker_id" bson:"worker_id"`       // 执行Worker的ID
	WorkerIP    string `json:"worker_ip" bson:"worker_ip"`       // 执行Worker的IP

	// 时间信息
	ScheduleTime time.Time `json:"schedule_time" bson:"schedule_time"` // 调度时间
	StartTime    time.Time `json:"start_time" bson:"start_time"`       // 开始执行时间
	EndTime      time.Time `json:"end_time" bson:"end_time"`           // 结束时间

	// 执行信息
	Command    string `json:"command" bson:"command"`         // 执行的命令
	Status     string `json:"status" bson:"status"`           // 执行状态
	ExitCode   int    `json:"exit_code" bson:"exit_code"`     // 退出码
	Error      string `json:"error,omitempty" bson:"error"`   // 错误信息
	Output     string `json:"output,omitempty" bson:"output"` // 命令输出(可能很长)
	RetryCount int    `json:"retry_count" bson:"retry_count"` // 重试次数
	IsManual   bool   `json:"is_manual" bson:"is_manual"`     // 是否手动触发
}

// NewJobLog 创建新的任务执行日志
func NewJobLog(jobID, jobName, workerID, workerIP, command string, isManual bool) *JobLog {
	now := time.Now()
	return &JobLog{
		ID:           primitive.NewObjectID(),
		ExecutionID:  uuid.New().String(),
		JobID:        jobID,
		JobName:      jobName,
		WorkerID:     workerID,
		WorkerIP:     workerIP,
		Command:      command,
		Status:       constants.JobStatusPending,
		ScheduleTime: now,
		RetryCount:   0,
		IsManual:     isManual,
	}
}

// SetStarted 设置日志为已开始状态
func (l *JobLog) SetStarted() {
	l.Status = constants.JobStatusRunning
	l.StartTime = time.Now()
}

// SetCompleted 设置日志为已完成状态
func (l *JobLog) SetCompleted(exitCode int, output string) {
	now := time.Now()
	l.Status = constants.JobStatusSucceeded
	l.EndTime = now
	l.ExitCode = exitCode
	l.Output = output
}

// SetFailed 设置日志为失败状态
func (l *JobLog) SetFailed(exitCode int, errMsg string, output string) {
	now := time.Now()
	l.Status = constants.JobStatusFailed
	l.EndTime = now
	l.ExitCode = exitCode
	l.Error = errMsg
	l.Output = output
}

// SetCancelled 设置日志为已取消状态
func (l *JobLog) SetCancelled(reason string) {
	now := time.Now()
	l.Status = constants.JobStatusCancelled
	l.EndTime = now
	l.Error = reason
}

// Duration 计算任务执行时长(秒)
func (l *JobLog) Duration() float64 {
	// 如果尚未完成，使用当前时间计算运行时间
	if l.Status == constants.JobStatusRunning {
		return time.Since(l.StartTime).Seconds()
	}

	// 如果还未开始，返回0
	if l.StartTime.IsZero() {
		return 0
	}

	// 如果已完成，使用结束时间计算
	return l.EndTime.Sub(l.StartTime).Seconds()
}

// IncrementRetry 增加重试计数
func (l *JobLog) IncrementRetry() {
	l.RetryCount++
}

// IsComplete 检查日志是否已完成(无论成功还是失败)
func (l *JobLog) IsComplete() bool {
	return l.Status == constants.JobStatusSucceeded ||
		l.Status == constants.JobStatusFailed ||
		l.Status == constants.JobStatusCancelled
}

// ToJSON 将日志转换为JSON字符串
func (l *JobLog) ToJSON() (string, error) {
	bytes, err := json.Marshal(l)
	if err != nil {
		return "", fmt.Errorf("marshal job log error: %w", err)
	}
	return string(bytes), nil
}

// LogFromJSON 从JSON字符串解析日志
func LogFromJSON(jsonStr string) (*JobLog, error) {
	log := &JobLog{}
	err := json.Unmarshal([]byte(jsonStr), log)
	if err != nil {
		return nil, fmt.Errorf("unmarshal job log error: %w", err)
	}
	return log, nil
}

// TruncatedOutput 获取截断的输出(用于显示)
func (l *JobLog) TruncatedOutput(maxLen int) string {
	if len(l.Output) <= maxLen {
		return l.Output
	}
	return l.Output[:maxLen] + "...(truncated)"
}
