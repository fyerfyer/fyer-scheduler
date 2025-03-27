package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/joblock"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/jobmgr"
	"go.uber.org/zap"
)

// Scheduler 实现调度器接口，负责调度和执行任务
type Scheduler struct {
	options         *SchedulerOptions        // 调度器配置选项
	jobQueue        IJobQueue                // 任务队列
	cronParser      *CronParser              // Cron表达式解析器
	lockManager     *joblock.LockManager     // 分布式锁管理器
	jobManager      jobmgr.IWorkerJobManager // Worker节点任务管理器
	executorFunc    JobExecutionFunc         // 任务执行函数
	isRunning       bool                     // 调度器是否运行中
	stopChan        chan struct{}            // 停止信号通道
	workerID        string                   // 当前Worker ID
	runningJobs     sync.Map                 // 正在运行的任务映射表 jobID -> *ScheduledJob
	schedulerStatus SchedulerStatus          // 调度器状态
	statusMutex     sync.RWMutex             // 状态读写锁
	jobEventHandler *jobEventHandler         // 任务事件处理器
}

// jobEventHandler 处理来自JobManager的事件
type jobEventHandler struct {
	scheduler *Scheduler
}

// HandleJobEvent 实现IJobEventHandler接口
func (h *jobEventHandler) HandleJobEvent(event *jobmgr.JobEvent) {
	if event == nil || event.Job == nil {
		return
	}

	switch event.Type {
	case jobmgr.JobEventSave:
		h.scheduler.handleJobSaveEvent(event.Job)
	case jobmgr.JobEventDelete:
		h.scheduler.handleJobDeleteEvent(event.Job)
	case jobmgr.JobEventKill:
		h.scheduler.handleJobKillEvent(event.Job)
	}
}

// NewScheduler 创建一个新的调度器
func NewScheduler(
	jobManager jobmgr.IWorkerJobManager,
	lockManager *joblock.LockManager,
	executorFunc JobExecutionFunc,
	options ...SchedulerOption,
) *Scheduler {
	// 应用默认配置
	opts := NewSchedulerOptions()

	// 应用用户配置选项
	for _, option := range options {
		option(opts)
	}

	// 创建任务队列
	jobQueueOpts := NewJobQueueOptions()
	jobQueueOpts.Capacity = opts.JobQueueCapacity
	jobQueueOpts.SortByPriority = true
	jobQueue := NewJobQueue(jobQueueOpts)

	scheduler := &Scheduler{
		options:      opts,
		jobQueue:     jobQueue,
		cronParser:   NewCronParser(),
		lockManager:  lockManager,
		jobManager:   jobManager,
		executorFunc: executorFunc,
		workerID:     jobManager.GetWorkerID(),
		stopChan:     make(chan struct{}),
		schedulerStatus: SchedulerStatus{
			StartTime: time.Now(),
		},
	}

	// 创建任务事件处理器
	scheduler.jobEventHandler = &jobEventHandler{scheduler: scheduler}

	return scheduler
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()

	if s.isRunning {
		return nil // 已经运行中
	}

	s.isRunning = true
	s.stopChan = make(chan struct{})
	s.schedulerStatus.IsRunning = true
	s.schedulerStatus.StartTime = time.Now()

	// 注册为任务事件处理器
	s.jobManager.RegisterHandler(s.jobEventHandler)

	// 加载现有任务
	go s.loadJobs()

	// 启动调度循环
	go s.scheduleLoop()

	utils.Info("scheduler started",
		zap.String("worker_id", s.workerID),
		zap.Duration("check_interval", s.options.CheckInterval))
	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() error {
	s.statusMutex.Lock()
	wasRunning := s.isRunning
	s.isRunning = false
	s.schedulerStatus.IsRunning = false
	s.statusMutex.Unlock()

	if !wasRunning {
		return nil
	}

	// 发送停止信号
	close(s.stopChan)

	// 等待正在执行的任务完成
	// 这里不会真正等待，因为我们不想阻塞Stop方法
	// 只是记录日志，通知正在执行的任务应该停止
	runningCount := 0
	s.runningJobs.Range(func(key, value interface{}) bool {
		runningCount++
		return true
	})

	utils.Info("scheduler stopped",
		zap.Int("running_jobs", runningCount),
		zap.String("worker_id", s.workerID))
	return nil
}

// AddJob 添加任务到调度器
func (s *Scheduler) AddJob(job *models.Job) error {
	// 验证任务
	if job == nil {
		return fmt.Errorf("job cannot be nil")
	}

	// 计算下一次执行时间
	nextRunTime := time.Time{}
	if job.CronExpr != "" {
		var err error
		nextRunTime, err = s.cronParser.GetNextRunTime(job.CronExpr, time.Now())
		if err != nil {
			return fmt.Errorf("failed to calculate next run time: %w", err)
		}
	}

	// 创建调度任务
	scheduledJob := &ScheduledJob{
		Job:            job,
		NextRunTime:    nextRunTime,
		IsRunning:      false,
		ExecutionCount: 0,
	}

	// 添加到队列
	err := s.jobQueue.Push(scheduledJob)
	if err != nil {
		return fmt.Errorf("failed to add job to queue: %w", err)
	}

	// 更新调度器状态
	s.updateStatus()

	utils.Info("job added to scheduler",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name),
		zap.String("cron", job.CronExpr),
		zap.Time("next_run_time", nextRunTime))
	return nil
}

