package jobmgr

import (
	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// JobEventHandler 实现任务事件处理器接口
type JobEventHandler struct {
	jobMgr      IWorkerJobManager           // 任务管理器
	scheduleJob func(job *models.Job) error // 调度任务的函数
	killJob     func(job *models.Job) error // 终止任务的函数
}

// NewJobEventHandler 创建一个新的任务事件处理器
func NewJobEventHandler(
	jobMgr IWorkerJobManager,
	scheduleJob func(job *models.Job) error,
	killJob func(job *models.Job) error,
) *JobEventHandler {
	return &JobEventHandler{
		jobMgr:      jobMgr,
		scheduleJob: scheduleJob,
		killJob:     killJob,
	}
}

// HandleJobEvent 处理任务事件
func (h *JobEventHandler) HandleJobEvent(event *JobEvent) {
	if event == nil {
		utils.Warn("received nil job event")
		return
	}

	if event.Job == nil {
		utils.Warn("received job event with nil job", zap.Int("event_type", int(event.Type)))
		return
	}

	// 处理不同类型的事件
	switch event.Type {
	case JobEventSave:
		h.handleSaveEvent(event.Job)
	case JobEventDelete:
		h.handleDeleteEvent(event.Job)
	case JobEventKill:
		h.handleKillEvent(event.Job)
	default:
		utils.Warn("received unknown job event type",
			zap.Int("event_type", int(event.Type)),
			zap.String("job_id", event.Job.ID))
	}
}

// handleSaveEvent 处理任务保存事件（新增或更新）
func (h *JobEventHandler) handleSaveEvent(job *models.Job) {
	// 检查任务是否分配给当前Worker
	if !h.jobMgr.IsJobAssignedToWorker(job) {
		utils.Debug("job not assigned to this worker, ignoring",
			zap.String("job_id", job.ID),
			zap.String("worker_id", h.jobMgr.GetWorkerID()))
		return
	}

	utils.Info("handling job save event",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name),
		zap.String("status", job.Status))

	// 根据任务状态处理
	switch job.Status {
	case constants.JobStatusPending:
		// 新增任务或待执行任务
		h.handlePendingJob(job)
	case constants.JobStatusRunning:
		// 任务运行中，可能是其他节点上传的状态，或者重启后恢复
		h.handleRunningJob(job)
	case constants.JobStatusSucceeded, constants.JobStatusFailed, constants.JobStatusCancelled:
		// 已完成任务，可能是清理任务或处理后续操作
		h.handleCompletedJob(job)
	default:
		utils.Warn("unknown job status in save event",
			zap.String("job_id", job.ID),
			zap.String("status", job.Status))
	}
}

// handleDeleteEvent 处理任务删除事件
func (h *JobEventHandler) handleDeleteEvent(job *models.Job) {
	utils.Info("handling job delete event",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name))

	// 如果任务正在运行，需要终止它
	if job.Status == constants.JobStatusRunning {
		if h.killJob != nil {
			err := h.killJob(job)
			if err != nil {
				utils.Error("failed to kill deleted job",
					zap.String("job_id", job.ID),
					zap.Error(err))
			}
		}
	}

	// 处理清理工作，例如移除任务相关资源
	h.cleanupJobResources(job)
}

// handleKillEvent 处理任务终止事件
func (h *JobEventHandler) handleKillEvent(job *models.Job) {
	utils.Info("handling job kill event",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name))

	// 终止任务执行
	if h.killJob != nil {
		err := h.killJob(job)
		if err != nil {
			utils.Error("failed to kill job",
				zap.String("job_id", job.ID),
				zap.Error(err))
		}
	}
}

// handlePendingJob 处理待执行的任务
func (h *JobEventHandler) handlePendingJob(job *models.Job) {
	// 只处理已启用的任务
	if !job.Enabled {
		utils.Debug("job is disabled, not scheduling",
			zap.String("job_id", job.ID))
		return
	}

	// 检查任务是否可以执行
	if !job.IsRunnable() {
		utils.Debug("job is not runnable, not scheduling",
			zap.String("job_id", job.ID),
			zap.String("status", job.Status))
		return
	}

	// 调度任务执行
	if h.scheduleJob != nil {
		utils.Info("scheduling pending job",
			zap.String("job_id", job.ID),
			zap.String("name", job.Name))

		err := h.scheduleJob(job)
		if err != nil {
			utils.Error("failed to schedule job",
				zap.String("job_id", job.ID),
				zap.Error(err))
		}
	}
}

// handleRunningJob 处理正在运行的任务
func (h *JobEventHandler) handleRunningJob(job *models.Job) {
	// 检查这个任务是否已经在本地执行中
	runningJobs, err := h.jobMgr.ListRunningJobs()
	if err != nil {
		utils.Error("failed to list running jobs",
			zap.String("job_id", job.ID),
			zap.Error(err))
		return
	}

	// 判断任务是否在运行中列表
	isRunningLocally := false
	for _, runningJob := range runningJobs {
		if runningJob.ID == job.ID {
			isRunningLocally = true
			break
		}
	}

	// 如果任务标记为运行中但本地未运行，可能是状态不一致或分配给此Worker的新任务
	if !isRunningLocally {
		utils.Info("job marked as running but not found locally, checking assignment",
			zap.String("job_id", job.ID))

		// 重新检查任务是否分配给当前Worker
		if h.jobMgr.IsJobAssignedToWorker(job) && h.scheduleJob != nil {
			// 尝试启动任务
			utils.Info("starting job that was assigned to this worker",
				zap.String("job_id", job.ID))

			err := h.scheduleJob(job)
			if err != nil {
				utils.Error("failed to start assigned running job",
					zap.String("job_id", job.ID),
					zap.Error(err))
			}
		}
	}
}

// handleCompletedJob 处理已完成的任务
func (h *JobEventHandler) handleCompletedJob(job *models.Job) {
	// 检查是否需要清理相关资源
	h.cleanupJobResources(job)

	// 记录任务完成事件
	utils.Info("job completed with status",
		zap.String("job_id", job.ID),
		zap.String("status", job.Status),
		zap.String("result", job.LastResult))
}

// cleanupJobResources 清理任务相关资源
func (h *JobEventHandler) cleanupJobResources(job *models.Job) {
	// 这里可以实现清理临时文件、释放资源等操作
	utils.Debug("cleaning up resources for job",
		zap.String("job_id", job.ID))

	// 示例：如果有任务工作目录需要清理
	// 如果有锁需要释放
	// 如果有临时文件需要删除
}
