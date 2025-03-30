package models

import (
	"fmt"
	"time"
)

// SystemStatus 表示系统状态数据模型，用于CLI工具中展示系统状态
type SystemStatus struct {
	// 任务统计
	TotalJobs        int     `json:"total_jobs"`         // 总任务数
	RunningJobs      int     `json:"running_jobs"`       // 运行中的任务数
	SuccessRate      float64 `json:"success_rate"`       // 任务成功率(%)
	AvgExecutionTime float64 `json:"avg_execution_time"` // 平均执行时间(秒)

	// Worker统计
	ActiveWorkers int `json:"active_workers"` // 活跃Worker数
	TotalWorkers  int `json:"total_workers"`  // 总Worker数

	// 系统信息
	Version   string `json:"version"`    // 系统版本
	StartTime string `json:"start_time"` // 系统启动时间
	Uptime    string `json:"uptime"`     // 系统运行时长
}

// SystemDailyStats 表示系统每日统计数据
type SystemDailyStats struct {
	Date         string  `json:"date"`          // 日期
	JobsRun      int     `json:"jobs_run"`      // 运行的任务数
	SuccessCount int     `json:"success_count"` // 成功任务数
	FailureCount int     `json:"failure_count"` // 失败任务数
	SuccessRate  float64 `json:"success_rate"`  // 成功率
	AvgDuration  float64 `json:"avg_duration"`  // 平均执行时间
}

// SystemWorkerStats 表示系统中Worker的统计数据
type SystemWorkerStats struct {
	TotalWorkers    int     `json:"total_workers"`    // 总Worker数
	OnlineWorkers   int     `json:"online_workers"`   // 在线Worker数
	OfflineWorkers  int     `json:"offline_workers"`  // 离线Worker数
	DisabledWorkers int     `json:"disabled_workers"` // 禁用Worker数
	TotalCPUCores   int     `json:"total_cpu_cores"`  // 总CPU核心数
	TotalMemoryGB   float64 `json:"total_memory_gb"`  // 总内存(GB)
	TotalDiskGB     float64 `json:"total_disk_gb"`    // 总磁盘空间(GB)
}

// SystemResources 表示系统资源统计
type SystemResources struct {
	CPUCores    int     `json:"cpu_cores"`    // CPU核心数
	MemoryTotal int64   `json:"memory_total"` // 内存总量(MB)
	MemoryFree  int64   `json:"memory_free"`  // 可用内存(MB)
	DiskTotal   int64   `json:"disk_total"`   // 磁盘总量(GB)
	DiskFree    int64   `json:"disk_free"`    // 可用磁盘(GB)
	LoadAvg     float64 `json:"load_avg"`     // 负载平均值
}

// HealthStatus 表示系统健康状态
type HealthStatus struct {
	Status     string            `json:"status"`     // 状态: ok, warning, error
	Components map[string]bool   `json:"components"` // 各组件状态
	LastCheck  time.Time         `json:"last_check"` // 最后检查时间
	Details    map[string]string `json:"details"`    // 详细信息
}

// FormatSystemStatus 格式化系统状态为人类可读的字符串
func FormatSystemStatus(status *SystemStatus) string {
	result := "System Status:\n"
	result += fmt.Sprintf("  Version:    %s\n", status.Version)
	result += fmt.Sprintf("  Start Time: %s\n", status.StartTime)
	result += fmt.Sprintf("  Uptime:     %s\n", status.Uptime)
	result += fmt.Sprintf("  Jobs:       %d total, %d running\n", status.TotalJobs, status.RunningJobs)
	result += fmt.Sprintf("  Workers:    %d active, %d total\n", status.ActiveWorkers, status.TotalWorkers)
	result += fmt.Sprintf("  Success Rate: %.1f%%\n", status.SuccessRate)
	result += fmt.Sprintf("  Avg Execution Time: %s\n", FormatDuration(status.AvgExecutionTime))

	return result
}

// FormatUptime 格式化系统运行时间为人类可读形式
func FormatUptime(uptimeStr string) string {
	// 尝试将字符串解析为持续时间
	uptime, err := time.ParseDuration(uptimeStr)
	if err != nil {
		return uptimeStr
	}

	days := int(uptime.Hours() / 24)
	hours := int(uptime.Hours()) % 24
	minutes := int(uptime.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}

	return fmt.Sprintf("%dm", minutes)
}