// RemoveJob 从调度器移除任务
func (s *Scheduler) RemoveJob(jobID string) error {
	// 从队列中移除
	err := s.jobQueue.Remove(jobID)
	if err != nil {
		// 任务可能不在队列中，但可能正在运行
		utils.Debug("job not found in queue, checking running jobs", zap.String("job_id", jobID))
	}

	// 检查任务是否正在运行
	if _, ok := s.runningJobs.Load(jobID); ok {
		utils.Info("job is currently running, marking for removal", zap.String("job_id", jobID))
		// 我们不会强制停止任务，而是标记它，让它在完成后自行清理
		s.runningJobs.Delete(jobID)
	}

	// 更新调度器状态
	s.updateStatus()

	utils.Info("job removed from scheduler", zap.String("job_id", jobID))
	return nil
}

// TriggerJob 立即触发一个任务执行
func (s *Scheduler) TriggerJob(jobID string) error {
	// 先检查任务是否在队列中
	var scheduledJob *ScheduledJob
	var found bool

	// 在队列中查找
	jobs := s.jobQueue.GetAll()
	for _, job := range jobs {
		if job.Job.ID == jobID {
			scheduledJob = job
			found = true
			break
		}
	}

	// 如果不在队列中，可能是之前没有添加过
	if !found {
		// 从Job Manager获取任务
		job, err := s.jobManager.GetJob(jobID)
		if err != nil {
			return fmt.Errorf("job not found: %w", err)
		}

		// 创建临时调度任务
		scheduledJob = &ScheduledJob{
			Job:            job,
			NextRunTime:    time.Now(), // 立即执行
			IsRunning:      false,
			ExecutionCount: 0,
		}
	}

	// 检查任务是否正在运行
	if _, ok := s.runningJobs.Load(jobID); ok {
		return fmt.Errorf("job is already running")
	}

	// 执行任务
	go s.executeJob(scheduledJob)

	utils.Info("job triggered manually",
		zap.String("job_id", jobID),
		zap.String("name", scheduledJob.Job.Name))
	return nil
}

// GetStatus 获取调度器状态
func (s *Scheduler) GetStatus() SchedulerStatus {
	s.statusMutex.RLock()
	defer s.statusMutex.RUnlock()

	status := s.schedulerStatus
	// 复制一份防止外部修改
	return status
}

