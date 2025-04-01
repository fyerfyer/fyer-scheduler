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
	require.NoError(t, err, "First worker should not error when acquiring lock")
	require.True(t, acquired1, "First worker should successfully acquire lock")

	// 第二个worker尝试获取同一个锁
	acquired2, err := lock2.TryLock()
	assert.False(t, acquired2, "Second worker should fail to acquire already locked job")

	// 第一个worker释放锁
	err = lock1.Unlock()
	require.NoError(t, err, "First worker should not error when releasing lock")

	// 等待一秒确保锁释放完成
	time.Sleep(1 * time.Second)

	// 第二个worker应该能够获取锁
	acquired2, err = lock2.TryLock()
	require.NoError(t, err, "Second worker should not error when acquiring lock after release")
	assert.True(t, acquired2, "Second worker should successfully acquire lock after first worker released it")
}

func TestJobLockExpiration(t *testing.T) {
	// 创建锁管理器，设置非常短的TTL
	lockManager1 := createTestLockManager(t)
	defer lockManager1.Stop()

	// 创建一个短TTL的锁
	jobID := testutils.GenerateUniqueID("job")
	lock1 := lockManager1.CreateLock(jobID, joblock.WithTTL(1)) // 1秒TTL

	// 获取锁
	acquired1, err := lock1.TryLock()
	require.NoError(t, err, "Should not error when acquiring lock")
	require.True(t, acquired1, "Should successfully acquire lock")

	// 停止自动续期
	lock1.StopRenewal()

	// 等待锁过期，稍微多等几秒确保过期
	time.Sleep(3 * time.Second)

	// 创建另一个worker尝试获取锁
	lockManager2 := createTestLockManager(t)
	defer lockManager2.Stop()

	lock2 := lockManager2.CreateLock(jobID, joblock.WithTTL(5))
	acquired2, err := lock2.TryLock()
	require.NoError(t, err, "Second worker should not error when acquiring lock after expiration")
	assert.True(t, acquired2, "Second worker should successfully acquire lock after TTL expiration")
}

func TestJobLockConcurrent(t *testing.T) {
	// 创建一个etcd客户端供所有工作节点使用
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")
	defer etcdClient.Close()

	// 创建一个所有工作节点都会尝试锁定的公共作业ID
	jobID := testutils.GenerateUniqueID("job")

	// 并发工作节点的数量
	workerCount := 5

	// 创建WaitGroup用于同步
	var readyGroup sync.WaitGroup // 确保所有goroutine同时开始尝试
	var doneGroup sync.WaitGroup  // 等待所有goroutine完成

	// 用于所有goroutine开始尝试获取锁的启动信号
	startSignal := make(chan struct{})

	// 用于通知释放锁的信号
	releaseSignal := make(chan struct{})

	// 成功获取锁的计数器
	var successCount int
	successCountMutex := sync.Mutex{}

	// 成功获取锁的工作节点ID
	successWorkerIDs := make([]string, 0)

	// 在启动goroutine之前创建所有锁管理器
	lockManagers := make([]*joblock.LockManager, workerCount)
	workerIDs := make([]string, workerCount)

	for i := 0; i < workerCount; i++ {
		workerIDs[i] = fmt.Sprintf("worker-%d", i)
		lockManagers[i] = joblock.NewLockManager(etcdClient, workerIDs[i])
		err := lockManagers[i].Start()
		require.NoError(t, err, "Failed to start lock manager")
		defer lockManagers[i].Stop()
	}

	// 启动多个goroutine以模拟并发锁定尝试
	for i := 0; i < workerCount; i++ {
		workerID := workerIDs[i]
		lockManager := lockManagers[i]

		readyGroup.Add(1)
		doneGroup.Add(1)

		go func(wID string, lm *joblock.LockManager, idx int) {
			defer doneGroup.Done()

			// 创建锁
			lock := lm.CreateLock(jobID, joblock.WithTTL(5), joblock.WithMaxRetries(1))

			// 发出准备信号
			readyGroup.Done()

			// 等待启动信号
			<-startSignal

			// 尝试获取锁
			acquired, err := lock.TryLock()
			if err != nil {
				t.Logf("Worker %s failed to try lock: %v", wID, err)
				return
			}

			if acquired {
				successCountMutex.Lock()
				successCount++
				successWorkerIDs = append(successWorkerIDs, wID)
				t.Logf("Worker %s successfully acquired lock", wID)
				successCountMutex.Unlock()

				// 等待测试发出释放信号
				<-releaseSignal
				if err := lock.Unlock(); err != nil {
					t.Logf("Worker %s failed to release lock: %v", wID, err)
				}
			} else {
				t.Logf("Worker %s failed to acquire lock", wID)
			}
		}(workerID, lockManager, i)
	}

	// 给goroutine一些时间到达等待点
	readyGroup.Wait()

	// 发出信号让所有goroutine同时尝试获取锁
	close(startSignal)

	// 等待一段时间让所有工作节点尝试获取锁
	time.Sleep(1 * time.Second)

	// 验证只有一个工作节点成功获取了锁
	successCountMutex.Lock()
	assert.Equal(t, 1, successCount, "Only one worker should have successfully acquired the lock")
	assert.Len(t, successWorkerIDs, 1, "There should be exactly one successful worker ID")
	successCountMutex.Unlock()

	// 测试成功工作节点的GetLockedJobIDs
	t.Logf("Testing GetLockedJobIDs on successful worker")
	if len(successWorkerIDs) > 0 {
		successWorkerIdx := -1
		for i, wID := range workerIDs {
			if wID == successWorkerIDs[0] {
				successWorkerIdx = i
				break
			}
		}

		if successWorkerIdx >= 0 {
			lockedJobs := lockManagers[successWorkerIdx].GetLockedJobIDs()
			assert.Contains(t, lockedJobs, jobID, "Successful worker should have the job ID in its locked jobs list")
		}
	}

	// 现在通知工作节点释放锁
	close(releaseSignal)

	// 等待所有goroutine完成
	doneGroup.Wait()
}

