package worker

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/worker/joblock"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 创建测试锁管理器的辅助函数
func createTestLockManager(t *testing.T) *joblock.LockManager {
	// 创建etcd客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")

	// 创建锁管理器
	workerID := testutils.GenerateUniqueID("worker")
	lockManager := joblock.NewLockManager(etcdClient, workerID)

	// 启动锁管理器
	err = lockManager.Start()
	require.NoError(t, err, "Failed to start lock manager")

	return lockManager
}

func TestJobLockCreate(t *testing.T) {
	// 创建锁管理器
	lockManager := createTestLockManager(t)
	defer lockManager.Stop()

	// 创建作业锁
	jobID := testutils.GenerateUniqueID("job")
	lock := lockManager.CreateLock(jobID, joblock.WithTTL(5))

	// 验证锁已创建并配置正确
	assert.NotNil(t, lock, "Lock should be created")
	assert.False(t, lock.IsLocked(), "Lock should not be locked initially")
}

func TestJobLockAcquire(t *testing.T) {
	// 创建锁管理器
	lockManager := createTestLockManager(t)
	defer lockManager.Stop()

	// 创建作业锁
	jobID := testutils.GenerateUniqueID("job")
	lock := lockManager.CreateLock(jobID, joblock.WithTTL(5))

	// 尝试获取锁
	acquired, err := lock.TryLock()
	require.NoError(t, err, "Should not error when acquiring lock")
	assert.True(t, acquired, "Should successfully acquire lock")
	assert.True(t, lock.IsLocked(), "Lock should be locked after acquisition")

	// 检查锁管理器是否跟踪该锁
	managedLock, exists := lockManager.GetLock(jobID)
	assert.True(t, exists, "Lock manager should track the lock")
	assert.Equal(t, lock, managedLock, "Managed lock should be the same instance")
}

func TestJobLockRelease(t *testing.T) {
	// 创建锁管理器
	lockManager := createTestLockManager(t)
	defer lockManager.Stop()

	// 创建并获取作业锁
	jobID := testutils.GenerateUniqueID("job")
	lock := lockManager.CreateLock(jobID, joblock.WithTTL(5))

	acquired, err := lock.TryLock()
	require.NoError(t, err, "Should not error when acquiring lock")
	require.True(t, acquired, "Should successfully acquire lock")

	// 释放锁
	err = lockManager.ReleaseLock(jobID)
	require.NoError(t, err, "Should not error when releasing lock")
	assert.False(t, lock.IsLocked(), "Lock should not be locked after release")

	// 验证锁管理器不再跟踪该锁
	_, exists := lockManager.GetLock(jobID)
	assert.False(t, exists, "Lock manager should not track the lock after release")
}

func TestJobLockContention(t *testing.T) {
	// 创建两个锁管理器模拟两个不同的worker
	lockManager1 := createTestLockManager(t)
	defer lockManager1.Stop()

	lockManager2 := createTestLockManager(t)
	defer lockManager2.Stop()

	// 创建相同jobID的两个锁实例
	jobID := testutils.GenerateUniqueID("job")
	lock1 := lockManager1.CreateLock(jobID, joblock.WithTTL(5))
	lock2 := lockManager2.CreateLock(jobID, joblock.WithTTL(5))

	// 第一个worker获取锁
	acquired1, err := lock1.TryLock()
	require.NoError(t, err, "Should not error when first worker acquires lock")
	assert.True(t, acquired1, "First worker should acquire lock")

	// 第二个worker尝试获取同一个锁
	acquired2, err := lock2.TryLock()
	assert.Error(t, err, "Should error when second worker tries to acquire an already held lock")
	assert.False(t, acquired2, "Second worker should fail to acquire lock")

	// 第一个worker释放锁
	err = lock1.Unlock()
	require.NoError(t, err, "Should not error when releasing lock")

	// 第二个worker应该能够获取锁
	acquired2, err = lock2.TryLock()
	require.NoError(t, err, "Should not error when second worker tries again")
	assert.True(t, acquired2, "Second worker should now acquire lock")
}

