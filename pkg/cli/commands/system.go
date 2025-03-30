package commands

import (
	"fmt"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/cli/models"
)

// SystemCommand 处理系统相关命令
type SystemCommand struct {
	*CommandManager
}

// NewSystemCommand 创建一个新的系统命令处理器
func NewSystemCommand(cmdManager *CommandManager) *SystemCommand {
	return &SystemCommand{
		CommandManager: cmdManager,
	}
}

// HandleSystemCommands 处理系统相关的交互式命令
func (s *SystemCommand) HandleSystemCommands() error {
	for {
		// 显示系统命令菜单
		options := []string{
			"Show System Status",
			"Check System Health",
			"Show System Resources",
			"Show Worker Statistics",
			"Back to Main Menu",
		}

		selected, err := s.Prompt.Select("Select System Command", options)
		if err != nil {
			return fmt.Errorf("failed to select command: %w", err)
		}

		// 根据选择执行相应的命令
		switch selected {
		case "Show System Status":
			err = s.ShowSystemStatus()
		case "Check System Health":
			err = s.CheckSystemHealth()
		case "Show System Resources":
			err = s.ShowSystemResources()
		case "Show Worker Statistics":
			err = s.ShowWorkerStatistics()
		case "Back to Main Menu":
			return nil
		}

		if err != nil {
			s.DisplayError(fmt.Sprintf("Command failed: %v", err))
		}

		// 等待用户按键继续
		fmt.Println("\nPress Enter to continue...")
		fmt.Scanln()
	}
}

// ShowSystemStatus 显示系统状态信息
func (s *SystemCommand) ShowSystemStatus() error {
	// 显示加载动画
	s.Spinner.Start("Fetching system status...")

	// 获取系统状态
	status, err := s.Client.GetSystemStatus()
	s.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get system status: %w", err)
	}

	// 格式化系统状态为可读字符串
	statusStr := fmt.Sprintf("System Version: %s\n", status.Version)
	statusStr += fmt.Sprintf("Start Time: %s\n", status.StartTime)

	// 计算运行时间
	uptime := "Unknown"
	if status.StartTime != "" {
		startTime, err := time.Parse(time.RFC3339, status.StartTime)
		if err == nil {
			uptime = models.FormatDuration(time.Since(startTime).Seconds())
		}
	}
	statusStr += fmt.Sprintf("Uptime: %s\n", uptime)

	statusStr += fmt.Sprintf("\nJobs Summary:\n")
	statusStr += fmt.Sprintf("  Total Jobs: %d\n", status.TotalJobs)
	statusStr += fmt.Sprintf("  Running Jobs: %d\n", status.RunningJobs)
	statusStr += fmt.Sprintf("  Success Rate: %.1f%%\n", status.SuccessRate)
	statusStr += fmt.Sprintf("  Avg Execution Time: %s\n", models.FormatDuration(status.AvgExecutionTime))

	statusStr += fmt.Sprintf("\nWorkers Summary:\n")
	statusStr += fmt.Sprintf("  Active Workers: %d\n", status.ActiveWorkers)

	// 显示系统状态
	fmt.Println("\n===== System Status =====")
	fmt.Println(statusStr)

	return nil
}

// CheckSystemHealth 检查系统健康状态
func (s *SystemCommand) CheckSystemHealth() error {
	// 显示加载动画
	s.Spinner.Start("Checking system health...")

	// 检查系统健康状态
	healthy, err := s.Client.CheckHealth()
	s.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to check system health: %w", err)
	}

	// 显示健康状态
	fmt.Println("\n===== System Health Check =====")
	if healthy {
		s.DisplaySuccess("System is healthy and responding normally")
	} else {
		s.DisplayError("System health check failed - the system may be experiencing issues")
	}

	return nil
}

// ShowSystemResources 显示系统资源信息
func (s *SystemCommand) ShowSystemResources() error {
	// 显示加载动画
	s.Spinner.Start("Fetching system resources...")

	// 获取系统状态，包含资源信息
	status, err := s.Client.GetSystemStatus()
	s.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get system resources: %w", err)
	}

	// 显示资源信息
	fmt.Println("\n===== System Resources =====")
	fmt.Printf("Active Workers: %d\n", status.ActiveWorkers)
	fmt.Printf("Jobs: %d total, %d running\n", status.TotalJobs, status.RunningJobs)
	fmt.Printf("Average Job Success Rate: %.1f%%\n", status.SuccessRate)
	fmt.Printf("Average Job Execution Time: %s\n", models.FormatDuration(status.AvgExecutionTime))

	return nil
}

// ShowWorkerStatistics 显示Worker统计信息
func (s *SystemCommand) ShowWorkerStatistics() error {
	// 显示加载动画
	s.Spinner.Start("Fetching worker statistics...")

	// 获取活跃Worker列表
	activeWorkers, err := s.Client.GetActiveWorkers()
	s.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get active workers: %w", err)
	}

	if len(activeWorkers) == 0 {
		s.DisplayInfo("No active workers found")
		return nil
	}

	// 准备表格数据
	headers := []string{"ID", "Hostname", "IP", "Status", "Running Jobs", "Last Heartbeat"}
	data := make([][]string, len(activeWorkers))

	for i, worker := range activeWorkers {
		// 计算上次心跳的相对时间
		lastHeartbeat := "Never"
		if !worker.LastHeartbeat.IsZero() {
			lastHeartbeat = models.FormatTimeAgo(worker.LastHeartbeat)
		}

		// 填充表格行
		data[i] = []string{
			worker.ID,
			worker.Hostname,
			worker.IP,
			worker.Status,
			fmt.Sprintf("%d", len(worker.RunningJobs)),
			lastHeartbeat,
		}
	}

	// 显示Worker统计表格
	fmt.Println("\n===== Worker Statistics =====")
	s.Table.DisplayStatusTable(headers, data, 3) // 状态列是第4列（索引3）

	return nil
}

// ValidateYesNo 验证Yes/No输入
func ValidateYesNo(input string) error {
	if input != "y" && input != "n" && input != "Y" && input != "N" &&
		input != "yes" && input != "no" && input != "Yes" && input != "No" {
		return fmt.Errorf("please enter y/n or yes/no")
	}
	return nil
}
