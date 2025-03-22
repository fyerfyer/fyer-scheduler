package register

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// WorkerRegister 实现了 Worker 注册器
type WorkerRegister struct {
	worker        *models.Worker
	etcdClient    *utils.EtcdClient
	resourceInfo  ResourceInfo
	options       RegisterOptions
	status        Status
	resourceMutex sync.RWMutex
	statusMutex   sync.RWMutex
	stopChan      chan struct{}
	isRunning     bool
	lock          sync.Mutex
}

// NewWorkerRegister 创建一个新的 Worker 注册器
func NewWorkerRegister(etcdClient *utils.EtcdClient, options RegisterOptions) (*WorkerRegister, error) {
	// 如果没有提供 NodeID，生成主机名作为 ID
	if options.NodeID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get hostname: %w", err)
		}
		options.NodeID = hostname
	}

	// 如果没有提供主机名，使用 NodeID
	if options.Hostname == "" {
		options.Hostname = options.NodeID
	}

	// 如果没有提供 IP，尝试获取本机 IP
	if options.IP == "" {
		// 简单实现，实际应根据网络情况选择合适的 IP
		options.IP = "127.0.0.1"
	}

	// 设置默认心跳超时
	if options.HeartbeatTimeout <= 0 {
		options.HeartbeatTimeout = constants.WorkerHeartbeatTimeout
	}

	// 设置默认租约 TTL
	if options.HeartbeatTTL <= 0 {
		options.HeartbeatTTL = int64(constants.WorkerHeartbeatTimeout)
	}

	// 创建 Worker 对象
	worker := models.NewWorker(options.NodeID, options.Hostname, options.IP)

	// 设置标签
	if options.Labels != nil {
		for k, v := range options.Labels {
			worker.Labels[k] = v
		}
	}

	return &WorkerRegister{
		worker:     worker,
		etcdClient: etcdClient,
		options:    options,
		status: Status{
			IsRegistered: false,
			State:        constants.WorkerStatusOffline,
			RunningJobs:  make([]string, 0),
		},
		stopChan:  make(chan struct{}),
		isRunning: false,
	}, nil
}

// Register 将 Worker 注册到 etcd
func (wr *WorkerRegister) Register() (clientv3.LeaseID, error) {
	wr.statusMutex.Lock()
	defer wr.statusMutex.Unlock()

	// 更新资源信息
	err := wr.UpdateResources()
	if err != nil {
		utils.Warn("failed to update resources during registration", zap.Error(err))
		// 继续注册过程，不让资源更新失败影响注册
	}

	// 准备 Worker 状态
	wr.worker.Status = constants.WorkerStatusOnline
	wr.worker.UpdateTime = time.Now()
	wr.worker.LastHeartbeat = time.Now()

	// 序列化 Worker 对象
	workerJSON, err := wr.worker.ToJSON()
	if err != nil {
		return 0, fmt.Errorf("failed to serialize worker: %w", err)
	}

	// 使用租约注册到 etcd
	leaseID, err := wr.etcdClient.PutWithLease(
		wr.worker.Key(),
		workerJSON,
		wr.options.HeartbeatTTL,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to register worker to etcd: %w", err)
	}

	// 更新状态
	wr.status.IsRegistered = true
	wr.status.LeaseID = leaseID
	wr.status.HeartbeatAt = time.Now()
	wr.status.State = constants.WorkerStatusOnline

	utils.Info("worker registered successfully",
		zap.String("worker_id", wr.worker.ID),
		zap.String("hostname", wr.worker.Hostname),
		zap.String("ip", wr.worker.IP),
		zap.Int64("lease_id", int64(leaseID)))

	return leaseID, nil
}

