package master

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"github.com/fyerfyer/fyer-scheduler/pkg/master/logmgr"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogManagerBasicOperations 测试日志管理器的基本操作
func TestLogManagerBasicOperations(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建日志仓库
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建日志管理器
	lm := logmgr.NewLogManager(logRepo)

	// 创建测试任务和日志
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	worker := testutils.CreateWorker("test-worker-1", nil)

	// 创建一个测试日志
	jobLog := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	jobLog.Status = constants.JobStatusSucceeded
	jobLog.Output = "Test output"
	jobLog.ExitCode = 0
	jobLog.StartTime = time.Now().Add(-1 * time.Minute)
	jobLog.EndTime = time.Now()

	// 保存日志
	err := logRepo.Save(jobLog)
	require.NoError(t, err, "Failed to save job log")

	// 测试根据执行ID获取日志
	retrievedLog, err := lm.GetLogByExecutionID(jobLog.ExecutionID)
	require.NoError(t, err, "Failed to get log by execution ID")
	assert.Equal(t, jobLog.ExecutionID, retrievedLog.ExecutionID, "Execution IDs should match")
	assert.Equal(t, constants.JobStatusSucceeded, retrievedLog.Status, "Log status should be succeeded")

	// 测试根据任务ID获取日志
	logs, total, err := lm.GetLogsByJobID(job.ID, 1, 10)
	require.NoError(t, err, "Failed to get logs by job ID")
	assert.Equal(t, int64(1), total, "Total log count should be 1")
	assert.Len(t, logs, 1, "Should return 1 log")
	assert.Equal(t, jobLog.ExecutionID, logs[0].ExecutionID, "Log execution ID should match")

	// 测试获取最新日志
	latestLog, err := lm.GetLatestLogByJobID(job.ID)
	require.NoError(t, err, "Failed to get latest log")
	assert.Equal(t, jobLog.ExecutionID, latestLog.ExecutionID, "Latest log execution ID should match")
}

// TestLogManagerTimeRangeQuery 测试按时间范围查询日志
func TestLogManagerTimeRangeQuery(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建日志仓库
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建日志管理器
	lm := logmgr.NewLogManager(logRepo)

	// 创建测试任务和日志
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	worker := testutils.CreateWorker("test-worker-1", nil)

	// 创建3个不同时间的日志
	now := time.Now()

	// 1小时前的日志
	log1 := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	log1.Status = constants.JobStatusSucceeded
	log1.StartTime = now.Add(-1 * time.Hour)
	log1.EndTime = log1.StartTime.Add(5 * time.Minute)

	// 2小时前的日志
	log2 := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	log2.Status = constants.JobStatusFailed
	log2.StartTime = now.Add(-2 * time.Hour)
	log2.EndTime = log2.StartTime.Add(5 * time.Minute)

	// 3小时前的日志
	log3 := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	log3.Status = constants.JobStatusSucceeded
	log3.StartTime = now.Add(-3 * time.Hour)
	log3.EndTime = log3.StartTime.Add(5 * time.Minute)

	// 保存日志
	require.NoError(t, logRepo.Save(log1), "Failed to save log1")
	require.NoError(t, logRepo.Save(log2), "Failed to save log2")
	require.NoError(t, logRepo.Save(log3), "Failed to save log3")

	// 测试时间范围查询 - 过去2.5小时内
	startTime := now.Add(-(2*time.Hour + 30*time.Minute))
	logs, total, err := lm.GetLogsByTimeRange(startTime, now, 1, 10)
	require.NoError(t, err, "Failed to get logs by time range")
	assert.Equal(t, int64(2), total, "Total log count should be 2")
	assert.Len(t, logs, 2, "Should return 2 logs")

	// 检查结果是否按时间倒序排列
	if len(logs) >= 2 {
		assert.True(t, logs[0].StartTime.After(logs[1].StartTime), "Logs should be sorted by start time desc")
	}
}

