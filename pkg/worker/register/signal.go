package register

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// SignalHandler 处理系统信号和优雅关闭流程
type SignalHandler struct {
	workerRegister    *WorkerRegister
	healthChecker     *HealthChecker
	resourceCollector IResourceCollector
	shutdownTimeout   time.Duration
	sigChan           chan os.Signal
	stopChan          chan struct{}
	isRunning         bool
	lock              sync.Mutex
}

// NewSignalHandler 创建一个新的信号处理器
func NewSignalHandler(register *WorkerRegister, health *HealthChecker, resources IResourceCollector) *SignalHandler {
	return &SignalHandler{
		workerRegister:    register,
		healthChecker:     health,
		resourceCollector: resources,
		shutdownTimeout:   30 * time.Second, // 默认30秒关闭超时
		sigChan:           make(chan os.Signal, 1),
		stopChan:          make(chan struct{}),
	}
}

// Start 开始监听系统信号
func (sh *SignalHandler) Start() {
	sh.lock.Lock()
	defer sh.lock.Unlock()

	if sh.isRunning {
		return
	}

	sh.isRunning = true
	sh.stopChan = make(chan struct{})

	// 注册要监听的信号
	signal.Notify(sh.sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// 启动信号处理循环
	go sh.handleSignals()

	utils.Info("signal handler started")
}

// Stop 停止信号处理器
func (sh *SignalHandler) Stop() {
	sh.lock.Lock()
	defer sh.lock.Unlock()

	if !sh.isRunning {
		return
	}

	sh.isRunning = false
	close(sh.stopChan)
	signal.Stop(sh.sigChan)

	utils.Info("signal handler stopped")
}

// handleSignals 处理系统信号
func (sh *SignalHandler) handleSignals() {
	for {
		select {
		case sig := <-sh.sigChan:
			utils.Info("received system signal",
				zap.String("signal", sig.String()))

			sh.gracefulShutdown()
			return

		case <-sh.stopChan:
			return
		}
	}
}

// gracefulShutdown 执行优雅关闭流程
func (sh *SignalHandler) gracefulShutdown() {
	utils.Info("initiating graceful shutdown")

	// 创建一个通道用于表示关闭完成
	done := make(chan struct{})

	go func() {
		// 按顺序关闭组件 - 顺序很重要

		// 1. 首先停止健康检查
		if sh.healthChecker != nil {
			utils.Info("stopping health checker")
			sh.healthChecker.Stop()
		}

		// 2. 然后停止资源收集器
		if sh.resourceCollector != nil {
			utils.Info("stopping resource collector")
			sh.resourceCollector.Stop()
		}

		// 3. 最后停止Worker注册服务
		if sh.workerRegister != nil {
			utils.Info("stopping worker register")
			sh.workerRegister.Stop()
		}

		close(done)
	}()

	// 等待关闭完成或超时
	select {
	case <-done:
		utils.Info("graceful shutdown completed")

	case <-time.After(sh.shutdownTimeout):
		utils.Warn("graceful shutdown timed out",
			zap.Duration("timeout", sh.shutdownTimeout))
	}

	// 等待一段时间确保所有资源清理完成
	time.Sleep(2 * time.Second)

	utils.Info("exiting application")
	os.Exit(0)
}

// TriggerShutdown 手动触发优雅关闭
func (sh *SignalHandler) TriggerShutdown() {
	utils.Info("manual shutdown triggered")
	go sh.gracefulShutdown()
}

// HandleShutdownForComponents 为多个组件设置信号处理
func HandleShutdownForComponents(workerRegister *WorkerRegister, healthChecker *HealthChecker, resourceCollector *ResourceCollector) *SignalHandler {
	// 创建并启动信号处理器
	handler := NewSignalHandler(workerRegister, healthChecker, resourceCollector)
	handler.Start()

	utils.Info("shutdown handler setup for all components")
	return handler
}

// WaitForShutdownSignal 等待关闭信号并执行回调函数
func WaitForShutdownSignal(shutdownCallback func()) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	sig := <-sigChan
	utils.Info("received shutdown signal", zap.String("signal", sig.String()))

	if shutdownCallback != nil {
		shutdownCallback()
	}

	// 等待一段时间确保清理完成
	time.Sleep(2 * time.Second)

	utils.Info("exiting application")
	os.Exit(0)
}
