package testutils

import (
	"context"
	"fmt"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/jobmgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/workermgr"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/register"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson"
	"math/rand"
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/config"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestEtcdClient 创建测试用的etcd客户端
func TestEtcdClient() (*utils.EtcdClient, error) {
	cfg := &config.EtcdConfig{
		Endpoints: []string{"localhost:2379"},
	}
	return utils.NewEtcdClient(cfg)
}

// TestMongoDBClient 创建测试用的MongoDB客户端
func TestMongoDBClient() (*utils.MongoDBClient, error) {
	cfg := &config.MongoDBConfig{
		URI:      "mongodb://localhost:27017",
		Database: "fyer-scheduler-test",
	}
	return utils.NewMongoDBClient(cfg)
}

// CleanEtcdPrefix 清理etcd中指定前缀的所有键
func CleanEtcdPrefix(client *utils.EtcdClient, prefix string) error {
	return client.DeleteWithPrefix(prefix)
}

// GenerateUniqueID 生成唯一ID用于测试
func GenerateUniqueID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.New().String()[:8])
}

// RandomInt 生成随机整数 (min <= x < max)
func RandomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

// RandomString 生成指定长度的随机字符串
func RandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)
	for i := range result {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}

// WaitForCondition 等待条件满足或超时
func WaitForCondition(checkFn func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if checkFn() {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// SetupTestEnvironment 设置测试环境
func SetupTestEnvironment(t *testing.T) (*utils.EtcdClient, *utils.MongoDBClient) {
	// 初始化随机种子
	rand.Seed(time.Now().UnixNano())

	// 创建客户端
	etcdClient, err := TestEtcdClient()
	require.NoError(t, err, "Failed to create etcd client")

	mongoClient, err := TestMongoDBClient()
	if err != nil {
		etcdClient.Close()
		require.NoError(t, err, "Failed to create MongoDB client")
	}

	// 清理测试数据
	CleanTestData(t, etcdClient)
	AddMongoCleanup(t, mongoClient)

	return etcdClient, mongoClient
}

// AddMongoCleanup 添加MongoDB清理到CleanTestData
func AddMongoCleanup(t *testing.T, mongoClient *utils.MongoDBClient) {
	// 清理作业日志集合
	_, err := mongoClient.DeleteMany("job_logs", bson.M{})
	require.NoError(t, err, "Failed to clean logs from MongoDB")

	// 根据需要清理其他集合
	_, err = mongoClient.DeleteMany("jobs", bson.M{})
	require.NoError(t, err, "Failed to clean jobs from MongoDB")

	_, err = mongoClient.DeleteMany("workers", bson.M{})
	require.NoError(t, err, "Failed to clean workers from MongoDB")
}

// CleanTestData 清理测试数据
func CleanTestData(t *testing.T, etcdClient *utils.EtcdClient) {
	err := CleanEtcdPrefix(etcdClient, constants.JobPrefix)
	require.NoError(t, err, "Failed to clean jobs from etcd")

	err = CleanEtcdPrefix(etcdClient, constants.WorkerPrefix)
	require.NoError(t, err, "Failed to clean workers from etcd")

	err = CleanEtcdPrefix(etcdClient, constants.LockPrefix)
	require.NoError(t, err, "Failed to clean locks from etcd")
}

// TeardownTestEnvironment 清理测试环境
func TeardownTestEnvironment(t *testing.T, etcdClient *utils.EtcdClient, mongoClient *utils.MongoDBClient) {
	if etcdClient != nil {
		CleanTestData(t, etcdClient)
		etcdClient.Close()
	}

	if mongoClient != nil {
		AddMongoCleanup(t, mongoClient)
		mongoClient.Close()
	}
}

// WaitWithTimeout 等待某个操作完成，带超时
func WaitWithTimeout(operation func(ctx context.Context) error, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- operation(ctx)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return fmt.Errorf("operation timed out after %v", timeout)
	}
}

func CreateTestWorker(etcdClient *utils.EtcdClient, workerID string, labels map[string]string) (*register.WorkerRegister, error) {
	registerOpts := register.RegisterOptions{
		NodeID:           workerID,
		Hostname:         "test-host-" + workerID,
		IP:               "127.0.0.1",
		HeartbeatTimeout: 5,
		HeartbeatTTL:     10,
		Labels:           labels,
		ResourceInterval: 1 * time.Second,
	}

	workerRegister, err := register.NewWorkerRegister(etcdClient, registerOpts)
	if err != nil {
		return nil, err
	}

	err = workerRegister.Start()
	if err != nil {
		return nil, err
	}

	return workerRegister, nil
}

// CreateTestJobRepo 创建用于测试的作业仓库
func CreateTestJobRepo(t *testing.T, etcdClient *utils.EtcdClient, mongoClient *utils.MongoDBClient) repo.IJobRepo {
	return repo.NewJobRepo(etcdClient, mongoClient)
}

// CreateTestWorkerRepo 创建用于测试的worker仓库
func CreateTestWorkerRepo(t *testing.T, etcdClient *utils.EtcdClient, mongoClient *utils.MongoDBClient) repo.IWorkerRepo {
	return repo.NewWorkerRepo(etcdClient, mongoClient)
}

// CreateTestLogRepo 创建用于测试的日志仓库
func CreateTestLogRepo(t *testing.T, mongoClient *utils.MongoDBClient) repo.ILogRepo {
	return repo.NewLogRepo(mongoClient)
}

// CreateTestJobManager 创建用于测试的作业管理器
func CreateTestJobManager(t *testing.T, jobRepo repo.IJobRepo, logRepo repo.ILogRepo, workerRepo repo.IWorkerRepo, etcdClient *utils.EtcdClient) jobmgr.JobManager {
	manager := jobmgr.NewJobManager(jobRepo, logRepo, workerRepo, etcdClient)
	err := manager.Start()
	require.NoError(t, err, "Failed to start job manager")

	// 注册清理函数，在测试完成时停止管理器
	t.Cleanup(func() {
		manager.Stop()
	})

	return manager
}

// CreateTestWorkerSelector 创建用于测试的worker选择器
func CreateTestWorkerSelector(t *testing.T, workerRepo repo.IWorkerRepo) jobmgr.IWorkerSelector {
	return jobmgr.NewWorkerSelector(workerRepo, jobmgr.LeastJobsStrategy)
}

// CreateTestScheduler 创建用于测试的调度器
func CreateTestScheduler(t *testing.T, jobMgr jobmgr.JobManager, workerRepo repo.IWorkerRepo) jobmgr.IScheduler {
	scheduler := jobmgr.NewScheduler(jobMgr, workerRepo, 1*time.Second)
	err := scheduler.Start()
	require.NoError(t, err, "Failed to start scheduler")

	// 注册清理函数，在测试完成时停止调度器
	t.Cleanup(func() {
		scheduler.Stop()
	})

	return scheduler
}

// CreateTestJobOperations 创建用于测试的作业操作
func CreateTestJobOperations(t *testing.T, jobMgr jobmgr.JobManager, workerRepo repo.IWorkerRepo, logRepo repo.ILogRepo) *jobmgr.JobOperations {
	// 创建依赖项
	workerSelector := CreateTestWorkerSelector(t, workerRepo)
	scheduler := CreateTestScheduler(t, jobMgr, workerRepo)

	// 创建作业操作
	return jobmgr.NewJobOperations(jobMgr, workerRepo, logRepo, workerSelector, scheduler)
}

func MockAny() interface{} {
	return mock.Anything
}

// CreateTestWorkerManager 创建用于测试的WorkerManager
func CreateTestWorkerManager(t *testing.T, workerRepo repo.IWorkerRepo, etcdClient *utils.EtcdClient) workermgr.WorkerManager {
	manager := workermgr.NewWorkerManager(workerRepo, 10*time.Second, etcdClient)
	err := manager.Start()
	require.NoError(t, err, "Failed to start worker manager")

	// 注册清理函数
	t.Cleanup(func() {
		manager.Stop()
	})

	return manager
}
