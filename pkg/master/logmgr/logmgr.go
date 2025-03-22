package logmgr

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// MasterLogManager 实现日志管理器接口
type MasterLogManager struct {
	logRepo             repo.ILogRepo
	activeSubscriptions map[string]*LogSubscription
	subscriptionsMutex  sync.RWMutex
	isRunning           bool
	stopChan            chan struct{}
}

// NewLogManager 创建一个新的日志管理器实例
func NewLogManager(logRepo repo.ILogRepo) LogManager {
	return &MasterLogManager{
		logRepo:             logRepo,
		activeSubscriptions: make(map[string]*LogSubscription),
		stopChan:            make(chan struct{}),
	}
}

// GetLogByExecutionID 根据执行ID获取日志
func (m *MasterLogManager) GetLogByExecutionID(executionID string) (*models.JobLog, error) {
	log, err := m.logRepo.GetByExecutionID(executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get log: %w", err)
	}
	return log, nil
}

// GetLogsByJobID 获取指定任务的执行日志
func (m *MasterLogManager) GetLogsByJobID(jobID string, page, pageSize int64) ([]*models.JobLog, int64, error) {
	logs, total, err := m.logRepo.GetByJobID(jobID, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get logs for job: %w", err)
	}
	return logs, total, nil
}

// GetLatestLogByJobID 获取指定任务的最新日志
func (m *MasterLogManager) GetLatestLogByJobID(jobID string) (*models.JobLog, error) {
	log, err := m.logRepo.GetLatestByJobID(jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest log for job: %w", err)
	}
	return log, nil
}

// GetLogsByTimeRange 获取指定时间范围内的日志
func (m *MasterLogManager) GetLogsByTimeRange(startTime, endTime time.Time, page, pageSize int64) ([]*models.JobLog, int64, error) {
	logs, total, err := m.logRepo.GetByTimeRange(startTime, endTime, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get logs by time range: %w", err)
	}
	return logs, total, nil
}

// GetLogsByWorkerID 获取指定Worker的执行日志
func (m *MasterLogManager) GetLogsByWorkerID(workerID string, page, pageSize int64) ([]*models.JobLog, int64, error) {
	logs, total, err := m.logRepo.GetByWorkerID(workerID, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get logs for worker: %w", err)
	}
	return logs, total, nil
}

// GetLogsByStatus 获取指定状态的日志
func (m *MasterLogManager) GetLogsByStatus(status string, page, pageSize int64) ([]*models.JobLog, int64, error) {
	logs, total, err := m.logRepo.GetByStatus(status, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get logs by status: %w", err)
	}
	return logs, total, nil
}

// GetJobStatusCounts 获取任务状态计数
func (m *MasterLogManager) GetJobStatusCounts(jobID string) (map[string]int64, error) {
	counts, err := m.logRepo.CountByJobStatus(jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job status counts: %w", err)
	}
	return counts, nil
}

// GetJobSuccessRate 获取任务成功率
func (m *MasterLogManager) GetJobSuccessRate(jobID string, period time.Duration) (float64, error) {
	rate, err := m.logRepo.GetJobSuccessRate(jobID, period)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate job success rate: %w", err)
	}
	return rate, nil
}

// GetSystemStatusSummary 获取系统状态摘要
func (m *MasterLogManager) GetSystemStatusSummary() (map[string]interface{}, error) {
	// 获取成功和失败任务的统计数据
	successLogs, _, err := m.logRepo.GetByStatus("succeeded", 1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get success logs: %w", err)
	}

	failedLogs, _, err := m.logRepo.GetByStatus("failed", 1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get failed logs: %w", err)
	}

	// 获取最近24小时的日志
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	recentLogs, _, err := m.logRepo.GetByTimeRange(yesterday, now, 1, 1000) // 假设不会超过1000条
	if err != nil {
		return nil, fmt.Errorf("failed to get recent logs: %w", err)
	}

	// 计算统计信息
	result := map[string]interface{}{
		"total_success_jobs": len(successLogs),
		"total_failed_jobs":  len(failedLogs),
		"jobs_last_24h":      len(recentLogs),
		"timestamp":          time.Now(),
	}

	return result, nil
}

