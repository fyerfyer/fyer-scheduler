package logmgr

import (
	"fmt"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"sort"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

// 时间范围类型常量
const (
	TimeRangeDay   = "day"
	TimeRangeWeek  = "week"
	TimeRangeMonth = "month"
)

// JobExecutionTrend 表示任务执行趋势数据
type JobExecutionTrend struct {
	Period         string                   `json:"period"`         // 统计周期（天、周、月）
	TimePoints     []string                 `json:"time_points"`    // 时间点
	SuccessCount   []int                    `json:"success_count"`  // 成功任务数
	FailureCount   []int                    `json:"failure_count"`  // 失败任务数
	SuccessRates   []float64                `json:"success_rates"`  // 成功率
	AvgDurations   []float64                `json:"avg_durations"`  // 平均执行时间
	TopJobs        []map[string]interface{} `json:"top_jobs"`       // 排名靠前的任务
	TopFailedJobs  []map[string]interface{} `json:"top_failed_jobs"` // 排名靠前的失败任务
}

// WorkerPerformance 表示Worker节点性能数据
type WorkerPerformance struct {
	WorkerID      string    `json:"worker_id"`
	Hostname      string    `json:"hostname"`
	JobCount      int       `json:"job_count"`       // 处理任务总数
	SuccessCount  int       `json:"success_count"`   // 成功任务数
	FailureCount  int       `json:"failure_count"`   // 失败任务数
	SuccessRate   float64   `json:"success_rate"`    // 成功率
	AvgDuration   float64   `json:"avg_duration"`    // 平均执行时间
	MaxDuration   float64   `json:"max_duration"`    // 最长执行时间
	MinDuration   float64   `json:"min_duration"`    // 最短执行时间
	LastExecution time.Time `json:"last_execution"`  // 最近一次执行时间
}

// SystemHealthMetrics 表示系统健康状态指标
type SystemHealthMetrics struct {
	TotalJobs           int       `json:"total_jobs"`            // 总任务数
	ActiveJobs          int       `json:"active_jobs"`           // 活跃任务数
	TotalExecutions     int       `json:"total_executions"`      // 总执行次数
	SuccessCount        int       `json:"success_count"`         // 成功执行次数
	FailureCount        int       `json:"failure_count"`         // 失败执行次数
	SuccessRate         float64   `json:"success_rate"`          // 成功率
	AvgExecutionTime    float64   `json:"avg_execution_time"`    // 平均执行时间
	ActiveWorkers       int       `json:"active_workers"`        // 活跃Worker数
	SystemLoad          float64   `json:"system_load"`           // 系统负载
	LastExecutionTime   time.Time `json:"last_execution_time"`   // 最近一次执行时间
	ExecutionsLast24h   int       `json:"executions_last_24h"`   // 过去24小时执行次数
	FailuresLast24h     int       `json:"failures_last_24h"`     // 过去24小时失败次数
	LongestRunningJob   string    `json:"longest_running_job"`   // 运行时间最长的任务
	LongestRunningTime  float64   `json:"longest_running_time"`  // 最长运行时间(秒)
	MostFrequentFailure string    `json:"most_frequent_failure"` // 最常见的失败原因
}

// GetJobExecutionTrend 获取任务执行趋势分析
func (m *MasterLogManager) GetJobExecutionTrend(timeRange string, jobID string) (*JobExecutionTrend, error) {
	// 确定时间范围
	startTime, timePoints, period := calculateTimeRange(timeRange)

	// 构建基础查询条件
	match := bson.M{
		"start_time": bson.M{"$gte": startTime},
	}

	// 如果指定了任务ID，则过滤特定任务
	if jobID != "" {
		match["job_id"] = jobID
	}

	// 创建返回结果
	trend := &JobExecutionTrend{
		Period:       timeRange,
		TimePoints:   timePoints,
		SuccessCount: make([]int, len(timePoints)),
		FailureCount: make([]int, len(timePoints)),
		SuccessRates: make([]float64, len(timePoints)),
		AvgDurations: make([]float64, len(timePoints)),
	}

	// 获取按时间段分组的聚合数据
	pipeline := []bson.M{
		{"$match": match},
		{"$project": bson.M{
			"job_id":     1,
			"job_name":   1,
			"status":     1,
			"start_time": 1,
			"end_time":   1,
			"duration": bson.M{
				"$cond": bson.M{
					"if": bson.M{"$eq": []string{"$status", "running"}},
					"then": bson.M{"$divide": []interface{}{
						bson.M{"$subtract": []interface{}{time.Now(), "$start_time"}},
						1000, // 转换为秒
					}},
					"else": bson.M{"$divide": []interface{}{
						bson.M{"$subtract": []interface{}{"$end_time", "$start_time"}},
						1000, // 转换为秒
					}},
				},
			},
			"time_bucket": calculateTimeBucket(period),
		}},
		{"$group": bson.M{
			"_id": "$time_bucket",
			"total_count": bson.M{"$sum": 1},
			"success_count": bson.M{"$sum": bson.M{
				"$cond": []interface{}{
					bson.M{"$eq": []string{"$status", "succeeded"}},
					1,
					0,
				},
			}},
			"failure_count": bson.M{"$sum": bson.M{
				"$cond": []interface{}{
					bson.M{"$eq": []string{"$status", "failed"}},
					1,
					0,
				},
			}},
			"avg_duration": bson.M{"$avg": "$duration"},
		}},
		{"$sort": bson.M{"_id": 1}},
	}

	var results []struct {
		ID           string  `bson:"_id"`
		TotalCount   int     `bson:"total_count"`
		SuccessCount int     `bson:"success_count"`
		FailureCount int     `bson:"failure_count"`
		AvgDuration  float64 `bson:"avg_duration"`
	}

	// 使用MongoDB的聚合查询
	err := m.mongoAggregate("job_logs", pipeline, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate job execution trend: %w", err)
	}

	// 处理结果
	for _, r := range results {
		for i, tp := range timePoints {
			if r.ID == tp {
				trend.SuccessCount[i] = r.SuccessCount
				trend.FailureCount[i] = r.FailureCount
				if r.TotalCount > 0 {
					trend.SuccessRates[i] = float64(r.SuccessCount) / float64(r.TotalCount) * 100
				}
				trend.AvgDurations[i] = r.AvgDuration
				break
			}
		}
	}

	// 获取排名靠前的任务
	trend.TopJobs, err = m.getTopJobs(startTime, time.Now(), 5, "")
	if err != nil {
		utils.Warn("failed to get top jobs for trend analysis", zap.Error(err))
	}

	// 获取排名靠前的失败任务
	trend.TopFailedJobs, err = m.getTopJobs(startTime, time.Now(), 5, "failed")
	if err != nil {
		utils.Warn("failed to get top failed jobs for trend analysis", zap.Error(err))
	}

	return trend, nil
}

// GetWorkerPerformance 获取Worker节点性能分析
func (m *MasterLogManager) GetWorkerPerformance(startTime, endTime time.Time) ([]WorkerPerformance, error) {
	// 构建聚合管道
	pipeline := []bson.M{
		{"$match": bson.M{
			"start_time": bson.M{"$gte": startTime, "$lte": endTime},
		}},
		{"$project": bson.M{
			"worker_id":  1,
			"worker_ip":  1,
			"status":     1,
			"start_time": 1,
			"end_time":   1,
			"duration": bson.M{
				"$cond": bson.M{
					"if": bson.M{"$eq": []string{"$status", "running"}},
					"then": bson.M{"$divide": []interface{}{
						bson.M{"$subtract": []interface{}{time.Now(), "$start_time"}},
						1000, // 转换为秒
					}},
					"else": bson.M{"$divide": []interface{}{
						bson.M{"$subtract": []interface{}{"$end_time", "$start_time"}},
						1000, // 转换为秒
					}},
				},
			},
		}},
		{"$group": bson.M{
			"_id": "$worker_id",
			"worker_ip":      bson.M{"$first": "$worker_ip"},
			"job_count":      bson.M{"$sum": 1},
			"success_count": bson.M{"$sum": bson.M{
				"$cond": []interface{}{
					bson.M{"$eq": []string{"$status", "succeeded"}},
					1,
					0,
				},
			}},
			"failure_count": bson.M{"$sum": bson.M{
				"$cond": []interface{}{
					bson.M{"$eq": []string{"$status", "failed"}},
					1,
					0,
				},
			}},
			"avg_duration": bson.M{"$avg": "$duration"},
			"max_duration": bson.M{"$max": "$duration"},
			"min_duration": bson.M{"$min": "$duration"},
			"last_execution": bson.M{"$max": "$start_time"},
		}},
		{"$sort": bson.M{"job_count": -1}},
	}

	var results []struct {
		ID            string    `bson:"_id"`
		WorkerIP      string    `bson:"worker_ip"`
		JobCount      int       `bson:"job_count"`
		SuccessCount  int       `bson:"success_count"`
		FailureCount  int       `bson:"failure_count"`
		AvgDuration   float64   `bson:"avg_duration"`
		MaxDuration   float64   `bson:"max_duration"`
		MinDuration   float64   `bson:"min_duration"`
		LastExecution time.Time `bson:"last_execution"`
	}

	// 执行聚合查询
	err := m.mongoAggregate("job_logs", pipeline, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate worker performance: %w", err)
	}

	// 处理结果
	performances := make([]WorkerPerformance, 0, len(results))
	for _, r := range results {
		perf := WorkerPerformance{
			WorkerID:      r.ID,
			Hostname:      r.WorkerIP, // 这里使用IP代替主机名，因为日志中可能没有保存主机名
			JobCount:      r.JobCount,
			SuccessCount:  r.SuccessCount,
			FailureCount:  r.FailureCount,
			AvgDuration:   r.AvgDuration,
			MaxDuration:   r.MaxDuration,
			MinDuration:   r.MinDuration,
			LastExecution: r.LastExecution,
		}

		if r.JobCount > 0 {
			perf.SuccessRate = float64(r.SuccessCount) / float64(r.JobCount) * 100
		}

		performances = append(performances, perf)
	}

	return performances, nil
}

// GetSystemHealthMetrics 获取系统健康状态指标
func (m *MasterLogManager) GetSystemHealthMetrics() (*SystemHealthMetrics, error) {
	metrics := &SystemHealthMetrics{}

	// 获取基本统计数据
	// 1. 总执行次数和成功/失败次数
	pipeline := []bson.M{
		{"$group": bson.M{
			"_id": nil,
			"total_executions": bson.M{"$sum": 1},
			"success_count": bson.M{"$sum": bson.M{
				"$cond": []interface{}{
					bson.M{"$eq": []string{"$status", "succeeded"}},
					1,
					0,
				},
			}},
			"failure_count": bson.M{"$sum": bson.M{
				"$cond": []interface{}{
					bson.M{"$eq": []string{"$status", "failed"}},
					1,
					0,
				},
			}},
			"avg_execution_time": bson.M{"$avg": bson.M{
				"$cond": bson.M{
					"if": bson.M{"$and": []bson.M{
						{"$ne": []string{"$status", "running"}},
						{"$ne": []interface{}{"$end_time", nil}},
						{"$ne": []interface{}{"$start_time", nil}},
					}},
					"then": bson.M{"$divide": []interface{}{
						bson.M{"$subtract": []interface{}{"$end_time", "$start_time"}},
						1000, // 转换为秒
					}},
					"else": 0,
				},
			}},
			"last_execution_time": bson.M{"$max": "$start_time"},
		}},
	}

	var basicStats []struct {
		TotalExecutions   int       `bson:"total_executions"`
		SuccessCount      int       `bson:"success_count"`
		FailureCount      int       `bson:"failure_count"`
		AvgExecutionTime  float64   `bson:"avg_execution_time"`
		LastExecutionTime time.Time `bson:"last_execution_time"`
	}

	err := m.mongoAggregate("job_logs", pipeline, &basicStats)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate system health metrics: %w", err)
	}

	if len(basicStats) > 0 {
		metrics.TotalExecutions = basicStats[0].TotalExecutions
		metrics.SuccessCount = basicStats[0].SuccessCount
		metrics.FailureCount = basicStats[0].FailureCount
		metrics.AvgExecutionTime = basicStats[0].AvgExecutionTime
		metrics.LastExecutionTime = basicStats[0].LastExecutionTime

		if metrics.TotalExecutions > 0 {
			metrics.SuccessRate = float64(metrics.SuccessCount) / float64(metrics.TotalExecutions) * 100
		}
	}

	// 2. 过去24小时的执行和失败次数
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	pipeline = []bson.M{
		{"$match": bson.M{
			"start_time": bson.M{"$gte": oneDayAgo},
		}},
		{"$group": bson.M{
			"_id": nil,
			"executions_count": bson.M{"$sum": 1},
			"failures_count": bson.M{"$sum": bson.M{
				"$cond": []interface{}{
					bson.M{"$eq": []string{"$status", "failed"}},
					1,
					0,
				},
			}},
		}},
	}

	var last24hStats []struct {
		ExecutionsCount int `bson:"executions_count"`
		FailuresCount   int `bson:"failures_count"`
	}

	err = m.mongoAggregate("job_logs", pipeline, &last24hStats)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate last 24h stats: %w", err)
	}

	if len(last24hStats) > 0 {
		metrics.ExecutionsLast24h = last24hStats[0].ExecutionsCount
		metrics.FailuresLast24h = last24hStats[0].FailuresCount
	}

	// 3. 获取运行时间最长的任务
	pipeline = []bson.M{
		{"$match": bson.M{
			"status": "running",
		}},
		{"$project": bson.M{
			"job_id":   1,
			"job_name": 1,
			"duration": bson.M{"$divide": []interface{}{
				bson.M{"$subtract": []interface{}{time.Now(), "$start_time"}},
				1000, // 转换为秒
			}},
		}},
		{"$sort": bson.M{"duration": -1}},
		{"$limit": 1},
	}

	var longestRunning []struct {
		JobID    string  `bson:"job_id"`
		JobName  string  `bson:"job_name"`
		Duration float64 `bson:"duration"`
	}

	err = m.mongoAggregate("job_logs", pipeline, &longestRunning)
	if err == nil && len(longestRunning) > 0 {
		metrics.LongestRunningJob = longestRunning[0].JobName
		metrics.LongestRunningTime = longestRunning[0].Duration
	}

	// 4. 获取最常见的失败原因
	pipeline = []bson.M{
		{"$match": bson.M{
			"status": "failed",
			"error":  bson.M{"$ne": ""},
		}},
		{"$group": bson.M{
			"_id":   "$error",
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"count": -1}},
		{"$limit": 1},
	}

	var frequentFailures []struct {
		Error string `bson:"_id"`
		Count int    `bson:"count"`
	}

	err = m.mongoAggregate("job_logs", pipeline, &frequentFailures)
	if err == nil && len(frequentFailures) > 0 {
		metrics.MostFrequentFailure = frequentFailures[0].Error
	}

	// 获取活跃Worker数
	pipeline = []bson.M{
		{"$group": bson.M{
			"_id": "$worker_id",
		}},
		{"$group": bson.M{
			"_id": nil,
			"count": bson.M{"$sum": 1},
		}},
	}

	var workerCount []struct {
		Count int `bson:"count"`
	}

	err = m.mongoAggregate("job_logs", pipeline, &workerCount)
	if err == nil && len(workerCount) > 0 {
		metrics.ActiveWorkers = workerCount[0].Count
	}

	return metrics, nil
}

