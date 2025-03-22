package workermgr

import (
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// WorkerEventType 定义Worker事件类型
type WorkerEventType string

const (
	// 定义Worker事件类型常量
	WorkerEventAdded      WorkerEventType = "WORKER_ADDED"      // Worker添加
	WorkerEventRemoved    WorkerEventType = "WORKER_REMOVED"    // Worker移除
	WorkerEventUpdated    WorkerEventType = "WORKER_UPDATED"    // Worker更新
	WorkerEventOnline     WorkerEventType = "WORKER_ONLINE"     // Worker上线
	WorkerEventOffline    WorkerEventType = "WORKER_OFFLINE"    // Worker离线
	WorkerEventOverloaded WorkerEventType = "WORKER_OVERLOADED" // Worker负载过高
)

// WorkerEvent 定义Worker事件
type WorkerEvent struct {
	Type      WorkerEventType // 事件类型
	Worker    *models.Worker  // 关联的Worker
	Timestamp time.Time       // 事件发生时间
	Data      map[string]any  // 附加数据
}

// WorkerEventHandler 定义事件处理器函数类型
type WorkerEventHandler func(event *WorkerEvent)

// EventManager 管理Worker事件的分发
type EventManager struct {
	handlers      map[WorkerEventType][]WorkerEventHandler
	handlersLock  sync.RWMutex
	workerManager WorkerManager
}

// NewEventManager 创建一个新的事件管理器
func NewEventManager(workerManager WorkerManager) *EventManager {
	return &EventManager{
		handlers:      make(map[WorkerEventType][]WorkerEventHandler),
		workerManager: workerManager,
	}
}

// RegisterHandler 注册事件处理器
func (e *EventManager) RegisterHandler(eventType WorkerEventType, handler WorkerEventHandler) {
	e.handlersLock.Lock()
	defer e.handlersLock.Unlock()

	if _, exists := e.handlers[eventType]; !exists {
		e.handlers[eventType] = make([]WorkerEventHandler, 0)
	}

	e.handlers[eventType] = append(e.handlers[eventType], handler)
	utils.Info("registered handler for worker event", zap.String("event_type", string(eventType)))
}

// UnregisterHandler 注销事件处理器
func (e *EventManager) UnregisterHandler(eventType WorkerEventType, handler WorkerEventHandler) {
	e.handlersLock.Lock()
	defer e.handlersLock.Unlock()

	if handlers, exists := e.handlers[eventType]; exists {
		for i, h := range handlers {
			if &h == &handler {
				// 从切片中移除处理器
				e.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
				utils.Info("unregistered handler for worker event", zap.String("event_type", string(eventType)))
				break
			}
		}
	}
}

// BroadcastEvent 广播事件到所有注册的处理器
func (e *EventManager) BroadcastEvent(event *WorkerEvent) {
	// 添加时间戳
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// 获取与事件类型匹配的处理器
	e.handlersLock.RLock()
	eventHandlers := make([]WorkerEventHandler, 0)

	// 获取特定事件类型的处理器
	if handlers, exists := e.handlers[event.Type]; exists {
		eventHandlers = append(eventHandlers, handlers...)
	}

	// 获取通配符处理器（订阅所有事件类型）
	if handlers, exists := e.handlers["*"]; exists {
		eventHandlers = append(eventHandlers, handlers...)
	}
	e.handlersLock.RUnlock()

	// 没有处理器，直接返回
	if len(eventHandlers) == 0 {
		return
	}

	// 记录事件信息
	utils.Info("broadcasting worker event",
		zap.String("event_type", string(event.Type)),
		zap.String("worker_id", event.Worker.ID),
		zap.String("worker_status", event.Worker.Status))

	// 并发调用所有处理器
	for _, handler := range eventHandlers {
		go handler(event)
	}
}

// HandleWorkerStateChange 处理Worker状态变更
func (e *EventManager) HandleWorkerStateChange(worker *models.Worker, oldStatus string) {
	// 如果状态未变，不处理
	if worker.Status == oldStatus {
		return
	}

	// 根据新状态触发对应事件
	var eventType WorkerEventType
	switch worker.Status {
	case constants.WorkerStatusOnline:
		eventType = WorkerEventOnline
	case constants.WorkerStatusOffline:
		eventType = WorkerEventOffline
	default:
		eventType = WorkerEventUpdated
	}

	// 创建事件并广播
	event := &WorkerEvent{
		Type:   eventType,
		Worker: worker,
		Data: map[string]any{
			"old_status": oldStatus,
			"new_status": worker.Status,
		},
	}

	e.BroadcastEvent(event)
}

// HandleWorkerRegistration 处理Worker注册事件
func (e *EventManager) HandleWorkerRegistration(worker *models.Worker, isNew bool) {
	var eventType WorkerEventType
	if isNew {
		eventType = WorkerEventAdded
	} else {
		eventType = WorkerEventUpdated
	}

	event := &WorkerEvent{
		Type:   eventType,
		Worker: worker,
	}

	e.BroadcastEvent(event)
}

// HandleWorkerRemoval 处理Worker移除事件
func (e *EventManager) HandleWorkerRemoval(worker *models.Worker) {
	event := &WorkerEvent{
		Type:   WorkerEventRemoved,
		Worker: worker,
	}

	e.BroadcastEvent(event)
}

// MonitorWorkerLoad 监控Worker负载，超过阈值时触发事件
func (e *EventManager) MonitorWorkerLoad(loadThreshold float64) {
	workers, err := e.workerManager.GetActiveWorkers()
	if err != nil {
		utils.Error("failed to get active workers for load monitoring", zap.Error(err))
		return
	}

	for _, worker := range workers {
		if worker.LoadAvg > loadThreshold {
			// 创建负载过高事件
			event := &WorkerEvent{
				Type:   WorkerEventOverloaded,
				Worker: worker,
				Data: map[string]any{
					"load_avg":     worker.LoadAvg,
					"threshold":    loadThreshold,
					"running_jobs": len(worker.RunningJobs),
					"memory_free":  worker.MemoryFree,
					"memory_total": worker.MemoryTotal,
					"memory_usage": float64(worker.MemoryTotal-worker.MemoryFree) / float64(worker.MemoryTotal),
				},
			}

			e.BroadcastEvent(event)
		}
	}
}

// Start 启动事件管理器
func (e *EventManager) Start() {
	// 这里可以启动定时任务，如监控Worker负载等
	utils.Info("worker event manager started")
}

// HandleWorkerChange 处理Worker变更事件（从Worker Manager的通知）
func (e *EventManager) HandleWorkerChange(eventType string, worker *models.Worker) {
	if worker == nil {
		return
	}

	var evType WorkerEventType
	switch eventType {
	case "PUT":
		// 对应更新或添加
		evType = WorkerEventUpdated
	case "DELETE":
		// 对应移除
		evType = WorkerEventRemoved
	case "STATUS_CHANGE":
		// 根据状态确定事件类型
		if worker.Status == constants.WorkerStatusOnline {
			evType = WorkerEventOnline
		} else if worker.Status == constants.WorkerStatusOffline {
			evType = WorkerEventOffline
		} else {
			evType = WorkerEventUpdated
		}
	default:
		return // 未知事件类型不处理
	}

	event := &WorkerEvent{
		Type:   evType,
		Worker: worker,
	}

	e.BroadcastEvent(event)
}
