package interaction

import (
	"fmt"
	"github.com/fyerfyer/fyer-scheduler/pkg/cli"
	"os"
	"strings"

	"github.com/fyerfyer/fyer-scheduler/pkg/cli/commands"
	"github.com/fyerfyer/fyer-scheduler/pkg/cli/ui"
)

// InteractiveManager 管理CLI的交互式模式
type InteractiveManager struct {
	client       *cli.Client
	promptUI     *ui.PromptManager
	tableUI      *ui.TableManager
	spinnerUI    *ui.SpinnerManager
	cmdManager   *commands.CommandManager
	jobCmd       *commands.JobCommand
	workerCmd    *commands.WorkerCommand
	systemCmd    *commands.SystemCommand
	isConfigured bool
}

// NewInteractiveManager 创建一个新的交互式管理器
func NewInteractiveManager(client *cli.Client) *InteractiveManager {
	promptUI := ui.NewPromptManager()
	tableUI := ui.NewTableManager()
	spinnerUI := ui.NewSpinnerManager()

	cmdManager := commands.NewCommandManager(client)

	return &InteractiveManager{
		client:       client,
		promptUI:     promptUI,
		tableUI:      tableUI,
		spinnerUI:    spinnerUI,
		cmdManager:   cmdManager,
		jobCmd:       commands.NewJobCommand(cmdManager),
		workerCmd:    commands.NewWorkerCommand(cmdManager),
		systemCmd:    commands.NewSystemCommand(cmdManager),
		isConfigured: client != nil && client.APIKey != "",
	}
}

// Start 启动交互式模式
func (m *InteractiveManager) Start() error {
	fmt.Println("\n🔥 Welcome to Fyer Scheduler CLI 🔥")
	fmt.Println("Interactive mode activated. Type 'help' for available commands, 'exit' to quit.\n")

	// 检查配置
	if !m.isConfigured {
		err := m.setupInitialConfig()
		if err != nil {
			return fmt.Errorf("failed to setup initial configuration: %w", err)
		}
	}

	// 显示系统信息
	err := m.showSystemInfo()
	if err != nil {
		fmt.Printf("Warning: Unable to fetch system information: %v\n", err)
	}

	// 进入主交互循环
	return m.mainLoop()
}

// setupInitialConfig 设置初始配置
func (m *InteractiveManager) setupInitialConfig() error {
	fmt.Println("No configuration found. Let's set up your connection to Fyer Scheduler.")

	// 获取API URL
	apiURL, err := m.promptUI.Input("API Base URL", "http://localhost:8080", ui.ValidateRequired)
	if err != nil {
		return err
	}

	// 获取API密钥
	apiKey, err := m.promptUI.Input("API Key (leave empty if not required)", "", nil)
	if err != nil {
		return err
	}

	// 创建和保存配置
	config := cli.NewConfig()
	config.SetAPIBaseURL(apiURL)
	config.SetAPIKey(apiKey)

	err = config.Save()
	if err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// 创建客户端
	m.client = cli.NewClient(config)
	m.cmdManager = commands.NewCommandManager(m.client)
	m.jobCmd = commands.NewJobCommand(m.cmdManager)
	m.workerCmd = commands.NewWorkerCommand(m.cmdManager)
	m.systemCmd = commands.NewSystemCommand(m.cmdManager)
	m.isConfigured = true

	fmt.Printf("Configuration saved to %s\n", config.GetConfigPath())
	return nil
}

// showSystemInfo 显示系统信息
func (m *InteractiveManager) showSystemInfo() error {
	m.spinnerUI.Start("Connecting to Fyer Scheduler server...")

	// 检查服务器连接
	healthy, err := m.client.CheckHealth()
	if err != nil {
		m.spinnerUI.ErrorStop("Failed to connect to server")
		return err
	}

	if !healthy {
		m.spinnerUI.ErrorStop("Server is not healthy")
		return fmt.Errorf("server health check failed")
	}

	// 获取系统状态
	status, err := m.client.GetSystemStatus()
	if err != nil {
		m.spinnerUI.ErrorStop("Connected to server, but failed to get system status")
		return err
	}

	m.spinnerUI.SuccessStop("Connected to Fyer Scheduler server")

	// 显示系统信息
	fmt.Printf("\n=== System Information ===\n")
	fmt.Printf("Version: %s\n", status.Version)
	fmt.Printf("Jobs: %d total, %d running\n", status.TotalJobs, status.RunningJobs)
	fmt.Printf("Workers: %d active\n", status.ActiveWorkers)
	fmt.Printf("Success Rate: %.1f%%\n", status.SuccessRate)
	fmt.Printf("Average Execution Time: %.2f seconds\n", status.AvgExecutionTime)
	fmt.Printf("==========================\n\n")

	return nil
}