// GetJobFailureAnalysis 分析任务失败情况
func (m *MasterLogManager) GetJobFailureAnalysis(jobID string, startTime, endTime time.Time) (map[string]interface{}, error) {
	// 构建匹配条件
	match := bson.M{
		"status":     "failed",
		"start_time": bson.M{"$gte": startTime, "$lte": endTime},
	}

	if jobID != "" {
		match["job_id"] = jobID
	}

	// 按错误类型分组统计
	pipeline := []bson.M{
		{"$match": match},
		{"$group": bson.M{
			"_id":   "$error",
			"count": bson.M{"$sum": 1},
			"job_ids": bson.M{"$addToSet": "$job_id"},
			"examples": bson.M{"$push": bson.M{
				"execution_id": "$execution_id",
				"job_name":     "$job_name",
				"start_time":   "$start_time",
				"worker_id":    "$worker_id",
				"exit_code":    "$exit_code",
			}},
		}},
		{"$project": bson.M{
			"error":     "$_id",
			"count":     1,
			"job_count": bson.M{"$size": "$job_ids"},
			"examples":  bson.M{"$slice": `["$examples", 5]`}, // 只取前5个示例
		}},
		{"$sort": bson.M{"count": -1}},
	}

	var results []map[string]interface{}
	err := m.mongoAggregate("job_logs", pipeline, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze job failures: %w", err)
	}

	// 获取总失败次数
	totalFailures := 0
	for _, r := range results {
		count, ok := r["count"].(int32)
		if ok {
			totalFailures += int(count)
		}
	}

	// 计算错误率
	for i, r := range results {
		count, ok := r["count"].(int32)
		if ok && totalFailures > 0 {
			results[i]["percentage"] = float64(count) / float64(totalFailures) * 100
		} else {
			results[i]["percentage"] = 0.0
		}
	}

	// 返回分析结果
	return map[string]interface{}{
		"total_failures": totalFailures,
		"error_types":    results,
		"start_time":     startTime,
		"end_time":       endTime,
		"job_id":         jobID,
	}, nil
}

