package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/google/uuid"
)

// Worker 表示一个工作节点
type Worker struct {
	// 基本信息
	ID         string    `json:"id"`          // Worker唯一标识
	Hostname   string    `json:"hostname"`    // 主机名
	IP         string    `json:"ip"`          // IP地址
	CreateTime time.Time `json:"create_time"` // 首次注册时间
	UpdateTime time.Time `json:"update_time"` // 最近更新时间

	// 状态信息
	Status        string    `json:"status"`         // Worker状态：online, offline, disabled
	LastHeartbeat time.Time `json:"last_heartbeat"` // 最近一次心跳时间

	// 资源信息
	CPUCores    int     `json:"cpu_cores"`    // CPU核心数
	MemoryTotal int64   `json:"memory_total"` // 内存总量(MB)
	MemoryFree  int64   `json:"memory_free"`  // 可用内存(MB)
	DiskTotal   int64   `json:"disk_total"`   // 磁盘总量(GB)
	DiskFree    int64   `json:"disk_free"`    // 可用磁盘(GB)
	LoadAvg     float64 `json:"load_avg"`     // 负载平均值

	// 任务信息
	RunningJobs   []string `json:"running_jobs"`   // 正在运行的任务ID列表
	CompletedJobs int      `json:"completed_jobs"` // 已完成任务数量
	FailedJobs    int      `json:"failed_jobs"`    // 失败任务数量

	// 标签信息，用于任务调度策略
	Labels map[string]string `json:"labels"`
}

// NewWorker 创建一个新的Worker实例
func NewWorker(id, hostname, ip string) *Worker {
	if id == "" {
		id = uuid.New().String()
	}

	now := time.Now()
	return &Worker{
		ID:            id,
		Hostname:      hostname,
		IP:            ip,
		CreateTime:    now,
		UpdateTime:    now,
		Status:        constants.WorkerStatusOnline,
		LastHeartbeat: now,
		Labels:        make(map[string]string),
		RunningJobs:   make([]string, 0),
	}
}

// UpdateHeartbeat 更新Worker心跳时间
func (w *Worker) UpdateHeartbeat() {
	now := time.Now()
	w.LastHeartbeat = now
	w.UpdateTime = now
}

// AddRunningJob 添加正在运行的任务
func (w *Worker) AddRunningJob(jobID string) {
	// 检查任务是否已存在
	for _, id := range w.RunningJobs {
		if id == jobID {
			return
		}
	}
	w.RunningJobs = append(w.RunningJobs, jobID)
	w.UpdateTime = time.Now()
}

// RemoveRunningJob 移除正在运行的任务
func (w *Worker) RemoveRunningJob(jobID string) {
	for i, id := range w.RunningJobs {
		if id == jobID {
			// 从切片中移除元素
			w.RunningJobs = append(w.RunningJobs[:i], w.RunningJobs[i+1:]...)
			w.UpdateTime = time.Now()
			return
		}
	}
}

// IncrementCompletedJobs 增加已完成任务计数
func (w *Worker) IncrementCompletedJobs() {
	w.CompletedJobs++
	w.UpdateTime = time.Now()
}

// IncrementFailedJobs 增加失败任务计数
func (w *Worker) IncrementFailedJobs() {
	w.FailedJobs++
	w.UpdateTime = time.Now()
}

// SetStatus 设置Worker状态
func (w *Worker) SetStatus(status string) {
	if status != constants.WorkerStatusOnline &&
		status != constants.WorkerStatusOffline &&
		status != constants.WorkerStatusDisabled {
		return
	}

	w.Status = status
	w.UpdateTime = time.Now()
}

// IsActive 检查Worker是否活跃
func (w *Worker) IsActive() bool {
	// 如果被禁用，则不活跃
	if w.Status == constants.WorkerStatusDisabled {
		return false
	}

	// 如果最后心跳时间超过超时时间，则不活跃
	heartbeatTimeout := time.Duration(constants.WorkerHeartbeatTimeout) * time.Second
	if time.Since(w.LastHeartbeat) > heartbeatTimeout {
		return false
	}

	return true
}

// Key 获取Worker在etcd中的键
func (w *Worker) Key() string {
	return constants.WorkerPrefix + w.ID
}

// ToJSON 将Worker序列化为JSON
func (w *Worker) ToJSON() (string, error) {
	bytes, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal worker error: %w", err)
	}
	return string(bytes), nil
}

// WorkerFromJSON 从JSON反序列化为Worker
func WorkerFromJSON(jsonStr string) (*Worker, error) {
	worker := &Worker{}
	err := json.Unmarshal([]byte(jsonStr), worker)
	if err != nil {
		return nil, fmt.Errorf("unmarshal worker error: %w", err)
	}
	return worker, nil
}
