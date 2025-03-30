package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/cli"
	"github.com/fyerfyer/fyer-scheduler/pkg/cli/models"
)

// JobCommand 处理任务相关命令
type JobCommand struct {
	*CommandManager
}

// NewJobCommand 创建一个新的任务命令处理器
func NewJobCommand(cmdManager *CommandManager) *JobCommand {
	return &JobCommand{
		CommandManager: cmdManager,
	}
}

// HandleJobCommands 处理任务相关的交互式命令
func (j *JobCommand) HandleJobCommands() error {
	for {
		// 显示任务命令菜单
		options := []string{
			"List Jobs",
			"View Job Details",
			"Create Job",
			"Edit Job",
			"Delete Job",
			"Enable/Disable Job",
			"Run Job Now",
			"Kill Job",
			"View Job Logs",
			"Back to Main Menu",
		}

		selected, err := j.Prompt.Select("Select Job Command", options)
		if err != nil {
			return fmt.Errorf("failed to select command: %w", err)
		}

		// 根据选择执行相应的命令
		switch selected {
		case "List Jobs":
			err = j.ListJobs()
		case "View Job Details":
			err = j.ViewJobDetails()
		case "Create Job":
			err = j.CreateJob()
		case "Edit Job":
			err = j.EditJob()
		case "Delete Job":
			err = j.DeleteJob()
		case "Enable/Disable Job":
			err = j.EnableDisableJob()
		case "Run Job Now":
			err = j.RunJob()
		case "Kill Job":
			err = j.KillJob()
		case "View Job Logs":
			err = j.ViewJobLogs()
		case "Back to Main Menu":
			return nil
		}

		if err != nil {
			j.DisplayError(fmt.Sprintf("Command failed: %v", err))
		}

		// 等待用户按键继续
		fmt.Println("\nPress Enter to continue...")
		fmt.Scanln()
	}
}

// ListJobs 显示任务列表
func (j *JobCommand) ListJobs() error {
	// 询问分页参数
	params, err := j.AskForPagination()
	if err != nil {
		return err
	}

	// 显示加载动画
	j.Spinner.Start("Fetching jobs list...")

	// 获取任务列表
	jobs, total, err := j.Client.ListJobs(params.Page, params.PageSize)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	if len(jobs) == 0 {
		j.DisplayInfo("No jobs found")
		return nil
	}

	// 准备表格数据
	headers := []string{"ID", "Name", "Status", "Schedule", "Last Run", "Next Run"}
	data := make([][]string, len(jobs))

	for i, job := range jobs {
		// 格式化下次运行时间
		nextRun := "Not scheduled"
		if job.NextRunTime != nil && !job.NextRunTime.IsZero() {
			nextRun = models.FormatTimeAgo(*job.NextRunTime)
		}

		// 格式化上次运行时间
		lastRun := "Never"
		if job.LastRunTime != nil && !job.LastRunTime.IsZero() {
			lastRun = models.FormatTimeAgo(*job.LastRunTime)
		}

		// 填充表格行
		data[i] = []string{
			job.ID,
			job.Name,
			job.Status,
			job.CronExpr,
			lastRun,
			nextRun,
		}
	}

	// 显示表格，状态列添加颜色和emoji
	j.Table.DisplayStatusTable(headers, data, 2)

	// 显示分页信息
	footer := GetPaginationFooter(params.Page, params.PageSize, total)
	fmt.Printf("\n%s\n", strings.Join(footer, " | "))

	return nil
}

// ViewJobDetails 查看任务详情
func (j *JobCommand) ViewJobDetails() error {
	// 先列出任务供用户选择
	jobID, err := j.selectJob("Select job to view")
	if err != nil {
		return err
	}

	// 显示加载动画
	j.Spinner.Start("Fetching job details...")

	// 获取任务详情
	job, err := j.Client.GetJob(jobID)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get job details: %w", err)
	}

	// 格式化任务详情为键值对
	data := map[string]string{
		"ID":           job.ID,
		"Name":         job.Name,
		"Description":  job.Description,
		"Command":      job.Command,
		"Arguments":    ArgsToString(job.Args),
		"Working Dir":  job.WorkDir,
		"Environment":  EnvMapToString(job.Env),
		"Schedule":     job.CronExpr,
		"Timeout":      fmt.Sprintf("%d seconds", job.Timeout),
		"Max Retry":    fmt.Sprintf("%d", job.MaxRetry),
		"Retry Delay":  fmt.Sprintf("%d seconds", job.RetryDelay),
		"Status":       GetStatusDisplay(job.Status),
		"Enabled":      fmt.Sprintf("%t", job.Enabled),
		"Last Run":     FormatTime(job.LastRunTime),
		"Next Run":     FormatTime(job.NextRunTime),
		"Last Result":  job.LastResult,
		"Last Exit":    fmt.Sprintf("%d", job.LastExitCode),
		"Created":      job.CreateTime.Format(time.RFC3339),
		"Last Updated": job.UpdateTime.Format(time.RFC3339),
	}

	// 显示任务详情表格
	j.Table.DisplayKeyValueTable(data)

	return nil
}