// UpdateJob 更新已有任务
func (s *Scheduler) UpdateJob(job *models.Job) error {
	// 验证任务
	if job == nil {
		return fmt.Errorf("job cannot be nil")
	}

	// 检查任务是否在队列中
	if !s.jobQueue.Contains(job.ID) {
		// 不在队列中，可能需要添加
		return s.AddJob(job)
	}

	// 计算下一次执行时间
	nextRunTime := time.Time{}
	if job.CronExpr != "" {
		var err error
		nextRunTime, err = s.cronParser.GetNextRunTime(job.CronExpr, time.Now())
		if err != nil {
			return fmt.Errorf("failed to calculate next run time: %w", err)
		}
	}

	// 获取现有任务
	existingJob, found := s.GetJob(job.ID)
	if !found {
		return fmt.Errorf("job not found in scheduler")
	}

	// 保留运行状态
	isRunning := existingJob.IsRunning
	lastExecutionID := existingJob.LastExecutionID
	executionCount := existingJob.ExecutionCount

	// 更新调度任务
	scheduledJob := &ScheduledJob{
		Job:             job,
		NextRunTime:     nextRunTime,
		IsRunning:       isRunning,
		LastExecutionID: lastExecutionID,
		ExecutionCount:  executionCount,
	}

	// 更新队列
	err := s.jobQueue.Update(scheduledJob)
	if err != nil {
		return fmt.Errorf("failed to update job in queue: %w", err)
	}

	// 更新调度器状态
	s.updateStatus()

	utils.Info("job updated in scheduler",
		zap.String("job_id", job.ID),
		zap.String("name", job.Name),
		zap.String("cron", job.CronExpr),
		zap.Time("next_run_time", nextRunTime))
	return nil
}

// GetJob 获取任务详情
func (s *Scheduler) GetJob(jobID string) (*ScheduledJob, bool) {
	// 先检查正在运行的任务
	if job, ok := s.runningJobs.Load(jobID); ok {
		return job.(*ScheduledJob), true
	}

	// 在队列中查找
	jobs := s.jobQueue.GetAll()
	for _, job := range jobs {
		if job.Job.ID == jobID {
			return job, true
		}
	}

	return nil, false
}

// ListJobs 列出所有调度的任务
func (s *Scheduler) ListJobs() []*ScheduledJob {
	// 获取队列中的任务
	queueJobs := s.jobQueue.GetAll()

	// 创建结果映射以去重
	jobMap := make(map[string]*ScheduledJob)

	// 添加队列中的任务
	for _, job := range queueJobs {
		jobMap[job.Job.ID] = job
	}

	// 添加正在运行的任务
	s.runningJobs.Range(func(key, value interface{}) bool {
		jobID := key.(string)
		job := value.(*ScheduledJob)
		jobMap[jobID] = job
		return true
	})

	// 转换为切片
	result := make([]*ScheduledJob, 0, len(jobMap))
	for _, job := range jobMap {
		result = append(result, job)
	}

	return result
}

// 内部方法

// scheduleLoop 调度循环，定期检查需要执行的任务
func (s *Scheduler) scheduleLoop() {
	ticker := time.NewTicker(s.options.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case now := <-ticker.C:
			s.checkDueJobs(now)
			s.updateStatus()
		}
	}
}

// checkDueJobs 检查并执行到期的任务
func (s *Scheduler) checkDueJobs(now time.Time) {
	// 更新状态
	s.statusMutex.Lock()
	s.schedulerStatus.LastScheduleTime = now
	s.schedulerStatus.NextCheckTime = now.Add(s.options.CheckInterval)
	s.statusMutex.Unlock()

	// 获取当前时间之前应该执行的任务
	dueJobs := s.jobQueue.GetDueJobs(now)
	if len(dueJobs) == 0 {
		return
	}

	utils.Debug("found due jobs", zap.Int("count", len(dueJobs)))

	// 检查可用的并发槽
	availableSlots := s.options.MaxConcurrent
	if availableSlots > 0 {
		// 计算正在运行的任务数
		runningCount := 0
		s.runningJobs.Range(func(_, _ interface{}) bool {
			runningCount++
			return true
		})

		// 可用槽 = 最大并发 - 正在运行的任务数
		availableSlots -= runningCount
		if availableSlots <= 0 {
			utils.Warn("max concurrent jobs reached, some due jobs will be delayed",
				zap.Int("running", runningCount),
				zap.Int("max_concurrent", s.options.MaxConcurrent))
			return
		}
	}

	// 按照下一次执行时间排序，执行最早应该执行的任务
	for i, job := range dueJobs {
		// 检查并发限制
		if availableSlots > 0 && i < availableSlots {
			// 移除任务并执行
			if s.jobQueue.Remove(job.Job.ID) == nil {
				go s.executeJob(job)
			}
		} else {
			// 已达到并发限制，停止处理
			break
		}
	}
}

