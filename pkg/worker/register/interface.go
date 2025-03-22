package register

import (
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// WorkerRegistrar 定义Worker注册器的接口
type WorkerRegistrar interface {
	// Register 将Worker注册到etcd，成功后返回租约ID
	Register() (clientv3.LeaseID, error)

	// Heartbeat 发送心跳，更新Worker状态
	Heartbeat() error

	// UpdateResources 更新Worker资源信息
	UpdateResources() error

	// Start 启动注册服务，包括定期心跳和资源更新
	Start() error

	// Stop 停止注册服务，优雅关闭
	Stop() error

	// GetWorker 获取当前Worker信息
	GetWorker() *models.Worker

	// SetStatus 设置Worker状态
	SetStatus(status string) error
}

// IResourceCollector 定义资源收集器接口
type IResourceCollector interface {
	// Start 启动资源收集
	Start()

	// Stop 停止资源收集
	Stop()

	// GetResourceInfo 获取最新的资源信息
	GetResourceInfo() ResourceInfo
}

// ResourceInfo 包含Worker节点的资源信息
type ResourceInfo struct {
	CPUCores    int     // CPU核心数
	MemoryTotal int64   // 内存总量(MB)
	MemoryFree  int64   // 可用内存(MB)
	DiskTotal   int64   // 磁盘总量(GB)
	DiskFree    int64   // 可用磁盘(GB)
	LoadAvg     float64 // 负载平均值
}

// RegisterOptions 注册选项
type RegisterOptions struct {
	NodeID           string            // Worker节点ID
	Hostname         string            // 主机名
	IP               string            // IP地址
	HeartbeatTimeout int               // 心跳超时时间(秒)
	HeartbeatTTL     int64             // 租约TTL(秒)
	Labels           map[string]string // 标签
	ResourceInterval time.Duration     // 资源更新间隔
}

// Status 表示Worker状态信息
type Status struct {
	IsRegistered bool             // 是否已注册
	HeartbeatAt  time.Time        // 最后一次心跳时间
	LeaseID      clientv3.LeaseID // 租约ID
	RunningJobs  []string         // 正在运行的任务
	State        string           // 当前状态
}
