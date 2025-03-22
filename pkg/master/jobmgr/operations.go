package jobmgr

import (
	"fmt"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// JobOperations 提供任务操作相关的高级功能
type JobOperations struct {
	jobMgr         JobManager
	workerRepo     repo.IWorkerRepo
	logRepo        repo.ILogRepo
	workerSelector IWorkerSelector
	scheduler      IScheduler
}

// NewJobOperations 创建一个新的任务操作实例
func NewJobOperations(
	jobMgr JobManager,
	workerRepo repo.IWorkerRepo,
	logRepo repo.ILogRepo,
	workerSelector IWorkerSelector,
	scheduler IScheduler,
) *JobOperations {
	return &JobOperations{
		jobMgr:         jobMgr,
		workerRepo:     workerRepo,
		logRepo:        logRepo,
		workerSelector: workerSelector,
		scheduler:      scheduler,
	}
}

// TriggerJobExecution 立即触发一个任务执行
func (o *JobOperations) TriggerJobExecution(jobID string) (string, error) {
	// 获取任务信息
	job, err := o.jobMgr.GetJob(jobID)
	if err != nil {
		return "", fmt.Errorf("failed to get job: %w", err)
	}

	// 验证任务是否可执行
	if !job.Enabled {
		return "", fmt.Errorf("cannot trigger disabled job")
	}

	if job.Status == constants.JobStatusRunning {
		return "", fmt.Errorf("job is already running")
	}

	// 选择一个Worker执行任务
	worker, err := o.workerSelector.SelectWorker(job)
	if err != nil {
		return "", fmt.Errorf("failed to select worker: %w", err)
	}

	// 创建执行日志
	jobLog := models.NewJobLog(job.ID, job.Name, worker.ID, worker.IP, job.Command, true)

	// 保存执行日志
	err = o.logRepo.Save(jobLog)
	if err != nil {
		utils.Error("failed to save execution log",
			zap.String("job_id", job.ID),
			zap.Error(err))
		// 继续执行，不因日志保存失败而阻止任务执行
	}

	// 更新任务状态为运行中
	job.SetStatus(constants.JobStatusRunning)
	job.LastRunTime = time.Now()

	if err := o.jobMgr.UpdateJob(job); err != nil {
		return "", fmt.Errorf("failed to update job status: %w", err)
	}

	// 更新Worker状态
	worker.AddRunningJob(job.ID)
	if err := o.workerRepo.UpdateRunningJobs(worker); err != nil {
		utils.Warn("failed to update worker running jobs",
			zap.String("worker_id", worker.ID),
			zap.Error(err))
		// 不阻塞任务执行
	}

	utils.Info("job triggered manually",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name),
		zap.String("execution_id", jobLog.ExecutionID),
		zap.String("worker_id", worker.ID))

	return jobLog.ExecutionID, nil
}