// GetHealthStatusEmoji 根据健康状态返回对应的emoji
func GetHealthStatusEmoji(status string) string {
	switch status {
	case "ok", "healthy":
		return "✅"
	case "warning":
		return "⚠️"
	case "error", "unhealthy":
		return "❌"
	default:
		return "❓"
	}
}

// GetHealthStatusColor 根据健康状态返回对应的颜色
func GetHealthStatusColor(status string) string {
	switch status {
	case "ok", "healthy":
		return "\033[32m" // 绿色
	case "warning":
		return "\033[33m" // 黄色
	case "error", "unhealthy":
		return "\033[31m" // 红色
	default:
		return "\033[0m" // 默认色
	}
}

// FormatMemoryUsage 格式化内存使用情况
func FormatMemoryUsage(total, free int64) string {
	used := total - free
	usedPercent := float64(used) / float64(total) * 100

	return fmt.Sprintf("%s used of %s (%.1f%%)",
		FormatMemory(used),
		FormatMemory(total),
		usedPercent)
}

// FormatDiskUsage 格式化磁盘使用情况
func FormatDiskUsage(total, free int64) string {
	used := total - free
	usedPercent := float64(used) / float64(total) * 100

	return fmt.Sprintf("%s used of %s (%.1f%%)",
		FormatDisk(used),
		FormatDisk(total),
		usedPercent)
}

// FormatSystemSummary 格式化系统摘要信息
func FormatSystemSummary(status *SystemStatus) string {
	if status == nil {
		return "System information not available"
	}

	// 基于系统状态构建简短摘要
	summary := fmt.Sprintf("%s Version %s | Uptime: %s\n",
		GetStatusEmoji("online"),
		status.Version,
		FormatUptime(status.Uptime))

	summary += fmt.Sprintf("Jobs: %d total, %d running | Success Rate: %.1f%%\n",
		status.TotalJobs,
		status.RunningJobs,
		status.SuccessRate)

	summary += fmt.Sprintf("Workers: %d active of %d total\n",
		status.ActiveWorkers,
		status.TotalWorkers)

	return summary
}

// FormatDailyStatsTable 格式化每日统计数据为表格行数据
func FormatDailyStatsTable(stats []SystemDailyStats) [][]string {
	rows := make([][]string, 0, len(stats)+1)

	// 表头
	rows = append(rows, []string{"Date", "Jobs Run", "Success", "Failure", "Success Rate", "Avg Duration"})

	// 数据行
	for _, stat := range stats {
		row := []string{
			stat.Date,
			fmt.Sprintf("%d", stat.JobsRun),
			fmt.Sprintf("%d", stat.SuccessCount),
			fmt.Sprintf("%d", stat.FailureCount),
			fmt.Sprintf("%.1f%%", stat.SuccessRate),
			FormatDuration(stat.AvgDuration),
		}
		rows = append(rows, row)
	}

	return rows
}

// FormatWorkerStatsTable 格式化Worker统计数据为表格行数据
func FormatWorkerStatsTable(stats SystemWorkerStats) [][]string {
	rows := make([][]string, 0, 8)

	// 添加Worker统计行
	rows = append(rows, []string{"Metric", "Value"})
	rows = append(rows, []string{"Total Workers", fmt.Sprintf("%d", stats.TotalWorkers)})
	rows = append(rows, []string{"Online Workers", fmt.Sprintf("%d", stats.OnlineWorkers)})
	rows = append(rows, []string{"Offline Workers", fmt.Sprintf("%d", stats.OfflineWorkers)})
	rows = append(rows, []string{"Disabled Workers", fmt.Sprintf("%d", stats.DisabledWorkers)})
	rows = append(rows, []string{"Total CPU Cores", fmt.Sprintf("%d", stats.TotalCPUCores)})
	rows = append(rows, []string{"Total Memory", fmt.Sprintf("%.1f GB", stats.TotalMemoryGB)})
	rows = append(rows, []string{"Total Disk Space", fmt.Sprintf("%.1f GB", stats.TotalDiskGB)})

	return rows
}
