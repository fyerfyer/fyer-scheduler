package repo

import (
	"fmt"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// WorkerRepo 提供对Worker节点数据的访问
type WorkerRepo struct {
	EtcdClient      *utils.EtcdClient
	MongoClient     *utils.MongoDBClient
	workersCollName string
}

// NewWorkerRepo 创建一个新的Worker仓库
func NewWorkerRepo(etcdClient *utils.EtcdClient, mongoClient *utils.MongoDBClient) *WorkerRepo {
	return &WorkerRepo{
		EtcdClient:      etcdClient,
		MongoClient:     mongoClient,
		workersCollName: "workers",
	}
}

// Register 注册Worker到etcd，使其在集群中可发现
func (r *WorkerRepo) Register(worker *models.Worker, ttl int64) (clientv3.LeaseID, error) {
	// 序列化Worker
	workerJSON, err := worker.ToJSON()
	if err != nil {
		return 0, fmt.Errorf("failed to serialize worker: %w", err)
	}

	// 使用租约注册，以便可以通过心跳检测Worker活跃状态
	leaseID, err := r.EtcdClient.PutWithLease(worker.Key(), workerJSON, ttl)
	if err != nil {
		return 0, fmt.Errorf("failed to register worker: %w", err)
	}

	// 在MongoDB中保存Worker记录（用于历史记录和查询）
	filter := bson.M{"id": worker.ID}
	update := bson.M{"$set": worker}
	opts := options.Update().SetUpsert(true)

	_, err = r.MongoClient.UpdateOne(r.workersCollName, filter, update, opts)
	if err != nil {
		// 如果MongoDB更新失败，回滚etcd变更以保持一致性
		rErr := r.EtcdClient.Delete(worker.Key())
		if rErr != nil {
			utils.Error("failed to rollback etcd after MongoDB error",
				zap.String("worker_id", worker.ID),
				zap.Error(rErr))
		}
		return 0, fmt.Errorf("failed to save worker to MongoDB: %w", err)
	}

	return leaseID, nil
}

// Heartbeat 更新Worker的心跳时间
func (r *WorkerRepo) Heartbeat(worker *models.Worker) error {
	// 更新心跳时间
	worker.UpdateHeartbeat()

	// 重新序列化并更新etcd中的数据
	workerJSON, err := worker.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize worker: %w", err)
	}

	err = r.EtcdClient.Put(worker.Key(), workerJSON)
	if err != nil {
		return fmt.Errorf("failed to update worker heartbeat: %w", err)
	}

	// 更新MongoDB中的心跳记录
	filter := bson.M{"id": worker.ID}
	update := bson.M{
		"$set": bson.M{
			"last_heartbeat": worker.LastHeartbeat,
			"update_time":    worker.UpdateTime,
			"status":         worker.Status,
			"memory_free":    worker.MemoryFree,
			"disk_free":      worker.DiskFree,
			"load_avg":       worker.LoadAvg,
		},
	}

	_, err = r.MongoClient.UpdateOne(r.workersCollName, filter, update)
	if err != nil {
		utils.Error("failed to update worker heartbeat in MongoDB, data inconsistency may occur",
			zap.String("worker_id", worker.ID),
			zap.Error(err))
		// 在生产环境中，可能需要更严格的处理，例如回滚etcd更改或触发一致性修复
		// 但对于心跳这种高频操作，可以容忍短暂的不一致
	}

	return nil
}

// GetByID 根据ID获取Worker
func (r *WorkerRepo) GetByID(workerID string) (*models.Worker, error) {
	// 优先从etcd获取最新数据
	workerKey := constants.WorkerPrefix + workerID
	workerJSON, err := r.EtcdClient.Get(workerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get worker from etcd: %w", err)
	}

	// 如果etcd中存在，直接返回
	if workerJSON != "" {
		return models.WorkerFromJSON(workerJSON)
	}

	// 如果etcd中不存在，尝试从MongoDB获取
	worker := &models.Worker{}
	err = r.MongoClient.FindOne(r.workersCollName, bson.M{"id": workerID}, worker)
	if err != nil {
		return nil, fmt.Errorf("worker not found: %w", err)
	}

	return worker, nil
}

// ListAll 获取所有Worker列表
func (r *WorkerRepo) ListAll() ([]*models.Worker, error) {
	var workers []*models.Worker

	// 从etcd获取所有Worker
	kvs, err := r.EtcdClient.GetWithPrefix(constants.WorkerPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list workers from etcd: %w", err)
	}

	for _, v := range kvs {
		worker, err := models.WorkerFromJSON(v)
		if err != nil {
			utils.Warn("failed to parse worker from etcd", zap.Error(err))
			continue
		}
		workers = append(workers, worker)
	}

	return workers, nil
}

