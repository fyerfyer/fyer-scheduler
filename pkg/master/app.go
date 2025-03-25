package master

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/config"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/api"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/jobmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/logmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/workermgr"
	"go.uber.org/zap"
)

// MasterApp 整合所有 Master 组件的应用程序结构体
type MasterApp struct {
	// 配置
	config *config.Config

	// 客户端
	etcdClient  *utils.EtcdClient
	mongoClient *utils.MongoDBClient

	// 数据仓库
	jobRepo    *repo.JobRepo
	workerRepo *repo.WorkerRepo
	logRepo    *repo.LogRepo

	// 核心组件
	jobManager    jobmgr.JobManager
	logManager    logmgr.LogManager
	workerManager workermgr.WorkerManager

	// 任务调度和Worker选择器
	scheduler      jobmgr.IScheduler
	workerSelector jobmgr.IWorkerSelector

	// API服务器
	apiServer *api.APIServer

	// 状态控制
	isRunning bool
	stopChan  chan struct{}
	lock      sync.Mutex
}

// NewMasterApp 创建一个新的 MasterApp 实例
func NewMasterApp(config *config.Config) (*MasterApp, error) {
	app := &MasterApp{
		config:   config,
		stopChan: make(chan struct{}),
	}

	// 初始化各组件
	if err := app.initClients(); err != nil {
		return nil, err
	}

	if err := app.initRepositories(); err != nil {
		return nil, err
	}

	if err := app.initManagers(); err != nil {
		return nil, err
	}

	if err := app.initAPIServer(); err != nil {
		return nil, err
	}

	return app, nil
}

// initClients 初始化客户端连接
func (app *MasterApp) initClients() error {
	// 初始化 etcd 客户端
	var err error
	app.etcdClient, err = utils.NewEtcdClient(&app.config.Etcd)
	if err != nil {
		return fmt.Errorf("failed to initialize etcd client: %w", err)
	}

	// 初始化 MongoDB 客户端
	app.mongoClient, err = utils.NewMongoDBClient(&app.config.MongoDB)
	if err != nil {
		app.etcdClient.Close()
		return fmt.Errorf("failed to initialize MongoDB client: %w", err)
	}

	utils.Info("clients initialized successfully")
	return nil
}

// initRepositories 初始化数据仓库
func (app *MasterApp) initRepositories() error {
	app.jobRepo = repo.NewJobRepo(app.etcdClient, app.mongoClient)
	app.workerRepo = repo.NewWorkerRepo(app.etcdClient, app.mongoClient)
	app.logRepo = repo.NewLogRepo(app.mongoClient)

	utils.Info("repositories initialized successfully")
	return nil
}

// initManagers 初始化管理器组件
func (app *MasterApp) initManagers() error {
	// 初始化 Worker 选择器
	app.workerSelector = jobmgr.NewWorkerSelector(app.workerRepo, jobmgr.LeastJobsStrategy)

	// 初始化 JobManager
	app.jobManager = jobmgr.NewJobManager(app.jobRepo, app.logRepo, app.workerRepo, app.etcdClient)

	// 初始化调度器
	app.scheduler = jobmgr.NewScheduler(app.jobManager, app.workerRepo, 10*time.Second)

	// 初始化 LogManager
	app.logManager = logmgr.NewLogManager(app.logRepo)

	// 初始化 WorkerManager
	app.workerManager = workermgr.NewWorkerManager(app.workerRepo, 30*time.Second, app.etcdClient)

	utils.Info("managers initialized successfully")
	return nil
}

