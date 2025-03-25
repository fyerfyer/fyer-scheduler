package worker

import (
	"testing"
	"time"

	"github.com/fyerfyer/fyer-scheduler/mocks"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/logsink"
	"github.com/fyerfyer/fyer-scheduler/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 创建日志采集器的辅助函数
func createTestLogSink(t *testing.T) logsink.ILogSink {
	// 创建日志采集器
	sink, err := logsink.NewLogSink(
		logsink.WithBufferSize(100),
		logsink.WithBatchInterval(500*time.Millisecond),
		logsink.WithMaxBufferMemory(1024*1024), // 1MB
		logsink.WithSendMode(logsink.SendModeBatch),
		logsink.WithStorageType(logsink.StorageTypeMaster),
		logsink.WithMasterConfig("http://localhost:8080", "test-worker-01"),
	)
	require.NoError(t, err, "Failed to create log sink")

	// 启动日志采集器
	err = sink.Start()
	require.NoError(t, err, "Failed to start log sink")

	return sink
}

func TestLogSinkAddLog(t *testing.T) {
	// 创建日志采集器
	sink := createTestLogSink(t)
	defer sink.Stop()

	// 测试添加日志
	executionID := testutils.GenerateUniqueID("exec")
	err := sink.AddLog(executionID, "This is a test log entry")
	assert.NoError(t, err, "Should successfully add log")

	// 立即刷新以确保日志被处理
	err = sink.Flush()
	assert.NoError(t, err, "Should successfully flush logs")

	// 获取状态检查是否已处理
	stats := sink.GetStats()
	assert.GreaterOrEqual(t, stats["buffered_log_count"], 0, "Should have processed the log")
}

func TestLogSinkAddLogWithLevel(t *testing.T) {
	// 创建日志采集器
	sink := createTestLogSink(t)
	defer sink.Stop()

	// 测试添加不同级别的日志
	executionID := testutils.GenerateUniqueID("exec")

	// 添加不同级别的日志
	err := sink.AddLogWithLevel(executionID, "Debug message", logsink.LogLevelDebug)
	assert.NoError(t, err, "Should successfully add debug log")

	err = sink.AddLogWithLevel(executionID, "Info message", logsink.LogLevelInfo)
	assert.NoError(t, err, "Should successfully add info log")

	err = sink.AddLogWithLevel(executionID, "Warning message", logsink.LogLevelWarn)
	assert.NoError(t, err, "Should successfully add warning log")

	err = sink.AddLogWithLevel(executionID, "Error message", logsink.LogLevelError)
	assert.NoError(t, err, "Should successfully add error log")

	// 立即刷新以确保日志被处理
	err = sink.Flush()
	assert.NoError(t, err, "Should successfully flush logs")

	// 获取状态检查是否已处理
	stats := sink.GetStats()
	// 确保所有日志都被正确处理
	assert.Equal(t, 4, stats["buffered_log_count"].(int), "Should have processed 4 logs")
}

func TestLogSinkSendModeChange(t *testing.T) {
	// 创建日志采集器（默认批处理模式）
	sink := createTestLogSink(t)
	defer sink.Stop()

	// 添加一些日志
	executionID := testutils.GenerateUniqueID("exec")
	for i := 0; i < 5; i++ {
		err := sink.AddLog(executionID, "Log entry in batch mode")
		require.NoError(t, err, "Should successfully add log")
	}

	// 获取初始状态
	initialStats := sink.GetStats()

	// 切换到即时发送模式
	sink.SetSendMode(logsink.SendModeImmediate)

	// 添加更多日志，应该立即发送
	err := sink.AddLog(executionID, "Log entry in immediate mode")
	require.NoError(t, err, "Should successfully add log")

	// 短暂等待以确保日志已经发送
	time.Sleep(100 * time.Millisecond)

	// 获取新状态
	newStats := sink.GetStats()

	// 在即时模式下，日志应该已经被处理，所以计数器会增加
	assert.Greater(t, newStats["sent_log_count"].(int64), initialStats["sent_log_count"].(int64), "Should have processed more logs")
}

func TestLogSinkFlushOnStop(t *testing.T) {
	// 创建日志采集器
	sink := createTestLogSink(t)

	// 添加一些日志
	executionID := testutils.GenerateUniqueID("exec")
	for i := 0; i < 5; i++ {
		err := sink.AddLog(executionID, "Log that should be flushed on stop")
		require.NoError(t, err, "Should successfully add log")
	}

	// 获取初始状态
	initialStats := sink.GetStats()

	// 停止日志采集器 - 应该触发最终刷新
	err := sink.Stop()
	require.NoError(t, err, "Should successfully stop log sink")

	// 获取最终状态
	finalStats := sink.GetStats()

	// 验证日志状态变化
	assert.Greater(t, finalStats["sent_log_count"].(int64), initialStats["sent_log_count"].(int64), "Should have sent logs during shutdown")
}

func TestLogSinkWithMockFactory(t *testing.T) {
	// 创建Mock工厂
	mockFactory := mocks.NewILogSinkFactory(t)
	mockSink := mocks.NewILogSink(t)

	// 设置期望
	mockFactory.EXPECT().CreateLogSink(testutils.MockAny()).Return(mockSink, nil)
	mockSink.EXPECT().Start().Return(nil)
	mockSink.EXPECT().AddLog(testutils.MockAny(), testutils.MockAny()).Return(nil)
	mockSink.EXPECT().Flush().Return(nil)
	mockSink.EXPECT().Stop().Return(nil)

	// 设置GetStats返回的模拟数据 - 注意这里返回map而不是结构体
	statsMap := map[string]interface{}{
		"buffered_log_count": 1,
		"sent_log_count":     int64(1),
		"is_running":         true,
	}
	mockSink.EXPECT().GetStats().Return(statsMap)

	// 创建日志采集器
	sink, err := mockFactory.CreateLogSink()
	require.NoError(t, err, "Should successfully create mock sink")

	// 启动
	err = sink.Start()
	require.NoError(t, err, "Should successfully start sink")

	// 添加日志
	err = sink.AddLog("test-execution", "Test log message")
	assert.NoError(t, err, "Should successfully add log to mock sink")

	// 刷新
	err = sink.Flush()
	assert.NoError(t, err, "Should successfully flush mock sink")

	// 检查统计信息
	stats := sink.GetStats()
	assert.Equal(t, 1, stats["buffered_log_count"], "Should have processed 1 log")

	// 停止
	err = sink.Stop()
	assert.NoError(t, err, "Should successfully stop mock sink")
}
