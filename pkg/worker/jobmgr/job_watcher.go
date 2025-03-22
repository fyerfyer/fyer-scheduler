package jobmgr

import (
	"strings"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// JobWatcher 监控etcd中的任务变化并转发给事件处理器
type JobWatcher struct {
	etcdClient       *utils.EtcdClient  // etcd客户端
	jobMgr           IWorkerJobManager  // 任务管理器
	handlers         []IJobEventHandler // 事件处理器列表
	isRunning        bool               // 是否正在运行
	stopChan         chan struct{}      // 停止信号
	lock             sync.RWMutex       // 保护并发访问
	reconnectBackoff time.Duration      // 重连延迟时间
	maxBackoff       time.Duration      // 最大重连时间
}

// NewJobWatcher 创建新的任务监控器
func NewJobWatcher(etcdClient *utils.EtcdClient, jobMgr IWorkerJobManager) *JobWatcher {
	return &JobWatcher{
		etcdClient:       etcdClient,
		jobMgr:           jobMgr,
		handlers:         make([]IJobEventHandler, 0),
		stopChan:         make(chan struct{}),
		reconnectBackoff: 1 * time.Second,  // 初始重连延迟1秒
		maxBackoff:       30 * time.Second, // 最大延迟30秒
	}
}

// RegisterHandler 注册事件处理器
func (w *JobWatcher) RegisterHandler(handler IJobEventHandler) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.handlers = append(w.handlers, handler)
}

// Start 启动监控
func (w *JobWatcher) Start() error {
	w.lock.Lock()
	defer w.lock.Unlock()

	if w.isRunning {
		return nil
	}

	w.isRunning = true
	w.stopChan = make(chan struct{})

	// 启动监控
	go w.watchKillSignals()
	go w.watchJobChanges()

	utils.Info("job watcher started")
	return nil
}

// Stop 停止监控
func (w *JobWatcher) Stop() error {
	w.lock.Lock()
	defer w.lock.Unlock()

	if !w.isRunning {
		return nil
	}

	w.isRunning = false
	close(w.stopChan)

	utils.Info("job watcher stopped")
	return nil
}

// watchJobChanges 监控任务变化
func (w *JobWatcher) watchJobChanges() {
	utils.Info("starting job changes monitoring")

	for {
		select {
		case <-w.stopChan:
			return
		default:
			// 使用etcd客户端的WatchWithPrefix方法监控变化
			watchClosed := make(chan struct{})
			go func() {
				w.etcdClient.WatchWithPrefix(constants.JobPrefix, func(eventType, key, value string) {
					// 跳过kill信号，由专门的方法处理
					if strings.HasSuffix(key, "/kill") {
						return
					}

					// 解析任务ID
					jobID := key[len(constants.JobPrefix):]

					// 根据事件类型处理
					switch eventType {
					case "PUT": // 任务创建或更新
						if value == "" {
							utils.Warn("received empty job value", zap.String("job_id", jobID))
							return
						}

						// 解析任务数据
						job, err := models.FromJSON(value)
						if err != nil {
							utils.Error("failed to parse job data",
								zap.String("job_id", jobID),
								zap.Error(err))
							return
						}

						// 检查任务是否分配给当前Worker
						if w.jobMgr.IsJobAssignedToWorker(job) {
							utils.Info("job assigned to this worker",
								zap.String("job_id", job.ID),
								zap.String("status", job.Status))
							w.notifyJobEvent(JobEventSave, job)
						}

					case "DELETE": // 任务删除
						// 尝试从缓存获取任务信息
						job, err := w.jobMgr.GetJob(jobID)
						if err != nil {
							// 如果找不到任务，创建一个只有ID的任务对象
							job = &models.Job{ID: jobID}
						}

						if w.jobMgr.IsJobAssignedToWorker(job) {
							utils.Info("job deleted",
								zap.String("job_id", job.ID))
							w.notifyJobEvent(JobEventDelete, job)
						}
					}
				})
				close(watchClosed)
			}()

			// 等待监控结束或被停止
			select {
			case <-w.stopChan:
				return
			case <-watchClosed:
				// 连接断开，等待重新连接
				utils.Warn("job changes watch disconnected, reconnecting...",
					zap.Duration("backoff", w.reconnectBackoff))
				select {
				case <-w.stopChan:
					return
				case <-time.After(w.reconnectBackoff):
					// 使用指数退避策略增加重连延迟
					w.reconnectBackoff *= 2
					if w.reconnectBackoff > w.maxBackoff {
						w.reconnectBackoff = w.maxBackoff
					}
				}
			}
		}
	}
}

// watchKillSignals 监控任务终止信号
func (w *JobWatcher) watchKillSignals() {
	utils.Info("starting kill signals monitoring")

	for {
		select {
		case <-w.stopChan:
			return
		default:
			// 使用etcd客户端的WatchWithPrefix方法监控变化
			watchClosed := make(chan struct{})
			go func() {
				w.etcdClient.WatchWithPrefix(constants.JobPrefix, func(eventType, key, value string) {
					// 只处理PUT事件和kill信号
					if eventType != "PUT" || !strings.HasSuffix(key, "/kill") {
						return
					}

					// 解析任务ID
					fullPath := key[len(constants.JobPrefix):]
					jobID := fullPath[:len(fullPath)-len("/kill")]

					utils.Info("received kill signal",
						zap.String("job_id", jobID))

					// 获取任务信息
					job, err := w.jobMgr.GetJob(jobID)
					if err != nil {
						utils.Warn("received kill signal for unknown job",
							zap.String("job_id", jobID),
							zap.Error(err))
						return
					}

					// 检查任务是否分配给当前Worker
					if w.jobMgr.IsJobAssignedToWorker(job) {
						utils.Info("processing kill signal for job",
							zap.String("job_id", job.ID))
						w.notifyJobEvent(JobEventKill, job)
					}
				})
				close(watchClosed)
			}()

			// 等待监控结束或被停止
			select {
			case <-w.stopChan:
				return
			case <-watchClosed:
				// 连接断开，等待重新连接
				utils.Warn("kill signals watch disconnected, reconnecting...",
					zap.Duration("backoff", w.reconnectBackoff))
				select {
				case <-w.stopChan:
					return
				case <-time.After(w.reconnectBackoff):
					// 使用指数退避策略增加重连延迟
					w.reconnectBackoff *= 2
					if w.reconnectBackoff > w.maxBackoff {
						w.reconnectBackoff = w.maxBackoff
					}
				}
			}
		}
	}
}

// notifyJobEvent 通知所有处理器任务事件
func (w *JobWatcher) notifyJobEvent(eventType JobEventType, job *models.Job) {
	if job == nil {
		return
	}

	// 创建任务事件
	event := &JobEvent{
		Type: eventType,
		Job:  job,
	}

	// 复制处理器列表避免并发问题
	w.lock.RLock()
	handlers := make([]IJobEventHandler, len(w.handlers))
	copy(handlers, w.handlers)
	w.lock.RUnlock()

	// 通知所有处理器
	for _, handler := range handlers {
		handler.HandleJobEvent(event)
	}
}

// GetEventHandlers 获取当前注册的事件处理器
func (w *JobWatcher) GetEventHandlers() []IJobEventHandler {
	w.lock.RLock()
	defer w.lock.RUnlock()

	handlers := make([]IJobEventHandler, len(w.handlers))
	copy(handlers, w.handlers)
	return handlers
}

// ResetBackoff 重置重连延迟
func (w *JobWatcher) ResetBackoff() {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.reconnectBackoff = 1 * time.Second
}