// CreateJob 创建新任务
func (j *JobCommand) CreateJob() error {
	// 显示提示
	j.DisplayInfo("Creating a new job. Please provide the following information:")

	// 获取任务名称
	name, err := j.Prompt.Input("Job Name", "", ValidateJobName)
	if err != nil {
		return err
	}

	// 获取任务描述
	description, err := j.Prompt.Input("Description (optional)", "", NoValidation)
	if err != nil {
		return err
	}

	// 获取命令
	command, err := j.Prompt.Input("Command", "", ValidateCommand)
	if err != nil {
		return err
	}

	// 获取参数
	argsStr, err := j.Prompt.Input("Arguments (space separated, optional)", "", NoValidation)
	if err != nil {
		return err
	}
	var args []string
	if argsStr != "" {
		args = strings.Fields(argsStr)
	}

	// 获取Cron表达式
	cronExpr, err := j.Prompt.Input("Cron Expression (optional)", "", ValidateCronExpression)
	if err != nil {
		return err
	}

	// 获取工作目录
	workDir, err := j.Prompt.Input("Working Directory (optional)", "", NoValidation)
	if err != nil {
		return err
	}

	// 获取超时设置
	timeoutStr, err := j.Prompt.Input("Timeout in seconds (default: 3600)", "3600", ValidateTimeout)
	if err != nil {
		return err
	}
	timeout, _ := strconv.Atoi(timeoutStr)

	// 获取最大重试次数
	maxRetryStr, err := j.Prompt.Input("Max Retry Count (default: 0)", "0", ValidateRetry)
	if err != nil {
		return err
	}
	maxRetry, _ := strconv.Atoi(maxRetryStr)

	// 获取重试延迟
	retryDelayStr, err := j.Prompt.Input("Retry Delay in seconds (default: 60)", "60", ValidateTimeout)
	if err != nil {
		return err
	}
	retryDelay, _ := strconv.Atoi(retryDelayStr)

	// 获取启用状态
	enabledStr, err := j.Prompt.Input("Enable Job? (y/n, default: y)", "y", ValidateYesNo)
	if err != nil {
		return err
	}
	enabled := strings.ToLower(enabledStr) == "y" || strings.ToLower(enabledStr) == "yes"

	// 询问确认
	confirmed, err := j.Prompt.Confirm("Create this job?")
	if err != nil {
		return err
	}

	if !confirmed {
		j.DisplayInfo("Job creation cancelled")
		return nil
	}

	// 准备创建任务请求
	req := &cli.CreateJobRequest{
		Name:        name,
		Description: description,
		Command:     command,
		Args:        args,
		CronExpr:    cronExpr,
		WorkDir:     workDir,
		Timeout:     timeout,
		MaxRetry:    maxRetry,
		RetryDelay:  retryDelay,
		Enabled:     enabled,
		Env:         make(map[string]string), // 简化版不添加环境变量
	}

	// 显示加载动画
	j.Spinner.Start("Creating job...")

	// 发送创建请求
	job, err := j.Client.CreateJob(req)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	j.DisplaySuccess(fmt.Sprintf("Job created successfully with ID: %s", job.ID))
	return nil
}

