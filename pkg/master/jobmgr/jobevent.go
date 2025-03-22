package jobmgr

import (
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// EventHandler 处理任务相关事件
type EventHandler struct {
	jobMgr JobManager
}

// NewEventHandler 创建一个新的事件处理器
func NewEventHandler(jobMgr JobManager) *EventHandler {
	return &EventHandler{
		jobMgr: jobMgr,
	}
}

// HandleJobEvent 处理任务事件（主入口函数）
func (h *EventHandler) HandleJobEvent(eventType string, job *models.Job) {
	// 根据事件类型处理
	switch eventType {
	case "PUT":
		h.handleJobUpdate(job)
	case "DELETE":
		h.handleJobDelete(job)
	}
}

// handleJobUpdate 处理任务更新事件
func (h *EventHandler) handleJobUpdate(job *models.Job) {
	if job == nil {
		return
	}

	// 记录事件
	utils.Info("job update event received",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name),
		zap.String("status", job.Status))

	// 根据任务状态处理
	switch job.Status {
	case constants.JobStatusRunning:
		h.handleJobRunning(job)
	case constants.JobStatusSucceeded:
		h.handleJobSucceeded(job)
	case constants.JobStatusFailed:
		h.handleJobFailed(job)
	case constants.JobStatusCancelled:
		h.handleJobCancelled(job)
	}
}

// handleJobDelete 处理任务删除事件
func (h *EventHandler) handleJobDelete(job *models.Job) {
	jobID := "unknown"
	if job != nil {
		jobID = job.ID
	}

	utils.Info("job delete event received", zap.String("job_id", jobID))
}

// handleJobRunning 处理任务开始运行事件
func (h *EventHandler) handleJobRunning(job *models.Job) {
	utils.Info("job started running",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name))
}

// handleJobSucceeded 处理任务成功完成事件
func (h *EventHandler) handleJobSucceeded(job *models.Job) {
	utils.Info("job completed successfully",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name))

	// 如果是定时任务，计算下次运行时间
	if job.CronExpr != "" {
		if err := h.updateNextRunTime(job); err != nil {
			utils.Error("failed to update next run time",
				zap.String("job_id", job.ID),
				zap.Error(err))
		}
	}
}

// handleJobFailed 处理任务执行失败事件
func (h *EventHandler) handleJobFailed(job *models.Job) {
	utils.Warn("job execution failed",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name),
		zap.String("last_result", job.LastResult),
		zap.Int("exit_code", job.LastExitCode))

	// 如果是定时任务，计算下次运行时间
	if job.CronExpr != "" {
		if err := h.updateNextRunTime(job); err != nil {
			utils.Error("failed to update next run time",
				zap.String("job_id", job.ID),
				zap.Error(err))
		}
	}
}

// handleJobCancelled 处理任务被取消事件
func (h *EventHandler) handleJobCancelled(job *models.Job) {
	utils.Info("job was cancelled",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name))

	// 如果是定时任务，计算下次运行时间
	if job.CronExpr != "" {
		if err := h.updateNextRunTime(job); err != nil {
			utils.Error("failed to update next run time",
				zap.String("job_id", job.ID),
				zap.Error(err))
		}
	}
}

// updateNextRunTime 更新任务的下一次运行时间
func (h *EventHandler) updateNextRunTime(job *models.Job) error {
	// 设置上次运行时间为当前时间
	job.LastRunTime = time.Now()

	// 计算下次运行时间
	if err := job.CalculateNextRunTime(); err != nil {
		return err
	}

	// 更新任务
	if err := h.jobMgr.UpdateJob(job); err != nil {
		return err
	}

	utils.Info("scheduled next run time",
		zap.String("job_id", job.ID),
		zap.Time("next_run_time", job.NextRunTime))

	return nil
}

// RegisterWithJobManager 注册事件处理器到任务管理器
func (h *EventHandler) RegisterWithJobManager() {
	h.jobMgr.WatchJobChanges(h.HandleJobEvent)
	utils.Info("job event handler registered with job manager")
}