// GetJobDurationAnalysis 分析任务执行时间
func (m *MasterLogManager) GetJobDurationAnalysis(jobID string, startTime, endTime time.Time) (map[string]interface{}, error) {
	// 构建匹配条件
	match := bson.M{
		"start_time": bson.M{"$gte": startTime, "$lte": endTime},
		"end_time":   bson.M{"$ne": nil},
	}

	if jobID != "" {
		match["job_id"] = jobID
	}

	// 计算执行时间统计
	pipeline := []bson.M{
		{"$match": match},
		{"$project": bson.M{
			"job_id":    1,
			"job_name":  1,
			"worker_id": 1,
			"status":    1,
			"duration": bson.M{"$divide": []interface{}{
				bson.M{"$subtract": []interface{}{"$end_time", "$start_time"}},
				1000, // 转换为秒
			}},
		}},
		{"$group": bson.M{
			"_id": "$job_id",
			"job_name":     bson.M{"$first": "$job_name"},
			"count":        bson.M{"$sum": 1},
			"avg_duration": bson.M{"$avg": "$duration"},
			"min_duration": bson.M{"$min": "$duration"},
			"max_duration": bson.M{"$max": "$duration"},
			"total_duration": bson.M{"$sum": "$duration"},
			"durations": bson.M{"$push": "$duration"},
			"success_count": bson.M{"$sum": bson.M{
				"$cond": []interface{}{
					bson.M{"$eq": []string{"$status", "succeeded"}},
					1,
					0,
				},
			}},
		}},
		{"$sort": bson.M{"avg_duration": -1}},
	}

	var results []map[string]interface{}
	err := m.mongoAggregate("job_logs", pipeline, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze job durations: %w", err)
	}

	// 处理结果 - 计算百分位数等
	for i, r := range results {
		durationsRaw, ok := r["durations"].([]interface{})
		if ok && len(durationsRaw) > 0 {
			// 转换和排序持续时间
			durations := make([]float64, 0, len(durationsRaw))
			for _, d := range durationsRaw {
				if f, ok := d.(float64); ok {
					durations = append(durations, f)
				}
			}

			// 排序
			sort.Float64s(durations)

			// 计算百分位数
			n := len(durations)
			if n > 0 {
				results[i]["p50"] = percentile(durations, 50)
				results[i]["p90"] = percentile(durations, 90)
				results[i]["p95"] = percentile(durations, 95)
				results[i]["p99"] = percentile(durations, 99)
			}

			// 计算成功率
			count, _ := r["count"].(int32)
			successCount, _ := r["success_count"].(int32)
			if count > 0 {
				results[i]["success_rate"] = float64(successCount) / float64(count) * 100
			}

			// 移除原始数据以减少响应大小
			delete(results[i], "durations")
		}
	}

	return map[string]interface{}{
		"results":    results,
		"start_time": startTime,
		"end_time":   endTime,
		"job_id":     jobID,
	}, nil
}

