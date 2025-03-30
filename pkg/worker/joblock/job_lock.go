package joblock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// JobLock 实现了分布式锁接口
type JobLock struct {
	etcdClient  *utils.EtcdClient                       // etcd客户端
	jobID       string                                  // 任务ID
	workerID    string                                  // 工作节点ID
	lockKey     string                                  // 锁的etcd键
	leaseID     clientv3.LeaseID                        // 租约ID
	cancelFunc  context.CancelFunc                      // 用于取消自动续期
	isLocked    bool                                    // 是否已获取锁
	ttl         int64                                   // 锁超时时间(秒)
	maxRetries  int                                     // 最大重试次数
	retryDelay  time.Duration                           // 重试间隔
	mutex       sync.Mutex                              // 保护并发访问
	keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse // 自动续期channel
	notifyCh    chan struct{}                           // 通知channel，用于处理续期失败
}

// NewJobLock 创建一个新的任务锁
func NewJobLock(etcdClient *utils.EtcdClient, jobID, workerID string, options ...JobLockOption) *JobLock {
	// 默认选项
	lock := &JobLock{
		etcdClient: etcdClient,
		jobID:      jobID,
		workerID:   workerID,
		lockKey:    constants.LockPrefix + jobID,
		ttl:        constants.JobLockTTL,
		maxRetries: 3,
		retryDelay: 200 * time.Millisecond,
		isLocked:   false,
		notifyCh:   make(chan struct{}),
	}

	// 应用选项
	for _, option := range options {
		option(lock)
	}

	return lock
}

// TryLock 尝试获取任务锁
func (j *JobLock) TryLock() (bool, error) {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	// 如果已经持有锁，直接返回成功
	if j.isLocked {
		utils.Debug("already holding lock, returning success",
            zap.String("job_id", j.jobID),
            zap.String("worker_id", j.workerID))
		return true, nil
	}

	// 尝试获取锁，最多重试指定次数
	var err error
	var success bool
	var leaseID clientv3.LeaseID

	utils.Info("attempting to acquire lock",
        zap.String("job_id", j.jobID),
        zap.String("worker_id", j.workerID),
        zap.String("lock_key", j.lockKey),
        zap.Int64("ttl", j.ttl))

	for i := 0; i < j.maxRetries; i++ {
		// 尝试获取锁
		leaseID, err = j.etcdClient.TryAcquireLock(j.lockKey, j.ttl)
		if err == nil {
			success = true
			break
		}

		utils.Warn("failed to acquire lock, retrying",
			zap.String("job_id", j.jobID),
			zap.String("worker_id", j.workerID),
			zap.Int("attempt", i+1),
			zap.Error(err))

		// 如果达到最大重试次数，返回错误
		if i == j.maxRetries-1 {
			utils.Error("max retry attempts reached",
                zap.String("job_id", j.jobID),
                zap.String("worker_id", j.workerID),
                zap.Int("max_retries", j.maxRetries),
                zap.Error(err))
			return false, fmt.Errorf("failed to acquire lock after %d attempts: %w", j.maxRetries, err)
		}

		// 等待一段时间后重试
		time.Sleep(j.retryDelay)
	}

	if success {
		j.leaseID = leaseID
		j.isLocked = true

		// 启动自动续期
		j.startAutoRenew()

		utils.Info("lock acquired successfully",
			zap.String("job_id", j.jobID),
			zap.String("worker_id", j.workerID),
			zap.Int64("lease_id", int64(j.leaseID)))
		return true, nil
	}

	utils.Info("failed to acquire lock",
        zap.String("job_id", j.jobID),
        zap.String("worker_id", j.workerID))

	return false, fmt.Errorf("failed to acquire lock for job %s", j.jobID)
}

// Unlock 释放任务锁
func (j *JobLock) Unlock() error {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	// 如果未持有锁，直接返回
	if !j.isLocked {
		return nil
	}

	// 停止自动续期
	j.stopAutoRenew()

	// 释放锁
	err := j.etcdClient.ReleaseLock(j.leaseID)
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	j.isLocked = false
	j.leaseID = 0

	utils.Info("lock released successfully",
		zap.String("job_id", j.jobID),
		zap.String("worker_id", j.workerID))
	return nil
}

// GetLeaseID 获取租约ID
func (j *JobLock) GetLeaseID() clientv3.LeaseID {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	return j.leaseID
}

// IsLocked 检查是否持有锁
func (j *JobLock) IsLocked() bool {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	return j.isLocked
}

// startAutoRenew 启动自动续期
func (j *JobLock) startAutoRenew() {
	// 创建一个可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	j.cancelFunc = cancel

	// 启动keepalive
	keepAliveCh, err := j.etcdClient.KeepAliveLease(ctx, j.leaseID)
	if err != nil {
		utils.Error("failed to start lease keepalive",
			zap.String("job_id", j.jobID),
			zap.Error(err))
		return
	}

	j.keepAliveCh = keepAliveCh

	// 启动一个goroutine监听keepalive响应
	go j.monitorKeepAlive()
}

// stopAutoRenew 停止自动续期
func (j *JobLock) stopAutoRenew() {
	if j.cancelFunc != nil {
		j.cancelFunc()
		j.cancelFunc = nil
	}

	// 清理keepalive channel
	j.keepAliveCh = nil
}

// monitorKeepAlive 监控keepalive响应
func (j *JobLock) monitorKeepAlive() {
	for {
		select {
		case resp, ok := <-j.keepAliveCh:
			if !ok {
				// channel已关闭，可能是续约失败
				utils.Warn("lease keepalive channel closed",
					zap.String("job_id", j.jobID),
					zap.String("worker_id", j.workerID))
				j.handleKeepAliveFailure()
				return
			}

			if resp == nil {
				// 收到nil响应，可能是续约失败
				utils.Warn("received nil keepalive response",
					zap.String("job_id", j.jobID),
					zap.String("worker_id", j.workerID))
				j.handleKeepAliveFailure()
				return
			}

			// 续约成功
			utils.Debug("lease keepalive successful",
				zap.String("job_id", j.jobID),
				zap.Int64("lease_id", int64(j.leaseID)),
				zap.Int64("ttl", resp.TTL))

		case <-j.notifyCh:
			// 收到退出信号
			return
		}
	}
}

// handleKeepAliveFailure 处理续约失败
func (j *JobLock) handleKeepAliveFailure() {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	// 标记为未锁定
	j.isLocked = false
	j.leaseID = 0 // 清除租约ID

	// 在这里，我们不自动尝试重新获取锁
	// 这应该是由调用者决定的
	utils.Warn("keepalive failed, lock has been released",
        zap.String("job_id", j.jobID),
        zap.String("worker_id", j.workerID))
}

// StopRenewal 停止自动续期
func (j *JobLock) StopRenewal() {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	if j.cancelFunc != nil {
		j.cancelFunc()
		j.cancelFunc = nil
	}

	// 清除keepalive channel而不释放锁
	j.keepAliveCh = nil
}
