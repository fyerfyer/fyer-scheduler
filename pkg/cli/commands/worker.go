package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/cli/models"
)

// WorkerCommand 处理Worker相关命令
type WorkerCommand struct {
	*CommandManager
}

// NewWorkerCommand 创建一个新的Worker命令处理器
func NewWorkerCommand(cmdManager *CommandManager) *WorkerCommand {
	return &WorkerCommand{
		CommandManager: cmdManager,
	}
}

// HandleWorkerCommands 处理Worker相关的交互式命令
func (w *WorkerCommand) HandleWorkerCommands() error {
	for {
		// 显示Worker命令菜单
		options := []string{
			"List Workers",
			"View Worker Details",
			"List Active Workers",
			"Enable/Disable Worker",
			"Check Worker Health",
			"Back to Main Menu",
		}

		selected, err := w.Prompt.Select("Select Worker Command", options)
		if err != nil {
			return fmt.Errorf("failed to select command: %w", err)
		}

		// 根据选择执行相应的命令
		switch selected {
		case "List Workers":
			err = w.ListWorkers()
		case "View Worker Details":
			err = w.ViewWorkerDetails()
		case "List Active Workers":
			err = w.ListActiveWorkers()
		case "Enable/Disable Worker":
			err = w.EnableDisableWorker()
		case "Check Worker Health":
			err = w.CheckWorkerHealth()
		case "Back to Main Menu":
			return nil
		}

		if err != nil {
			w.DisplayError(fmt.Sprintf("Command failed: %v", err))
		}

		// 等待用户按键继续
		fmt.Println("\nPress Enter to continue...")
		fmt.Scanln()
	}
}

// ListWorkers 显示Worker列表
func (w *WorkerCommand) ListWorkers() error {
	// 询问分页参数
	params, err := w.AskForPagination()
	if err != nil {
		return err
	}

	// 显示加载动画
	w.Spinner.Start("Fetching workers list...")

	// 获取Worker列表
	workers, total, err := w.Client.ListWorkers(params.Page, params.PageSize)
	w.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to list workers: %w", err)
	}

	if len(workers) == 0 {
		w.DisplayInfo("No workers found")
		return nil
	}

	// 准备表格数据
	headers := []string{"ID", "Hostname", "IP", "Status", "CPU Cores", "Running Jobs", "Last Heartbeat"}
	data := make([][]string, len(workers))

	for i, worker := range workers {
		// 格式化上次心跳时间
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
			fmt.Sprintf("%d", worker.CPUCores),
			fmt.Sprintf("%d", len(worker.RunningJobs)),
			lastHeartbeat,
		}
	}

	// 显示表格，状态列添加颜色和emoji
	w.Table.DisplayStatusTable(headers, data, 3) // 状态列是第4列（索引3）

	// 显示分页信息
	footer := GetPaginationFooter(params.Page, params.PageSize, total)
	fmt.Printf("\n%s\n", strings.Join(footer, " | "))

	return nil
}

// ViewWorkerDetails 查看Worker详情
func (w *WorkerCommand) ViewWorkerDetails() error {
	// 选择要查看的Worker
	workerID, err := w.selectWorker("Select worker to view")
	if err != nil {
		return err
	}

	// 显示加载动画
	w.Spinner.Start("Fetching worker details...")

	// 获取Worker详情
	worker, err := w.Client.GetWorker(workerID)
	w.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get worker details: %w", err)
	}

	// 格式化Worker详情为键值对
	memoryUsagePercent := float64(0)
	if worker.MemoryTotal > 0 {
		memoryUsed := worker.MemoryTotal - worker.MemoryFree
		memoryUsagePercent = float64(memoryUsed) / float64(worker.MemoryTotal) * 100
	}

	diskUsagePercent := float64(0)
	if worker.DiskTotal > 0 {
		diskUsed := worker.DiskTotal - worker.DiskFree
		diskUsagePercent = float64(diskUsed) / float64(worker.DiskTotal) * 100
	}

	// 格式化标签
	labels := "None"
	if len(worker.Labels) > 0 {
		labelPairs := make([]string, 0, len(worker.Labels))
		for k, v := range worker.Labels {
			labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
		}
		labels = strings.Join(labelPairs, ", ")
	}

	// 格式化运行中的任务
	runningJobs := "None"
	if len(worker.RunningJobs) > 0 {
		runningJobs = strings.Join(worker.RunningJobs, ", ")
	}

	data := map[string]string{
		"ID":             worker.ID,
		"Hostname":       worker.Hostname,
		"IP Address":     worker.IP,
		"Status":         GetStatusDisplay(worker.Status),
		"CPU Cores":      fmt.Sprintf("%d", worker.CPUCores),
		"Memory":         fmt.Sprintf("%s / %s (%.1f%%)", models.FormatMemory(worker.MemoryTotal-worker.MemoryFree), models.FormatMemory(worker.MemoryTotal), memoryUsagePercent),
		"Disk":           fmt.Sprintf("%s / %s (%.1f%%)", models.FormatDisk(worker.DiskTotal-worker.DiskFree), models.FormatDisk(worker.DiskTotal), diskUsagePercent),
		"Load Average":   fmt.Sprintf("%.2f", worker.LoadAvg),
		"Running Jobs":   runningJobs,
		"Labels":         labels,
		"Last Heartbeat": models.FormatTimeAgo(worker.LastHeartbeat),
		"Created":        worker.CreateTime.Format(time.RFC3339),
	}

	// 显示Worker详情表格
	w.Table.DisplayKeyValueTable(data)

	return nil
}

