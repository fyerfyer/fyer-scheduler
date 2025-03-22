package executor

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// OutputBuffer 管理命令输出缓冲区
type OutputBuffer struct {
	// 缓冲区
	buffer bytes.Buffer
	// 互斥锁保护并发访问
	mutex sync.RWMutex
	// 最大缓冲区大小(字节)
	maxSize int
	// 当前大小
	currentSize int
	// 是否已截断
	truncated bool
	// 最后更新时间
	lastUpdate time.Time
	// 通知新输出的通道
	notifyCh chan struct{}
}

// NewOutputBuffer 创建一个新的输出缓冲区
func NewOutputBuffer(maxSize int) *OutputBuffer {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // 默认10MB
	}

	return &OutputBuffer{
		maxSize:     maxSize,
		lastUpdate:  time.Now(),
		notifyCh:    make(chan struct{}, 1),
		truncated:   false,
		currentSize: 0,
	}
}

// Write 写入数据到缓冲区
func (b *OutputBuffer) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	// 如果添加这些数据会超过最大大小，则进行截断处理
	if b.currentSize+len(p) > b.maxSize {
		remaining := b.maxSize - b.currentSize
		if remaining > 0 {
			// 写入部分数据
			n, err = b.buffer.Write(p[:remaining])
			b.currentSize += n
			b.truncated = true

			// 添加截断消息
			truncateMsg := "\n... output truncated ..."
			b.buffer.WriteString(truncateMsg)
			b.currentSize += len(truncateMsg)
		} else {
			// 已经没有空间，返回写入的字节数
			return 0, nil
		}
	} else {
		// 有足够空间，写入全部数据
		n, err = b.buffer.Write(p)
		b.currentSize += n
	}

	// 更新时间戳
	b.lastUpdate = time.Now()

	// 非阻塞地通知有新输出
	select {
	case b.notifyCh <- struct{}{}:
	default:
		// 通道已满，跳过这次通知
	}

	return n, err
}

// GetContent 获取当前缓冲区内容
func (b *OutputBuffer) GetContent() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.buffer.String()
}

// GetSize 获取当前缓冲区大小
func (b *OutputBuffer) GetSize() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.currentSize
}

// IsTruncated 检查输出是否被截断
func (b *OutputBuffer) IsTruncated() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.truncated
}

// Reset 重置缓冲区
func (b *OutputBuffer) Reset() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.buffer.Reset()
	b.currentSize = 0
	b.truncated = false
	b.lastUpdate = time.Now()
}

// GetNotifyChannel 获取通知通道
func (b *OutputBuffer) GetNotifyChannel() <-chan struct{} {
	return b.notifyCh
}

// LastUpdateTime 获取最后更新时间
func (b *OutputBuffer) LastUpdateTime() time.Time {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.lastUpdate
}

// OutputManager 管理命令输出
type OutputManager struct {
	// 输出缓冲区
	buffer *OutputBuffer
	// 执行ID
	executionID string
	// 执行上下文
	execCtx *ExecutionContext
	// 订阅者映射
	subscribers map[string]chan string
	// 订阅者锁
	subMutex sync.RWMutex
	// 流处理器
	streamHandlers []func(string)
	// 流处理器锁
	handlerMutex sync.RWMutex
}

// NewOutputManager 创建一个新的输出管理器
func NewOutputManager(executionID string, maxSize int, execCtx *ExecutionContext) *OutputManager {
	return &OutputManager{
		buffer:         NewOutputBuffer(maxSize),
		executionID:    executionID,
		execCtx:        execCtx,
		subscribers:    make(map[string]chan string),
		streamHandlers: make([]func(string), 0),
	}
}

// AddOutput 添加输出内容
func (m *OutputManager) AddOutput(content string) error {
	if content == "" {
		return nil
	}

	// 写入缓冲区
	_, err := m.buffer.Write([]byte(content))
	if err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	// 通知订阅者
	m.notifySubscribers(content)

	// 调用流处理器
	m.processOutput(content)

	// 如果有报告器，向报告器报告输出
	if m.execCtx != nil && m.execCtx.Reporter != nil {
		if err := m.execCtx.Reporter.ReportOutput(m.executionID, content); err != nil {
			utils.Warn("failed to report output",
				zap.String("execution_id", m.executionID),
				zap.Error(err))
		}
	}

	return nil
}

// GetOutput 获取完整输出
func (m *OutputManager) GetOutput() string {
	return m.buffer.GetContent()
}

// GetOutputSize 获取输出大小
func (m *OutputManager) GetOutputSize() int {
	return m.buffer.GetSize()
}