// Heartbeat 发送心跳更新 Worker 状态
func (wr *WorkerRegister) Heartbeat() error {
	wr.statusMutex.Lock()
	defer wr.statusMutex.Unlock()

	if !wr.status.IsRegistered {
		return fmt.Errorf("worker is not registered")
	}

	// 更新心跳时间
	wr.worker.UpdateHeartbeat()
	wr.worker.UpdateTime = time.Now()

	// 序列化 Worker 对象
	workerJSON, err := wr.worker.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize worker: %w", err)
	}

	// 更新 etcd 中的数据
	err = wr.etcdClient.Put(wr.worker.Key(), workerJSON)
	if err != nil {
		return fmt.Errorf("failed to update worker heartbeat: %w", err)
	}

	// 更新状态
	wr.status.HeartbeatAt = time.Now()

	utils.Debug("heartbeat sent",
		zap.String("worker_id", wr.worker.ID),
		zap.Time("heartbeat_time", wr.status.HeartbeatAt))

	return nil
}

// UpdateResources 更新 Worker 资源信息
func (wr *WorkerRegister) UpdateResources() error {
	wr.resourceMutex.Lock()
	defer wr.resourceMutex.Unlock()

	// 收集系统资源信息
	resourceInfo := CollectSystemResources()
	wr.resourceInfo = resourceInfo

	// 更新 Worker 对象中的资源信息
	wr.worker.CPUCores = resourceInfo.CPUCores
	wr.worker.MemoryTotal = resourceInfo.MemoryTotal
	wr.worker.MemoryFree = resourceInfo.MemoryFree
	wr.worker.DiskTotal = resourceInfo.DiskTotal
	wr.worker.DiskFree = resourceInfo.DiskFree
	wr.worker.LoadAvg = resourceInfo.LoadAvg
	wr.worker.UpdateTime = time.Now()

	// 如果已注册，则更新 etcd 中的数据
	if wr.status.IsRegistered {
		workerJSON, err := wr.worker.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize worker: %w", err)
		}

		err = wr.etcdClient.Put(wr.worker.Key(), workerJSON)
		if err != nil {
			return fmt.Errorf("failed to update worker resources: %w", err)
		}
	}

	utils.Debug("resources updated",
		zap.String("worker_id", wr.worker.ID),
		zap.Int64("memory_free_mb", resourceInfo.MemoryFree),
		zap.Int64("disk_free_gb", resourceInfo.DiskFree),
		zap.Float64("load_avg", resourceInfo.LoadAvg))

	return nil
}

// Start 启动注册服务
func (wr *WorkerRegister) Start() error {
	wr.lock.Lock()
	defer wr.lock.Unlock()

	if wr.isRunning {
		return nil // 已经启动
	}

	wr.isRunning = true
	wr.stopChan = make(chan struct{})

	// 注册 Worker
	leaseID, err := wr.Register()
	if err != nil {
		wr.isRunning = false
		return fmt.Errorf("failed to register worker: %w", err)
	}

	// 启动租约保持活跃的协程
	go wr.keepAliveLease(leaseID)

	utils.Info("worker register service started",
		zap.String("worker_id", wr.worker.ID),
		zap.String("hostname", wr.worker.Hostname),
		zap.String("ip", wr.worker.IP))

	return nil
}

// Stop 停止注册服务
func (wr *WorkerRegister) Stop() error {
	wr.lock.Lock()
	defer wr.lock.Unlock()

	if !wr.isRunning {
		return nil
	}

	wr.isRunning = false
	close(wr.stopChan)

	// 将状态更新为离线
	err := wr.SetStatus(constants.WorkerStatusOffline)
	if err != nil {
		utils.Error("failed to set worker status to offline", zap.Error(err))
		// 继续停止过程
	}

	// 释放租约
	wr.statusMutex.RLock()
	leaseID := wr.status.LeaseID
	wr.statusMutex.RUnlock()

	if leaseID != 0 {
		err := wr.etcdClient.ReleaseLock(leaseID)
		if err != nil {
			utils.Error("failed to release lease",
				zap.Int64("lease_id", int64(leaseID)),
				zap.Error(err))
			// 继续停止过程
		}
	}

	utils.Info("worker register service stopped", zap.String("worker_id", wr.worker.ID))
	return nil
}

// GetWorker 获取当前 Worker 信息
func (wr *WorkerRegister) GetWorker() *models.Worker {
	wr.resourceMutex.RLock()
	defer wr.resourceMutex.RUnlock()
	return wr.worker
}