// percentile 计算百分位数
func percentile(data []float64, p int) float64 {
	if len(data) == 0 {
		return 0
	}

	if p <= 0 {
		return data[0]
	}

	if p >= 100 {
		return data[len(data)-1]
	}

	// 计算位置
	pos := float64(len(data)-1) * float64(p) / 100.0
	posIdx := int(pos)

	// 如果是整数位置，直接返回
	if float64(posIdx) == pos {
		return data[posIdx]
	}

	// 否则，插值计算
	return data[posIdx] + (data[posIdx+1]-data[posIdx])*(pos-float64(posIdx))
}

// 辅助函数 - 获取排名靠前的任务
func (m *MasterLogManager) getTopJobs(startTime, endTime time.Time, limit int, status string) ([]map[string]interface{}, error) {
	// 构建匹配条件
	match := bson.M{
		"start_time": bson.M{"$gte": startTime, "$lte": endTime},
	}

	if status != "" {
		match["status"] = status
	}

	// 按任务分组统计
	pipeline := []bson.M{
		{"$match": match},
		{"$group": bson.M{
			"_id":          "$job_id",
			"job_name":     bson.M{"$first": "$job_name"},
			"count":        bson.M{"$sum": 1},
			"avg_duration": bson.M{"$avg": bson.M{
				"$cond": bson.M{
					"if": bson.M{"$and": []bson.M{
						{"$ne": []interface{}{"$end_time", nil}},
						{"$ne": []interface{}{"$start_time", nil}},
					}},
					"then": bson.M{"$divide": []interface{}{
						bson.M{"$subtract": []interface{}{"$end_time", "$start_time"}},
						1000, // 转换为秒
					}},
					"else": 0,
				},
			}},
			"success_count": bson.M{"$sum": bson.M{
				"$cond": []interface{}{
					bson.M{"$eq": []string{"$status", "succeeded"}},
					1,
					0,
				},
			}},
		}},
		{"$project": bson.M{
			"job_id":       "$_id",
			"job_name":     1,
			"count":        1,
			"avg_duration": 1,
			"success_rate": bson.M{
				"$cond": bson.M{
					"if": bson.M{"$gt": []interface{}{"$count", 0}},
					"then": bson.M{"$multiply": []interface{}{
						bson.M{"$divide": []interface{}{"$success_count", "$count"}},
						100,
					}},
					"else": 0,
				},
			},
		}},
		{"$sort": bson.M{"count": -1}},
		{"$limit": limit},
	}

	var results []map[string]interface{}
	err := m.mongoAggregate("job_logs", pipeline, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to get top jobs: %w", err)
	}

	return results, nil
}

