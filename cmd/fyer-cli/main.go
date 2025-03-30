package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fyerfyer/fyer-scheduler/pkg/cli"
	"github.com/fyerfyer/fyer-scheduler/pkg/cli/interaction"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	urfavecli "github.com/urfave/cli/v2"
)

// 版本信息，可在构建时通过 -ldflags "-X main.version=x.x.x" 设置
var (
	version = "0.1.0"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// 打印启动信息
	fmt.Printf("Fyer Scheduler CLI %s (commit: %s, built: %s)\n\n", version, commit, date)

	// 初始化配置
	config, err := cli.LoadConfig()
	if err != nil {
		fmt.Printf("Warning: Failed to load configuration: %v\n", err)
		config = cli.NewConfig()
	}

	// 创建API客户端
	apiClient := cli.NewClient(config)

	// 创建交互式管理器
	interactiveManager := interaction.NewInteractiveManager(apiClient)

	// 创建CLI应用
	app := &urfavecli.App{
		Name:    "fyer-cli",
		Usage:   "Command line client for Fyer Scheduler",
		Version: version,
		Authors: []*urfavecli.Author{
			{Name: "FyerFyer Team"},
		},
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{
				Name:    "api-url",
				Aliases: []string{"u"},
				Usage:   "API base URL",
				EnvVars: []string{"FYER_API_URL"},
				Value:   config.APIBaseURL,
			},
			&urfavecli.StringFlag{
				Name:    "api-key",
				Aliases: []string{"k"},
				Usage:   "API key for authentication",
				EnvVars: []string{"FYER_API_KEY"},
				Value:   config.APIKey,
			},
			&urfavecli.BoolFlag{
				Name:    "interactive",
				Aliases: []string{"i"},
				Usage:   "Enable interactive mode",
				Value:   true,
			},
			//&urfavecli.BoolFlag{
			//	Name:    "verbose",
			//	Aliases: []string{"v"},
			//	Usage:   "Enable verbose output",
			//	Value:   false,
			//},
		},
		Before: func(c *urfavecli.Context) error {
			// 更新客户端配置
			if c.String("api-url") != "" && c.String("api-url") != config.APIBaseURL {
				config.SetAPIBaseURL(c.String("api-url"))
				apiClient.BaseURL = c.String("api-url")
			}

			if c.String("api-key") != "" && c.String("api-key") != config.APIKey {
				config.SetAPIKey(c.String("api-key"))
				apiClient.APIKey = c.String("api-key")
			}

			// 设置日志级别
			if c.Bool("verbose") {
				utils.SetLogLevel("debug")
			} else {
				utils.SetLogLevel("info")
			}

			return nil
		},
		Commands: []*urfavecli.Command{
			{
				Name:  "job",
				Usage: "Manage jobs",
				Subcommands: []*urfavecli.Command{
					{
						Name:  "list",
						Usage: "List all jobs",
						Action: func(c *urfavecli.Context) error {
							// 使用默认分页
							jobs, total, err := apiClient.ListJobs(1, 10)
							if err != nil {
								return fmt.Errorf("failed to list jobs: %w", err)
							}

							fmt.Printf("Total jobs: %d\n", total)
							for _, job := range jobs {
								// 格式化下次运行时间
								nextRun := "Not scheduled"
								if job.NextRunTime != nil && !job.NextRunTime.IsZero() {
									nextRun = job.NextRunTime.Format("2006-01-02 15:04:05")
								}

								fmt.Printf("%s - %s (Status: %s, Next Run: %s)\n",
									job.ID, job.Name, job.Status, nextRun)
							}
							return nil
						},
					},
					{
						Name:  "get",
						Usage: "Get job details",
						Flags: []urfavecli.Flag{
							&urfavecli.StringFlag{
								Name:     "id",
								Aliases:  []string{"i"},
								Usage:    "Job ID",
								Required: true,
							},
						},
						Action: func(c *urfavecli.Context) error {
							job, err := apiClient.GetJob(c.String("id"))
							if err != nil {
								return fmt.Errorf("failed to get job: %w", err)
							}

							fmt.Printf("ID: %s\nName: %s\nDescription: %s\nCommand: %s\nStatus: %s\n",
								job.ID, job.Name, job.Description, job.Command, job.Status)
							return nil
						},
					},
				},
			},
			{
				Name:  "worker",
				Usage: "Manage workers",
				Subcommands: []*urfavecli.Command{
					{
						Name:  "list",
						Usage: "List all workers",
						Action: func(c *urfavecli.Context) error {
							workers, total, err := apiClient.ListWorkers(1, 10)
							if err != nil {
								return fmt.Errorf("failed to list workers: %w", err)
							}

							fmt.Printf("Total workers: %d\n", total)
							for _, worker := range workers {
								fmt.Printf("%s - %s (%s, Status: %s)\n",
									worker.ID, worker.Hostname, worker.IP, worker.Status)
							}
							return nil
						},
					},
				},
			},
			{
				Name:  "system",
				Usage: "System management",
				Subcommands: []*urfavecli.Command{
					{
						Name:  "status",
						Usage: "Show system status",
						Action: func(c *urfavecli.Context) error {
							status, err := apiClient.GetSystemStatus()
							if err != nil {
								return fmt.Errorf("failed to get system status: %w", err)
							}

							fmt.Printf("Version: %s\n", status.Version)
							fmt.Printf("Active Workers: %d\n", status.ActiveWorkers)
							fmt.Printf("Jobs: %d total, %d running\n", status.TotalJobs, status.RunningJobs)
							fmt.Printf("Success Rate: %.1f%%\n", status.SuccessRate)
							return nil
						},
					},
					{
						Name:  "health",
						Usage: "Check system health",
						Action: func(c *urfavecli.Context) error {
							healthy, err := apiClient.CheckHealth()
							if err != nil {
								return fmt.Errorf("failed to check health: %w", err)
							}

							if healthy {
								fmt.Println("System is healthy")
							} else {
								fmt.Println("System is NOT healthy!")
							}
							return nil
						},
					},
				},
			},
			{
				Name:  "config",
				Usage: "Manage configuration",
				Subcommands: []*urfavecli.Command{
					{
						Name:  "set",
						Usage: "Set configuration value",
						Flags: []urfavecli.Flag{
							&urfavecli.StringFlag{
								Name:     "key",
								Usage:    "Configuration key (e.g. api-url, api-key)",
								Required: true,
							},
							&urfavecli.StringFlag{
								Name:     "value",
								Usage:    "Configuration value",
								Required: true,
							},
						},
						Action: func(c *urfavecli.Context) error {
							key := c.String("key")
							value := c.String("value")

							switch strings.ToLower(key) {
							case "api-url", "api_url", "apiurl":
								config.SetAPIBaseURL(value)
							case "api-key", "api_key", "apikey":
								config.SetAPIKey(value)
							default:
								return fmt.Errorf("unknown configuration key: %s", key)
							}

							err := config.Save()
							if err != nil {
								return fmt.Errorf("failed to save configuration: %w", err)
							}

							fmt.Printf("Configuration updated: %s = %s\n", key, value)
							return nil
						},
					},
					{
						Name:  "get",
						Usage: "Get configuration value",
						Flags: []urfavecli.Flag{
							&urfavecli.StringFlag{
								Name:     "key",
								Usage:    "Configuration key (e.g. api-url, api-key)",
								Required: true,
							},
						},
						Action: func(c *urfavecli.Context) error {
							key := c.String("key")

							switch strings.ToLower(key) {
							case "api-url", "api_url", "apiurl":
								fmt.Printf("api-url = %s\n", config.APIBaseURL)
							case "api-key", "api_key", "apikey":
								fmt.Printf("api-key = %s\n", config.APIKey)
							default:
								return fmt.Errorf("unknown configuration key: %s", key)
							}

							return nil
						},
					},
				},
			},
		},
		Action: func(c *urfavecli.Context) error {
			// 默认行为: 如果没有指定命令但启用了交互模式，则启动交互式界面
			if c.Bool("interactive") {
				return interactiveManager.Start()
			}

			// 否则显示帮助
			return urfavecli.ShowAppHelp(c)
		},
	}

	// 运行应用
	err = app.Run(os.Args)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