// EditJob 编辑现有任务
func (j *JobCommand) EditJob() error {
	// 选择要编辑的任务
	jobID, err := j.selectJob("Select job to edit")
	if err != nil {
		return err
	}

	// 获取任务详情
	j.Spinner.Start("Fetching job details...")
	job, err := j.Client.GetJob(jobID)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get job details: %w", err)
	}

	// 显示当前信息并获取更新后的信息
	j.DisplayInfo(fmt.Sprintf("Editing job: %s (%s)", job.Name, job.ID))
	j.DisplayInfo("Press Enter to keep current values")

	// 获取任务名称
	name, err := j.Prompt.Input("Job Name", job.Name, ValidateJobName)
	if err != nil {
		return err
	}

	// 获取任务描述
	description, err := j.Prompt.Input("Description", job.Description, NoValidation)
	if err != nil {
		return err
	}

	// 获取命令
	command, err := j.Prompt.Input("Command", job.Command, ValidateCommand)
	if err != nil {
		return err
	}

	// 获取参数
	argsStr, err := j.Prompt.Input("Arguments (space separated)", strings.Join(job.Args, " "), NoValidation)
	if err != nil {
		return err
	}
	var args []string
	if argsStr != "" {
		args = strings.Fields(argsStr)
	}

	// 获取Cron表达式
	cronExpr, err := j.Prompt.Input("Cron Expression", job.CronExpr, ValidateCronExpression)
	if err != nil {
		return err
	}

	// 获取工作目录
	workDir, err := j.Prompt.Input("Working Directory", job.WorkDir, NoValidation)
	if err != nil {
		return err
	}

	// 获取超时设置
	timeoutStr, err := j.Prompt.Input("Timeout in seconds", strconv.Itoa(job.Timeout), ValidateTimeout)
	if err != nil {
		return err
	}
	timeout, _ := strconv.Atoi(timeoutStr)

	// 获取最大重试次数
	maxRetryStr, err := j.Prompt.Input("Max Retry Count", strconv.Itoa(job.MaxRetry), ValidateRetry)
	if err != nil {
		return err
	}
	maxRetry, _ := strconv.Atoi(maxRetryStr)

	// 获取重试延迟
	retryDelayStr, err := j.Prompt.Input("Retry Delay in seconds", strconv.Itoa(job.RetryDelay), ValidateTimeout)
	if err != nil {
		return err
	}
	retryDelay, _ := strconv.Atoi(retryDelayStr)

	// 获取启用状态
	enabledDefault := "y"
	if !job.Enabled {
		enabledDefault = "n"
	}
	enabledStr, err := j.Prompt.Input("Enable Job? (y/n)", enabledDefault, ValidateYesNo)
	if err != nil {
		return err
	}
	enabled := strings.ToLower(enabledStr) == "y" || strings.ToLower(enabledStr) == "yes"

	// 询问确认
	confirmed, err := j.Prompt.Confirm("Save changes to this job?")
	if err != nil {
		return err
	}

	if !confirmed {
		j.DisplayInfo("Job update cancelled")
		return nil
	}

	// 准备更新任务请求
	req := &cli.CreateJobRequest{
		Name:        name,
		Description: description,
		Command:     command,
		Args:        args,
		CronExpr:    cronExpr,
		WorkDir:     workDir,
		Timeout:     timeout,
		MaxRetry:    maxRetry,
		RetryDelay:  retryDelay,
		Enabled:     enabled,
		Env:         job.Env, // 保留原有环境变量
	}

	// 显示加载动画
	j.Spinner.Start("Updating job...")

	// 发送更新请求
	_, err = j.Client.UpdateJob(jobID, req)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	j.DisplaySuccess("Job updated successfully")
	return nil
}

// DeleteJob 删除任务
func (j *JobCommand) DeleteJob() error {
	// 选择要删除的任务
	jobID, err := j.selectJob("Select job to delete")
	if err != nil {
		return err
	}

	// 获取任务详情以显示确认信息
	j.Spinner.Start("Fetching job details...")
	job, err := j.Client.GetJob(jobID)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get job details: %w", err)
	}

	// 显示警告并要求确认
	j.DisplayWarning(fmt.Sprintf("You are about to delete job: %s (%s)", job.Name, job.ID))
	confirmed, err := j.Prompt.Confirm("Are you sure you want to delete this job? This action cannot be undone.")
	if err != nil {
		return err
	}

	if !confirmed {
		j.DisplayInfo("Job deletion cancelled")
		return nil
	}

	// 显示加载动画
	j.Spinner.Start("Deleting job...")

	// 发送删除请求
	err = j.Client.DeleteJob(jobID)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	j.DisplaySuccess("Job deleted successfully")
	return nil
}

