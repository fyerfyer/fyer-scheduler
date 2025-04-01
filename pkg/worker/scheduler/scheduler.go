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
	// 先获取作业
	schedJob, found := s.GetJob(jobID)
	if !found {
		// 如果在调度器中找不到，尝试从jobManager加载
		job, err := s.jobManager.GetJob(jobID)
		if err != nil {
			return fmt.Errorf("failed to get job: %w", err)
		}

		// 检查作业是否分配给当前Worker
		if !s.jobManager.IsJobAssignedToWorker(job) {
			utils.Info("skipping job trigger - not assigned to this worker",
				zap.String("job_id", jobID),
				zap.String("worker_id", s.workerID))
			return nil // 不是错误，只是这个worker不处理这个作业
		}

		// 继续执行...
	} else {
		// 作业已在调度器中，检查是否已分配给当前Worker
		origJob, err := s.jobManager.GetJob(jobID)
		if err == nil && !s.jobManager.IsJobAssignedToWorker(origJob) {
			utils.Info("skipping job trigger - not assigned to this worker",
				zap.String("job_id", jobID),
				zap.String("worker_id", s.workerID))
			return nil
		}
	}

	// 执行作业...
	go s.executeJob(schedJob)

	utils.Info("job triggered manually",
		zap.String("job_id", jobID),
		zap.String("name", schedJob.Job.Name))

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
		scheduledJob := job.(*ScheduledJob)
		utils.Debug("GetJob found running job",
			zap.String("job_id", jobID),
			zap.Bool("is_running", scheduledJob.IsRunning))
		return scheduledJob, true
	}

	// 在队列中查找
	jobs := s.jobQueue.GetAll()
	for _, job := range jobs {
		if job.Job.ID == jobID {
			utils.Debug("GetJob found job in queue",
				zap.String("job_id", jobID),
				zap.Bool("is_running", job.IsRunning))
			return job, true
		}
	}

	utils.Debug("GetJob could not find job", zap.String("job_id", jobID))
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
	// 先打印详细日志
	utils.Info("starting job execution",
		zap.String("job_id", job.Job.ID),
		zap.String("name", job.Job.Name),
		zap.String("worker_id", s.workerID))

	// 首先检查作业是否分配给当前Worker
	if (!s.jobManager.IsJobAssignedToWorker(job.Job)) {
		utils.Info("job not assigned to this worker, skipping execution",
			zap.String("job_id", job.Job.ID),
			zap.String("worker_id", s.workerID))
		return
	}

	// 创建任务锁
	lockOpts := joblock.FromLockOptions(joblock.DefaultLockOptions())
	lockID := "job-" + job.Job.ID
	lock := s.lockManager.CreateLock(lockID, lockOpts...)

	utils.Info("attempting to acquire lock for job",
		zap.String("job_id", job.Job.ID),
		zap.String("lock_id", lockID),
		zap.String("worker_id", s.workerID))

	// 尝试获取锁
	acquired, err := lock.TryLock()
	if err != nil {
		utils.Error("failed to acquire job lock",
			zap.String("job_id", job.Job.ID),
			zap.Error(err))
		return
	}

	if !acquired {
		utils.Info("job lock already acquired by another worker, skipping execution",
			zap.String("job_id", job.Job.ID),
			zap.String("worker_id", s.workerID))
		return
	}

	// 成功获取锁，记录日志
	utils.Info("job lock acquired successfully",
		zap.String("job_id", job.Job.ID),
		zap.String("worker_id", s.workerID),
		zap.String("lock_id", lockID))

	// 标记任务为运行中
	job.Lock = lock
	job.IsRunning = true
	job.ExecutionCount++
	job.LastExecutionID = fmt.Sprintf("%s-%d", job.Job.ID, time.Now().Unix())

	// 添加到运行映射
	s.runningJobs.Store(job.Job.ID, job)

	// 更新调度器状态
	s.updateStatus()

	// 创建上下文，支持超时和取消
	_, cancel := context.WithTimeout(context.Background(), s.options.Timeout)
	defer cancel()

	// 执行任务前更新任务状态为运行中
	err = s.jobManager.ReportJobStatus(job.Job, constants.JobStatusRunning)
	if err != nil {
		utils.Warn("failed to report job running status",
			zap.String("job_id", job.Job.ID),
			zap.Error(err))
	}

	// 设置作业的开始执行时间
	startTime := time.Now()
	job.Job.LastRunTime = startTime

	// 执行任务
	utils.Info("executing job",
		zap.String("job_id", job.Job.ID),
		zap.String("name", job.Job.Name),
		zap.String("worker_id", s.workerID))

	var execErr error
	if s.executorFunc != nil {
		execErr = s.executorFunc(job)
	}

	// 记录结束时间
	endTime := time.Now()

	// 任务完成后的处理
	var status string
	if execErr != nil {
		status = constants.JobStatusFailed
		utils.Error("job execution failed",
			zap.String("job_id", job.Job.ID),
			zap.String("name", job.Job.Name),
			zap.Error(execErr))

		// 处理失败策略
		if s.options.FailStrategy == FailStrategyRetry && job.RetryCount < s.options.MaxRetries {
			job.RetryCount++
			utils.Info("scheduling job for retry",
				zap.String("job_id", job.Job.ID),
				zap.Int("retry_count", job.RetryCount),
				zap.Int("max_retries", s.options.MaxRetries))

			// 重要修改：直接使用goroutine处理重试，而不是依赖调度周期
			if job.RetryCount <= s.options.MaxRetries {
				go func(retryJob *ScheduledJob) {
					// 等待重试延迟
					time.Sleep(s.options.RetryDelay)

					// 检查调度器是否还在运行
					if !s.isRunning {
						return
					}

					// 重新执行任务
					utils.Info("retrying job execution",
						zap.String("job_id", retryJob.Job.ID),
						zap.Int("retry_attempt", retryJob.RetryCount))
					s.executeJob(retryJob)
				}(job)
			}
		} else {
			utils.Info("no more retries for job",
				zap.String("job_id", job.Job.ID),
				zap.Int("retry_count", job.RetryCount))

			// 不重试，重置重试计数
			job.RetryCount = 0
		}
	} else {
		status = constants.JobStatusSucceeded
		utils.Info("job execution succeeded",
			zap.String("job_id", job.Job.ID),
			zap.String("name", job.Job.Name),
			zap.Duration("duration", endTime.Sub(startTime)))

		// 成功执行后重置重试计数
		job.RetryCount = 0
	}

	// 释放锁 - 确保在状态更新之前释放锁
	if job.Lock != nil {
		utils.Info("releasing job lock",
			zap.String("job_id", job.Job.ID),
			zap.String("worker_id", s.workerID))

		err = job.Lock.Unlock()
		if err != nil {
			utils.Error("failed to release job lock",
				zap.String("job_id", job.Job.ID),
				zap.Error(err))
		} else {
			utils.Info("job lock released successfully",
				zap.String("job_id", job.Job.ID),
				zap.String("worker_id", s.workerID))
		}
		job.Lock = nil
	}

	// 检查任务是否已从etcd中删除
	jobExists := true
	existingJob, err := s.jobManager.GetJob(job.Job.ID)
	if err != nil || existingJob == nil {
		jobExists = false
		utils.Info("job no longer exists in etcd, will not reschedule",
			zap.String("job_id", job.Job.ID))
	}

	// 处理下一次调度 - 只有当不是重试的情况下且任务仍然存在时
	if jobExists && (execErr == nil || job.RetryCount == 0) {
		s.scheduleNextRun(job)
	}

	// 更新作业状态
	if jobExists {
		err = s.jobManager.ReportJobStatus(job.Job, status)
		if err != nil {
			utils.Warn("failed to report job status",
				zap.String("job_id", job.Job.ID),
				zap.String("status", status),
				zap.Error(err))
		}
	}

	// 设置作业的运行状态
	utils.Info("setting job running state to false",
		zap.String("job_id", job.Job.ID),
		zap.Bool("current_is_running", job.IsRunning))
	job.IsRunning = false

	// 更新队列中的任务状态，确保一致性
	queueJob := &ScheduledJob{
		Job:             job.Job,
		NextRunTime:     job.NextRunTime,
		IsRunning:       false,
		RetryCount:      job.RetryCount,
		ExecutionCount:  job.ExecutionCount,
		LastExecutionID: job.LastExecutionID,
	}

	// 只有在不是重试过程中且任务仍然存在时才更新队列
	if jobExists && job.RetryCount == 0 {
		// 尝试更新队列中的任务
		err = s.jobQueue.Update(queueJob)
		if err != nil {
			utils.Warn("failed to update job in queue", 
				zap.String("job_id", job.Job.ID), 
				zap.Error(err))
		}
	}

	// 从运行映射中移除
	s.runningJobs.Delete(job.Job.ID)

	// 更新调度器状态
	s.updateStatus()

	utils.Info("job execution completed",
		zap.String("job_id", job.Job.ID),
		zap.String("status", status),
		zap.String("worker_id", s.workerID))
}

