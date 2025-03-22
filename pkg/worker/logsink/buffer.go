package logsink

import (
	"fmt"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// LogBuffer 管理日志缓冲区
type LogBuffer struct {
	entries         []*LogEntry            // 缓冲区中的日志条目
	mutex           sync.RWMutex           // 并发访问保护
	maxSize         int                    // 缓冲区最大条目数
	maxMemory       int64                  // 最大内存使用量(字节)
	currentMemory   int64                  // 当前内存使用量(字节)
	executionGroups map[string][]*LogEntry // 按执行ID分组的日志条目
	lastFlushTime   time.Time              // 上次刷新时间
}

// NewLogBuffer 创建一个新的日志缓冲区
func NewLogBuffer(maxSize int, maxMemory int64) *LogBuffer {
	if maxSize <= 0 {
		maxSize = DefaultBufferSize
	}
	if maxMemory <= 0 {
		maxMemory = DefaultMaxBufferMemory
	}

	return &LogBuffer{
		entries:         make([]*LogEntry, 0, maxSize),
		executionGroups: make(map[string][]*LogEntry),
		maxSize:         maxSize,
		maxMemory:       maxMemory,
		currentMemory:   0,
		lastFlushTime:   time.Now(),
	}
}

// AddEntry 添加日志条目到缓冲区
func (b *LogBuffer) AddEntry(entry *LogEntry) error {
	if entry == nil {
		return fmt.Errorf("cannot add nil entry to buffer")
	}

	// 估算日志条目内存大小(保守估计)
	entrySize := int64(len(entry.Content) + len(entry.ExecutionID) + len(entry.JobID) + 200)

	b.mutex.Lock()
	defer b.mutex.Unlock()

	// 检查是否达到内存限制
	if b.currentMemory+entrySize > b.maxMemory {
		utils.Warn("buffer memory limit reached, entry will be rejected",
			zap.Int64("current_memory", b.currentMemory),
			zap.Int64("max_memory", b.maxMemory),
			zap.String("execution_id", entry.ExecutionID))
		return fmt.Errorf("buffer memory limit reached: %d bytes", b.currentMemory)
	}

	// 添加到缓冲区
	b.entries = append(b.entries, entry)
	b.currentMemory += entrySize

	// 添加到执行ID分组
	if _, exists := b.executionGroups[entry.ExecutionID]; !exists {
		b.executionGroups[entry.ExecutionID] = make([]*LogEntry, 0, 10)
	}
	b.executionGroups[entry.ExecutionID] = append(b.executionGroups[entry.ExecutionID], entry)

	utils.Debug("added log entry to buffer",
		zap.String("execution_id", entry.ExecutionID),
		zap.Int("buffer_size", len(b.entries)))

	return nil
}

// GetBatch 获取指定大小的日志批次
func (b *LogBuffer) GetBatch(batchSize int) []*LogEntry {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if len(b.entries) == 0 {
		return make([]*LogEntry, 0)
	}

	if batchSize <= 0 || batchSize > len(b.entries) {
		batchSize = len(b.entries)
	}

	// 获取前batchSize个条目
	batch := b.entries[0:batchSize]

	// 更新缓冲区，移除已获取的条目
	b.entries = b.entries[batchSize:]

	// 更新内存使用量和分组
	var releasedMemory int64 = 0
	executionUpdates := make(map[string]bool)

	for _, entry := range batch {
		entrySize := int64(len(entry.Content) + len(entry.ExecutionID) + len(entry.JobID) + 200)
		releasedMemory += entrySize
		executionUpdates[entry.ExecutionID] = true
	}

	// 更新内存使用量
	b.currentMemory -= releasedMemory

	// 更新执行ID分组
	for execID := range executionUpdates {
		b.updateExecutionGroup(execID)
	}

	utils.Debug("retrieved batch from buffer",
		zap.Int("batch_size", len(batch)),
		zap.Int("remaining_entries", len(b.entries)))

	return batch
}

// GetEntriesByExecutionID 获取指定执行ID的所有日志条目
func (b *LogBuffer) GetEntriesByExecutionID(executionID string) []*LogEntry {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	entries, exists := b.executionGroups[executionID]
	if !exists {
		return make([]*LogEntry, 0)
	}

	// 创建副本以避免外部修改
	result := make([]*LogEntry, len(entries))
	copy(result, entries)

	return result
}

// RemoveEntriesByExecutionID 从缓冲区移除指定执行ID的所有日志条目
func (b *LogBuffer) RemoveEntriesByExecutionID(executionID string) int {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	entries, exists := b.executionGroups[executionID]
	if !exists {
		return 0
	}

	// 创建新的entries切片，排除要删除的条目
	newEntries := make([]*LogEntry, 0, len(b.entries)-len(entries))
	var removedMemory int64 = 0

	for _, entry := range b.entries {
		if entry.ExecutionID != executionID {
			newEntries = append(newEntries, entry)
		} else {
			entrySize := int64(len(entry.Content) + len(entry.ExecutionID) + len(entry.JobID) + 200)
			removedMemory += entrySize
		}
	}

	removedCount := len(entries)

	// 更新缓冲区
	b.entries = newEntries
	b.currentMemory -= removedMemory
	delete(b.executionGroups, executionID)

	utils.Info("removed entries from buffer",
		zap.String("execution_id", executionID),
		zap.Int("removed_count", removedCount))

	return removedCount
}

// Clear 清空缓冲区
func (b *LogBuffer) Clear() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.entries = make([]*LogEntry, 0, b.maxSize)
	b.executionGroups = make(map[string][]*LogEntry)
	b.currentMemory = 0
	b.lastFlushTime = time.Now()

	utils.Info("buffer cleared")
}