func TestJobLockExpiration(t *testing.T) {
	// 创建锁管理器，设置非常短的TTL
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")

	workerID := testutils.GenerateUniqueID("worker")
	lockManager := joblock.NewLockManager(etcdClient, workerID)
	err = lockManager.Start()
	require.NoError(t, err, "Failed to start lock manager")
	defer lockManager.Stop()

	// 创建一个短TTL的锁
	jobID := testutils.GenerateUniqueID("job")
	lock := lockManager.CreateLock(jobID, joblock.WithTTL(1)) // 1秒TTL

	// 获取锁
	acquired, err := lock.TryLock()
	require.NoError(t, err, "Should not error when acquiring lock")
	require.True(t, acquired, "Should successfully acquire lock")

	// 停止自动续期
	lock.StopRenewal()

	// 等待锁过期，稍微多等几秒确保过期
	time.Sleep(3 * time.Second)

	// 创建另一个worker尝试获取锁
	lockManager2 := joblock.NewLockManager(etcdClient, workerID+"-2")
	err = lockManager2.Start()
	require.NoError(t, err, "Failed to start second lock manager")
	defer lockManager2.Stop()

	lock2 := lockManager2.CreateLock(jobID)
	acquired2, err := lock2.TryLock()
	require.NoError(t, err, "Should not error when second worker tries to acquire lock")
	assert.True(t, acquired2, "Second worker should acquire lock after expiration")
}

func TestJobLockConcurrent(t *testing.T) {
	// 创建一个用于所有worker的etcd客户端
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")

	// 创建一个所有worker都会尝试锁定的公共job ID
	jobID := testutils.GenerateUniqueID("job")

	// 并发worker的数量
	workerCount := 5

	// 创建用于同步的WaitGroup
	var startWg sync.WaitGroup // 用于确保所有goroutine同时开始尝试
	var doneWg sync.WaitGroup  // 用于等待所有goroutine完成

	startWg.Add(1) // 这个计数器会减少一次，以同时释放所有goroutine
	doneWg.Add(workerCount)

	successCount := 0
	var mutex sync.Mutex
	var firstWorkerID string

	// 在启动goroutine之前创建所有锁管理器
	lockManagers := make([]*joblock.LockManager, workerCount)
	for i := 0; i < workerCount; i++ {
		workerID := testutils.GenerateUniqueID(fmt.Sprintf("worker-%d", i))
		lockManagers[i] = joblock.NewLockManager(etcdClient, workerID)
		err := lockManagers[i].Start()
		require.NoError(t, err, "Failed to start lock manager")
		defer lockManagers[i].Stop() // 确保清理
	}

	// 启动多个goroutine以模拟并发锁定尝试
	for i := 0; i < workerCount; i++ {
		go func(workerIndex int) {
			defer doneWg.Done()

			// 为当前worker创建一个锁
			lockManager := lockManagers[workerIndex]
			lock := lockManager.CreateLock(jobID,
				joblock.WithTTL(5),
				joblock.WithMaxRetries(3))

			// 等待信号开始
			startWg.Wait()

			// 尝试获取锁
			acquired, err := lock.TryLock()
			if err == nil && acquired {
				// 记录成功获取锁的情况
				mutex.Lock()
				if firstWorkerID == "" {
					// 只记录第一个成功获取锁的worker
					firstWorkerID = lockManager.GetWorkerID()
					successCount++
					t.Logf("Worker %s was first to acquire the lock", firstWorkerID)
				}
				mutex.Unlock()

				// 短暂持有锁以确保计数完成
				time.Sleep(100 * time.Millisecond)

				// 释放锁
				err = lockManager.ReleaseLock(jobID)
				if err != nil {
					t.Logf("Error releasing lock: %v", err)
				}
			}
		}(i)
	}

	// 给goroutine一些时间到达等待点
	time.Sleep(100 * time.Millisecond)

	// 发出信号让所有goroutine同时尝试获取锁
	startWg.Done()

	// 等待所有goroutine完成
	doneWg.Wait()

	// 验证只有一个worker应该成功获取锁
	assert.Equal(t, 1, successCount, "Only one worker should successfully acquire the lock")
}