// TestLogManagerStatusQuery 测试按状态查询日志
func TestLogManagerStatusQuery(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建日志仓库
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建日志管理器
	lm := logmgr.NewLogManager(logRepo)

	// 创建测试任务和日志
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	worker := testutils.CreateWorker("test-worker-1", nil)

	// 创建不同状态的日志
	log1 := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	log1.Status = constants.JobStatusSucceeded

	log2 := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	log2.Status = constants.JobStatusFailed

	log3 := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	log3.Status = constants.JobStatusSucceeded

	// 保存日志
	require.NoError(t, logRepo.Save(log1), "Failed to save log1")
	require.NoError(t, logRepo.Save(log2), "Failed to save log2")
	require.NoError(t, logRepo.Save(log3), "Failed to save log3")

	// 测试按状态查询 - 成功状态
	logs, total, err := lm.GetLogsByStatus(constants.JobStatusSucceeded, 1, 10)
	require.NoError(t, err, "Failed to get logs by status")
	assert.Equal(t, int64(2), total, "Total successful log count should be 2")
	assert.Len(t, logs, 2, "Should return 2 successful logs")

	// 测试按状态查询 - 失败状态
	logs, total, err = lm.GetLogsByStatus(constants.JobStatusFailed, 1, 10)
	require.NoError(t, err, "Failed to get logs by status")
	assert.Equal(t, int64(1), total, "Total failed log count should be 1")
	assert.Len(t, logs, 1, "Should return 1 failed log")
}

// TestLogManagerWorkerQuery 测试按Worker查询日志
func TestLogManagerWorkerQuery(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建日志仓库
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建日志管理器
	lm := logmgr.NewLogManager(logRepo)

	// 创建测试任务和日志
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	worker1 := testutils.CreateWorker("test-worker-1", nil)
	worker2 := testutils.CreateWorker("test-worker-2", nil)

	// 创建不同Worker的日志
	log1 := jobFactory.CreateJobLog(job, worker1.ID, worker1.IP)
	log2 := jobFactory.CreateJobLog(job, worker1.ID, worker1.IP)
	log3 := jobFactory.CreateJobLog(job, worker2.ID, worker2.IP)

	// 保存日志
	require.NoError(t, logRepo.Save(log1), "Failed to save log1")
	require.NoError(t, logRepo.Save(log2), "Failed to save log2")
	require.NoError(t, logRepo.Save(log3), "Failed to save log3")

	// 测试按Worker查询
	logs, total, err := lm.GetLogsByWorkerID(worker1.ID, 1, 10)
	require.NoError(t, err, "Failed to get logs by worker ID")
	assert.Equal(t, int64(2), total, "Total log count for worker1 should be 2")
	assert.Len(t, logs, 2, "Should return 2 logs for worker1")

	logs, total, err = lm.GetLogsByWorkerID(worker2.ID, 1, 10)
	require.NoError(t, err, "Failed to get logs by worker ID")
	assert.Equal(t, int64(1), total, "Total log count for worker2 should be 1")
	assert.Len(t, logs, 1, "Should return 1 log for worker2")
}

// TestLogManagerAggregation 测试日志聚合功能
func TestLogManagerAggregation(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建日志仓库
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建日志管理器
	lm := logmgr.NewLogManager(logRepo)

	// 创建测试任务和日志
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	worker := testutils.CreateWorker("test-worker-1", nil)

	// 创建不同状态的日志
	log1 := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	log1.Status = constants.JobStatusSucceeded
	log1.StartTime = time.Now().Add(-1 * time.Hour)
	log1.EndTime = log1.StartTime.Add(5 * time.Minute)

	log2 := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	log2.Status = constants.JobStatusFailed
	log2.StartTime = time.Now().Add(-2 * time.Hour)
	log2.EndTime = log2.StartTime.Add(10 * time.Minute)

	log3 := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	log3.Status = constants.JobStatusSucceeded
	log3.StartTime = time.Now().Add(-3 * time.Hour)
	log3.EndTime = log3.StartTime.Add(5 * time.Minute)

	// 保存日志
	require.NoError(t, logRepo.Save(log1), "Failed to save log1")
	require.NoError(t, logRepo.Save(log2), "Failed to save log2")
	require.NoError(t, logRepo.Save(log3), "Failed to save log3")

	// 测试任务状态计数
	counts, err := lm.GetJobStatusCounts(job.ID)
	require.NoError(t, err, "Failed to get job status counts")
	assert.Equal(t, int64(2), counts[constants.JobStatusSucceeded], "Should have 2 successful logs")
	assert.Equal(t, int64(1), counts[constants.JobStatusFailed], "Should have 1 failed log")

	// 测试任务成功率
	successRate, err := lm.GetJobSuccessRate(job.ID, 24*time.Hour)
	require.NoError(t, err, "Failed to get job success rate")
	assert.Equal(t, float64(2)/float64(3), successRate, "Success rate should be 2/3")

	// 测试系统状态摘要
	summary, err := lm.GetSystemStatusSummary()
	require.NoError(t, err, "Failed to get system status summary")
	assert.NotNil(t, summary, "System status summary should not be nil")

	// 测试每日执行统计
	dailyStats, err := lm.GetDailyJobExecutionStats(7)
	require.NoError(t, err, "Failed to get daily job execution stats")
	assert.NotNil(t, dailyStats, "Daily job execution stats should not be nil")
}

