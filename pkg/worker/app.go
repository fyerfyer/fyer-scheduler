package worker

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/config"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/executor"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/joblock"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/jobmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/logsink"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/register"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/scheduler"
	"go.uber.org/zap"
)

// WorkerApp 整合所有 Worker 组件的应用程序结构体
type WorkerApp struct {
	// 配置
	config *config.Config

	// 客户端
	etcdClient *utils.EtcdClient

	// 组件
	workerRegister    *register.WorkerRegister
	healthChecker     *register.HealthChecker
	resourceCollector *register.ResourceCollector
	signalHandler     *register.SignalHandler
	jobManager        jobmgr.IWorkerJobManager
	lockManager       *joblock.LockManager
	jobScheduler      *scheduler.Scheduler
	jobExecutor       *executor.Executor
	logSink           logsink.ILogSink

	// 状态控制
	isRunning bool
	stopChan  chan struct{}
	lock      sync.Mutex
}

// NewWorkerApp 创建一个新的 WorkerApp 实例
func NewWorkerApp(config *config.Config) (*WorkerApp, error) {
	app := &WorkerApp{
		config:   config,
		stopChan: make(chan struct{}),
	}

	// 初始化各组件
	if err := app.initClients(); err != nil {
		return nil, err
	}

	if err := app.initComponents(); err != nil {
		app.etcdClient.Close()
		return nil, err
	}

	return app, nil
}

// initClients 初始化客户端连接
func (app *WorkerApp) initClients() error {
	// 初始化 etcd 客户端
	var err error
	app.etcdClient, err = utils.NewEtcdClient(&app.config.Etcd)
	if err != nil {
		return fmt.Errorf("failed to initialize etcd client: %w", err)
	}

	utils.Info("etcd client initialized successfully")
	return nil
}

// initComponents 初始化 Worker 组件
func (app *WorkerApp) initComponents() error {
	var err error

	// 初始化资源收集器
	app.resourceCollector = register.NewResourceCollector(60 * time.Second)

	// 初始化 Worker 注册器
	registerOptions := register.RegisterOptions{
		NodeID:           app.config.NodeID,
		HeartbeatTTL:     10,
		HeartbeatTimeout: 30,
		ResourceInterval: 60 * time.Second,
	}

	app.workerRegister, err = register.NewWorkerRegister(app.etcdClient, registerOptions)
	if err != nil {
		return fmt.Errorf("failed to create worker register: %w", err)
	}

	// 初始化健康检查器
	app.healthChecker = register.NewHealthChecker(
		app.workerRegister,
		app.resourceCollector,
		5*time.Second, // 心跳间隔
	)

	// 初始化信号处理器
	app.signalHandler = register.NewSignalHandler(
		app.workerRegister,
		app.healthChecker,
		app.resourceCollector,
	)

	// 初始化任务管理器
	app.jobManager = jobmgr.NewWorkerJobManager(app.etcdClient, app.config.NodeID)

	// 初始化分布式锁管理器
	app.lockManager = joblock.NewLockManager(app.etcdClient, app.config.NodeID)

	// 初始化日志接收器
	logSinkOptions := []logsink.LogSinkOption{
		logsink.WithMasterConfig("http://"+app.config.GetServerAddress()+"/api/v1", app.config.NodeID),
		logsink.WithBufferSize(1000),
		logsink.WithBatchInterval(5 * time.Second),
	}
	app.logSink, err = logsink.NewLogSink(logSinkOptions...)
	if err != nil {
		return fmt.Errorf("failed to create log sink: %w", err)
	}

	// 初始化执行报告器
	executionReporter := executor.NewExecutionReporter(
		"http://"+app.config.GetServerAddress()+"/api/v1",
		app.config.NodeID,
	)

	// 初始化执行器
	app.jobExecutor = executor.NewExecutor(
		executionReporter,
		executor.WithSystemDefault(),
		executor.WithBufferSize(1024*8),
	)

	// 初始化调度器
	app.jobScheduler = scheduler.NewScheduler(
		app.jobManager,
		app.lockManager,
		app.executeJob,
		scheduler.WithCheckInterval(2*time.Second),
		scheduler.WithJobQueueCapacity(100),
	)

	utils.Info("worker components initialized successfully")
	return nil
}

// executeJob 执行任务的回调函数
func (app *WorkerApp) executeJob(scheduledJob *scheduler.ScheduledJob) error {
	job := scheduledJob.Job
	utils.Info("executing job",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name),
		zap.String("command", job.Command))

	// 创建执行上下文
	executionID := executor.GenerateExecutionID()
	executionCtx := executor.NewExecutionContext(
		executionID,
		job,
		job.Command,
		job.Args,
		job.WorkDir,
		job.Env,
		time.Duration(job.Timeout)*time.Second,
	)

	// 注册任务日志关联
	app.logSink.RegisterExecutionJob(executionID, job.ID)

	// 在新的 goroutine 中执行，以避免阻塞调度器
	go func() {
		// 执行任务
		result, err := app.jobExecutor.Execute(executionCtx.CreateContext(), executionCtx)
		if err != nil {
			utils.Error("job execution failed",
				zap.String("job_id", job.ID),
				zap.String("execution_id", executionID),
				zap.Error(err))

			// 确保日志被发送
			app.logSink.FlushExecutionLogs(executionID)
			return
		}

		// 记录执行结果
		utils.Info("job execution completed",
			zap.String("job_id", job.ID),
			zap.String("execution_id", executionID),
			zap.Int("exit_code", result.ExitCode),
			zap.String("state", string(result.State)))

		// 确保日志被发送
		app.logSink.FlushExecutionLogs(executionID)
	}()

	return nil
}

