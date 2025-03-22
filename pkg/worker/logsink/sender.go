package logsink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// LogSender 负责将日志发送到目标存储
type LogSender struct {
	options         *LogSinkOptions    // 配置选项
	httpClient      *http.Client       // HTTP客户端
	mongoClient     *mongo.Client      // MongoDB客户端
	mongoCollection *mongo.Collection  // MongoDB集合
	resultCallback  ResultCallback     // 发送结果回调函数
	failedLogs      []*LogEntry        // 发送失败的日志条目
	failedLogsMutex sync.Mutex         // 保护失败日志的互斥锁
	stats           SenderStats        // 发送统计信息
	statsMutex      sync.RWMutex       // 保护统计信息的互斥锁
	isConnected     bool               // 是否已连接到目标
}

// SenderStats 包含发送器的统计信息
type SenderStats struct {
	SentCount    int64     // 已发送日志数
	FailCount    int64     // 发送失败日志数
	RetryCount   int64     // 重试次数
	LastSendTime time.Time // 最后发送时间
	LastError    string    // 最后一次错误信息
}

// NewLogSender 创建一个新的日志发送器
func NewLogSender(options *LogSinkOptions, resultCallback ResultCallback) (*LogSender, error) {
	sender := &LogSender{
		options:        options,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		resultCallback: resultCallback,
		failedLogs:     make([]*LogEntry, 0),
		isConnected:    false,
	}

	// 如果需要直接存储到MongoDB，初始化MongoDB客户端
	if options.StorageType == StorageTypeMongoDB || options.StorageType == StorageTypeBoth {
		if err := sender.initMongoClient(); err != nil {
			return nil, fmt.Errorf("failed to initialize MongoDB client: %w", err)
		}
	}

	return sender, nil
}