// initAPIServer 初始化 API 服务器
func (app *MasterApp) initAPIServer() error {
	// 创建 API 服务器选项
	apiOptions := &api.APIServerOptions{
		Address:          app.config.GetServerAddress(),
		ReadTimeout:      30 * time.Second,
		WriteTimeout:     30 * time.Second,
		IdleTimeout:      60 * time.Second,
		ShutdownTimeout:  15 * time.Second,
		ReleaseMode:      true, // 生产模式
		APIVersion:       constants.APIVersion,
		MaxRequestBodyMB: 5,          // 限制请求体大小为 5MB
		APIKeys:          []string{}, // 可从配置中加载
	}

	// 创建 API 服务器
	app.apiServer = api.NewAPIServer(apiOptions)

	// 创建任务操作对象
	jobOps := jobmgr.NewJobOperations(
		app.jobManager,
		app.workerRepo,
		app.logRepo,
		app.workerSelector,
		app.scheduler,
	)

	// 创建各处理器
	jobHandler := api.NewJobHandler(app.jobManager, jobOps, app.scheduler)
	logHandler := api.NewLogHandler(app.logManager)
	workerHandler := api.NewWorkerHandler(app.workerManager)
	systemHandler := api.NewSystemHandler(
		app.jobManager,
		app.workerManager,
		app.logManager,
		app.config.Version,
	)

	// 创建路由器
	router := api.NewRouter(
		jobHandler,
		logHandler,
		workerHandler,
		systemHandler,
		apiOptions.APIKeys,
	)

	// 设置路由器
	app.apiServer.SetRouter(router)

	utils.Info("API server initialized successfully")
	return nil
}

// Start 启动所有组件
func (app *MasterApp) Start() error {
	app.lock.Lock()
	defer app.lock.Unlock()

	if app.isRunning {
		return nil
	}

	// 启动 JobManager
	if err := app.jobManager.Start(); err != nil {
		return fmt.Errorf("failed to start job manager: %w", err)
	}

	// 启动调度器
	if err := app.scheduler.Start(); err != nil {
		app.jobManager.Stop()
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	// 启动 LogManager
	if err := app.logManager.Start(); err != nil {
		app.scheduler.Stop()
		app.jobManager.Stop()
		return fmt.Errorf("failed to start log manager: %w", err)
	}

	// 启动 WorkerManager
	if err := app.workerManager.Start(); err != nil {
		app.logManager.Stop()
		app.scheduler.Stop()
		app.jobManager.Stop()
		return fmt.Errorf("failed to start worker manager: %w", err)
	}

	// 启动 API 服务器
	if err := app.apiServer.Start(); err != nil {
		app.workerManager.Stop()
		app.logManager.Stop()
		app.scheduler.Stop()
		app.jobManager.Stop()
		return fmt.Errorf("failed to start API server: %w", err)
	}

	app.isRunning = true
	utils.Info("master application started successfully")

	return nil
}

// Stop 停止所有组件
func (app *MasterApp) Stop() error {
	app.lock.Lock()
	defer app.lock.Unlock()

	if !app.isRunning {
		return nil
	}

	utils.Info("stopping master application...")

	// 停止 API 服务器
	if err := app.apiServer.Stop(); err != nil {
		utils.Error("failed to stop API server", zap.Error(err))
	}

	// 停止 WorkerManager
	if err := app.workerManager.Stop(); err != nil {
		utils.Error("failed to stop worker manager", zap.Error(err))
	}

	// 停止 LogManager
	if err := app.logManager.Stop(); err != nil {
		utils.Error("failed to stop log manager", zap.Error(err))
	}

	// 停止调度器
	if err := app.scheduler.Stop(); err != nil {
		utils.Error("failed to stop scheduler", zap.Error(err))
	}

	// 停止 JobManager
	if err := app.jobManager.Stop(); err != nil {
		utils.Error("failed to stop job manager", zap.Error(err))
	}

	// 关闭客户端
	if app.mongoClient != nil {
		if err := app.mongoClient.Close(); err != nil {
			utils.Error("failed to close MongoDB client", zap.Error(err))
		}
	}

	if app.etcdClient != nil {
		if err := app.etcdClient.Close(); err != nil {
			utils.Error("failed to close etcd client", zap.Error(err))
		}
	}

	app.isRunning = false
	utils.Info("master application stopped")

	return nil
}

// IsRunning 返回应用程序是否正在运行
func (app *MasterApp) IsRunning() bool {
	app.lock.Lock()
	defer app.lock.Unlock()
	return app.isRunning
}

// WaitForSignal 等待系统信号以停止应用程序
func (app *MasterApp) WaitForSignal() {
	// 设置信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	sig := <-sigCh
	utils.Info("received signal", zap.String("signal", sig.String()))

	// 停止应用程序
	app.Stop()
}

// GetAPIServer 返回API服务器实例
func (app *MasterApp) GetAPIServer() *api.APIServer {
	return app.apiServer
}