// GetDailyJobExecutionStats 获取每日任务执行统计
func (m *MasterLogManager) GetDailyJobExecutionStats(days int) ([]map[string]interface{}, error) {
	now := time.Now()
	startTime := now.AddDate(0, 0, -days).Truncate(24 * time.Hour) // 截断到天
	endTime := now

	// 获取时间范围内的所有日志
	logs, _, err := m.logRepo.GetByTimeRange(startTime, endTime, 1, 10000) // 假设不会超过10000条
	if err != nil {
		return nil, fmt.Errorf("failed to get logs for daily stats: %w", err)
	}

	// 按天统计
	dailyStats := make(map[string]map[string]int)
	for _, log := range logs {
		day := log.StartTime.Format("2006-01-02")
		if _, exists := dailyStats[day]; !exists {
			dailyStats[day] = map[string]int{
				"total":     0,
				"succeeded": 0,
				"failed":    0,
			}
		}

		dailyStats[day]["total"]++
		if log.Status == "succeeded" {
			dailyStats[day]["succeeded"]++
		} else if log.Status == "failed" {
			dailyStats[day]["failed"]++
		}
	}

	// 转换为切片
	result := make([]map[string]interface{}, 0, len(dailyStats))
	for day, stats := range dailyStats {
		result = append(result, map[string]interface{}{
			"date":         day,
			"total":        stats["total"],
			"succeeded":    stats["succeeded"],
			"failed":       stats["failed"],
			"success_rate": float64(stats["succeeded"]) / float64(stats["total"]) * 100,
		})
	}

	return result, nil
}

// StreamLogOutput 流式输出日志内容
func (m *MasterLogManager) StreamLogOutput(executionID string, writer io.Writer) error {
	// 获取日志
	log, err := m.logRepo.GetByExecutionID(executionID)
	if err != nil {
		return fmt.Errorf("failed to get log: %w", err)
	}

	// 写入现有输出
	if log.Output != "" {
		_, err = writer.Write([]byte(log.Output))
		if err != nil {
			return fmt.Errorf("failed to write log output: %w", err)
		}
	}

	// 如果日志已完成，直接返回
	if log.IsComplete() {
		return nil
	}

	// 订阅更新
	updates, err := m.SubscribeToLogUpdates(executionID)
	if err != nil {
		return fmt.Errorf("failed to subscribe to log updates: %w", err)
	}
	defer m.UnsubscribeFromLogUpdates(executionID)

	// 流式处理新的输出
	for output := range updates {
		_, err = writer.Write([]byte(output))
		if err != nil {
			return fmt.Errorf("failed to write output update: %w", err)
		}
	}

	return nil
}

// SubscribeToLogUpdates 订阅日志更新
func (m *MasterLogManager) SubscribeToLogUpdates(executionID string) (<-chan string, error) {
	m.subscriptionsMutex.Lock()
	defer m.subscriptionsMutex.Unlock()

	// 检查日志是否存在
	_, err := m.logRepo.GetByExecutionID(executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get log: %w", err)
	}

	// 创建订阅
	channel := make(chan string, 100) // 缓冲区，防止阻塞
	subscription := &LogSubscription{
		ExecutionID: executionID,
		Channel:     channel,
		LastUpdated: time.Now(),
	}

	// 添加到活跃订阅列表
	m.activeSubscriptions[executionID] = subscription

	utils.Info("subscribed to log updates",
		zap.String("execution_id", executionID))

	return channel, nil
}

