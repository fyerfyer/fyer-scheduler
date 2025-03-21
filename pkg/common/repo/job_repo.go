package repo

import (
	"fmt"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// JobRepo 提供对任务数据的访问
type JobRepo struct {
	etcdClient   *utils.EtcdClient
	mongoClient  *utils.MongoDBClient
	jobsCollName string
}

// NewJobRepo 创建一个新的任务仓库
func NewJobRepo(etcdClient *utils.EtcdClient, mongoClient *utils.MongoDBClient) *JobRepo {
	return &JobRepo{
		etcdClient:   etcdClient,
		mongoClient:  mongoClient,
		jobsCollName: "jobs",
	}
}

// Save 保存任务到etcd和MongoDB
func (r *JobRepo) Save(job *models.Job) error {
	// 保存到etcd (用于实时任务调度)
	jobJSON, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	err = r.etcdClient.Put(job.Key(), jobJSON)
	if err != nil {
		return fmt.Errorf("failed to save job to etcd: %w", err)
	}

	// 保存到MongoDB (用于历史记录和查询)
	filter := bson.M{"id": job.ID}
	update := bson.M{"$set": job}
	opts := options.Update().SetUpsert(true)

	_, err = r.mongoClient.UpdateOne(r.jobsCollName, filter, update, opts)
	if err != nil {
		utils.Error("failed to save job to MongoDB",
			zap.String("job_id", job.ID),
			zap.Error(err))
		// 不因MongoDB失败而阻止任务调度，仅记录错误
	}

	return nil
}

// GetByID 根据ID获取任务
func (r *JobRepo) GetByID(jobID string) (*models.Job, error) {
	// 优先从etcd获取最新数据
	jobKey := constants.JobPrefix + jobID
	jobJSON, err := r.etcdClient.Get(jobKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get job from etcd: %w", err)
	}

	// 如果etcd中存在，直接返回
	if jobJSON != "" {
		return models.FromJSON(jobJSON)
	}

	// 如果etcd中不存在，尝试从MongoDB获取
	job := &models.Job{}
	err = r.mongoClient.FindOne(r.jobsCollName, bson.M{"id": jobID}, job)
	if err != nil {
		return nil, fmt.Errorf("job not found: %w", err)
	}

	return job, nil
}

// ListAll 获取所有任务列表
func (r *JobRepo) ListAll() ([]*models.Job, error) {
	var jobs []*models.Job

	// 从etcd获取所有任务
	kvs, err := r.etcdClient.GetWithPrefix(constants.JobPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs from etcd: %w", err)
	}

	for _, v := range kvs {
		job, err := models.FromJSON(v)
		if err != nil {
			utils.Warn("failed to parse job from etcd", zap.Error(err))
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// Delete 删除任务
func (r *JobRepo) Delete(jobID string) error {
	jobKey := constants.JobPrefix + jobID

	// 从etcd删除
	err := r.etcdClient.Delete(jobKey)
	if err != nil {
		return fmt.Errorf("failed to delete job from etcd: %w", err)
	}

	// 从MongoDB删除 (可选，也可以保留历史记录并标记删除状态)
	_, err = r.mongoClient.DeleteOne(r.jobsCollName, bson.M{"id": jobID})
	if err != nil {
		utils.Warn("failed to delete job from MongoDB",
			zap.String("job_id", jobID),
			zap.Error(err))
		// 不因MongoDB失败而阻止任务删除，仅记录错误
	}

	return nil
}

// ListByStatus 根据状态获取任务列表
func (r *JobRepo) ListByStatus(status string, page, pageSize int64) ([]*models.Job, int64, error) {
	var jobs []*models.Job

	// 构建查询
	query := utils.NewQuery().
		Where("status", status).
		SetPage(page, pageSize).
		OrderBy("create_time", false)

	// 执行查询
	err := r.mongoClient.FindWithOptions(
		r.jobsCollName,
		query.GetFilter(),
		&jobs,
		query.GetOptions(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query jobs by status: %w", err)
	}

	// 获取总记录数
	total, err := r.mongoClient.Count(r.jobsCollName, query.GetFilter())
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count jobs by status: %w", err)
	}

	return jobs, total, nil
}

// ListByNextRunTime 获取在指定时间之前需要运行的任务
func (r *JobRepo) ListByNextRunTime(beforeTime time.Time) ([]*models.Job, error) {
	var jobs []*models.Job

	// 构建查询
	query := utils.NewQuery().
		Where("enabled", true).
		WhereLte("next_run_time", beforeTime).
		OrderBy("next_run_time", true)

	// 执行查询
	err := r.mongoClient.FindWithOptions(
		r.jobsCollName,
		query.GetFilter(),
		&jobs,
		query.GetOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query jobs by next run time: %w", err)
	}

	return jobs, nil
}

// Search 搜索任务
func (r *JobRepo) Search(keyword string, page, pageSize int64) ([]*models.Job, int64, error) {
	var jobs []*models.Job

	// 构建查询 (支持名称和描述的模糊搜索)
	filter := bson.M{
		"$or": []bson.M{
			{"name": bson.M{"$regex": keyword, "$options": "i"}},
			{"description": bson.M{"$regex": keyword, "$options": "i"}},
		},
	}

	// 设置分页和排序
	opts := options.Find().
		SetSkip((page - 1) * pageSize).
		SetLimit(pageSize).
		SetSort(bson.D{{Key: "create_time", Value: -1}})

	// 执行查询
	err := r.mongoClient.FindWithOptions(r.jobsCollName, filter, &jobs, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search jobs: %w", err)
	}

	// 获取总记录数
	total, err := r.mongoClient.Count(r.jobsCollName, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	return jobs, total, nil
}

// UpdateStatus 更新任务状态
func (r *JobRepo) UpdateStatus(jobID, status string) error {
	// 先获取当前任务
	job, err := r.GetByID(jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// 更新状态
	job.SetStatus(status)
	job.UpdateTime = time.Now()

	// 保存更改
	return r.Save(job)
}

// UpdateNextRunTime 更新任务的下次运行时间
func (r *JobRepo) UpdateNextRunTime(jobID string, lastRunTime time.Time) error {
	// 先获取当前任务
	job, err := r.GetByID(jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// 更新上次运行时间
	job.LastRunTime = lastRunTime
	job.UpdateTime = time.Now()

	// 计算下次运行时间
	err = job.CalculateNextRunTime()
	if err != nil {
		return fmt.Errorf("failed to calculate next run time: %w", err)
	}

	// 保存更改
	return r.Save(job)
}

// EnableJob 启用任务
func (r *JobRepo) EnableJob(jobID string) error {
	job, err := r.GetByID(jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	job.Enabled = true
	job.UpdateTime = time.Now()

	return r.Save(job)
}

// DisableJob 禁用任务
func (r *JobRepo) DisableJob(jobID string) error {
	job, err := r.GetByID(jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	job.Enabled = false
	job.UpdateTime = time.Now()

	return r.Save(job)
}