func TestReleaseAllLocks(t *testing.T) {
	// 创建锁管理器
	lockManager := createTestLockManager(t)
	defer lockManager.Stop()

	// 创建并获取多个锁
	jobCount := 3
	jobIDs := make([]string, jobCount)
	locks := make([]*joblock.JobLock, jobCount)

	for i := 0; i < jobCount; i++ {
		jobIDs[i] = testutils.GenerateUniqueID("job")
		locks[i] = lockManager.CreateLock(jobIDs[i], joblock.WithTTL(5))

		acquired, err := locks[i].TryLock()
		require.NoError(t, err, "Should not error when acquiring lock")
		require.True(t, acquired, "Should successfully acquire lock")
	}

	// 验证所有锁都已经获取
	for i := 0; i < jobCount; i++ {
		managedLock, exists := lockManager.GetLock(jobIDs[i])
		assert.True(t, exists, "Lock manager should track the lock")
		assert.Equal(t, locks[i], managedLock, "Managed lock should be the same instance")
	}

	// 释放所有锁
	err := lockManager.ReleaseAllLocks()
	require.NoError(t, err, "Should not error when releasing all locks")

	// 验证所有锁都已经释放
	for i := 0; i < jobCount; i++ {
		_, exists := lockManager.GetLock(jobIDs[i])
		assert.False(t, exists, "Lock manager should not track the lock after releasing all")
	}
}

func TestLockManagerCleanup(t *testing.T) {
	// 创建锁管理器
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")

	workerID := testutils.GenerateUniqueID("worker")
	lockManager := joblock.NewLockManager(etcdClient, workerID)

	// 启动锁管理器
	err = lockManager.Start()
	require.NoError(t, err, "Failed to start lock manager")
	defer lockManager.Stop()

	// 创建一个具有短TTL的锁
	jobID := testutils.GenerateUniqueID("job")
	lock := lockManager.CreateLock(jobID, joblock.WithTTL(1))

	acquired, err := lock.TryLock()
	require.NoError(t, err, "Should not error when acquiring lock")
	require.True(t, acquired, "Should successfully acquire lock")

	// 验证锁最初存在
	_, exists := lockManager.GetLock(jobID)
	assert.True(t, exists, "Lock should exist initially")

	// 停止续约并标记为已解锁以确保清理正常工作
	lock.StopRenewal()
	lock.MarkAsUnlocked() // 添加此行以显式标记为已解锁

	// 等待一会儿
	time.Sleep(2 * time.Second)

	// 手动触发清理
	lockManager.TriggerCleanup()

	// 验证清理是否正常工作
	_, exists = lockManager.GetLock(jobID)
	assert.False(t, exists, "Expired lock should be cleaned up")
}

func TestLockManagerGetWorkerID(t *testing.T) {
	// 创建锁管理器
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")

	workerID := testutils.GenerateUniqueID("worker")
	lockManager := joblock.NewLockManager(etcdClient, workerID)

	// 测试GetWorkerID方法
	assert.Equal(t, workerID, lockManager.GetWorkerID(), "GetWorkerID should return the correct worker ID")
}

func TestLockManagerStartStop(t *testing.T) {
	// 创建锁管理器
	etcdClient, err := testutils.TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")

	workerID := testutils.GenerateUniqueID("worker")
	lockManager := joblock.NewLockManager(etcdClient, workerID)

	// 测试启动
	err = lockManager.Start()
	require.NoError(t, err, "Should not error when starting lock manager")

	// 测试重复启动
	err = lockManager.Start()
	assert.NoError(t, err, "Should not error when starting lock manager that's already running")

	// 测试停止
	err = lockManager.Stop()
	require.NoError(t, err, "Should not error when stopping lock manager")

	// 测试重复停止
	err = lockManager.Stop()
	assert.NoError(t, err, "Should not error when stopping lock manager that's already stopped")
}
