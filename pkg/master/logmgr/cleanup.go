package logmgr

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// LogCleanup 管理日志的清理和归档
type LogCleanup struct {
	logMgr            *MasterLogManager
	retentionDays     int
	archiveEnabled    bool
	archiveDirectory  string
	cleanupInterval   time.Duration
	isRunning         bool
	stopChan          chan struct{}
	cleanupInProgress bool
	lock              sync.Mutex
}

// NewLogCleanup 创建一个新的日志清理管理器
func NewLogCleanup(logMgr *MasterLogManager, retentionDays int, archiveEnabled bool, archiveDirectory string) *LogCleanup {
	return &LogCleanup{
		logMgr:           logMgr,
		retentionDays:    retentionDays,
		archiveEnabled:   archiveEnabled,
		archiveDirectory: archiveDirectory,
		cleanupInterval:  24 * time.Hour, // 默认每天清理一次
		isRunning:        false,
		stopChan:         make(chan struct{}),
	}
}

// Start 启动日志清理服务
func (lc *LogCleanup) Start() error {
	lc.lock.Lock()
	defer lc.lock.Unlock()

	if lc.isRunning {
		return nil
	}

	lc.isRunning = true
	lc.stopChan = make(chan struct{})

	// 启动定期清理协程
	go lc.cleanupLoop()

	utils.Info("log cleanup service started",
		zap.Int("retention_days", lc.retentionDays),
		zap.Bool("archive_enabled", lc.archiveEnabled))

	return nil
}

// Stop 停止日志清理服务
func (lc *LogCleanup) Stop() error {
	lc.lock.Lock()
	defer lc.lock.Unlock()

	if !lc.isRunning {
		return nil
	}

	lc.isRunning = false
	close(lc.stopChan)

	utils.Info("log cleanup service stopped")
	return nil
}

// cleanupLoop 定期执行清理操作
func (lc *LogCleanup) cleanupLoop() {
	ticker := time.NewTicker(lc.cleanupInterval)
	defer ticker.Stop()

	// 立即执行一次清理
	lc.RunCleanup()

	for {
		select {
		case <-ticker.C:
			lc.RunCleanup()
		case <-lc.stopChan:
			return
		}
	}
}

// RunCleanup 执行日志清理
func (lc *LogCleanup) RunCleanup() {
	lc.lock.Lock()
	if lc.cleanupInProgress {
		lc.lock.Unlock()
		utils.Warn("cleanup already in progress, skipping")
		return
	}
	lc.cleanupInProgress = true
	lc.lock.Unlock()

	defer func() {
		lc.lock.Lock()
		lc.cleanupInProgress = false
		lc.lock.Unlock()
	}()

	utils.Info("starting log cleanup process", zap.Int("retention_days", lc.retentionDays))

	// 计算截止时间
	cutoffTime := time.Now().AddDate(0, 0, -lc.retentionDays)

	// 如果启用了归档，则先归档
	if lc.archiveEnabled {
		err := lc.archiveLogs(cutoffTime)
		if err != nil {
			utils.Error("failed to archive logs", zap.Error(err))
		}
	}

	// 执行清理
	count, err := lc.logMgr.CleanupOldLogs(cutoffTime)
	if err != nil {
		utils.Error("failed to cleanup old logs", zap.Error(err))
		return
	}

	utils.Info("log cleanup completed", zap.Int64("deleted_count", count))
}

// archiveLogs 归档指定时间之前的日志
func (lc *LogCleanup) archiveLogs(cutoffTime time.Time) error {
	// 确保归档目录存在
	if err := os.MkdirAll(lc.archiveDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// 生成归档文件名
	archiveFileName := fmt.Sprintf("logs_archive_%s.json", cutoffTime.Format("20060102"))
	archivePath := filepath.Join(lc.archiveDirectory, archiveFileName)

	// 获取需要归档的日志
	logs, _, err := lc.logMgr.GetLogsByTimeRange(time.Time{}, cutoffTime, 1, 1000)
	if err != nil {
		return fmt.Errorf("failed to get logs for archiving: %w", err)
	}

	// 如果没有日志需要归档，直接返回
	if len(logs) == 0 {
		utils.Info("no logs to archive")
		return nil
	}

	// 创建归档文件
	file, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive file: %w", err)
	}
	defer file.Close()

	// 写入日志
	err = lc.exportLogsToFile(logs, file)
	if err != nil {
		return fmt.Errorf("failed to export logs: %w", err)
	}

	utils.Info("logs archived successfully",
		zap.String("archive_file", archivePath),
		zap.Int("log_count", len(logs)))

	return nil
}

