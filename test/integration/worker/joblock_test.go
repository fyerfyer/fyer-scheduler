package worker

import (
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
	err = lock.Unlock()
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
	require.NoError(t, err, "Should not error when second worker tries to acquire lock")
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

	// 获取锁后停止自动续期
	acquired, err := lock.TryLock()
	require.NoError(t, err, "Should not error when acquiring lock")
	require.True(t, acquired, "Should successfully acquire lock")

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
	// 创建锁管理器
	lockManager := createTestLockManager(t)
	defer lockManager.Stop()

	// 创建多个并发worker模拟竞争
	workerCount := 5
	jobID := testutils.GenerateUniqueID("job")
	var wg sync.WaitGroup
	wg.Add(workerCount)

	successCount := 0
	var mutex sync.Mutex

	// 开启多个goroutine同时尝试获取锁
	for i := 0; i < workerCount; i++ {
		go func(workerIndex int) {
			defer wg.Done()

			// 每个worker创建自己的锁
			lock := lockManager.CreateLock(jobID,
				joblock.WithTTL(5),
				joblock.WithMaxRetries(3))

			// 尝试获取锁
			acquired, _ := lock.TryLock()
			if acquired {
				// 记录成功获取锁的worker数量
				mutex.Lock()
				successCount++
				mutex.Unlock()

				// 持有锁一段时间
				time.Sleep(100 * time.Millisecond)

				// 释放锁
				lock.Unlock()
			}
		}(i)
	}

	// 等待所有goroutine完成
	wg.Wait()

	// 验证只有一个worker能成功获取锁
	assert.Equal(t, 1, successCount, "Only one worker should successfully acquire the lock")
}