// ListByStatus 根据状态获取Worker列表
func (r *WorkerRepo) ListByStatus(status string, page, pageSize int64) ([]*models.Worker, int64, error) {
	var workers []*models.Worker

	// 构建查询
	query := utils.NewQuery().
		Where("status", status).
		SetPage(page, pageSize).
		OrderBy("last_heartbeat", false)

	// 执行查询
	err := r.MongoClient.FindWithOptions(
		r.workersCollName,
		query.GetFilter(),
		&workers,
		query.GetOptions(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query workers by status: %w", err)
	}

	// 获取总记录数
	total, err := r.MongoClient.Count(r.workersCollName, query.GetFilter())
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count workers by status: %w", err)
	}

	return workers, total, nil
}

// ListActive 获取所有活跃Worker
func (r *WorkerRepo) ListActive() ([]*models.Worker, error) {
	var activeWorkers []*models.Worker

	// 获取所有Worker
	workers, err := r.ListAll()
	if err != nil {
		return nil, err
	}

	// 过滤出活跃的Worker
	for _, worker := range workers {
		if worker.IsActive() {
			activeWorkers = append(activeWorkers, worker)
		}
	}

	return activeWorkers, nil
}

// EnableWorker 启用Worker
func (r *WorkerRepo) EnableWorker(workerID string) error {
	// 获取Worker
	worker, err := r.GetByID(workerID)
	if err != nil {
		return fmt.Errorf("failed to get worker: %w", err)
	}

	// 更新状态
	worker.SetStatus(constants.WorkerStatusOnline)

	// 开始一致性更新
	workerJSON, err := worker.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize worker: %w", err)
	}

	// 第一步：更新etcd
	err = r.EtcdClient.Put(worker.Key(), workerJSON)
	if err != nil {
		return fmt.Errorf("failed to enable worker in etcd: %w", err)
	}

	// 第二步：更新MongoDB
	filter := bson.M{"id": worker.ID}
	update := bson.M{
		"$set": bson.M{
			"status":      worker.Status,
			"update_time": worker.UpdateTime,
		},
	}

	_, err = r.MongoClient.UpdateOne(r.workersCollName, filter, update)
	if err != nil {
		// MongoDB更新失败，尝试回滚etcd更改
		worker.SetStatus(constants.WorkerStatusOffline) // 恢复原状态
		rollbackJSON, _ := worker.ToJSON()
		rErr := r.EtcdClient.Put(worker.Key(), rollbackJSON)
		if rErr != nil {
			utils.Error("failed to rollback etcd change after MongoDB error",
				zap.String("worker_id", worker.ID),
				zap.Error(rErr))
		}
		return fmt.Errorf("failed to enable worker in MongoDB: %w", err)
	}

	utils.Info("worker enabled successfully", zap.String("worker_id", workerID))
	return nil
}

// DisableWorker 禁用Worker
func (r *WorkerRepo) DisableWorker(workerID string) error {
	// 获取Worker
	worker, err := r.GetByID(workerID)
	if err != nil {
		return fmt.Errorf("failed to get worker: %w", err)
	}

	// 更新状态
	worker.SetStatus(constants.WorkerStatusDisabled)

	// 保存到etcd
	workerJSON, err := worker.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize worker: %w", err)
	}

	err = r.EtcdClient.Put(worker.Key(), workerJSON)
	if err != nil {
		return fmt.Errorf("failed to disable worker: %w", err)
	}

	// 更新MongoDB
	filter := bson.M{"id": worker.ID}
	update := bson.M{
		"$set": bson.M{
			"status":      worker.Status,
			"update_time": worker.UpdateTime,
		},
	}

	_, err = r.MongoClient.UpdateOne(r.workersCollName, filter, update)
	if err != nil {
		utils.Warn("failed to update worker status in MongoDB",
			zap.String("worker_id", worker.ID),
			zap.Error(err))
	}

	return nil
}

// Delete 删除Worker
func (r *WorkerRepo) Delete(workerID string) error {
	workerKey := constants.WorkerPrefix + workerID

	// 从etcd删除
	err := r.EtcdClient.Delete(workerKey)
	if err != nil {
		return fmt.Errorf("failed to delete worker from etcd: %w", err)
	}

	// 从MongoDB删除 (或标记为已删除)
	filter := bson.M{"id": workerID}
	update := bson.M{
		"$set": bson.M{
			"status":      constants.WorkerStatusOffline,
			"update_time": time.Now(),
		},
	}

	_, err = r.MongoClient.UpdateOne(r.workersCollName, filter, update)
	if err != nil {
		utils.Warn("failed to mark worker as deleted in MongoDB",
			zap.String("worker_id", workerID),
			zap.Error(err))
	}

	return nil
}

