package repo

import (
	"fmt"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// LogRepo 提供对任务执行日志的访问
type LogRepo struct {
	mongoClient  *utils.MongoDBClient
	logsCollName string
}

// NewLogRepo 创建一个新的日志仓库
func NewLogRepo(mongoClient *utils.MongoDBClient) *LogRepo {
	return &LogRepo{
		mongoClient:  mongoClient,
		logsCollName: "job_logs",
	}
}

// Save 保存任务执行日志
func (r *LogRepo) Save(log *models.JobLog) error {
	// 日志只保存在MongoDB中
	if log.ID.IsZero() {
		log.ID = primitive.NewObjectID()
	}

	// 构建保存操作
	filter := bson.M{"execution_id": log.ExecutionID}
	update := bson.M{"$set": log}
	opts := options.Update().SetUpsert(true)

	_, err := r.mongoClient.UpdateOne(r.logsCollName, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save job log: %w", err)
	}

	return nil
}

// GetByExecutionID 根据执行ID获取日志
func (r *LogRepo) GetByExecutionID(executionID string) (*models.JobLog, error) {
	log := &models.JobLog{}
	err := r.mongoClient.FindOne(r.logsCollName, bson.M{"execution_id": executionID}, log)
	if err != nil {
		return nil, fmt.Errorf("failed to get job log: %w", err)
	}

	return log, nil
}

// GetByJobID 获取指定任务的执行日志
func (r *LogRepo) GetByJobID(jobID string, page, pageSize int64) ([]*models.JobLog, int64, error) {
	var logs []*models.JobLog

	// 构建查询
	query := utils.NewQuery().
		Where("job_id", jobID).
		SetPage(page, pageSize).
		OrderBy("start_time", false)

	// 执行查询
	err := r.mongoClient.FindWithOptions(
		r.logsCollName,
		query.GetFilter(),
		&logs,
		query.GetOptions(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query job logs: %w", err)
	}

	// 获取总记录数
	total, err := r.mongoClient.Count(r.logsCollName, query.GetFilter())
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count job logs: %w", err)
	}

	return logs, total, nil
}

// GetLatestByJobID 获取指定任务的最新一条日志
func (r *LogRepo) GetLatestByJobID(jobID string) (*models.JobLog, error) {
	var logs []*models.JobLog

	// 构建查询
	opts := options.Find().
		SetSort(bson.D{{Key: "start_time", Value: -1}}).
		SetLimit(1)

	// 执行查询
	err := r.mongoClient.FindWithOptions(
		r.logsCollName,
		bson.M{"job_id": jobID},
		&logs,
		opts,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest job log: %w", err)
	}

	if len(logs) == 0 {
		return nil, fmt.Errorf("no log found for job: %s", jobID)
	}

	return logs[0], nil
}

// GetByTimeRange 获取指定时间范围内的日志
func (r *LogRepo) GetByTimeRange(startTime, endTime time.Time, page, pageSize int64) ([]*models.JobLog, int64, error) {
	var logs []*models.JobLog

	// 构建查询条件
	filter := bson.M{
		"start_time": bson.M{
			"$gte": startTime,
			"$lte": endTime,
		},
	}

	// 设置分页和排序
	opts := options.Find().
		SetSkip((page - 1) * pageSize).
		SetLimit(pageSize).
		SetSort(bson.D{{Key: "start_time", Value: -1}})

	// 执行查询
	err := r.mongoClient.FindWithOptions(r.logsCollName, filter, &logs, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query logs by time range: %w", err)
	}

	// 获取总记录数
	total, err := r.mongoClient.Count(r.logsCollName, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count logs by time range: %w", err)
	}

	return logs, total, nil
}

// GetByWorkerID 获取指定Worker的执行日志
func (r *LogRepo) GetByWorkerID(workerID string, page, pageSize int64) ([]*models.JobLog, int64, error) {
	var logs []*models.JobLog

	// 构建查询
	query := utils.NewQuery().
		Where("worker_id", workerID).
		SetPage(page, pageSize).
		OrderBy("start_time", false)

	// 执行查询
	err := r.mongoClient.FindWithOptions(
		r.logsCollName,
		query.GetFilter(),
		&logs,
		query.GetOptions(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query logs by worker: %w", err)
	}

	// 获取总记录数
	total, err := r.mongoClient.Count(r.logsCollName, query.GetFilter())
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count logs by worker: %w", err)
	}

	return logs, total, nil
}

// GetByStatus 获取指定状态的日志
func (r *LogRepo) GetByStatus(status string, page, pageSize int64) ([]*models.JobLog, int64, error) {
	var logs []*models.JobLog

	// 构建查询
	query := utils.NewQuery().
		Where("status", status).
		SetPage(page, pageSize).
		OrderBy("start_time", false)

	// 执行查询
	err := r.mongoClient.FindWithOptions(
		r.logsCollName,
		query.GetFilter(),
		&logs,
		query.GetOptions(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query logs by status: %w", err)
	}

	// 获取总记录数
	total, err := r.mongoClient.Count(r.logsCollName, query.GetFilter())
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count logs by status: %w", err)
	}

	return logs, total, nil
}

// CountByJobStatus 获取任务执行状态统计
func (r *LogRepo) CountByJobStatus(jobID string) (map[string]int64, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"job_id": jobID,
			},
		},
		{
			"$group": bson.M{
				"_id":   "$status",
				"count": bson.M{"$sum": 1},
			},
		},
	}

	var results []struct {
		ID    string `bson:"_id"`
		Count int64  `bson:"count"`
	}

	err := r.mongoClient.Aggregate(r.logsCollName, pipeline, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate job status counts: %w", err)
	}

	// 将结果转换为map
	countMap := make(map[string]int64)
	for _, result := range results {
		countMap[result.ID] = result.Count
	}

	return countMap, nil
}