// mainLoop 主交互循环
func (m *InteractiveManager) mainLoop() error {
	for {
		// 显示主菜单
		options := []string{
			"Job Management",
			"Worker Management",
			"System Management",
			"Configuration",
			"Help",
			"Exit",
		}

		selected, err := m.promptUI.Select("Select Option", options)
		if err != nil {
			if ui.IsInterruptError(err) {
				fmt.Println("\nGoodbye! 👋")
				return nil
			}
			return fmt.Errorf("failed to select option: %w", err)
		}

		// 根据选择执行相应的命令
		switch selected {
		case "Job Management":
			err = m.jobCmd.HandleJobCommands()
		case "Worker Management":
			err = m.workerCmd.HandleWorkerCommands()
		case "System Management":
			err = m.systemCmd.HandleSystemCommands()
		case "Configuration":
			err = m.handleConfigCommands()
		case "Help":
			m.showHelp()
		case "Exit":
			fmt.Println("\nGoodbye! 👋")
			return nil
		}

		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}

// handleConfigCommands 处理配置相关的命令
func (m *InteractiveManager) handleConfigCommands() error {
	options := []string{
		"View Current Configuration",
		"Update API URL",
		"Update API Key",
		"Reset Configuration",
		"Back to Main Menu",
	}

	for {
		selected, err := m.promptUI.Select("Configuration Options", options)
		if err != nil {
			if ui.IsInterruptError(err) {
				return nil
			}
			return fmt.Errorf("failed to select option: %w", err)
		}

		switch selected {
		case "View Current Configuration":
			m.viewCurrentConfig()
		case "Update API URL":
			err = m.updateAPIURL()
		case "Update API Key":
			err = m.updateAPIKey()
		case "Reset Configuration":
			err = m.resetConfig()
		case "Back to Main Menu":
			return nil
		}

		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}

		fmt.Println("\nPress Enter to continue...")
		fmt.Scanln()
	}
}

// viewCurrentConfig 查看当前配置
func (m *InteractiveManager) viewCurrentConfig() {
	// 显示配置表格
	configData := map[string]string{
		"API Base URL": m.client.BaseURL,
		"API Key":      maskAPIKey(m.client.APIKey),
	}

	fmt.Println("\n=== Current Configuration ===")
	m.tableUI.DisplayKeyValueTable(configData)
}

// maskAPIKey 遮盖API密钥
func maskAPIKey(key string) string {
	if key == "" {
		return "(not set)"
	}

	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}

	// 只显示前4位和后4位，中间用星号代替
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

// updateAPIURL 更新API URL
func (m *InteractiveManager) updateAPIURL() error {
	// 显示当前URL
	fmt.Printf("Current API URL: %s\n", m.client.BaseURL)

	// 获取新URL
	newURL, err := m.promptUI.Input("New API URL", m.client.BaseURL, ui.ValidateRequired)
	if err != nil {
		return err
	}

	// 确认更改
	confirm, err := m.promptUI.Confirm("Are you sure you want to update the API URL?")
	if err != nil {
		return err
	}

	if !confirm {
		fmt.Println("Update cancelled")
		return nil
	}

	// 更新配置
	config := cli.NewConfig()
	config.SetAPIBaseURL(newURL)
	config.SetAPIKey(m.client.APIKey)

	err = config.Save()
	if err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// 更新客户端
	m.client.BaseURL = newURL

	fmt.Println("API URL updated successfully")
	return nil
}

// updateAPIKey 更新API密钥
func (m *InteractiveManager) updateAPIKey() error {
	// 显示当前密钥（遮盖）
	fmt.Printf("Current API Key: %s\n", maskAPIKey(m.client.APIKey))

	// 获取新密钥
	newKey, err := m.promptUI.Password("New API Key (leave empty to remove)")
	if err != nil {
		return err
	}

	// 确认更改
	confirm, err := m.promptUI.Confirm("Are you sure you want to update the API Key?")
	if err != nil {
		return err
	}

	if !confirm {
		fmt.Println("Update cancelled")
		return nil
	}

	// 更新配置
	config := cli.NewConfig()
	config.SetAPIBaseURL(m.client.BaseURL)
	config.SetAPIKey(newKey)

	err = config.Save()
	if err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// 更新客户端
	m.client.APIKey = newKey

	fmt.Println("API Key updated successfully")
	return nil
}

// resetConfig 重置配置
func (m *InteractiveManager) resetConfig() error {
	// 确认重置
	confirm, err := m.promptUI.Confirm("Are you sure you want to reset all configuration? This cannot be undone.")
	if err != nil {
		return err
	}

	if !confirm {
		fmt.Println("Reset cancelled")
		return nil
	}

	// 创建默认配置
	config := cli.NewConfig()

	// 删除现有配置文件
	configPath := config.GetConfigPath()
	_, err = os.Stat(configPath)
	if err == nil {
		err = os.Remove(configPath)
		if err != nil {
			return fmt.Errorf("failed to delete configuration file: %w", err)
		}
	}

	// 重置为默认值
	m.client = cli.NewClient(config)
	m.cmdManager = commands.NewCommandManager(m.client)
	m.jobCmd = commands.NewJobCommand(m.cmdManager)
	m.workerCmd = commands.NewWorkerCommand(m.cmdManager)
	m.systemCmd = commands.NewSystemCommand(m.cmdManager)
	m.isConfigured = false

	fmt.Println("Configuration reset to defaults")

	// 重新设置配置
	return m.setupInitialConfig()
}

// showHelp 显示帮助信息
func (m *InteractiveManager) showHelp() {
	fmt.Println("\n=== Fyer Scheduler CLI Help ===")
	fmt.Println("This CLI tool allows you to interact with the Fyer Scheduler system.")
	fmt.Println("You can manage jobs, workers, and system settings.")
	fmt.Println()
	fmt.Println("Main Menu Options:")
	fmt.Println("  • Job Management - Create, list, edit, and control jobs")
	fmt.Println("  • Worker Management - Monitor and manage worker nodes")
	fmt.Println("  • System Management - View system status and statistics")
	fmt.Println("  • Configuration - Update connection settings")
	fmt.Println("  • Help - Show this help information")
	fmt.Println("  • Exit - Exit the application")
	fmt.Println()
	fmt.Println("Navigation Tips:")
	fmt.Println("  • Use arrow keys to navigate menus")
	fmt.Println("  • Press Enter to select an option")
	fmt.Println("  • Press Ctrl+C to cancel or go back")
	fmt.Println("===============================")
}