// SubscribeOutput 订阅输出流
func (m *OutputManager) SubscribeOutput(subscriberID string, bufferSize int) (<-chan string, error) {
	if bufferSize <= 0 {
		bufferSize = 100 // 默认缓冲区大小
	}

	m.subMutex.Lock()
	defer m.subMutex.Unlock()

	// 检查是否已存在
	if _, exists := m.subscribers[subscriberID]; exists {
		return nil, fmt.Errorf("subscriber already exists: %s", subscriberID)
	}

	// 创建新的通道
	ch := make(chan string, bufferSize)
	m.subscribers[subscriberID] = ch

	utils.Debug("output subscriber added",
		zap.String("execution_id", m.executionID),
		zap.String("subscriber_id", subscriberID))

	return ch, nil
}

// UnsubscribeOutput 取消订阅输出流
func (m *OutputManager) UnsubscribeOutput(subscriberID string) {
	m.subMutex.Lock()
	defer m.subMutex.Unlock()

	if ch, exists := m.subscribers[subscriberID]; exists {
		close(ch)
		delete(m.subscribers, subscriberID)
		utils.Debug("output subscriber removed",
			zap.String("execution_id", m.executionID),
			zap.String("subscriber_id", subscriberID))
	}
}

// AddStreamHandler 添加输出流处理器
func (m *OutputManager) AddStreamHandler(handler func(string)) {
	m.handlerMutex.Lock()
	defer m.handlerMutex.Unlock()
	m.streamHandlers = append(m.streamHandlers, handler)
}

// RemoveAllStreamHandlers 移除所有流处理器
func (m *OutputManager) RemoveAllStreamHandlers() {
	m.handlerMutex.Lock()
	defer m.handlerMutex.Unlock()
	m.streamHandlers = make([]func(string), 0)
}

// Close 关闭输出管理器
func (m *OutputManager) Close() {
	// 关闭所有订阅者
	m.subMutex.Lock()
	for id, ch := range m.subscribers {
		close(ch)
		delete(m.subscribers, id)
	}
	m.subMutex.Unlock()

	// 移除所有处理器
	m.RemoveAllStreamHandlers()
}

// notifySubscribers 通知所有订阅者
func (m *OutputManager) notifySubscribers(content string) {
	m.subMutex.RLock()
	defer m.subMutex.RUnlock()

	for id, ch := range m.subscribers {
		select {
		case ch <- content:
			// 成功发送
		default:
			// 通道已满，关闭并移除此订阅者
			utils.Warn("output subscriber channel full, removing",
				zap.String("execution_id", m.executionID),
				zap.String("subscriber_id", id))

			go func(subscriberID string) {
				m.UnsubscribeOutput(subscriberID)
			}(id)
		}
	}
}

// processOutput 处理输出内容
func (m *OutputManager) processOutput(content string) {
	m.handlerMutex.RLock()
	handlers := make([]func(string), len(m.streamHandlers))
	copy(handlers, m.streamHandlers)
	m.handlerMutex.RUnlock()

	for _, handler := range handlers {
		handler(content)
	}
}

// WaitForOutput 等待输出更新或超时
func (m *OutputManager) WaitForOutput(timeout time.Duration) (string, bool) {
	notifyCh := m.buffer.GetNotifyChannel()

	select {
	case <-notifyCh:
		return m.GetOutput(), true
	case <-time.After(timeout):
		return m.GetOutput(), false
	}
}

// OutputWriter 实现io.Writer接口，用于直接写入OutputManager
type OutputWriter struct {
	manager *OutputManager
}

// NewOutputWriter 创建一个新的输出写入器
func NewOutputWriter(manager *OutputManager) *OutputWriter {
	return &OutputWriter{
		manager: manager,
	}
}

// Write 实现io.Writer接口
func (w *OutputWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	// 将字节转换为字符串并添加到管理器
	content := string(p)
	if err := w.manager.AddOutput(content); err != nil {
		return 0, err
	}

	return len(p), nil
}

// CombineOutputWriters 合并多个io.Writer
func CombineOutputWriters(writers ...io.Writer) io.Writer {
	return &combinedWriter{
		writers: writers,
	}
}

// combinedWriter 将多个io.Writer合并为一个
type combinedWriter struct {
	writers []io.Writer
}

// Write 实现io.Writer接口
func (w *combinedWriter) Write(p []byte) (n int, err error) {
	for _, writer := range w.writers {
		_, err := writer.Write(p)
		if err != nil {
			return 0, err
		}
	}
	return len(p), nil
}