// EnableDisableJob 启用或禁用任务
func (j *JobCommand) EnableDisableJob() error {
	// 选择要操作的任务
	jobID, err := j.selectJob("Select job to enable/disable")
	if err != nil {
		return err
	}

	// 获取任务详情
	j.Spinner.Start("Fetching job details...")
	job, err := j.Client.GetJob(jobID)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get job details: %w", err)
	}

	// 显示当前状态
	currentStatus := "enabled"
	if !job.Enabled {
		currentStatus = "disabled"
	}
	j.DisplayInfo(fmt.Sprintf("Job '%s' is currently %s", job.Name, currentStatus))

	// 询问新状态
	action := "disable"
	if !job.Enabled {
		action = "enable"
	}

	confirmed, err := j.Prompt.Confirm(fmt.Sprintf("Do you want to %s this job?", action))
	if err != nil {
		return err
	}

	if !confirmed {
		j.DisplayInfo("Operation cancelled")
		return nil
	}

	// 显示加载动画
	j.Spinner.Start(fmt.Sprintf("%sing job...", strings.Title(action)))

	// 发送请求
	var actionErr error
	if job.Enabled {
		actionErr = j.Client.DisableJob(jobID)
	} else {
		actionErr = j.Client.EnableJob(jobID)
	}
	j.Spinner.Stop()

	if actionErr != nil {
		return fmt.Errorf("failed to %s job: %w", action, actionErr)
	}

	j.DisplaySuccess(fmt.Sprintf("Job successfully %sd", action))
	return nil
}

// RunJob 立即运行任务
func (j *JobCommand) RunJob() error {
	// 选择要运行的任务
	jobID, err := j.selectJob("Select job to run")
	if err != nil {
		return err
	}

	// 获取任务详情
	j.Spinner.Start("Fetching job details...")
	job, err := j.Client.GetJob(jobID)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get job details: %w", err)
	}

	// 显示任务信息
	j.DisplayInfo(fmt.Sprintf("You are about to run job: %s (%s)", job.Name, job.ID))
	j.DisplayInfo(fmt.Sprintf("Command: %s %s", job.Command, strings.Join(job.Args, " ")))

	// 询问确认
	confirmed, err := j.Prompt.Confirm("Do you want to run this job now?")
	if err != nil {
		return err
	}

	if !confirmed {
		j.DisplayInfo("Operation cancelled")
		return nil
	}

	// 显示加载动画
	j.Spinner.Start("Triggering job execution...")

	// 发送请求
	executionID, err := j.Client.TriggerJob(jobID)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to trigger job: %w", err)
	}

	j.DisplaySuccess(fmt.Sprintf("Job triggered successfully with execution ID: %s", executionID))

	// 询问是否查看日志
	viewLogs, err := j.Prompt.Confirm("Do you want to view the execution logs?")
	if err != nil || !viewLogs {
		return nil
	}

	// 等待任务开始执行
	j.Spinner.Start("Waiting for execution to start...")
	time.Sleep(2 * time.Second) // 简单等待，实际中应该轮询检查状态
	j.Spinner.Stop()

	// 获取执行日志
	return j.viewExecutionLogs(executionID)
}

// KillJob 终止正在运行的任务
func (j *JobCommand) KillJob() error {
	// 先列出任务供用户选择
	jobID, err := j.selectJob("Select job to kill")
	if err != nil {
		return err
	}

	// 获取任务详情
	j.Spinner.Start("Fetching job details...")
	job, err := j.Client.GetJob(jobID)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get job details: %w", err)
	}

	// 检查任务是否正在运行
	if job.Status != "running" {
		j.DisplayWarning(fmt.Sprintf("Job '%s' is not running (current status: %s)", job.Name, job.Status))
		return nil
	}

	// 显示警告并要求确认
	j.DisplayWarning(fmt.Sprintf("You are about to kill running job: %s (%s)", job.Name, job.ID))
	confirmed, err := j.Prompt.Confirm("Are you sure you want to kill this job?")
	if err != nil {
		return err
	}

	if !confirmed {
		j.DisplayInfo("Operation cancelled")
		return nil
	}

	// 显示加载动画
	j.Spinner.Start("Killing job...")

	// 发送终止请求
	err = j.Client.KillJob(jobID)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to kill job: %w", err)
	}

	j.DisplaySuccess("Job kill signal sent successfully")
	return nil
}