// initMongoClient 初始化MongoDB客户端
func (s *LogSender) initMongoClient() error {
	if s.options.MongoDBURI == "" {
		return fmt.Errorf("MongoDB URI is required for MongoDB storage")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(s.options.MongoDBURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// 测试连接
	err = client.Ping(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	s.mongoClient = client
	s.mongoCollection = client.Database(s.options.MongoDBDatabase).Collection(s.options.MongoDBCollection)
	s.isConnected = true

	utils.Info("connected to MongoDB",
		zap.String("database", s.options.MongoDBDatabase),
		zap.String("collection", s.options.MongoDBCollection))

	return nil
}

// Close 关闭发送器并释放资源
func (s *LogSender) Close() error {
	if s.mongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.mongoClient.Disconnect(ctx); err != nil {
			return fmt.Errorf("failed to disconnect MongoDB client: %w", err)
		}
	}
	return nil
}

// SendBatch 发送一批日志
func (s *LogSender) SendBatch(entries []*LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	s.updateStats(func(stats *SenderStats) {
		stats.LastSendTime = time.Now()
	})

	var err error

	// 根据存储类型发送日志
	switch s.options.StorageType {
	case StorageTypeMaster:
		err = s.sendToMaster(entries)
	case StorageTypeMongoDB:
		err = s.sendToMongoDB(entries)
	case StorageTypeBoth:
		// 同时发送到Master和MongoDB
		err1 := s.sendToMaster(entries)
		err2 := s.sendToMongoDB(entries)

		// 如果有任何一个出错就返回错误
		if err1 != nil || err2 != nil {
			if err1 != nil {
				err = err1
			} else {
				err = err2
			}
		}
	default:
		err = fmt.Errorf("unsupported storage type: %d", s.options.StorageType)
	}

	// 处理发送结果
	if err != nil {
		s.handleSendFailure(entries, err)
		return err
	}

	s.updateStats(func(stats *SenderStats) {
		stats.SentCount += int64(len(entries))
	})

	// 调用结果回调
	if s.resultCallback != nil {
		s.resultCallback(true, entries[0].ExecutionID, len(entries), nil)
	}

	return nil
}

// sendToMaster 通过HTTP发送日志到Master节点
func (s *LogSender) sendToMaster(entries []*LogEntry) error {
	if s.options.MasterURL == "" {
		return fmt.Errorf("master URL is not configured")
	}

	// 将日志条目转换为适合发送的格式
	logsToSend := make([]models.JobLog, 0, len(entries))
	for _, entry := range entries {
		jobLog := models.JobLog{
			ExecutionID: entry.ExecutionID,
			JobID:       entry.JobID,
			Output:      entry.Content,
			WorkerID:    entry.WorkerID,
		}
		logsToSend = append(logsToSend, jobLog)
	}

	// 序列化日志
	jsonData, err := json.Marshal(logsToSend)
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	// 构建URL
	url := fmt.Sprintf("%s/api/v1/logs", s.options.MasterURL)

	// 发送HTTP请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Worker-ID", s.options.WorkerID)

	// 执行发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send logs to master: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("master returned non-OK status: %d", resp.StatusCode)
	}

	utils.Debug("sent logs to master",
		zap.Int("count", len(entries)),
		zap.String("execution_id", entries[0].ExecutionID))

	return nil
}

// sendToMongoDB 直接发送日志到MongoDB
func (s *LogSender) sendToMongoDB(entries []*LogEntry) error {
	if s.mongoCollection == nil {
		return fmt.Errorf("MongoDB collection is not initialized")
	}

	// 将日志条目转换为适合MongoDB存储的文档
	documents := make([]interface{}, 0, len(entries))
	for _, entry := range entries {
		// 创建MongoDB文档
		doc := bson.M{
			"execution_id": entry.ExecutionID,
			"job_id":       entry.JobID,
			"content":      entry.Content,
			"timestamp":    entry.Timestamp,
			"level":        entry.Level,
			"worker_id":    entry.WorkerID,
			"metadata":     entry.Metadata,
		}
		documents = append(documents, doc)
	}

	// 设置上下文超时
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 批量插入文档
	result, err := s.mongoCollection.InsertMany(ctx, documents)
	if err != nil {
		return fmt.Errorf("failed to insert logs to MongoDB: %w", err)
	}

	utils.Debug("sent logs to MongoDB",
		zap.Int("count", len(result.InsertedIDs)),
		zap.String("execution_id", entries[0].ExecutionID))

	return nil
}

// handleSendFailure 处理发送失败的情况
func (s *LogSender) handleSendFailure(entries []*LogEntry, err error) {
	s.updateStats(func(stats *SenderStats) {
		stats.FailCount += int64(len(entries))
		stats.LastError = err.Error()
	})

	// 记录错误日志
	utils.Error("failed to send logs",
		zap.String("execution_id", entries[0].ExecutionID),
		zap.Int("count", len(entries)),
		zap.Error(err))

	// 存储失败的日志以便稍后重试
	s.failedLogsMutex.Lock()
	defer s.failedLogsMutex.Unlock()
	s.failedLogs = append(s.failedLogs, entries...)

	// 调用结果回调
	if s.resultCallback != nil {
		s.resultCallback(false, entries[0].ExecutionID, len(entries), err)
	}
}

// RetryFailedLogs 重试发送失败的日志
func (s *LogSender) RetryFailedLogs() error {
	s.failedLogsMutex.Lock()
	if len(s.failedLogs) == 0 {
		s.failedLogsMutex.Unlock()
		return nil
	}

	// 获取失败的日志并清空列表
	failedLogs := s.failedLogs
	s.failedLogs = make([]*LogEntry, 0)
	s.failedLogsMutex.Unlock()

	// 按执行ID分组日志
	logsByExecID := make(map[string][]*LogEntry)
	for _, entry := range failedLogs {
		logsByExecID[entry.ExecutionID] = append(logsByExecID[entry.ExecutionID], entry)
	}

	// 逐组重试
	var lastErr error
	for _, entries := range logsByExecID {
		s.updateStats(func(stats *SenderStats) {
			stats.RetryCount++
		})

		err := s.SendBatch(entries)
		if err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// GetStats 获取发送统计信息
func (s *LogSender) GetStats() SenderStats {
	s.statsMutex.RLock()
	defer s.statsMutex.RUnlock()
	return s.stats
}

// updateStats 更新统计信息
func (s *LogSender) updateStats(updater func(*SenderStats)) {
	s.statsMutex.Lock()
	defer s.statsMutex.Unlock()
	updater(&s.stats)
}

// GetFailedLogCount 获取待重试的日志数量
func (s *LogSender) GetFailedLogCount() int {
	s.failedLogsMutex.Lock()
	defer s.failedLogsMutex.Unlock()
	return len(s.failedLogs)
}

// SendSingleLog 发送单条日志（用于即时发送模式）
func (s *LogSender) SendSingleLog(entry *LogEntry) error {
	return s.SendBatch([]*LogEntry{entry})
}

// IsConnected 检查是否已连接到目标存储
func (s *LogSender) IsConnected() bool {
	return s.isConnected
}