// TestLogManagerUpdateAndAppend 测试更新和追加日志功能
func TestLogManagerUpdateAndAppend(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建日志仓库
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建日志管理器
	lm := logmgr.NewLogManager(logRepo)

	// 创建测试任务和日志
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	worker := testutils.CreateWorker("test-worker-1", nil)

	// 创建测试日志
	jobLog := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	jobLog.Status = constants.JobStatusRunning
	jobLog.Output = "Initial output"

	// 保存日志
	err := logRepo.Save(jobLog)
	require.NoError(t, err, "Failed to save job log")

	// 测试追加输出
	appendText := "\nMore output lines"
	err = lm.AppendOutput(jobLog.ExecutionID, appendText)
	require.NoError(t, err, "Failed to append output")

	// 验证输出已追加
	updatedLog, err := lm.GetLogByExecutionID(jobLog.ExecutionID)
	require.NoError(t, err, "Failed to get updated log")
	assert.Equal(t, "Initial output"+appendText, updatedLog.Output, "Output should be appended")

	// 测试更新状态
	err = lm.UpdateLogStatus(jobLog.ExecutionID, constants.JobStatusSucceeded, updatedLog.Output+"\nFinal output")
	require.NoError(t, err, "Failed to update log status")

	// 验证状态已更新
	finalLog, err := lm.GetLogByExecutionID(jobLog.ExecutionID)
	require.NoError(t, err, "Failed to get final log")
	assert.Equal(t, constants.JobStatusSucceeded, finalLog.Status, "Status should be updated to succeeded")
	assert.Contains(t, finalLog.Output, "Final output", "Output should be updated")
}

// TestLogManagerSubscription 测试日志订阅功能
func TestLogManagerSubscription(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建日志仓库
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建日志管理器
	lm := logmgr.NewLogManager(logRepo)
	require.NoError(t, lm.Start(), "Failed to start log manager")
	defer lm.Stop()

	// 创建测试任务和日志
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	worker := testutils.CreateWorker("test-worker-1", nil)

	// 创建测试日志
	jobLog := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	jobLog.Status = constants.JobStatusRunning
	jobLog.Output = "Initial output"

	// 保存日志
	err := logRepo.Save(jobLog)
	require.NoError(t, err, "Failed to save job log")

	// 订阅日志更新
	updates, err := lm.SubscribeToLogUpdates(jobLog.ExecutionID)
	require.NoError(t, err, "Failed to subscribe to log updates")

	// 在另一个goroutine中更新日志
	go func() {
		time.Sleep(100 * time.Millisecond)
		err := lm.AppendOutput(jobLog.ExecutionID, "\nNew log line 1")
		if err != nil {
			t.Logf("Failed to append output: %v", err)
		}

		time.Sleep(100 * time.Millisecond)
		err = lm.AppendOutput(jobLog.ExecutionID, "\nNew log line 2")
		if err != nil {
			t.Logf("Failed to append output: %v", err)
		}

		time.Sleep(100 * time.Millisecond)
		err = lm.UpdateLogStatus(jobLog.ExecutionID, constants.JobStatusSucceeded, jobLog.Output+"\nNew log line 3")
		if err != nil {
			t.Logf("Failed to update status: %v", err)
		}
	}()

	// 读取更新
	receivedUpdates := make([]string, 0)
	timeout := time.After(1 * time.Second)

	for i := 0; i < 3; i++ {
		select {
		case update := <-updates:
			receivedUpdates = append(receivedUpdates, update)
		case <-timeout:
			t.Log("Timeout waiting for updates")
			break
		}
	}

	// 取消订阅
	err = lm.UnsubscribeFromLogUpdates(jobLog.ExecutionID)
	require.NoError(t, err, "Failed to unsubscribe from log updates")

	// 验证收到的更新
	assert.GreaterOrEqual(t, len(receivedUpdates), 1, "Should receive at least one update")
	for i, update := range receivedUpdates {
		t.Logf("Update %d: %s", i+1, update)
	}
}