// executeJob 执行单个任务
func (s *Scheduler) executeJob(job *ScheduledJob) {
	jobID := job.Job.ID

	// 标记任务为运行中
	job.IsRunning = true
	s.runningJobs.Store(jobID, job)

	// 更新调度器状态
	s.updateStatus()

	utils.Info("executing job",
		zap.String("job_id", jobID),
		zap.String("name", job.Job.Name))

	// 获取任务锁
	jobLock := s.lockManager.CreateLock(jobID)
	job.Lock = jobLock

	// 尝试获取锁
	acquired, err := jobLock.TryLock()
	if err != nil || !acquired {
		utils.Error("failed to acquire job lock",
			zap.String("job_id", jobID),
			zap.Error(err))

		// 标记任务为非运行状态并重新加入队列
		job.IsRunning = false
		s.runningJobs.Delete(jobID)

		// 计算下一次执行时间
		if job.Job.CronExpr != "" {
			nextTime, err := s.cronParser.GetNextRunTime(job.Job.CronExpr, time.Now())
			if err == nil {
				job.NextRunTime = nextTime
				s.jobQueue.Push(job)
			}
		}

		s.updateStatus()
		return
	}

	// 创建上下文，支持超时和取消
	ctx := context.Background()
	if s.options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.options.Timeout)
		defer cancel()
	}

	// 执行任务前更新任务状态为运行中
	job.Job.Status = constants.JobStatusRunning
	s.jobManager.ReportJobStatus(job.Job, constants.JobStatusRunning)

	// 执行任务
	startTime := time.Now()
	err = s.executorFunc(job)

	// 记录执行信息
	job.ExecutionCount++
	executionTime := time.Since(startTime)

	// 任务完成后的处理
	if err != nil {
		// 任务执行失败
		utils.Error("job execution failed",
			zap.String("job_id", jobID),
			zap.String("name", job.Job.Name),
			zap.Error(err),
			zap.Duration("duration", executionTime))

		// 更新任务状态为失败
		job.Job.Status = constants.JobStatusFailed
		job.Job.LastResult = err.Error()
		s.jobManager.ReportJobStatus(job.Job, constants.JobStatusFailed)

		// 是否需要重试
		if s.options.FailStrategy == FailStrategyRetry && job.RetryCount < s.options.MaxRetries {
			// 更新重试次数
			job.RetryCount++

			utils.Info("scheduling job retry",
				zap.String("job_id", job.Job.ID),
				zap.String("name", job.Job.Name),
				zap.Int("retry_count", job.RetryCount),
				zap.Int("max_retries", s.options.MaxRetries),
				zap.Duration("retry_delay", s.options.RetryDelay))

			// 创建新的调度任务
			go func(retryJob *ScheduledJob) {
				// 等待重试延迟
				time.Sleep(s.options.RetryDelay)

				if !s.isRunning {
					return
				}

				// 重新执行任务
				s.executeJob(retryJob)
			}(job)

			// 更新队列中的任务
			s.jobQueue.Update(job)

			// 释放锁
			return
		}
	} else {
		// 任务执行成功
		job.RetryCount = 0
		utils.Info("job execution completed successfully",
			zap.String("job_id", jobID),
			zap.String("name", job.Job.Name),
			zap.Duration("duration", executionTime))

		// 更新任务状态为成功
		job.Job.Status = constants.JobStatusSucceeded
		job.Job.LastResult = "success"
		s.jobManager.ReportJobStatus(job.Job, constants.JobStatusSucceeded)
	}

	// 释放锁
	_ = jobLock.Unlock()

	// 处理下一次调度
	s.scheduleNextRun(job)

	// 任务执行完毕，从运行映射中移除
	job.IsRunning = false
	s.runningJobs.Delete(jobID)

	// 更新调度器状态
	s.updateStatus()
}