// GetJobSuccessRate 获取任务成功率
func (r *LogRepo) GetJobSuccessRate(jobID string, period time.Duration) (float64, error) {
	// 计算开始时间
	startTime := time.Now().Add(-period)

	// 构建聚合查询
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"job_id":     jobID,
				"start_time": bson.M{"$gte": startTime},
				"status": bson.M{
					"$in": []string{"succeeded", "failed"},
				},
			},
		},
		{
			"$group": bson.M{
				"_id": nil,
				"total": bson.M{
					"$sum": 1,
				},
				"succeeded": bson.M{
					"$sum": bson.M{
						"$cond": []interface{}{
							bson.M{"$eq": []string{"$status", "succeeded"}},
							1,
							0,
						},
					},
				},
			},
		},
	}

	var results []struct {
		Total     int64 `bson:"total"`
		Succeeded int64 `bson:"succeeded"`
	}

	err := r.mongoClient.Aggregate(r.logsCollName, pipeline, &results)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate job success rate: %w", err)
	}

	// 如果没有结果，返回0
	if len(results) == 0 || results[0].Total == 0 {
		return 0, nil
	}

	// 计算成功率
	return float64(results[0].Succeeded) / float64(results[0].Total), nil
}

// DeleteOldLogs 删除指定时间之前的日志
func (r *LogRepo) DeleteOldLogs(beforeTime time.Time) (int64, error) {
	filter := bson.M{
		"end_time": bson.M{
			"$lt": beforeTime,
		},
	}

	result, err := r.mongoClient.DeleteMany(r.logsCollName, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old logs: %w", err)
	}

	return result.DeletedCount, nil
}

// UpdateLogStatus 更新日志状态
func (r *LogRepo) UpdateLogStatus(executionID, status string, output string) error {
	// 构建更新条件
	filter := bson.M{"execution_id": executionID}
	update := bson.M{
		"$set": bson.M{
			"status":   status,
			"output":   output,
			"end_time": time.Now(),
		},
	}

	_, err := r.mongoClient.UpdateOne(r.logsCollName, filter, update)
	if err != nil {
		utils.Error("failed to update log status",
			zap.String("execution_id", executionID),
			zap.Error(err))
		return fmt.Errorf("failed to update log status: %w", err)
	}

	return nil
}

// AppendOutput 追加输出到日志
func (r *LogRepo) AppendOutput(executionID, newOutput string) error {
	// 获取当前日志
	log, err := r.GetByExecutionID(executionID)
	if err != nil {
		return fmt.Errorf("failed to get log for append: %w", err)
	}

	// 追加输出
	log.Output += newOutput

	// 保存更新
	return r.Save(log)
}
