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
	etcdClient      *utils.EtcdClient
	mongoClient     *utils.MongoDBClient
	workersCollName string
}

// NewWorkerRepo 创建一个新的Worker仓库
func NewWorkerRepo(etcdClient *utils.EtcdClient, mongoClient *utils.MongoDBClient) *WorkerRepo {
	return &WorkerRepo{
		etcdClient:      etcdClient,
		mongoClient:     mongoClient,
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
	leaseID, err := r.etcdClient.PutWithLease(worker.Key(), workerJSON, ttl)
	if err != nil {
		return 0, fmt.Errorf("failed to register worker: %w", err)
	}

	// 在MongoDB中保存Worker记录（用于历史记录和查询）
	filter := bson.M{"id": worker.ID}
	update := bson.M{"$set": worker}
	opts := options.Update().SetUpsert(true)

	_, err = r.mongoClient.UpdateOne(r.workersCollName, filter, update, opts)
	if err != nil {
		utils.Warn("failed to save worker to MongoDB",
			zap.String("worker_id", worker.ID),
			zap.Error(err))
		// 不因MongoDB失败而阻止Worker注册，仅记录错误
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

	err = r.etcdClient.Put(worker.Key(), workerJSON)
	if err != nil {
		return fmt.Errorf("failed to update worker heartbeat: %w", err)
	}

	// 可选：更新MongoDB中的心跳记录
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

	_, err = r.mongoClient.UpdateOne(r.workersCollName, filter, update)
	if err != nil {
		utils.Warn("failed to update worker heartbeat in MongoDB",
			zap.String("worker_id", worker.ID),
			zap.Error(err))
		// 不因MongoDB失败而阻止Worker心跳正常工作
	}

	return nil
}

// GetByID 根据ID获取Worker
func (r *WorkerRepo) GetByID(workerID string) (*models.Worker, error) {
	// 优先从etcd获取最新数据
	workerKey := constants.WorkerPrefix + workerID
	workerJSON, err := r.etcdClient.Get(workerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get worker from etcd: %w", err)
	}

	// 如果etcd中存在，直接返回
	if workerJSON != "" {
		return models.WorkerFromJSON(workerJSON)
	}

	// 如果etcd中不存在，尝试从MongoDB获取
	worker := &models.Worker{}
	err = r.mongoClient.FindOne(r.workersCollName, bson.M{"id": workerID}, worker)
	if err != nil {
		return nil, fmt.Errorf("worker not found: %w", err)
	}

	return worker, nil
}

// ListAll 获取所有Worker列表
func (r *WorkerRepo) ListAll() ([]*models.Worker, error) {
	var workers []*models.Worker

	// 从etcd获取所有Worker
	kvs, err := r.etcdClient.GetWithPrefix(constants.WorkerPrefix)
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
	err := r.mongoClient.FindWithOptions(
		r.workersCollName,
		query.GetFilter(),
		&workers,
		query.GetOptions(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query workers by status: %w", err)
	}

	// 获取总记录数
	total, err := r.mongoClient.Count(r.workersCollName, query.GetFilter())
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

	// 保存到etcd
	workerJSON, err := worker.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize worker: %w", err)
	}

	err = r.etcdClient.Put(worker.Key(), workerJSON)
	if err != nil {
		return fmt.Errorf("failed to enable worker: %w", err)
	}

	// 更新MongoDB
	filter := bson.M{"id": worker.ID}
	update := bson.M{
		"$set": bson.M{
			"status":      worker.Status,
			"update_time": worker.UpdateTime,
		},
	}

	_, err = r.mongoClient.UpdateOne(r.workersCollName, filter, update)
	if err != nil {
		utils.Warn("failed to update worker status in MongoDB",
			zap.String("worker_id", worker.ID),
			zap.Error(err))
	}

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

	err = r.etcdClient.Put(worker.Key(), workerJSON)
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

	_, err = r.mongoClient.UpdateOne(r.workersCollName, filter, update)
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
	err := r.etcdClient.Delete(workerKey)
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

	_, err = r.mongoClient.UpdateOne(r.workersCollName, filter, update)
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

	err = r.etcdClient.Put(worker.Key(), workerJSON)
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

	_, err = r.mongoClient.UpdateOne(r.workersCollName, filter, update)
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

	err = r.etcdClient.Put(worker.Key(), workerJSON)
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

	_, err = r.mongoClient.UpdateOne(r.workersCollName, filter, update)
	if err != nil {
		utils.Warn("failed to update worker jobs in MongoDB",
			zap.String("worker_id", worker.ID),
			zap.Error(err))
	}

	return nil
}