// scheduleNextRun 计算并安排任务的下一次执行
func (s *Scheduler) scheduleNextRun(job *ScheduledJob) {
	// 仅当作业有cron表达式时才继续
	if job.Job.CronExpr == "" {
		return
	}

	// 计算下一次运行时间
	now := time.Now()
	nextRunTime, err := s.cronParser.GetNextRunTime(job.Job.CronExpr, now)
	if err != nil {
		utils.Error("failed to calculate next run time",
			zap.String("job_id", job.Job.ID),
			zap.String("cron", job.Job.CronExpr),
			zap.Error(err))
		return
	}

	// 更新作业的下一次运行时间
	job.NextRunTime = nextRunTime

	// 为下一次执行重置重试计数
	job.RetryCount = 0

	// 检查作业是否已经在队列中，如果在则更新它
	if s.jobQueue.Contains(job.Job.ID) {
		err = s.jobQueue.Update(job)
		if err != nil {
			utils.Error("failed to update job in queue",
				zap.String("job_id", job.Job.ID),
				zap.Error(err))
		}
	} else {
		// 否则将其添加到队列中
		err = s.jobQueue.Push(job)
		if err != nil {
			utils.Error("failed to re-queue job for next execution",
				zap.String("job_id", job.Job.ID),
				zap.Error(err))
		}
	}
}

// loadJobs 加载已分配的任务
func (s *Scheduler) loadJobs() {
	utils.Info("loading existing jobs")

	// 从任务管理器获取所有任务
	jobs, err := s.jobManager.ListJobs()
	if err != nil {
		utils.Error("failed to load jobs", zap.Error(err))
		return
	}

	count := 0
	// 过滤出分配给本Worker的任务并添加到调度器
	for _, job := range jobs {
		// 检查任务是否分配给当前Worker
		if s.jobManager.IsJobAssignedToWorker(job) && job.Enabled {
			err := s.AddJob(job)
			if err != nil {
				utils.Error("failed to add job to scheduler",
					zap.String("job_id", job.ID),
					zap.Error(err))
				continue
			}
			count++
		}
	}

	utils.Info("loaded jobs", zap.Int("count", count))
}

// updateStatus 更新调度器状态信息
func (s *Scheduler) updateStatus() {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()

	// 计算运行中的任务数
	runningCount := 0
	s.runningJobs.Range(func(_, _ interface{}) bool {
		runningCount++
		return true
	})

	// 更新状态
	s.schedulerStatus.JobCount = s.jobQueue.Size() + runningCount
	s.schedulerStatus.RunningJobCount = runningCount

	// 确保IsRunning与当前状态一致
	s.schedulerStatus.IsRunning = s.isRunning
}

// handleJobSaveEvent 处理任务保存事件
func (s *Scheduler) handleJobSaveEvent(job *models.Job) {
	// 检查任务是否分配给当前Worker
	if !s.jobManager.IsJobAssignedToWorker(job) {
		return
	}

	// 检查任务是否已在调度器中
	if _, found := s.GetJob(job.ID); found {
		// 更新现有任务
		s.UpdateJob(job)
	} else if job.Enabled {
		// 添加新任务
		s.AddJob(job)
	}
}

// handleJobDeleteEvent 处理任务删除事件
func (s *Scheduler) handleJobDeleteEvent(job *models.Job) {
	// 从调度器移除任务
	s.RemoveJob(job.ID)
}

// handleJobKillEvent 处理任务终止事件
func (s *Scheduler) handleJobKillEvent(job *models.Job) {
	// 检查任务是否正在本Worker上运行
	if scheduledJob, ok := s.runningJobs.Load(job.ID); ok {
		utils.Info("received kill signal for job",
			zap.String("job_id", job.ID),
			zap.String("name", job.Name))

		// 这里可以添加终止任务的逻辑
		// 但当前实现中，我们只是将任务从运行列表中移除
		// 实际的终止逻辑可能需要通过ExecutorFunc提供的机制实现

		// 更新任务状态
		sJob := scheduledJob.(*ScheduledJob)
		sJob.Job.Status = constants.JobStatusCancelled
		s.jobManager.ReportJobStatus(sJob.Job, constants.JobStatusCancelled)

		// 从运行列表移除
		s.runningJobs.Delete(job.ID)

		// 更新调度器状态
		s.updateStatus()
	}
}