// Start 启动所有组件
func (app *WorkerApp) Start() error {
	app.lock.Lock()
	defer app.lock.Unlock()

	if app.isRunning {
		return nil
	}

	// 启动资源收集器
	app.resourceCollector.Start()

	// 启动 Worker 注册
	if err := app.workerRegister.Start(); err != nil {
		app.resourceCollector.Stop()
		return fmt.Errorf("failed to start worker register: %w", err)
	}

	// 启动健康检查
	if err := app.healthChecker.Start(); err != nil {
		app.workerRegister.Stop()
		app.resourceCollector.Stop()
		return fmt.Errorf("failed to start health checker: %w", err)
	}

	// 启动信号处理器
	app.signalHandler.Start()

	// 启动任务管理器
	if err := app.jobManager.Start(); err != nil {
		app.signalHandler.Stop()
		app.healthChecker.Stop()
		app.workerRegister.Stop()
		app.resourceCollector.Stop()
		return fmt.Errorf("failed to start job manager: %w", err)
	}

	// 启动锁管理器
	if err := app.lockManager.Start(); err != nil {
		app.jobManager.Stop()
		app.signalHandler.Stop()
		app.healthChecker.Stop()
		app.workerRegister.Stop()
		app.resourceCollector.Stop()
		return fmt.Errorf("failed to start lock manager: %w", err)
	}

	// 启动日志接收器
	if err := app.logSink.Start(); err != nil {
		app.lockManager.Stop()
		app.jobManager.Stop()
		app.signalHandler.Stop()
		app.healthChecker.Stop()
		app.workerRegister.Stop()
		app.resourceCollector.Stop()
		return fmt.Errorf("failed to start log sink: %w", err)
	}

	// 启动调度器
	if err := app.jobScheduler.Start(); err != nil {
		app.logSink.Stop()
		app.lockManager.Stop()
		app.jobManager.Stop()
		app.signalHandler.Stop()
		app.healthChecker.Stop()
		app.workerRegister.Stop()
		app.resourceCollector.Stop()
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	app.isRunning = true
	utils.Info("worker application started successfully",
		zap.String("node_id", app.config.NodeID),
		zap.String("version", app.config.Version))

	return nil
}

// Stop 停止所有组件
func (app *WorkerApp) Stop() error {
	app.lock.Lock()
	defer app.lock.Unlock()

	if !app.isRunning {
		return nil
	}

	utils.Info("stopping worker application...")

	// 按照依赖关系的相反顺序停止组件
	// 首先停止调度器，以避免新任务
	if err := app.jobScheduler.Stop(); err != nil {
		utils.Error("failed to stop scheduler", zap.Error(err))
	}

	// 停止日志接收器
	if err := app.logSink.Stop(); err != nil {
		utils.Error("failed to stop log sink", zap.Error(err))
	}

	// 停止锁管理器
	if err := app.lockManager.Stop(); err != nil {
		utils.Error("failed to stop lock manager", zap.Error(err))
	}

	// 停止任务管理器
	if err := app.jobManager.Stop(); err != nil {
		utils.Error("failed to stop job manager", zap.Error(err))
	}

	// 停止信号处理器
	app.signalHandler.Stop()

	// 停止健康检查
	if err := app.healthChecker.Stop(); err != nil {
		utils.Error("failed to stop health checker", zap.Error(err))
	}

	// 停止 Worker 注册
	if err := app.workerRegister.Stop(); err != nil {
		utils.Error("failed to stop worker register", zap.Error(err))
	}

	// 停止资源收集器
	app.resourceCollector.Stop()

	// 关闭客户端
	if app.etcdClient != nil {
		if err := app.etcdClient.Close(); err != nil {
			utils.Error("failed to close etcd client", zap.Error(err))
		}
	}

	app.isRunning = false
	utils.Info("worker application stopped")

	return nil
}

// IsRunning 返回应用程序是否正在运行
func (app *WorkerApp) IsRunning() bool {
	app.lock.Lock()
	defer app.lock.Unlock()
	return app.isRunning
}

// WaitForSignal 等待系统信号以停止应用程序
func (app *WorkerApp) WaitForSignal() {
	// 设置信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	sig := <-sigCh
	utils.Info("received signal", zap.String("signal", sig.String()))

	// 停止应用程序
	app.Stop()
}

// GetJobManager 返回任务管理器
func (app *WorkerApp) GetJobManager() jobmgr.IWorkerJobManager {
	return app.jobManager
}

// GetJobExecutor 返回任务执行器
func (app *WorkerApp) GetJobExecutor() *executor.Executor {
	return app.jobExecutor
}

// GetLogSink 返回日志接收器
func (app *WorkerApp) GetLogSink() logsink.ILogSink {
	return app.logSink
}

// GetScheduler 返回任务调度器
func (app *WorkerApp) GetScheduler() *scheduler.Scheduler {
	return app.jobScheduler
}