// mongoAggregate 封装MongoDB聚合操作
func (m *MasterLogManager) mongoAggregate(collection string, pipeline interface{}, results interface{}) error {
	// 这里使用MongoDB客户端的Aggregate方法
	// 由于logRepo没有直接提供Aggregate方法，我们可以从MongoDB客户端直接调用
	client, ok := m.logRepo.(*repo.LogRepo)
	if !ok {
		return fmt.Errorf("unexpected log repository type")
	}

	return client.MongoAggregate(collection, pipeline, results)
}

// 辅助函数 - 计算时间范围和时间点
func calculateTimeRange(timeRange string) (time.Time, []string, string) {
	now := time.Now()
	var startTime time.Time
	var timePoints []string
	var period string

	switch timeRange {
	case TimeRangeDay:
		startTime = now.AddDate(0, 0, -1)
		period = "hour"
		// 生成24小时的时间点
		for i := 0; i < 24; i++ {
			t := startTime.Add(time.Duration(i) * time.Hour)
			timePoints = append(timePoints, t.Format("15:00"))
		}
	case TimeRangeWeek:
		startTime = now.AddDate(0, 0, -7)
		period = "day"
		// 生成7天的时间点
		for i := 0; i < 7; i++ {
			t := startTime.AddDate(0, 0, i)
			timePoints = append(timePoints, t.Format("01-02"))
		}
	case TimeRangeMonth:
		startTime = now.AddDate(0, -1, 0)
		period = "day"
		// 生成30天的时间点
		for i := 0; i < 30; i++ {
			t := startTime.AddDate(0, 0, i)
			timePoints = append(timePoints, t.Format("01-02"))
		}
	default:
		// 默认为天
		startTime = now.AddDate(0, 0, -1)
		period = "hour"
		for i := 0; i < 24; i++ {
			t := startTime.Add(time.Duration(i) * time.Hour)
			timePoints = append(timePoints, t.Format("15:00"))
		}
	}

	return startTime, timePoints, period
}

// calculateTimeBucket 计算MongoDB聚合中的时间分组表达式
func calculateTimeBucket(period string) bson.M {
	switch period {
	case "hour":
		return bson.M{
			"$dateToString": bson.M{
				"format": "%H:00",
				"date":   "$start_time",
			},
		}
	case "day":
		return bson.M{
			"$dateToString": bson.M{
				"format": "%m-%d",
				"date":   "$start_time",
			},
		}
	case "month":
		return bson.M{
			"$dateToString": bson.M{
				"format": "%Y-%m",
				"date":   "$start_time",
			},
		}
	default:
		return bson.M{
			"$dateToString": bson.M{
				"format": "%H:00",
				"date":   "$start_time",
			},
		}
	}
}