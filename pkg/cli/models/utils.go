package models

import (
	"fmt"
	"time"
)

// GetStatusEmoji 根据任务状态返回对应的emoji表情
func GetStatusEmoji(status string) string {
	switch status {
	case "running":
		return "🔄"
	case "succeeded":
		return "✅"
	case "failed":
		return "❌"
	case "pending":
		return "⏳"
	case "cancelled":
		return "🛑"
	case "online":
		return "🟢"
	case "offline":
		return "🔴"
	case "disabled":
		return "⚫"
	case "busy":
		return "🟠"
	default:
		return "❓"
	}
}

// FormatDuration 格式化持续时间为人类可读形式
func FormatDuration(seconds float64) string {
	duration := time.Duration(seconds * float64(time.Second))

	// 对于非常短的时间，显示毫秒
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}

	// 对于一分钟以内的时间
	if duration < time.Minute {
		return fmt.Sprintf("%.2fs", seconds)
	}

	// 对于一小时以内的时间
	if duration < time.Hour {
		minutes := int(duration.Minutes())
		remainingSeconds := int(duration.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, remainingSeconds)
	}

	// 对于更长的时间
	hours := int(duration.Hours())
	remainingMinutes := int(duration.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, remainingMinutes)
}

// GetStatusColor 根据任务状态返回对应的颜色代码（ANSI转义序列）
func GetStatusColor(status string) string {
	switch status {
	case "running":
		return "\033[34m" // 蓝色
	case "succeeded":
		return "\033[32m" // 绿色
	case "failed":
		return "\033[31m" // 红色
	case "pending":
		return "\033[33m" // 黄色
	case "cancelled":
		return "\033[35m" // 紫色
	case "online":
		return "\033[32m" // 绿色
	case "offline":
		return "\033[31m" // 红色
	case "disabled":
		return "\033[90m" // 灰色
	case "busy":
		return "\033[33m" // 黄色
	default:
		return "\033[0m" // 默认色
	}
}

// ResetColor 返回ANSI重置颜色的转义序列
func ResetColor() string {
	return "\033[0m"
}