// Size 获取缓冲区大小(条目数)
func (b *LogBuffer) Size() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return len(b.entries)
}

// MemoryUsage 获取当前内存使用量(字节)
func (b *LogBuffer) MemoryUsage() int64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.currentMemory
}

// IsFull 检查缓冲区是否已满
func (b *LogBuffer) IsFull() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return len(b.entries) >= b.maxSize
}

// ShouldFlush 检查是否应该刷新缓冲区
func (b *LogBuffer) ShouldFlush(forceDuration time.Duration) bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	// 缓冲区为空不需要刷新
	if len(b.entries) == 0 {
		return false
	}

	// 缓冲区已满需要刷新
	if len(b.entries) >= b.maxSize {
		return true
	}

	// 内存使用量达到限制需要刷新
	if b.currentMemory >= b.maxMemory {
		return true
	}

	// 超过强制刷新间隔需要刷新
	if forceDuration > 0 && time.Since(b.lastFlushTime) > forceDuration {
		return true
	}

	return false
}

// updateExecutionGroup 更新执行ID对应的分组
// 调用者必须持有锁
func (b *LogBuffer) updateExecutionGroup(executionID string) {
	newGroup := make([]*LogEntry, 0, 10)

	for _, entry := range b.entries {
		if entry.ExecutionID == executionID {
			newGroup = append(newGroup, entry)
		}
	}

	if len(newGroup) > 0 {
		b.executionGroups[executionID] = newGroup
	} else {
		delete(b.executionGroups, executionID)
	}
}

// ExecutionCount 获取缓冲区中的执行ID数量
func (b *LogBuffer) ExecutionCount() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return len(b.executionGroups)
}

// GetAllExecutionIDs 获取所有执行ID
func (b *LogBuffer) GetAllExecutionIDs() []string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	ids := make([]string, 0, len(b.executionGroups))
	for id := range b.executionGroups {
		ids = append(ids, id)
	}

	return ids
}

// UpdateFlushTime 更新最后刷新时间
func (b *LogBuffer) UpdateFlushTime() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.lastFlushTime = time.Now()
}
