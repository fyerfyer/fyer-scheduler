package jobmgr

import (
	"fmt"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// Scheduler 负责任务的定时调度和分发
type Scheduler struct {
	jobMgr         JobManager
	workerRepo     repo.IWorkerRepo
	checkInterval  time.Duration
	isRunning      bool
	stopChan       chan struct{}
	scheduleLoopWg sync.WaitGroup
}

// NewScheduler 创建一个新的调度器
func NewScheduler(jobMgr JobManager, workerRepo repo.IWorkerRepo, checkInterval time.Duration) IScheduler {
	if checkInterval <= 0 {
		checkInterval = 10 * time.Second // 默认每10秒检查一次
	}

	return &Scheduler{
		jobMgr:        jobMgr,
		workerRepo:    workerRepo,
		checkInterval: checkInterval,
		isRunning:     false,
		stopChan:      make(chan struct{}),
	}
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	if s.isRunning {
		return nil // 已经在运行中
	}

	s.isRunning = true
	s.stopChan = make(chan struct{})

	// 启动调度循环
	s.scheduleLoopWg.Add(1)
	go s.scheduleLoop()

	utils.Info("scheduler started", zap.Duration("check_interval", s.checkInterval))
	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() error {
	if !s.isRunning {
		return nil
	}

	s.isRunning = false
	close(s.stopChan)

	// 等待调度循环结束
	s.scheduleLoopWg.Wait()

	utils.Info("scheduler stopped")
	return nil
}

// scheduleLoop 调度循环，定期检查需要执行的任务
func (s *Scheduler) scheduleLoop() {
	defer s.scheduleLoopWg.Done()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	// 立即执行一次检查
	s.checkDueJobs()

	for {
		select {
		case <-s.stopChan:
			// 收到停止信号，退出循环
			return
		case <-ticker.C:
			// 检查需要执行的任务
			s.checkDueJobs()
		}
	}
}

// checkDueJobs 检查并触发到期的任务
func (s *Scheduler) checkDueJobs() {
	// 获取所有到期的任务
	dueJobs, err := s.jobMgr.GetDueJobs()
	if err != nil {
		utils.Error("failed to get due jobs", zap.Error(err))
		return
	}

	// 如果没有到期任务，直接返回
	if len(dueJobs) == 0 {
		return
	}

	utils.Info("found due jobs", zap.Int("count", len(dueJobs)))

	// 处理每个到期任务
	for _, job := range dueJobs {
		// 只处理已启用的任务
		if !job.Enabled {
			continue
		}

		// 如果任务已在运行中，则跳过
		if job.Status == constants.JobStatusRunning {
			continue
		}

		// 触发任务执行
		err := s.scheduleJob(job)
		if err != nil {
			utils.Error("failed to schedule job",
				zap.String("job_id", job.ID),
				zap.String("name", job.Name),
				zap.Error(err))
		}
	}
}

// scheduleJob 调度执行单个任务
func (s *Scheduler) scheduleJob(job *models.Job) error {
	// 选择合适的Worker
	worker, err := s.selectWorker(job)
	if err != nil {
		return fmt.Errorf("failed to select worker: %w", err)
	}

	// 记录任务将分配给哪个Worker
	utils.Info("assigning job to worker",
		zap.String("job_id", job.ID),
		zap.String("job_name", job.Name),
		zap.String("worker_id", worker.ID),
		zap.String("worker_ip", worker.IP))

	// 标记任务为运行中
	job.SetStatus(constants.JobStatusRunning)

	// 更新任务状态
	if err := s.jobMgr.UpdateJob(job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// 更新Worker的运行任务列表
	worker.AddRunningJob(job.ID)
	if err := s.workerRepo.UpdateRunningJobs(worker); err != nil {
		utils.Warn("failed to update worker running jobs",
			zap.String("worker_id", worker.ID),
			zap.Error(err))
		// 不阻止任务调度继续进行
	}

	utils.Info("job scheduled successfully",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name),
		zap.String("worker_id", worker.ID))

	return nil
}

// selectWorker 选择合适的Worker处理任务
func (s *Scheduler) selectWorker(job *models.Job) (*models.Worker, error) {
	// 获取所有活跃Worker
	workers, err := s.workerRepo.ListActive()
	if err != nil {
		return nil, fmt.Errorf("failed to list active workers: %w", err)
	}

	// 如果没有可用Worker，返回错误
	if len(workers) == 0 {
		return nil, fmt.Errorf("no active workers available")
	}

	// 简单的选择策略：选择运行任务最少的Worker
	var selectedWorker *models.Worker
	var minJobs int = -1

	for _, worker := range workers {
		// 跳过禁用的Worker
		if worker.Status == constants.WorkerStatusDisabled {
			continue
		}

		// 第一个Worker或者任务数更少的Worker
		if minJobs == -1 || len(worker.RunningJobs) < minJobs {
			selectedWorker = worker
			minJobs = len(worker.RunningJobs)
		}
	}

	// 如果没有选择到Worker，返回错误
	if selectedWorker == nil {
		return nil, fmt.Errorf("no suitable worker found")
	}

	return selectedWorker, nil
}

// TriggerImmediateJob 立即触发任务执行
func (s *Scheduler) TriggerImmediateJob(jobID string) error {
	// 获取任务信息
	job, err := s.jobMgr.GetJob(jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// 检查任务是否可以执行
	if !job.IsRunnable() {
		return fmt.Errorf("job is not runnable (disabled or already running)")
	}

	// 调度任务执行
	err = s.scheduleJob(job)
	if err != nil {
		return fmt.Errorf("failed to trigger job: %w", err)
	}

	return nil
}

// GetSchedulerStatus 获取调度器状态
func (s *Scheduler) GetSchedulerStatus() map[string]interface{} {
	return map[string]interface{}{
		"running":         s.isRunning,
		"check_interval":  s.checkInterval.String(),
		"next_check_time": time.Now().Add(s.checkInterval).Format(time.RFC3339),
	}
}