// ViewJobLogs 查看任务日志
func (j *JobCommand) ViewJobLogs() error {
	// 选择要查看日志的任务
	jobID, err := j.selectJob("Select job to view logs")
	if err != nil {
		return err
	}

	// 询问分页参数
	params, err := j.AskForPagination()
	if err != nil {
		return err
	}

	// 显示加载动画
	j.Spinner.Start("Fetching job logs...")

	// 获取任务日志
	logs, total, err := j.Client.GetJobLogs(jobID, params.Page, params.PageSize)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get job logs: %w", err)
	}

	if len(logs) == 0 {
		j.DisplayInfo("No logs found for this job")
		return nil
	}

	// 准备表格数据
	headers := []string{"Execution ID", "Status", "Start Time", "Duration", "Exit Code"}
	data := make([][]string, len(logs))

	for i, log := range logs {
		// 计算持续时间
		duration := "N/A"
		if !log.StartTime.IsZero() && !log.EndTime.IsZero() {
			duration = models.FormatDuration(log.Duration)
		}

		// 填充表格行
		data[i] = []string{
			log.ExecutionID,
			log.Status,
			log.StartTime.Format(time.RFC3339),
			duration,
			strconv.Itoa(log.ExitCode),
		}
	}

	// 显示表格，状态列添加颜色
	j.Table.DisplayStatusTable(headers, data, 1)

	// 显示分页信息
	footer := GetPaginationFooter(params.Page, params.PageSize, total)
	fmt.Printf("\n%s\n", strings.Join(footer, " | "))

	// 询问是否查看特定执行的详细日志
	viewDetail, err := j.Prompt.Confirm("Do you want to view detailed logs for a specific execution?")
	if err != nil || !viewDetail {
		return nil
	}

	// 选择要查看的执行ID
	options := make([]string, len(logs))
	for i, log := range logs {
		options[i] = fmt.Sprintf("%s (%s, %s)", log.ExecutionID, log.Status, log.StartTime.Format(time.RFC3339))
	}

	selected, err := j.Prompt.Select("Select execution", options)
	if err != nil {
		return err
	}

	// 提取执行ID
	executionID := strings.Split(selected, " ")[0]
	return j.viewExecutionLogs(executionID)
}

// viewExecutionLogs 查看执行日志详情
func (j *JobCommand) viewExecutionLogs(executionID string) error {
	// 显示加载动画
	j.Spinner.Start("Fetching execution logs...")

	// 获取执行日志
	log, err := j.Client.GetExecutionLog(executionID)
	j.Spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to get execution logs: %w", err)
	}

	// 显示执行详情
	fmt.Println("\n==== Execution Details ====")
	fmt.Printf("Execution ID: %s\n", log.ExecutionID)
	fmt.Printf("Job: %s (%s)\n", log.JobName, log.JobID)
	fmt.Printf("Worker: %s (%s)\n", log.WorkerIP, log.WorkerID)
	fmt.Printf("Command: %s\n", log.Command)
	fmt.Printf("Status: %s\n", GetStatusDisplay(log.Status))
	fmt.Printf("Schedule Time: %s\n", log.ScheduleTime.Format(time.RFC3339))
	fmt.Printf("Start Time: %s\n", log.StartTime.Format(time.RFC3339))
	fmt.Printf("End Time: %s\n", log.EndTime.Format(time.RFC3339))
	fmt.Printf("Duration: %s\n", models.FormatDuration(log.Duration))
	fmt.Printf("Exit Code: %d\n", log.ExitCode)
	if log.IsManual {
		fmt.Println("Trigger: Manual")
	} else {
		fmt.Println("Trigger: Scheduled")
	}

	// 显示错误信息（如果有）
	if log.Error != "" {
		fmt.Println("\n==== Error ====")
		fmt.Println(log.Error)
	}

	// 显示输出
	fmt.Println("\n==== Output ====")
	fmt.Println(log.Output)

	return nil
}

// selectJob 辅助函数，用于选择任务
func (j *JobCommand) selectJob(prompt string) (string, error) {
	// 显示加载动画
	j.Spinner.Start("Fetching jobs list...")

	// 获取任务列表
	jobs, _, err := j.Client.ListJobs(1, 100) // 简化版获取前100个任务
	j.Spinner.Stop()

	if err != nil {
		return "", fmt.Errorf("failed to list jobs: %w", err)
	}

	if len(jobs) == 0 {
		return "", fmt.Errorf("no jobs found")
	}

	// 准备选项列表
	options := make([]string, len(jobs))
	for i, job := range jobs {
		options[i] = fmt.Sprintf("%s (%s)", job.Name, job.ID)
	}

	// 选择任务
	selected, err := j.Prompt.Select(prompt, options)
	if err != nil {
		return "", err
	}

	// 提取任务ID
	idStart := strings.LastIndex(selected, "(") + 1
	idEnd := strings.LastIndex(selected, ")")
	if idStart > 0 && idEnd > idStart {
		return selected[idStart:idEnd], nil
	}

	return "", fmt.Errorf("failed to extract job ID from selection")
}
