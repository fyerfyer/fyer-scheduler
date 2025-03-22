package logsink

import (
	"fmt"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// LogSink 实现 ILogSink 接口，是日志采集器的核心组件
type LogSink struct {
	options     *LogSinkOptions   // 配置选项
	buffer      *LogBuffer        // 日志缓冲
	sender      *LogSender        // 日志发送器
	isRunning   bool              // 是否正在运行
	stopChan    chan struct{}     // 停止信号
	flushTicker *time.Ticker      // 定时刷新计时器
	jobCache    map[string]string // 任务ID到名称的缓存 executionID -> jobID
	stats       LogSinkStatus     // 状态统计
	mutex       sync.Mutex        // 互斥锁
	statsLock   sync.RWMutex      // 状态统计锁
}

// NewLogSink 创建一个新的日志采集器
func NewLogSink(options ...LogSinkOption) (ILogSink, error) {
	// 应用配置选项
	opts := CreateLogSinkOptions(options...)

	// 验证选项
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// 创建日志缓冲
	buffer := NewLogBuffer(opts.BufferSize, opts.MaxBufferMemory)

	// 创建日志采集器
	sink := &LogSink{
		options:   opts,
		buffer:    buffer,
		isRunning: false,
		stopChan:  make(chan struct{}),
		jobCache:  make(map[string]string),
		stats: LogSinkStatus{
			SentLogCount:       0,
			FailedLogCount:     0,
			ExecutionLogCounts: make(map[string]int),
			SendMode:           opts.SendMode,
			StorageType:        opts.StorageType,
		},
	}

	// 创建发送器
	sender, err := NewLogSender(opts, sink.handleSendResult)
	if err != nil {
		return nil, fmt.Errorf("failed to create log sender: %w", err)
	}
	sink.sender = sender

	return sink, nil
}

// Start 启动日志采集服务
func (s *LogSink) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.isRunning {
		return nil // 已经在运行中
	}

	s.isRunning = true
	s.stopChan = make(chan struct{})

	// 启动定时刷新
	s.flushTicker = time.NewTicker(s.options.BatchInterval)

	// 后台刷新协程
	go s.flushLoop()

	utils.Info("log sink started",
		zap.Int("buffer_size", s.options.BufferSize),
		zap.Duration("batch_interval", s.options.BatchInterval),
		zap.Int("send_mode", int(s.options.SendMode)))

	return nil
}

// Stop 停止日志采集服务
func (s *LogSink) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.isRunning {
		return nil // 已经停止
	}

	// 停止定时器
	if s.flushTicker != nil {
		s.flushTicker.Stop()
	}

	s.isRunning = false
	close(s.stopChan)

	// 最后一次刷新确保所有日志发送
	err := s.FlushInternal()
	if err != nil {
		utils.Warn("error during final flush", zap.Error(err))
	}

	// 关闭发送器
	if s.sender != nil {
		err = s.sender.Close()
		if err != nil {
			utils.Error("error closing sender", zap.Error(err))
		}
	}

	utils.Info("log sink stopped")
	return nil
}

// AddLog 添加单条日志
func (s *LogSink) AddLog(executionID string, content string) error {
	if executionID == "" {
		return fmt.Errorf("execution ID cannot be empty")
	}

	// 创建带默认级别的日志条目
	return s.AddLogWithLevel(executionID, content, LogLevelInfo)
}

// AddLogWithLevel 添加带级别的日志
func (s *LogSink) AddLogWithLevel(executionID string, content string, level LogLevel) error {
	if executionID == "" {
		return fmt.Errorf("execution ID cannot be empty")
	}

	// 创建日志条目
	entry := &LogEntry{
		ExecutionID: executionID,
		JobID:       s.getJobIDFromCache(executionID),
		Content:     content,
		Timestamp:   time.Now(),
		Level:       level,
		WorkerID:    s.options.WorkerID,
	}

	return s.AddLogEntry(entry)
}

// AddLogEntry 添加完整的日志条目
func (s *LogSink) AddLogEntry(entry *LogEntry) error {
	if !s.isRunning {
		return fmt.Errorf("log sink is not running")
	}

	// 如果JobID为空，尝试从缓存获取
	if entry.JobID == "" {
		entry.JobID = s.getJobIDFromCache(entry.ExecutionID)
	}

	// 添加日志条目到缓冲区
	err := s.buffer.AddEntry(entry)
	if err != nil {
		s.updateStats(func(stats *LogSinkStatus) {
			stats.FailedLogCount++
			stats.LastError = err.Error()
		})
		return fmt.Errorf("failed to add log to buffer: %w", err)
	}

	// 更新统计信息
	s.updateStats(func(stats *LogSinkStatus) {
		stats.BufferedLogCount = s.buffer.Size()
		count := stats.ExecutionLogCounts[entry.ExecutionID]
		stats.ExecutionLogCounts[entry.ExecutionID] = count + 1
	})

	// 根据发送模式决定是否立即发送
	if s.options.SendMode == SendModeImmediate {
		return s.Flush()
	} else if s.options.SendMode == SendModeBufferFull && s.buffer.IsFull() {
		return s.Flush()
	}

	return nil
}

// Flush 立即将当前缓冲区的日志发送出去
func (s *LogSink) Flush() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.FlushInternal()
}