// SetStatus 设置 Worker 状态
func (wr *WorkerRegister) SetStatus(status string) error {
	wr.statusMutex.Lock()
	defer wr.statusMutex.Unlock()

	if status != constants.WorkerStatusOnline &&
		status != constants.WorkerStatusOffline &&
		status != constants.WorkerStatusDisabled {
		return fmt.Errorf("invalid worker status: %s", status)
	}

	// 更新 Worker 状态
	wr.worker.SetStatus(status)
	wr.status.State = status

	// 如果已注册且不是设置为离线状态，更新 etcd 中的数据
	if wr.status.IsRegistered && status != constants.WorkerStatusOffline {
		workerJSON, err := wr.worker.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize worker: %w", err)
		}

		err = wr.etcdClient.Put(wr.worker.Key(), workerJSON)
		if err != nil {
			return fmt.Errorf("failed to update worker status: %w", err)
		}
	}

	utils.Info("worker status changed",
		zap.String("worker_id", wr.worker.ID),
		zap.String("status", status))

	return nil
}

// keepAliveLease 保持租约活跃
func (wr *WorkerRegister) keepAliveLease(leaseID clientv3.LeaseID) {
	// 设置租约保持活跃
	err := wr.etcdClient.KeepAliveLock(leaseID)
	if err != nil {
		utils.Error("failed to setup lease keepalive", zap.Error(err))
		return
	}

	utils.Info("lease keepalive started", zap.Int64("lease_id", int64(leaseID)))

	// 监听停止信号
	<-wr.stopChan
	utils.Info("lease keepalive stopped", zap.Int64("lease_id", int64(leaseID)))
}

// GetStatus 获取当前状态
func (wr *WorkerRegister) GetStatus() Status {
	wr.statusMutex.RLock()
	defer wr.statusMutex.RUnlock()
	return wr.status
}

// AddRunningJob 添加正在运行的任务
func (wr *WorkerRegister) AddRunningJob(jobID string) error {
	wr.statusMutex.Lock()
	defer wr.statusMutex.Unlock()

	// 检查任务是否已存在
	for _, id := range wr.status.RunningJobs {
		if id == jobID {
			return nil // 任务已存在，不需要添加
		}
	}

	// 添加到状态中
	wr.status.RunningJobs = append(wr.status.RunningJobs, jobID)

	// 添加到 Worker 对象中
	wr.worker.AddRunningJob(jobID)

	// 更新 etcd 中的数据
	if wr.status.IsRegistered {
		workerJSON, err := wr.worker.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize worker: %w", err)
		}

		err = wr.etcdClient.Put(wr.worker.Key(), workerJSON)
		if err != nil {
			return fmt.Errorf("failed to update running jobs: %w", err)
		}
	}

	utils.Info("job added to running list",
		zap.String("worker_id", wr.worker.ID),
		zap.String("job_id", jobID))

	return nil
}

// RemoveRunningJob 移除正在运行的任务
func (wr *WorkerRegister) RemoveRunningJob(jobID string) error {
	wr.statusMutex.Lock()
	defer wr.statusMutex.Unlock()

	// 从状态中移除
	found := false
	for i, id := range wr.status.RunningJobs {
		if id == jobID {
			wr.status.RunningJobs = append(wr.status.RunningJobs[:i], wr.status.RunningJobs[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("job not found in running list: %s", jobID)
	}

	// 从 Worker 对象中移除
	wr.worker.RemoveRunningJob(jobID)

	// 更新 etcd 中的数据
	if wr.status.IsRegistered {
		workerJSON, err := wr.worker.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize worker: %w", err)
		}

		err = wr.etcdClient.Put(wr.worker.Key(), workerJSON)
		if err != nil {
			return fmt.Errorf("failed to update running jobs: %w", err)
		}
	}

	utils.Info("job removed from running list",
		zap.String("worker_id", wr.worker.ID),
		zap.String("job_id", jobID))

	return nil
}