// TestLogManagerStreamOutput 测试流式输出日志功能
func TestLogManagerStreamOutput(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建日志仓库
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建日志管理器
	lm := logmgr.NewLogManager(logRepo)
	require.NoError(t, lm.Start(), "Failed to start log manager")
	defer lm.Stop()

	// 创建测试任务和日志
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	worker := testutils.CreateWorker("test-worker-1", nil)

	// 创建测试日志
	jobLog := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	jobLog.Status = constants.JobStatusRunning
	jobLog.Output = "Initial output"

	// 保存日志
	err := logRepo.Save(jobLog)
	require.NoError(t, err, "Failed to save job log")

	// 准备一个缓冲区接收流式输出
	var buffer bytes.Buffer

	// 在另一个goroutine中更新日志
	go func() {
		time.Sleep(100 * time.Millisecond)
		err := lm.AppendOutput(jobLog.ExecutionID, "\nNew log line 1")
		if err != nil {
			t.Logf("Failed to append output: %v", err)
		}

		time.Sleep(100 * time.Millisecond)
		err = lm.AppendOutput(jobLog.ExecutionID, "\nNew log line 2")
		if err != nil {
			t.Logf("Failed to append output: %v", err)
		}

		time.Sleep(100 * time.Millisecond)
		err = lm.UpdateLogStatus(jobLog.ExecutionID, constants.JobStatusSucceeded, jobLog.Output+"\nFinal log line")
		if err != nil {
			t.Logf("Failed to update status: %v", err)
		}
	}()

	// 流式输出到缓冲区, 设置较短的超时
	err = testutils.WaitWithTimeout(func(ctx context.Context) error {
		return lm.StreamLogOutput(jobLog.ExecutionID, &buffer)
	}, 2*time.Second)

	// 检查输出内容
	output := buffer.String()
	t.Logf("Streamed output: %s", output)
	assert.Contains(t, output, "Initial output", "Output should contain initial content")
	assert.Contains(t, output, "New log line", "Output should contain new content")
}

// TestLogManagerCleanupOldLogs 测试清理旧日志功能
func TestLogManagerCleanupOldLogs(t *testing.T) {
	// 设置测试环境
	etcdClient, mongoClient := testutils.SetupTestEnvironment(t)
	defer testutils.TeardownTestEnvironment(t, etcdClient, mongoClient)

	// 创建日志仓库
	logRepo := repo.NewLogRepo(mongoClient)

	// 创建日志管理器
	lm := logmgr.NewLogManager(logRepo)

	// 创建测试任务和日志
	jobFactory := testutils.NewJobFactory()
	job := jobFactory.CreateSimpleJob()
	worker := testutils.CreateWorker("test-worker-1", nil)

	// 创建3个不同时间的日志
	now := time.Now()

	// 7天前的日志
	log1 := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	log1.Status = constants.JobStatusSucceeded
	log1.StartTime = now.Add(-7 * 24 * time.Hour)
	log1.EndTime = log1.StartTime.Add(5 * time.Minute)

	// 5天前的日志
	log2 := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	log2.Status = constants.JobStatusFailed
	log2.StartTime = now.Add(-5 * 24 * time.Hour)
	log2.EndTime = log2.StartTime.Add(5 * time.Minute)

	// 1天前的日志
	log3 := jobFactory.CreateJobLog(job, worker.ID, worker.IP)
	log3.Status = constants.JobStatusSucceeded
	log3.StartTime = now.Add(-1 * 24 * time.Hour)
	log3.EndTime = log3.StartTime.Add(5 * time.Minute)

	// 保存日志
	require.NoError(t, logRepo.Save(log1), "Failed to save log1")
	require.NoError(t, logRepo.Save(log2), "Failed to save log2")
	require.NoError(t, logRepo.Save(log3), "Failed to save log3")

	// 清理超过3天的日志
	cutoffTime := now.Add(-3 * 24 * time.Hour)
	deleted, err := lm.CleanupOldLogs(cutoffTime)
	require.NoError(t, err, "Failed to cleanup old logs")
	assert.Equal(t, int64(2), deleted, "Should delete 2 logs")

	// 验证仅3天内的日志保留
	logs, total, err := lm.GetLogsByJobID(job.ID, 1, 10)
	require.NoError(t, err, "Failed to get logs by job ID")
	assert.Equal(t, int64(1), total, "Total log count should be 1")
	assert.Len(t, logs, 1, "Should have only 1 log after cleanup")
	assert.Equal(t, log3.ExecutionID, logs[0].ExecutionID, "The remaining log should be the most recent one")
}
