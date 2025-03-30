package models

import (
	"fmt"
	"time"
)

// Worker 表示工作节点数据模型，用于CLI工具中展示和操作Worker
type Worker struct {
	// 基本信息
	ID         string    `json:"id"`          // Worker唯一标识
	Hostname   string    `json:"hostname"`    // 主机名
	IP         string    `json:"ip"`          // IP地址
	CreateTime time.Time `json:"create_time"` // 创建时间
	UpdateTime time.Time `json:"update_time"` // 更新时间

	// 状态信息
	Status        string    `json:"status"`         // Worker状态：online, offline, disabled
	LastHeartbeat time.Time `json:"last_heartbeat"` // 最近一次心跳时间

	// 资源信息
	CPUCores    int     `json:"cpu_cores"`    // CPU核心数
	MemoryTotal int64   `json:"memory_total"` // 内存总量(MB)
	MemoryFree  int64   `json:"memory_free"`  // 可用内存(MB)
	DiskTotal   int64   `json:"disk_total"`   // 磁盘总量(GB)
	DiskFree    int64   `json:"disk_free"`    // 可用磁盘(GB)
	LoadAvg     float64 `json:"load_avg"`     // 负载平均值

	// 任务信息
	RunningJobs   []string `json:"running_jobs"`   // 正在运行的任务ID列表
	CompletedJobs int      `json:"completed_jobs"` // 已完成任务数量
	FailedJobs    int      `json:"failed_jobs"`    // 失败任务数量

	// 标签信息，用于任务调度策略
	Labels map[string]string `json:"labels"`
}

// WorkerList 表示Worker列表，包含分页信息
type WorkerList struct {
	Total    int64    `json:"total"`     // 总Worker数
	Page     int      `json:"page"`      // 当前页码
	PageSize int      `json:"page_size"` // 每页大小
	Items    []Worker `json:"items"`     // Worker列表
}

// WorkerStats 表示Worker统计信息
type WorkerStats struct {
	ID             string  `json:"id"`              // Worker ID
	Hostname       string  `json:"hostname"`        // 主机名
	IP             string  `json:"ip"`              // IP地址
	Status         string  `json:"status"`          // 状态
	RunningJobs    int     `json:"running_jobs"`    // 运行中的任务数量
	CPUUsage       float64 `json:"cpu_usage"`       // CPU使用率(%)
	MemoryUsage    float64 `json:"memory_usage"`    // 内存使用率(%)
	DiskUsage      float64 `json:"disk_usage"`      // 磁盘使用率(%)
	LoadAvg        float64 `json:"load_avg"`        // 平均负载
	UptimeHours    float64 `json:"uptime_hours"`    // 运行时间(小时)
	SuccessRate    float64 `json:"success_rate"`    // 任务成功率(%)
	LastHeartbeat  string  `json:"last_heartbeat"`  // 最后心跳时间
	CompletedJobs  int     `json:"completed_jobs"`  // 已完成任务数量
	FailedJobs     int     `json:"failed_jobs"`     // 失败任务数量
	AvgCompletions float64 `json:"avg_completions"` // 每小时平均完成任务数
}

// FormatMemory 将内存大小格式化为人类可读形式
func FormatMemory(memoryMB int64) string {
	if memoryMB < 1024 {
		return fmt.Sprintf("%d MB", memoryMB)
	}

	// 转换为GB，保留1位小数
	memoryGB := float64(memoryMB) / 1024.0
	return fmt.Sprintf("%.1f GB", memoryGB)
}

// FormatDisk 将磁盘大小格式化为人类可读形式
func FormatDisk(diskGB int64) string {
	if diskGB < 1024 {
		return fmt.Sprintf("%d GB", diskGB)
	}

	// 转换为TB，保留1位小数
	diskTB := float64(diskGB) / 1024.0
	return fmt.Sprintf("%.1f TB", diskTB)
}

// FormatHeartbeatTime 将最后心跳时间格式化为"多久之前"的形式
func FormatHeartbeatTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	return FormatTimeAgo(t)
}

// IsActive 检查Worker是否处于活跃状态
func (w *Worker) IsActive() bool {
	// 如果被禁用，则不活跃
	if w.Status == "disabled" {
		return false
	}

	// 如果最后心跳时间超过60秒，则不活跃
	heartbeatTimeout := 60 * time.Second
	if time.Since(w.LastHeartbeat) > heartbeatTimeout {
		return false
	}

	return true
}

// GetMemoryUsagePercent 计算内存使用百分比
func (w *Worker) GetMemoryUsagePercent() float64 {
	if w.MemoryTotal == 0 {
		return 0
	}

	memoryUsed := w.MemoryTotal - w.MemoryFree
	return float64(memoryUsed) / float64(w.MemoryTotal) * 100
}

// GetDiskUsagePercent 计算磁盘使用百分比
func (w *Worker) GetDiskUsagePercent() float64 {
	if w.DiskTotal == 0 {
		return 0
	}

	diskUsed := w.DiskTotal - w.DiskFree
	return float64(diskUsed) / float64(w.DiskTotal) * 100
}

// GetLabelsString 将标签映射格式化为字符串
func (w *Worker) GetLabelsString() string {
	if len(w.Labels) == 0 {
		return "none"
	}

	result := ""
	for k, v := range w.Labels {
		if result != "" {
			result += ", "
		}
		result += fmt.Sprintf("%s=%s", k, v)
	}

	return result
}

// GetRunningJobsCount 获取运行中任务数量
func (w *Worker) GetRunningJobsCount() int {
	return len(w.RunningJobs)
}

// GetSuccessRate 计算任务成功率
func (w *Worker) GetSuccessRate() float64 {
	totalJobs := w.CompletedJobs + w.FailedJobs
	if totalJobs == 0 {
		return 0
	}

	return float64(w.CompletedJobs) / float64(totalJobs) * 100
}