// FlushInternal 内部刷新方法，不加锁
func (s *LogSink) FlushInternal() error {
	if !s.isRunning {
		return fmt.Errorf("log sink is not running")
	}

	if s.buffer.Size() == 0 {
		return nil // 缓冲区为空，无需刷新
	}

	// 获取全部日志批次
	logs := s.buffer.GetBatch(s.buffer.Size())

	// 发送日志
	err := s.sender.SendBatch(logs)
	if err != nil {
		utils.Error("failed to send logs", zap.Error(err), zap.Int("log_count", len(logs)))
		return fmt.Errorf("failed to send logs: %w", err)
	}

	// 更新刷新时间
	s.buffer.UpdateFlushTime()

	// 更新统计信息
	s.updateStats(func(stats *LogSinkStatus) {
		stats.BufferedLogCount = s.buffer.Size()
		stats.LastSendTime = time.Now()
	})

	return nil
}

// FlushExecutionLogs 立即发送指定执行ID的所有日志
func (s *LogSink) FlushExecutionLogs(executionID string) error {
	if executionID == "" {
		return fmt.Errorf("execution ID cannot be empty")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.isRunning {
		return fmt.Errorf("log sink is not running")
	}

	// 获取特定执行ID的日志
	logs := s.buffer.GetEntriesByExecutionID(executionID)

	if len(logs) == 0 {
		return nil // 无日志，不需要发送
	}

	// 发送日志
	err := s.sender.SendBatch(logs)
	if err != nil {
		utils.Error("failed to send execution logs",
			zap.Error(err),
			zap.String("execution_id", executionID),
			zap.Int("log_count", len(logs)))
		return fmt.Errorf("failed to send execution logs: %w", err)
	}

	// 从缓冲区移除已发送的日志
	s.buffer.RemoveEntriesByExecutionID(executionID)

	// 更新统计信息
	s.updateStats(func(stats *LogSinkStatus) {
		stats.BufferedLogCount = s.buffer.Size()
		stats.LastSendTime = time.Now()
		// 清除已发送的执行ID计数
		delete(stats.ExecutionLogCounts, executionID)
	})

	return nil
}

// SetSendMode 设置发送模式
func (s *LogSink) SetSendMode(mode LogSendMode) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.options.SendMode = mode

	s.updateStats(func(stats *LogSinkStatus) {
		stats.SendMode = mode
	})

	utils.Info("log send mode changed", zap.Int("mode", int(mode)))
}

// SetStorageType 设置存储类型
func (s *LogSink) SetStorageType(storageType LogStorageType) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.options.StorageType = storageType

	s.updateStats(func(stats *LogSinkStatus) {
		stats.StorageType = storageType
	})

	utils.Info("log storage type changed", zap.Int("type", int(storageType)))
}

// GetStats 获取日志采集器统计信息
func (s *LogSink) GetStats() map[string]interface{} {
	s.statsLock.RLock()
	defer s.statsLock.RUnlock()

	// 获取发送器统计
	senderStats := SenderStats{}
	if s.sender != nil {
		senderStats = s.sender.GetStats()
	}

	// 复制当前状态
	result := map[string]interface{}{
		"is_running":           s.isRunning,
		"buffered_log_count":   s.buffer.Size(),
		"memory_usage":         s.buffer.MemoryUsage(),
		"sent_log_count":       s.stats.SentLogCount + senderStats.SentCount,
		"failed_log_count":     s.stats.FailedLogCount + senderStats.FailCount,
		"execution_log_counts": s.stats.ExecutionLogCounts,
		"last_send_time":       s.stats.LastSendTime,
		"last_error":           s.stats.LastError,
		"send_mode":            int(s.stats.SendMode),
		"storage_type":         int(s.stats.StorageType),
	}

	return result
}

// 以下是内部辅助方法

// flushLoop 定期刷新缓冲区
func (s *LogSink) flushLoop() {
	for {
		select {
		case <-s.flushTicker.C:
			// 检查是否需要刷新
			if s.buffer.ShouldFlush(s.options.BatchInterval) {
				err := s.Flush()
				if err != nil {
					utils.Error("error during scheduled flush", zap.Error(err))
				}
			}

			// 重试失败的日志
			if s.sender.GetFailedLogCount() > 0 {
				err := s.sender.RetryFailedLogs()
				if err != nil {
					utils.Error("error retrying failed logs", zap.Error(err))
				}
			}

		case <-s.stopChan:
			return
		}
	}
}

// handleSendResult 处理发送结果回调
func (s *LogSink) handleSendResult(success bool, executionID string, logCount int, err error) {
	s.updateStats(func(stats *LogSinkStatus) {
		if success {
			stats.SentLogCount += int64(logCount)
		} else {
			stats.FailedLogCount += int64(logCount)
			if err != nil {
				stats.LastError = err.Error()
			}
		}
	})
}

// updateStats 更新统计信息
func (s *LogSink) updateStats(updater func(*LogSinkStatus)) {
	s.statsLock.Lock()
	defer s.statsLock.Unlock()

	updater(&s.stats)
}

// getJobIDFromCache 从缓存获取JobID
func (s *LogSink) getJobIDFromCache(executionID string) string {
	if jobID, ok := s.jobCache[executionID]; ok {
		return jobID
	}
	return ""
}

// RegisterExecutionJob 注册执行ID和任务ID的映射
func (s *LogSink) RegisterExecutionJob(executionID string, jobID string) {
	if executionID != "" && jobID != "" {
		s.jobCache[executionID] = jobID
	}
}

// LogSinkFactory 创建日志采集器的工厂
type LogSinkFactory struct{}

// NewLogSinkFactory 创建一个新的日志采集器工厂
func NewLogSinkFactory() ILogSinkFactory {
	return &LogSinkFactory{}
}

// CreateLogSink 创建一个新的日志采集器
func (f *LogSinkFactory) CreateLogSink(options ...LogSinkOption) (ILogSink, error) {
	return NewLogSink(options...)
}
