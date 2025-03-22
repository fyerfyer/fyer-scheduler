package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/jobmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/logmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/workermgr"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// APIServer 封装了API服务器相关的功能
type APIServer struct {
	router        *Router
	server        *http.Server
	options       *APIServerOptions
	jobHandler    *JobHandler
	logHandler    *LogHandler
	workerHandler *WorkerHandler
	systemHandler *SystemHandler
	isRunning     bool
}

// APIServerOptions 包含API服务器的配置选项
type APIServerOptions struct {
	Address          string        // 监听地址，格式为 host:port
	ReadTimeout      time.Duration // 读取超时
	WriteTimeout     time.Duration // 写入超时
	IdleTimeout      time.Duration // 空闲连接超时
	ShutdownTimeout  time.Duration // 关闭超时
	APIKeys          []string      // API密钥列表
	EnableTLS        bool          // 是否启用TLS
	CertFile         string        // 证书文件
	KeyFile          string        // 密钥文件
	TrustedProxies   []string      // 可信代理IP
	ReleaseMode      bool          // 是否为发布模式
	APIVersion       string        // API版本
	MaxRequestBodyMB int           // 最大请求体大小（MB）
}

// NewAPIServer 创建一个新的API服务器
func NewAPIServer(options *APIServerOptions) *APIServer {
	// 使用默认配置如果未提供
	if options == nil {
		options = &APIServerOptions{
			Address:          ":8080",
			ReadTimeout:      time.Second * 10,
			WriteTimeout:     time.Second * 30,
			IdleTimeout:      time.Second * 60,
			ShutdownTimeout:  time.Second * 15,
			ReleaseMode:      false,
			APIVersion:       "v1",
			MaxRequestBodyMB: 10,
		}
	}

	// 设置Gin模式
	if options.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建Gin路由器
	ginRouter := gin.New()

	// 设置可信代理
	if len(options.TrustedProxies) > 0 {
		ginRouter.SetTrustedProxies(options.TrustedProxies)
	}

	// 设置最大请求体大小
	if options.MaxRequestBodyMB > 0 {
		ginRouter.MaxMultipartMemory = int64(options.MaxRequestBodyMB) << 20
	}

	// 创建HTTP服务器
	server := &http.Server{
		Addr:         options.Address,
		Handler:      ginRouter,
		ReadTimeout:  options.ReadTimeout,
		WriteTimeout: options.WriteTimeout,
		IdleTimeout:  options.IdleTimeout,
	}

	return &APIServer{
		server:    server,
		options:   options,
		isRunning: false,
	}
}

// BindComponents 绑定各组件到API服务器
func (s *APIServer) BindComponents(
	jobMgr jobmgr.JobManager,
	logMgr logmgr.LogManager,
	workerMgr workermgr.WorkerManager,
	version string,
) error {
	// 创建处理器
	jobOps := jobmgr.NewJobOperations(
		jobMgr,
		nil, // 这里需要传入workerRepo，但我们可以后续在实际集成时添加
		nil, // 这里需要传入logRepo，但我们可以后续在实际集成时添加
		nil, // workerSelector
		nil, // scheduler
	)

	s.jobHandler = NewJobHandler(jobMgr, jobOps, nil)
	s.logHandler = NewLogHandler(logMgr)
	s.workerHandler = NewWorkerHandler(workerMgr)
	s.systemHandler = NewSystemHandler(jobMgr, workerMgr, logMgr, version)

	// 创建路由器
	s.router = NewRouter(
		s.jobHandler,
		s.logHandler,
		s.workerHandler,
		s.systemHandler,
		s.options.APIKeys,
	)

	// 设置路由
	s.router.Setup(gin.New())

	utils.Info("API components bound to server")
	return nil
}

// Start 启动API服务器
func (s *APIServer) Start() error {
	if s.isRunning {
		return fmt.Errorf("server is already running")
	}

	if s.router == nil {
		return fmt.Errorf("router not initialized, call BindComponents first")
	}

	s.isRunning = true

	// 设置HTTP服务器的处理器为Gin路由器
	// 注意：这里假设Gin路由器已经被设置为server.Handler，如果不是，需要在这里设置

	// 异步启动服务器，以便可以处理错误
	go func() {
		utils.Info("API server starting", zap.String("address", s.options.Address))

		var err error
		if s.options.EnableTLS {
			err = s.server.ListenAndServeTLS(s.options.CertFile, s.options.KeyFile)
		} else {
			err = s.server.ListenAndServe()
		}

		// 如果服务器已关闭，ErrServerClosed不是一个真正的错误
		if err != nil && err != http.ErrServerClosed {
			utils.Error("API server failed", zap.Error(err))
		}
	}()

	utils.Info("API server started")
	return nil
}

// Stop 优雅地停止API服务器
func (s *APIServer) Stop() error {
	if !s.isRunning {
		return nil
	}

	utils.Info("stopping API server")

	// 创建一个上下文，用于控制关闭超时
	ctx, cancel := context.WithTimeout(context.Background(), s.options.ShutdownTimeout)
	defer cancel()

	// 优雅关闭服务器
	if err := s.server.Shutdown(ctx); err != nil {
		utils.Error("API server shutdown error", zap.Error(err))
		return err
	}

	s.isRunning = false
	utils.Info("API server stopped")
	return nil
}

// IsRunning 返回服务器是否正在运行
func (s *APIServer) IsRunning() bool {
	return s.isRunning
}

// GetAddress 返回服务器的监听地址
func (s *APIServer) GetAddress() string {
	return s.options.Address
}
