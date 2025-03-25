package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/config"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker"
	"go.uber.org/zap"
)

var (
	configPath string
	version    = "1.0.0" // 构建时注入
)

func init() {
	// 解析命令行参数
	flag.StringVar(&configPath, "config", "./configs/worker.yaml", "配置文件路径")
	flag.Parse()
}

func main() {
	// 打印启动信息
	fmt.Println("Starting Fyer Scheduler Worker Node...")
	fmt.Println("Version:", version)
	fmt.Println("Config Path:", configPath)

	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 验证节点类型
	if cfg.NodeType != "worker" {
		fmt.Printf("Invalid node type: %s, must be 'worker' for worker node\n", cfg.NodeType)
		os.Exit(1)
	}

	// 设置版本号
	cfg.Version = version

	// 初始化日志
	utils.InitLogger(&cfg.Log)
	defer utils.CloseLogger()

	// 创建日志目录
	if cfg.Log.FilePath != "" {
		logDir := filepath.Dir(cfg.Log.FilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			utils.Error("failed to create log directory", zap.Error(err))
		}
	}

	// 记录启动信息
	utils.Info("worker node starting",
		zap.String("node_id", cfg.NodeID),
		zap.String("version", version),
		zap.String("config_path", configPath))

	// 创建Worker应用
	app, err := worker.NewWorkerApp(cfg)
	if err != nil {
		utils.Error("failed to create worker application", zap.Error(err))
		os.Exit(1)
	}

	// 启动应用
	if err := app.Start(); err != nil {
		utils.Error("failed to start worker application", zap.Error(err))
		os.Exit(1)
	}

	utils.Info("worker node started successfully",
		zap.String("node_id", cfg.NodeID),
		zap.String("master_address", cfg.GetServerAddress()))

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞直到收到信号
	sig := <-sigCh
	utils.Info("received signal, shutting down", zap.String("signal", sig.String()))

	// 优雅关闭
	if err := app.Stop(); err != nil {
		utils.Error("error during shutdown", zap.Error(err))
		os.Exit(1)
	}

	utils.Info("worker node stopped gracefully")
}
