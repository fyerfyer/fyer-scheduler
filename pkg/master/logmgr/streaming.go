package logmgr

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// 定义websocket配置
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 允许所有跨域请求，在生产环境可能需要更严格的设置
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// 缓冲区常量
const (
	maxBufferSize      = 1000 // 单个客户端最大缓冲消息数
	batchSize          = 10   // 批量发送的消息数
	flushInterval      = 500  // 定期刷新间隔（毫秒）
	defaultIdleTimeout = 60   // 默认空闲超时时间（秒）
)

// LogStreamManager 管理日志流的结构体
type LogStreamManager struct {
	logMgr *MasterLogManager
	// 跟踪所有活跃的SSE连接
	sseConnections      map[string]map[string]chan string // executionID -> connectionID -> channel
	sseConnectionsMutex sync.RWMutex
	// 跟踪所有活跃的WebSocket连接
	wsConnections      map[string]map[string]*websocket.Conn // executionID -> connectionID -> conn
	wsConnectionsMutex sync.RWMutex
	// 缓冲管理
	outputBuffers      map[string][]string // executionID -> output buffer
	outputBuffersMutex sync.RWMutex
}

// NewLogStreamManager 创建一个新的日志流管理器
func NewLogStreamManager(logMgr *MasterLogManager) *LogStreamManager {
	return &LogStreamManager{
		logMgr:         logMgr,
		sseConnections: make(map[string]map[string]chan string),
		wsConnections:  make(map[string]map[string]*websocket.Conn),
		outputBuffers:  make(map[string][]string),
	}
}

// HandleSSE 处理SSE连接的HTTP处理器
func (sm *LogStreamManager) HandleSSE(w http.ResponseWriter, r *http.Request, executionID string) {
	// 设置SSE所需的HTTP头部
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 创建一个唯一的连接ID
	connectionID := fmt.Sprintf("%s-%d", r.RemoteAddr, time.Now().UnixNano())

	// 创建消息通道
	messageChan := make(chan string, maxBufferSize)

	// 将连接加入管理器
	sm.sseConnectionsMutex.Lock()
	if _, exists := sm.sseConnections[executionID]; !exists {
		sm.sseConnections[executionID] = make(map[string]chan string)
	}
	sm.sseConnections[executionID][connectionID] = messageChan
	sm.sseConnectionsMutex.Unlock()

	// 确保连接关闭时清理资源
	defer func() {
		sm.sseConnectionsMutex.Lock()
		if conns, exists := sm.sseConnections[executionID]; exists {
			delete(conns, connectionID)
			if len(conns) == 0 {
				delete(sm.sseConnections, executionID)
			}
		}
		sm.sseConnectionsMutex.Unlock()
		close(messageChan)
		utils.Info("SSE connection closed",
			zap.String("execution_id", executionID),
			zap.String("connection_id", connectionID))
	}()

	// 获取日志历史记录
	includeHistory := r.URL.Query().Get("history") == "true"
	if includeHistory {
		log, err := sm.logMgr.GetLogByExecutionID(executionID)
		if err == nil && log.Output != "" {
			// 以行为单位发送历史日志
			lines := strings.Split(log.Output, "\n")
			for _, line := range lines {
				if line != "" {
					fmt.Fprintf(w, "data: %s\n\n", line)
					flusher, ok := w.(http.Flusher)
					if ok {
						flusher.Flush()
					}
					time.Sleep(1 * time.Millisecond) // 防止过快发送
				}
			}
		}
	}

	// 订阅日志更新
	logChan, err := sm.logMgr.SubscribeToLogUpdates(executionID)
	if err != nil {
		fmt.Fprintf(w, "data: {\"error\": \"Failed to subscribe to log updates\"}\n\n")
		flusher, ok := w.(http.Flusher)
		if ok {
			flusher.Flush()
		}
		return
	}

	// 客户端断开连接时自动取消订阅
	defer sm.logMgr.UnsubscribeFromLogUpdates(executionID)

	// 创建带取消功能的上下文，用于感知客户端关闭连接
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 当请求被取消时触发清理
	go func() {
		<-ctx.Done()
		// 客户端已断开连接，执行清理
		utils.Debug("client disconnected from SSE stream",
			zap.String("execution_id", executionID))
	}()

	// 周期性刷新缓冲区的计时器
	flushTicker := time.NewTicker(flushInterval * time.Millisecond)
	defer flushTicker.Stop()

	// 空闲超时
	idleTimeout := defaultIdleTimeout
	idleTimer := time.NewTimer(time.Duration(idleTimeout) * time.Second)
	defer idleTimer.Stop()

	utils.Info("SSE stream started",
		zap.String("execution_id", executionID),
		zap.String("connection_id", connectionID))

	// 临时缓冲区
	var buffer []string

	// 开始循环处理消息
	for {
		select {
		case msg, ok := <-logChan:
			// 重置空闲计时器
			idleTimer.Reset(time.Duration(idleTimeout) * time.Second)

			if !ok {
				// 通道已关闭
				utils.Debug("log channel closed", zap.String("execution_id", executionID))
				return
			}

			// 发送SSE消息
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher, ok := w.(http.Flusher)
			if ok {
				flusher.Flush()
			}

		case <-flushTicker.C:
			// 定期刷新逻辑
			if len(buffer) > 0 {
				for _, msg := range buffer {
					fmt.Fprintf(w, "data: %s\n\n", msg)
				}
				buffer = buffer[:0] // 清空缓冲区

				flusher, ok := w.(http.Flusher)
				if ok {
					flusher.Flush()
				}
			}

		case <-idleTimer.C:
			// 空闲超时
			utils.Info("SSE connection idle timeout",
				zap.String("execution_id", executionID),
				zap.String("connection_id", connectionID))
			return

		case <-ctx.Done():
			// 客户端断开连接
			utils.Debug("client context done", zap.String("execution_id", executionID))
			return
		}
	}
}

