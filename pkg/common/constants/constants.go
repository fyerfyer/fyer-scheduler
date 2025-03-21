package constants

// ETCD 路径前缀常量
const (
	// JobPrefix 是保存任务的etcd目录
	JobPrefix = "/fyer-scheduler/jobs/"

	// WorkerPrefix 是worker注册的etcd目录
	WorkerPrefix = "/fyer-scheduler/workers/"

	// LockPrefix 是分布式锁的etcd目录
	LockPrefix = "/fyer-scheduler/locks/"
)

// Job常量
const (
	// 任务状态
	JobStatusPending   = "pending"   // 等待执行
	JobStatusRunning   = "running"   // 正在执行
	JobStatusSucceeded = "succeeded" // 执行成功
	JobStatusFailed    = "failed"    // 执行失败
	JobStatusCancelled = "cancelled" // 已取消
)

// Worker常量
const (
	// Worker状态
	WorkerStatusOnline   = "online"   // 在线状态
	WorkerStatusOffline  = "offline"  // 离线状态
	WorkerStatusDisabled = "disabled" // 已禁用
)

// API相关常量
const (
	// 默认分页参数
	DefaultPageSize = 10
	DefaultPageNum  = 1

	// API版本
	APIVersion = "v1"
)

// 超时常量
const (
	// etcd操作超时时间
	EtcdDialTimeout = 5 // 秒
	EtcdOpTimeout   = 2 // 秒

	// 任务锁超时
	JobLockTTL = 5 // 秒

	// Worker心跳间隔和超时
	WorkerHeartbeatInterval = 5  // 秒
	WorkerHeartbeatTimeout  = 15 // 秒
)