// UnsubscribeFromLogUpdates 取消订阅日志更新
func (m *MasterLogManager) UnsubscribeFromLogUpdates(executionID string) error {
	m.subscriptionsMutex.Lock()
	defer m.subscriptionsMutex.Unlock()

	subscription, exists := m.activeSubscriptions[executionID]
	if !exists {
		return fmt.Errorf("subscription not found for execution ID: %s", executionID)
	}

	// 关闭通道并移除订阅
	close(subscription.Channel)
	delete(m.activeSubscriptions, executionID)

	utils.Info("unsubscribed from log updates",
		zap.String("execution_id", executionID))

	return nil
}

// CleanupOldLogs 清理老日志
func (m *MasterLogManager) CleanupOldLogs(beforeTime time.Time) (int64, error) {
	count, err := m.logRepo.DeleteOldLogs(beforeTime)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old logs: %w", err)
	}

	utils.Info("cleaned up old logs",
		zap.Time("before_time", beforeTime),
		zap.Int64("deleted_count", count))

	return count, nil
}

// UpdateLogStatus 更新日志状态
func (m *MasterLogManager) UpdateLogStatus(executionID, status string, output string) error {
	err := m.logRepo.UpdateLogStatus(executionID, status, output)
	if err != nil {
		return fmt.Errorf("failed to update log status: %w", err)
	}

	// 通知订阅者
	m.notifySubscribers(executionID, output)

	return nil
}

// AppendOutput 追加输出到日志
func (m *MasterLogManager) AppendOutput(executionID string, output string) error {
	err := m.logRepo.AppendOutput(executionID, output)
	if err != nil {
		return fmt.Errorf("failed to append log output: %w", err)
	}

	// 通知订阅者
	m.notifySubscribers(executionID, output)

	return nil
}

// notifySubscribers 通知日志订阅者
func (m *MasterLogManager) notifySubscribers(executionID string, output string) {
	m.subscriptionsMutex.RLock()
	defer m.subscriptionsMutex.RUnlock()

	subscription, exists := m.activeSubscriptions[executionID]
	if !exists {
		return
	}

	// 非阻塞发送
	select {
	case subscription.Channel <- output:
		subscription.LastUpdated = time.Now()
	default:
		// 通道已满，跳过而不阻塞
		utils.Warn("subscriber channel full, update skipped",
			zap.String("execution_id", executionID))
	}
}

// cleanupStaleSubscriptions 清理过期的订阅
func (m *MasterLogManager) cleanupStaleSubscriptions() {
	m.subscriptionsMutex.Lock()
	defer m.subscriptionsMutex.Unlock()

	now := time.Now()
	staleTimeout := 10 * time.Minute

	for executionID, subscription := range m.activeSubscriptions {
		if now.Sub(subscription.LastUpdated) > staleTimeout {
			// 关闭订阅通道
			close(subscription.Channel)
			delete(m.activeSubscriptions, executionID)

			utils.Info("cleaned up stale subscription",
				zap.String("execution_id", executionID),
				zap.Time("last_updated", subscription.LastUpdated))
		}
	}
}

// Start 启动日志管理器
func (m *MasterLogManager) Start() error {
	if m.isRunning {
		return nil // 已经运行中
	}

	m.isRunning = true
	m.stopChan = make(chan struct{})

	// 启动定期清理过期订阅的协程
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.cleanupStaleSubscriptions()
			case <-m.stopChan:
				return
			}
		}
	}()

	utils.Info("log manager started")
	return nil
}

// Stop 停止日志管理器
func (m *MasterLogManager) Stop() error {
	if !m.isRunning {
		return nil
	}

	m.isRunning = false
	close(m.stopChan)

	// 关闭所有活跃的订阅
	m.subscriptionsMutex.Lock()
	for executionID, subscription := range m.activeSubscriptions {
		close(subscription.Channel)
		delete(m.activeSubscriptions, executionID)
	}
	m.subscriptionsMutex.Unlock()

	utils.Info("log manager stopped")
	return nil
}