// HandleWebSocket 处理WebSocket连接
func (sm *LogStreamManager) HandleWebSocket(w http.ResponseWriter, r *http.Request, executionID string) {
	// 升级HTTP连接为WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		utils.Error("failed to upgrade to websocket", zap.Error(err))
		return
	}

	// 创建一个唯一的连接ID
	connectionID := fmt.Sprintf("%s-%d", r.RemoteAddr, time.Now().UnixNano())

	// 将WebSocket连接加入管理器
	sm.wsConnectionsMutex.Lock()
	if _, exists := sm.wsConnections[executionID]; !exists {
		sm.wsConnections[executionID] = make(map[string]*websocket.Conn)
	}
	sm.wsConnections[executionID][connectionID] = conn
	sm.wsConnectionsMutex.Unlock()

	// 确保连接关闭时清理资源
	defer func() {
		conn.Close()
		sm.wsConnectionsMutex.Lock()
		if conns, exists := sm.wsConnections[executionID]; exists {
			delete(conns, connectionID)
			if len(conns) == 0 {
				delete(sm.wsConnections, executionID)
			}
		}
		sm.wsConnectionsMutex.Unlock()
		utils.Info("WebSocket connection closed",
			zap.String("execution_id", executionID),
			zap.String("connection_id", connectionID))
	}()

	// 订阅日志更新
	logChan, err := sm.logMgr.SubscribeToLogUpdates(executionID)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to subscribe to log updates"))
		return
	}

	// 客户端断开连接时自动取消订阅
	defer sm.logMgr.UnsubscribeFromLogUpdates(executionID)

	// 通过判断WebSocket连接状态来处理客户端断开连接
	go func() {
		for {
			// 读取客户端发送的消息，主要是为了检测连接状态
			_, _, err := conn.ReadMessage()
			if err != nil {
				// 客户端断开了连接
				utils.Debug("websocket client disconnected",
					zap.String("execution_id", executionID),
					zap.Error(err))
				return
			}
		}
	}()

	// 获取日志历史记录
	messageType := websocket.TextMessage
	includeHistory := r.URL.Query().Get("history") == "true"
	if includeHistory {
		log, err := sm.logMgr.GetLogByExecutionID(executionID)
		if err == nil && log.Output != "" {
			// 以行为单位发送历史日志
			lines := strings.Split(log.Output, "\n")
			for _, line := range lines {
				if line != "" {
					conn.WriteMessage(messageType, []byte(line))
					time.Sleep(1 * time.Millisecond) // 防止过快发送
				}
			}
		}
	}

	utils.Info("WebSocket stream started",
		zap.String("execution_id", executionID),
		zap.String("connection_id", connectionID))

	// 批处理缓冲区
	buffer := make([]string, 0, batchSize)

	// 周期性刷新缓冲区的计时器
	flushTicker := time.NewTicker(flushInterval * time.Millisecond)
	defer flushTicker.Stop()

	// 空闲超时
	idleTimeout := defaultIdleTimeout
	idleTimer := time.NewTimer(time.Duration(idleTimeout) * time.Second)
	defer idleTimer.Stop()

	// 开始循环处理消息
	for {
		select {
		case msg, ok := <-logChan:
			// 重置空闲计时器
			idleTimer.Reset(time.Duration(idleTimeout) * time.Second)

			if !ok {
				// 通道已关闭
				utils.Debug("log channel closed for websocket",
					zap.String("execution_id", executionID))
				return
			}

			// 添加消息到缓冲区
			buffer = append(buffer, msg)

			// 如果缓冲区达到批处理大小，立即发送
			if len(buffer) >= batchSize {
				batchMsg := strings.Join(buffer, "\n")
				if err := conn.WriteMessage(messageType, []byte(batchMsg)); err != nil {
					utils.Error("failed to write websocket message", zap.Error(err))
					return
				}
				buffer = buffer[:0] // 清空缓冲区
			}

		case <-flushTicker.C:
			// 定期刷新缓冲区
			if len(buffer) > 0 {
				batchMsg := strings.Join(buffer, "\n")
				if err := conn.WriteMessage(messageType, []byte(batchMsg)); err != nil {
					utils.Error("failed to flush websocket buffer", zap.Error(err))
					return
				}
				buffer = buffer[:0] // 清空缓冲区
			}

		case <-idleTimer.C:
			// 空闲超时
			utils.Info("WebSocket connection idle timeout",
				zap.String("execution_id", executionID),
				zap.String("connection_id", connectionID))
			return
		}
	}
}

