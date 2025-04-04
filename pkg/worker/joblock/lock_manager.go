package joblock

import (
	clientv3 "go.etcd.io/etcd/client/v3"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// LockManager 管理任务锁的生命周期
type LockManager struct {
	etcdClient    *utils.EtcdClient   // etcd客户端
	locks         map[string]*JobLock // 任务ID -> 锁对象
	mutex         sync.RWMutex        // 保护锁映射表的互斥锁
	workerID      string              // 工作节点ID
	cleanupTicker *time.Ticker        // 定期清理的计时器
	stopChan      chan struct{}       // 停止信号
	isRunning     bool                // 是否正在运行
}

// NewLockManager 创建一个新的锁管理器
func NewLockManager(etcdClient *utils.EtcdClient, workerID string) *LockManager {
	return &LockManager{
		etcdClient: etcdClient,
		locks:      make(map[string]*JobLock),
		workerID:   workerID,
		stopChan:   make(chan struct{}),
	}
}

// Start 启动锁管理器
func (m *LockManager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isRunning {
		return nil
	}

	m.isRunning = true
	m.stopChan = make(chan struct{})
	m.cleanupTicker = time.NewTicker(30 * time.Second) // 每30秒清理一次过期锁

	// 启动清理协程
	go m.cleanupLoop()

	utils.Info("lock manager started", zap.String("worker_id", m.workerID))
	return nil
}

// Stop 停止锁管理器
func (m *LockManager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isRunning {
		return nil
	}

	m.isRunning = false
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
	}
	close(m.stopChan)

	// 释放所有锁
	m.releaseAllLocksInternal()

	utils.Info("lock manager stopped", zap.String("worker_id", m.workerID))
	return nil
}

// CreateLock 创建一个新的任务锁
func (m *LockManager) CreateLock(jobID string, options ...JobLockOption) *JobLock {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查是否已存在该任务的锁
	if existingLock, ok := m.locks[jobID]; ok {
		return existingLock
	}

	// 创建新锁
	lock := NewJobLock(m.etcdClient, jobID, m.workerID, options...)
	m.locks[jobID] = lock

	utils.Debug("lock created",
		zap.String("job_id", jobID),
		zap.String("worker_id", m.workerID))
	return lock
}

// GetLock 获取指定任务的锁
func (m *LockManager) GetLock(jobID string) (*JobLock, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	lock, exists := m.locks[jobID]
	return lock, exists
}

// ReleaseLock 释放指定任务的锁
func (m *LockManager) ReleaseLock(jobID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	lock, exists := m.locks[jobID]
	if !exists {
		// 锁不存在，无需释放
		return nil
	}

	// 释放锁
	err := lock.Unlock()
	if err != nil {
		return err
	}

	// 从映射表中移除
	delete(m.locks, jobID)
	utils.Debug("lock released and removed from manager",
		zap.String("job_id", jobID),
		zap.String("worker_id", m.workerID))
	return nil
}

// ReleaseAllLocks 释放所有锁
func (m *LockManager) ReleaseAllLocks() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.releaseAllLocksInternal()
}

// releaseAllLocksInternal 内部方法，释放所有锁
// 调用者必须持有mutex锁
func (m *LockManager) releaseAllLocksInternal() error {
	for jobID, lock := range m.locks {
		err := lock.Unlock()
		if err != nil {
			utils.Error("failed to release lock",
				zap.String("job_id", jobID),
				zap.Error(err))
		}
	}

	// 清空映射表
	m.locks = make(map[string]*JobLock)
	utils.Info("all locks released", zap.String("worker_id", m.workerID))
	return nil
}

// GetLockedJobIDs 获取所有已锁定的任务ID
func (m *LockManager) GetLockedJobIDs() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var jobIDs []string
	for jobID, lock := range m.locks {
		if lock.IsLocked() {
			jobIDs = append(jobIDs, jobID)
		}
	}
	return jobIDs
}

// cleanupLoop 定期清理无效的锁
func (m *LockManager) cleanupLoop() {
	for {
		select {
		case <-m.stopChan:
			return
		case <-m.cleanupTicker.C:
			m.cleanupInvalidLocks()
		}
	}
}

// cleanupInvalidLocks 清理无效或过期的锁
func (m *LockManager) cleanupInvalidLocks() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var toRemove []string

	for jobID, lock := range m.locks {
		utils.Info("checking lock validity",
			zap.String("job_id", jobID),
			zap.Bool("is_locked", lock.IsLocked()),
			zap.Int64("lease_id", int64(lock.GetLeaseID())))

		// 获取当前锁的租约ID
		leaseID := lock.GetLeaseID()
		
		// 检查锁是否仍然有效
		// 标记为无效的情况：
		// 1. 锁未锁定
		// 2. 租约ID为0（无效租约）
		// 3. 通过etcd查询确认租约已过期
		if !lock.IsLocked() || leaseID == 0 || !m.isLeaseValid(leaseID) {
			if !lock.IsLocked() {
				utils.Info("lock marked for removal - not locked",
					zap.String("job_id", jobID))
			} else {
				utils.Info("lock marked for removal - lease expired or invalid",
					zap.String("job_id", jobID),
					zap.Int64("lease_id", int64(leaseID)))
				// 如果租约过期，标记锁为未锁定状态
				lock.MarkAsUnlocked()
			}
			toRemove = append(toRemove, jobID)
		}
	}

	// 移除无效锁
	for _, jobID := range toRemove {
		lock := m.locks[jobID]
		utils.Info("attempting to unlock and remove lock",
			zap.String("job_id", jobID),
			zap.String("worker_id", m.workerID))

		// 尝试解锁，忽略可能的错误
		// 由于已经标记为未锁定，这里的解锁操作可能是空操作
		if lock.IsLocked() {
			if err := lock.Unlock(); err != nil {
				utils.Warn("error unlocking invalid lock during cleanup",
					zap.String("job_id", jobID),
					zap.Error(err))
			}
		}
		delete(m.locks, jobID)
		utils.Info("removed invalid lock during cleanup",
			zap.String("job_id", jobID),
			zap.String("worker_id", m.workerID))
	}

	if len(toRemove) > 0 {
		utils.Info("cleanup completed",
			zap.Int("removed_locks", len(toRemove)),
			zap.Int("remaining_locks", len(m.locks)))
	} else {
		utils.Info("cleanup completed - no locks removed",
			zap.Int("remaining_locks", len(m.locks)))
	}
}

// GetWorkerID 获取当前工作节点的ID
func (m *LockManager) GetWorkerID() string {
	return m.workerID
}

// TriggerCleanup 手动触发清理无效锁
func (m *LockManager) TriggerCleanup() {
	m.cleanupInvalidLocks()
}

// isLeaseValid 检查租约是否有效
func (m *LockManager) isLeaseValid(leaseID clientv3.LeaseID) bool {
	if leaseID == 0 {
		return false
	}

	// 为了更稳定的测试，增加日志
	isValid := m.etcdClient.IsLeaseValid(leaseID)
	utils.Info("checking lease validity",
		zap.Int64("lease_id", int64(leaseID)),
		zap.Bool("is_valid", isValid))
	return isValid
}