// UpdateWorkerResources 更新Worker资源使用情况
func (r *WorkerRepo) UpdateWorkerResources(worker *models.Worker) error {
	// 保存到etcd
	workerJSON, err := worker.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize worker: %w", err)
	}

	err = r.EtcdClient.Put(worker.Key(), workerJSON)
	if err != nil {
		return fmt.Errorf("failed to update worker resources: %w", err)
	}

	// 更新MongoDB中的资源记录
	filter := bson.M{"id": worker.ID}
	update := bson.M{
		"$set": bson.M{
			"memory_total": worker.MemoryTotal,
			"memory_free":  worker.MemoryFree,
			"disk_total":   worker.DiskTotal,
			"disk_free":    worker.DiskFree,
			"load_avg":     worker.LoadAvg,
			"cpu_cores":    worker.CPUCores,
			"update_time":  worker.UpdateTime,
		},
	}

	_, err = r.MongoClient.UpdateOne(r.workersCollName, filter, update)
	if err != nil {
		utils.Warn("failed to update worker resources in MongoDB",
			zap.String("worker_id", worker.ID),
			zap.Error(err))
	}

	return nil
}

// UpdateRunningJobs 更新Worker正在运行的任务列表
func (r *WorkerRepo) UpdateRunningJobs(worker *models.Worker) error {
	// 保存到etcd
	workerJSON, err := worker.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize worker: %w", err)
	}

	err = r.EtcdClient.Put(worker.Key(), workerJSON)
	if err != nil {
		return fmt.Errorf("failed to update worker running jobs: %w", err)
	}

	// 更新MongoDB中的任务记录
	filter := bson.M{"id": worker.ID}
	update := bson.M{
		"$set": bson.M{
			"running_jobs":   worker.RunningJobs,
			"completed_jobs": worker.CompletedJobs,
			"failed_jobs":    worker.FailedJobs,
			"update_time":    worker.UpdateTime,
		},
	}

	_, err = r.MongoClient.UpdateOne(r.workersCollName, filter, update)
	if err != nil {
		utils.Warn("failed to update worker jobs in MongoDB",
			zap.String("worker_id", worker.ID),
			zap.Error(err))
	}

	return nil
}

// GetEtcdClient 获取etcd客户端
func (r *WorkerRepo) GetEtcdClient() *utils.EtcdClient {
	return r.EtcdClient
}

func (r *WorkerRepo) CheckConsistency(workerID string) error {
	// 从两个存储获取数据
	worker, err := r.GetByID(workerID)
	if err != nil {
		return fmt.Errorf("failed to get worker: %w", err)
	}

	// 比较etcd和MongoDB的数据
	etcdJSON, err := r.EtcdClient.Get(worker.Key())
	if err != nil {
		return fmt.Errorf("failed to get worker from etcd: %w", err)
	}

	// 如果etcd中不存在此Worker
	if etcdJSON == "" {
		// 要么MongoDB中也不应该存在，要么我们应该将其添加回etcd
		if worker.Status != constants.WorkerStatusOffline {
			// 恢复到etcd
			workerJSON, err := worker.ToJSON()
			if err != nil {
				return fmt.Errorf("failed to serialize worker: %w", err)
			}
			err = r.EtcdClient.Put(worker.Key(), workerJSON)
			if err != nil {
				return fmt.Errorf("failed to restore worker to etcd: %w", err)
			}
			utils.Warn("restored worker to etcd during consistency check",
				zap.String("worker_id", worker.ID))
		}
		return nil
	}

	etcdWorker, err := models.WorkerFromJSON(etcdJSON)
	if err != nil {
		return fmt.Errorf("failed to parse worker from etcd: %w", err)
	}

	// 检查数据是否一致
	if worker.Status != etcdWorker.Status {
		// 使用etcd中的数据（它是最新的）来更新MongoDB
		filter := bson.M{"id": worker.ID}
		update := bson.M{"$set": bson.M{"status": etcdWorker.Status}}
		_, err = r.MongoClient.UpdateOne(r.workersCollName, filter, update)
		if err != nil {
			return fmt.Errorf("failed to update worker status in MongoDB: %w", err)
		}
		utils.Warn("fixed worker status inconsistency",
			zap.String("worker_id", worker.ID),
			zap.String("mongo_status", worker.Status),
			zap.String("etcd_status", etcdWorker.Status))
	}

	return nil
}