// CancelJobExecution 取消一个正在运行的任务
func (o *JobOperations) CancelJobExecution(jobID, reason string) error {
	// 获取任务信息
	job, err := o.jobMgr.GetJob(jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// 验证任务是否在运行
	if job.Status != constants.JobStatusRunning {
		return fmt.Errorf("job is not running")
	}

	// 获取最新的执行日志
	jobLog, err := o.logRepo.GetLatestByJobID(jobID)
	if err != nil {
		utils.Warn("failed to get latest execution log",
			zap.String("job_id", jobID),
			zap.Error(err))
		// 继续执行，不因获取日志失败而阻止取消操作
	} else {
		// 更新日志状态
		jobLog.SetCancelled(reason)
		if err := o.logRepo.Save(jobLog); err != nil {
			utils.Error("failed to update execution log status",
				zap.String("job_id", jobID),
				zap.String("execution_id", jobLog.ExecutionID),
				zap.Error(err))
		}
	}

	// 更新任务状态
	job.SetStatus(constants.JobStatusCancelled)
	if err := o.jobMgr.UpdateJob(job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// 如果有Cron表达式，计算下次运行时间
	if job.CronExpr != "" {
		if err := job.CalculateNextRunTime(); err != nil {
			utils.Warn("failed to calculate next run time",
				zap.String("job_id", jobID),
				zap.Error(err))
		} else {
			// 更新下次运行时间
			if err := o.jobMgr.UpdateJob(job); err != nil {
				utils.Warn("failed to update next run time",
					zap.String("job_id", jobID),
					zap.Error(err))
			}
		}
	}

	utils.Info("job cancelled",
		zap.String("job_id", jobID),
		zap.String("name", job.Name),
		zap.String("reason", reason))

	return nil
}

// RetryFailedJob 重试之前失败的任务
func (o *JobOperations) RetryFailedJob(jobID string) (string, error) {
	// 获取任务信息
	job, err := o.jobMgr.GetJob(jobID)
	if err != nil {
		return "", fmt.Errorf("failed to get job: %w", err)
	}

	// 验证任务状态
	if job.Status != constants.JobStatusFailed {
		return "", fmt.Errorf("only failed jobs can be retried")
	}

	// 执行逻辑与TriggerJobExecution类似
	return o.TriggerJobExecution(jobID)
}

// BatchTriggerJobs 批量触发多个任务
func (o *JobOperations) BatchTriggerJobs(jobIDs []string) (map[string]string, map[string]error) {
	results := make(map[string]string)
	errors := make(map[string]error)

	for _, jobID := range jobIDs {
		executionID, err := o.TriggerJobExecution(jobID)
		if err != nil {
			errors[jobID] = err
		} else {
			results[jobID] = executionID
		}
	}

	return results, errors
}

// GetJobStatistics 获取任务统计信息
func (o *JobOperations) GetJobStatistics(jobID string) (map[string]interface{}, error) {
	// 获取任务信息
	job, err := o.jobMgr.GetJob(jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// 获取状态统计
	statusCounts, err := o.logRepo.CountByJobStatus(jobID)
	if err != nil {
		utils.Warn("failed to get job status counts",
			zap.String("job_id", jobID),
			zap.Error(err))
		statusCounts = make(map[string]int64)
	}

	// 获取成功率
	successRate, err := o.logRepo.GetJobSuccessRate(jobID, 30*24*time.Hour) // 30天内
	if err != nil {
		utils.Warn("failed to calculate job success rate",
			zap.String("job_id", jobID),
			zap.Error(err))
		successRate = 0
	}

	// 构建结果
	stats := map[string]interface{}{
		"job_id":       job.ID,
		"name":         job.Name,
		"status":       job.Status,
		"enabled":      job.Enabled,
		"cron_expr":    job.CronExpr,
		"last_run":     job.LastRunTime,
		"next_run":     job.NextRunTime,
		"status_count": statusCounts,
		"success_rate": successRate,
		"last_result":  job.LastResult,
		"last_exit":    job.LastExitCode,
	}

	return stats, nil
}

// RunOnceOnSpecificWorker 在指定Worker上执行一次性任务
func (o *JobOperations) RunOnceOnSpecificWorker(jobID, workerID string) (string, error) {
	// 获取任务信息
	job, err := o.jobMgr.GetJob(jobID)
	if err != nil {
		return "", fmt.Errorf("failed to get job: %w", err)
	}

	// 验证任务是否可执行
	if !job.Enabled {
		return "", fmt.Errorf("cannot trigger disabled job")
	}

	if job.Status == constants.JobStatusRunning {
		return "", fmt.Errorf("job is already running")
	}

	// 获取指定Worker
	worker, err := o.workerRepo.GetByID(workerID)
	if err != nil {
		return "", fmt.Errorf("failed to get worker: %w", err)
	}

	// 检查Worker状态
	if worker.Status == constants.WorkerStatusDisabled {
		return "", fmt.Errorf("cannot run job on disabled worker")
	}

	if !worker.IsActive() {
		return "", fmt.Errorf("worker is not active")
	}

	// 创建执行日志
	jobLog := models.NewJobLog(job.ID, job.Name, worker.ID, worker.IP, job.Command, true)

	// 保存执行日志
	err = o.logRepo.Save(jobLog)
	if err != nil {
		utils.Error("failed to save execution log",
			zap.String("job_id", job.ID),
			zap.Error(err))
		// 继续执行，不因日志保存失败而阻止任务执行
	}

	// 更新任务状态为运行中
	job.SetStatus(constants.JobStatusRunning)
	job.LastRunTime = time.Now()

	if err := o.jobMgr.UpdateJob(job); err != nil {
		return "", fmt.Errorf("failed to update job status: %w", err)
	}

	// 更新Worker状态
	worker.AddRunningJob(job.ID)
	if err := o.workerRepo.UpdateRunningJobs(worker); err != nil {
		utils.Warn("failed to update worker running jobs",
			zap.String("worker_id", worker.ID),
			zap.Error(err))
		// 不阻塞任务执行
	}

	utils.Info("job triggered on specific worker",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name),
		zap.String("execution_id", jobLog.ExecutionID),
		zap.String("worker_id", worker.ID))

	return jobLog.ExecutionID, nil
}