// exportLogsToFile 将日志导出到文件
func (lc *LogCleanup) exportLogsToFile(logs []*models.JobLog, writer io.Writer) error {
	// 使用JSON编码器
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	// 导出日志
	return encoder.Encode(logs)
}

// ArchiveAndCleanupByJobID 归档并清理特定任务的日志
func (lc *LogCleanup) ArchiveAndCleanupByJobID(jobID string, keepCount int) error {
	// 获取任务的所有日志
	logs, total, err := lc.logMgr.GetLogsByJobID(jobID, 1, 1000)
	if err != nil {
		return fmt.Errorf("failed to get logs for job: %w", err)
	}

	// 如果日志数量少于保留数，不需要清理
	if int64(len(logs)) <= int64(keepCount) {
		return nil
	}

	// 如果启用了归档，则创建归档文件
	if lc.archiveEnabled {
		// 确保归档目录存在
		if err := os.MkdirAll(lc.archiveDirectory, 0755); err != nil {
			return fmt.Errorf("failed to create archive directory: %w", err)
		}

		// 生成归档文件名
		archiveFileName := fmt.Sprintf("job_%s_logs_%s.json", jobID, time.Now().Format("20060102"))
		archivePath := filepath.Join(lc.archiveDirectory, archiveFileName)

		// 创建归档文件
		file, err := os.Create(archivePath)
		if err != nil {
			return fmt.Errorf("failed to create archive file: %w", err)
		}
		defer file.Close()

		// 写入要归档的日志（最旧的那些日志）
		logsToArchive := logs[keepCount:]
		err = lc.exportLogsToFile(logsToArchive, file)
		if err != nil {
			return fmt.Errorf("failed to export logs: %w", err)
		}

		utils.Info("job logs archived successfully",
			zap.String("job_id", jobID),
			zap.String("archive_file", archivePath),
			zap.Int("archived_log_count", len(logsToArchive)))
	}

	// 执行清理（实际实现中，这里需要添加一个根据任务ID和保留数量清理日志的方法）
	// 由于当前接口和实现中没有这个特定方法，这里仅记录消息
	utils.Info("would clean up old logs for job",
		zap.String("job_id", jobID),
		zap.Int("keep_count", keepCount),
		zap.Int64("total_logs", total))

	return nil
}

// SetRetentionDays 设置日志保留天数
func (lc *LogCleanup) SetRetentionDays(days int) {
	lc.lock.Lock()
	defer lc.lock.Unlock()
	lc.retentionDays = days
	utils.Info("log retention days updated", zap.Int("days", days))
}

// SetArchiveSettings 设置归档设置
func (lc *LogCleanup) SetArchiveSettings(enabled bool, directory string) {
	lc.lock.Lock()
	defer lc.lock.Unlock()
	lc.archiveEnabled = enabled
	if directory != "" {
		lc.archiveDirectory = directory
	}
	utils.Info("log archive settings updated",
		zap.Bool("enabled", enabled),
		zap.String("directory", lc.archiveDirectory))
}

// ManualCleanup 手动触发清理
func (lc *LogCleanup) ManualCleanup(days int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -days)
	return lc.logMgr.CleanupOldLogs(cutoffTime)
}

// GetSettings 获取当前设置
func (lc *LogCleanup) GetSettings() map[string]interface{} {
	lc.lock.Lock()
	defer lc.lock.Unlock()

	return map[string]interface{}{
		"retention_days":    lc.retentionDays,
		"archive_enabled":   lc.archiveEnabled,
		"archive_directory": lc.archiveDirectory,
		"cleanup_interval":  lc.cleanupInterval.String(),
		"is_running":        lc.isRunning,
	}
}
