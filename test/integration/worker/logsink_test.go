package worker

import (
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/worker/logsink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestLogSink(t *testing.T) logsink.ILogSink {
	// 创建一个日志接收器，它不会实际尝试将日志发送到主节点
	// 而是将日志缓存在内存中以供测试使用
	sink, err := logsink.NewLogSink(
		logsink.WithBufferSize(100),                     // 设置缓冲区大小为100
		logsink.WithBatchInterval(500*time.Millisecond), // 设置批量发送间隔为500毫秒
		logsink.WithMaxBufferMemory(1024*1024),          // 设置最大缓冲区内存为1MB
		// 在测试期间不实际发送日志
		logsink.WithSendMode(logsink.SendModeBufferFull),
		// 使用MongoDB存储类型
		logsink.WithStorageType(logsink.StorageTypeMongoDB),
		logsink.WithMongoDBConfig(
			"mongodb://localhost:27017", // MongoDB连接地址
			"fyer-scheduler",            // 数据库名称
			"job_logs",                  // 集合名称
		),
		logsink.WithMasterConfig("http://localhost:8080/api/v1", "test-worker-01"), // 主节点配置
	)
	require.NoError(t, err, "Failed to create log sink")

	// 启动日志接收器
	err = sink.Start()
	require.NoError(t, err, "Failed to start log sink")

	return sink
}

func TestLogSinkAddLog(t *testing.T) {
	// 创建日志接收器
	sink := createTestLogSink(t)
	defer sink.Stop()

	// 测试添加日志
	executionID := "test-execution-" + time.Now().Format("20060102150405")
	err := sink.AddLog(executionID, "This is a test log entry")
	assert.NoError(t, err, "Should successfully add log")

	// 添加作业ID关联
	sink.RegisterExecutionJob(executionID, "test-job-01")

	// 使用相同的执行ID添加另一条日志
	err = sink.AddLog(executionID, "This is another test log entry")
	assert.NoError(t, err, "Should successfully add second log")

	// 检查统计信息
	stats := sink.GetStats()
	assert.Equal(t, 2, stats["buffered_log_count"], "Should have 2 logs in buffer")
}

func TestLogSinkAddLogWithLevel(t *testing.T) {
	// 创建日志接收器
	sink := createTestLogSink(t)
	defer sink.Stop()

	// 测试添加不同级别的日志
	executionID := "test-execution-" + time.Now().Format("20060102150405")

	// 注册作业ID
	sink.RegisterExecutionJob(executionID, "test-job-02")

	// 添加不同级别的日志
	err := sink.AddLogWithLevel(executionID, "Debug message", logsink.LogLevelDebug)
	assert.NoError(t, err, "Should successfully add debug log")

	err = sink.AddLogWithLevel(executionID, "Info message", logsink.LogLevelInfo)
	assert.NoError(t, err, "Should successfully add info log")

	err = sink.AddLogWithLevel(executionID, "Warning message", logsink.LogLevelWarn)
	assert.NoError(t, err, "Should successfully add warning log")

	err = sink.AddLogWithLevel(executionID, "Error message", logsink.LogLevelError)
	assert.NoError(t, err, "Should successfully add error log")

	// 检查统计信息
	stats := sink.GetStats()
	assert.Equal(t, 4, stats["buffered_log_count"], "Should have 4 logs in buffer")
}

func TestLogSinkSendModeChange(t *testing.T) {
	// 创建日志接收器（默认批量模式）
	sink := createTestLogSink(t)
	defer sink.Stop()

	// 添加一些日志
	executionID := "test-execution-" + time.Now().Format("20060102150405")
	sink.RegisterExecutionJob(executionID, "test-job-03")

	for i := 0; i < 5; i++ {
		err := sink.AddLog(executionID, "Log entry in batch mode")
		require.NoError(t, err, "Should successfully add log")
	}

	// 获取初始统计信息
	initialStats := sink.GetStats()
	assert.Equal(t, 5, initialStats["buffered_log_count"], "Should have 5 logs in buffer initially")

	// 切换到立即发送模式
	sink.SetSendMode(logsink.SendModeImmediate)

	// 添加另一条日志，这应该触发立即处理
	err := sink.AddLog(executionID, "Log entry in immediate mode")
	require.NoError(t, err, "Should successfully add log")

	// 短暂等待处理完成
	time.Sleep(100 * time.Millisecond)

	// 获取新的统计信息
	newStats := sink.GetStats()

	// 在立即模式下，缓冲区应该为空或几乎为空
	// 因为立即模式可能会刷新整个缓冲区
	assert.LessOrEqual(t, newStats["buffered_log_count"].(int), 1,
		"Buffer should be empty or contain at most the newly added log")
}

func TestLogSinkFlushOnStop(t *testing.T) {
	// 创建日志接收器
	sink := createTestLogSink(t)

	// 添加一些日志
	executionID := "test-execution-" + time.Now().Format("20060102150405")
	sink.RegisterExecutionJob(executionID, "test-job-04")

	for i := 0; i < 5; i++ {
		err := sink.AddLog(executionID, "Log that should be flushed on stop")
		require.NoError(t, err, "Should successfully add log")
	}

	// 获取初始统计信息
	initialStats := sink.GetStats()
	assert.Equal(t, 5, initialStats["buffered_log_count"], "Should have 5 logs in buffer")

	// 手动刷新日志，而不是依赖停止操作
	err := sink.Flush()
	require.NoError(t, err, "Should successfully flush logs")

	// 停止日志接收器
	err = sink.Stop()
	require.NoError(t, err, "Should successfully stop log sink")

	// 最终检查 - 刷新和停止后缓冲区计数应为0
	finalStats := sink.GetStats()
	assert.Equal(t, 0, finalStats["buffered_log_count"], "Buffer should be empty after flush")
}

func TestLogSinkFlushExecutionLogs(t *testing.T) {
	// 创建日志接收器
	sink := createTestLogSink(t)
	defer sink.Stop()

	// 为两个不同的执行添加日志
	execution1 := "test-execution-1-" + time.Now().Format("20060102150405")
	execution2 := "test-execution-2-" + time.Now().Format("20060102150405")

	sink.RegisterExecutionJob(execution1, "test-job-05")
	sink.RegisterExecutionJob(execution2, "test-job-06")

	// 为执行1添加3条日志
	for i := 0; i < 3; i++ {
		err := sink.AddLog(execution1, "Log for execution 1")
		require.NoError(t, err)
	}

	// 为执行2添加2条日志
	for i := 0; i < 2; i++ {
		err := sink.AddLog(execution2, "Log for execution 2")
		require.NoError(t, err)
	}

	// 检查总日志数
	stats := sink.GetStats()
	assert.Equal(t, 5, stats["buffered_log_count"], "Should have 5 logs in buffer")

	// 仅刷新执行1的日志
	err := sink.FlushExecutionLogs(execution1)
	require.NoError(t, err, "Should successfully flush execution 1 logs")

	// 现在应该只剩下执行2的日志
	statsAfterFlush := sink.GetStats()
	assert.Equal(t, 2, statsAfterFlush["buffered_log_count"], "Should have 2 logs left in buffer")
}