// BroadcastLogUpdate 广播日志更新到所有连接的客户端
func (sm *LogStreamManager) BroadcastLogUpdate(executionID, message string) {
	// 分别发送到SSE和WebSocket客户端
	sm.broadcastToSSE(executionID, message)
	sm.broadcastToWebSocket(executionID, message)
}

// broadcastToSSE 广播到SSE客户端
func (sm *LogStreamManager) broadcastToSSE(executionID, message string) {
	sm.sseConnectionsMutex.RLock()
	defer sm.sseConnectionsMutex.RUnlock()

	if connections, exists := sm.sseConnections[executionID]; exists {
		for _, ch := range connections {
			// 非阻塞发送
			select {
			case ch <- message:
				// 成功发送
			default:
				// 通道已满，丢弃消息
				utils.Warn("SSE channel full, message dropped",
					zap.String("execution_id", executionID))
			}
		}
	}
}

// broadcastToWebSocket 广播到WebSocket客户端
func (sm *LogStreamManager) broadcastToWebSocket(executionID, message string) {
	sm.wsConnectionsMutex.RLock()
	defer sm.wsConnectionsMutex.RUnlock()

	if connections, exists := sm.wsConnections[executionID]; exists {
		for id, conn := range connections {
			err := conn.WriteMessage(websocket.TextMessage, []byte(message))
			if err != nil {
				utils.Warn("failed to send message to websocket",
					zap.String("execution_id", executionID),
					zap.String("connection_id", id),
					zap.Error(err))

				// 连接可能已失效，尝试关闭它
				conn.Close()
			}
		}
	}
}

// GetActiveStreamCount 获取当前活跃的流连接数
func (sm *LogStreamManager) GetActiveStreamCount() int {
	sseCount := 0
	wsCount := 0

	sm.sseConnectionsMutex.RLock()
	for _, conns := range sm.sseConnections {
		sseCount += len(conns)
	}
	sm.sseConnectionsMutex.RUnlock()

	sm.wsConnectionsMutex.RLock()
	for _, conns := range sm.wsConnections {
		wsCount += len(conns)
	}
	sm.wsConnectionsMutex.RUnlock()

	return sseCount + wsCount
}

// CleanupIdleConnections 清理空闲连接
func (sm *LogStreamManager) CleanupIdleConnections() {
	utils.Info("cleaning up idle stream connections")
	// 这里可以实现更复杂的清理逻辑
	// 在本实现中，已通过每个连接的空闲超时来自动清理
}