// scheduleNextRun 计算并安排任务的下一次执行
func (s *Scheduler) scheduleNextRun(job *ScheduledJob) {
	// 如果是重试任务，设置下一次执行时间为当前时间+重试延迟
	if job.RetryCount > 0 {
		job.NextRunTime = time.Now().Add(s.options.RetryDelay)
		utils.Info("scheduling job retry",
			zap.String("job_id", job.Job.ID),
			zap.Int("retry_count", job.RetryCount),
			zap.Time("next_run_time", job.NextRunTime))
		return
	}

	// 仅当作业有cron表达式时才继续
	if job.Job.CronExpr == "" {
		return
	}

	// 计算下一次运行时间
	schedule, err := s.cronParser.Parse(job.Job.CronExpr)
	if err != nil {
		utils.Error("failed to parse cron expression",
			zap.String("job_id", job.Job.ID),
			zap.String("cron_expr", job.Job.CronExpr),
			zap.Error(err))
		return
	}

	// 更新作业的下一次运行时间
	job.NextRunTime = schedule.Next(time.Now())
	utils.Info("calculated next run time",
		zap.String("job_id", job.Job.ID),
		zap.Time("next_run_time", job.NextRunTime))

	// 为下一次执行重置重试计数
	job.RetryCount = 0

	// 检查作业是否已经在队列中，如果在则更新它
	if s.jobQueue.Contains(job.Job.ID) {
		err := s.jobQueue.Update(job)
		if err != nil {
			utils.Error("failed to update job in queue",
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

func (s *Scheduler) Contains(id string) bool {
	jobs := s.jobQueue.GetAll()
	for _, job := range jobs {
		if job.Job.ID == id {
			return true
		}
	}

	return false
}