// ListActiveWorkers 显示活跃的Worker列表
func (w *WorkerCommand) ListActiveWorkers() error {
	// 显示加载动画
	w.Spinner.Start("Fetching active workers...")

	// 获取活跃Worker列表
	workers, err := w.Client.GetActiveWorkers()
	w.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get active workers: %w", err)
	}

	if len(workers) == 0 {
		w.DisplayInfo("No active workers found")
		return nil
	}

	// 准备表格数据
	headers := []string{"ID", "Hostname", "IP", "Status", "CPU Cores", "Memory Usage", "Running Jobs"}
	data := make([][]string, len(workers))

	for i, worker := range workers {
		// 计算内存使用率
		memoryUsage := "N/A"
		if worker.MemoryTotal > 0 {
			memoryUsed := worker.MemoryTotal - worker.MemoryFree
			memoryUsagePercent := float64(memoryUsed) / float64(worker.MemoryTotal) * 100
			memoryUsage = fmt.Sprintf("%.1f%%", memoryUsagePercent)
		}

		// 填充表格行
		data[i] = []string{
			worker.ID,
			worker.Hostname,
			worker.IP,
			worker.Status,
			fmt.Sprintf("%d", worker.CPUCores),
			memoryUsage,
			fmt.Sprintf("%d", len(worker.RunningJobs)),
		}
	}

	// 显示表格，状态列添加颜色和emoji
	w.Table.DisplayStatusTable(headers, data, 3) // 状态列是第4列（索引3）

	return nil
}

// EnableDisableWorker 启用或禁用Worker
func (w *WorkerCommand) EnableDisableWorker() error {
	// 选择要操作的Worker
	workerID, err := w.selectWorker("Select worker to enable/disable")
	if err != nil {
		return err
	}

	// 获取Worker详情
	w.Spinner.Start("Fetching worker details...")
	worker, err := w.Client.GetWorker(workerID)
	w.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get worker details: %w", err)
	}

	// 显示当前状态
	currentStatus := "enabled"
	if worker.Status == "disabled" {
		currentStatus = "disabled"
	}
	w.DisplayInfo(fmt.Sprintf("Worker '%s' is currently %s", worker.Hostname, currentStatus))

	// 询问新状态
	action := "disable"
	if worker.Status == "disabled" {
		action = "enable"
	}

	confirmed, err := w.Prompt.Confirm(fmt.Sprintf("Do you want to %s this worker?", action))
	if err != nil {
		return err
	}

	if !confirmed {
		w.DisplayInfo("Operation cancelled")
		return nil
	}

	// 显示加载动画
	w.Spinner.Start(fmt.Sprintf("%sing worker...", strings.Title(action)))

	// 发送请求
	var actionErr error
	if worker.Status == "disabled" {
		actionErr = w.Client.EnableWorker(workerID)
	} else {
		actionErr = w.Client.DisableWorker(workerID)
	}
	w.Spinner.Stop()

	if actionErr != nil {
		return fmt.Errorf("failed to %s worker: %w", action, actionErr)
	}

	w.DisplaySuccess(fmt.Sprintf("Worker successfully %sd", action))
	return nil
}

// CheckWorkerHealth 检查Worker健康状态
func (w *WorkerCommand) CheckWorkerHealth() error {
	// 选择要检查的Worker
	workerID, err := w.selectWorker("Select worker to check health")
	if err != nil {
		return err
	}

	// 显示加载动画
	w.Spinner.Start("Checking worker health...")

	// 检查Worker健康状态
	healthy, err := w.Client.CheckWorkerHealth(workerID)
	w.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to check worker health: %w", err)
	}

	// 显示健康状态
	if healthy {
		w.DisplaySuccess("Worker is healthy and responding normally")
	} else {
		w.DisplayError("Worker is not healthy - it may be offline or experiencing issues")
	}

	return nil
}

// selectWorker 辅助函数，用于选择Worker
func (w *WorkerCommand) selectWorker(prompt string) (string, error) {
	// 显示加载动画
	w.Spinner.Start("Fetching workers list...")

	// 获取Worker列表
	workers, _, err := w.Client.ListWorkers(1, 100) // 简化版获取前100个Worker
	w.Spinner.Stop()

	if err != nil {
		return "", fmt.Errorf("failed to list workers: %w", err)
	}

	if len(workers) == 0 {
		return "", fmt.Errorf("no workers found")
	}

	// 准备选项列表
	options := make([]string, len(workers))
	for i, worker := range workers {
		options[i] = fmt.Sprintf("%s (%s - %s)", worker.Hostname, worker.IP, worker.ID)
	}

	// 选择Worker
	selected, err := w.Prompt.Select(prompt, options)
	if err != nil {
		return "", err
	}

	// 提取Worker ID
	idStart := strings.LastIndex(selected, "(") + 1
	idEnd := strings.LastIndex(selected, ")")
	if idStart > 0 && idEnd > idStart {
		parts := strings.Split(selected[idStart:idEnd], " - ")
		if len(parts) == 2 {
			return parts[1], nil
		}
	}

	return "", fmt.Errorf("failed to extract worker ID from selection")
}
