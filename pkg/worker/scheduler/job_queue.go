package scheduler

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// JobQueue 实现 IJobQueue 接口，管理调度任务
type JobQueue struct {
	jobs           []*ScheduledJob // 任务切片
	capacity       int             // 队列容量，0表示无限制
	sortByPriority bool            // 是否按优先级排序
	mutex          sync.RWMutex    // 用于保护并发访问
}

// NewJobQueue 创建一个新的任务队列
func NewJobQueue(options *JobQueueOptions) *JobQueue {
	if options == nil {
		options = NewJobQueueOptions()
	}

	return &JobQueue{
		jobs:           make([]*ScheduledJob, 0),
		capacity:       options.Capacity,
		sortByPriority: options.SortByPriority,
	}
}

// Push 添加任务到队列
func (q *JobQueue) Push(job *ScheduledJob) error {
	if job == nil {
		return fmt.Errorf("cannot push nil job")
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	// 检查容量限制
	if q.capacity > 0 && len(q.jobs) >= q.capacity {
		return fmt.Errorf("job queue is full (capacity: %d)", q.capacity)
	}

	// 检查是否已存在相同ID的任务
	for _, existingJob := range q.jobs {
		if existingJob.Job.ID == job.Job.ID {
			return fmt.Errorf("job with ID %s already exists in queue", job.Job.ID)
		}
	}

	// 添加任务
	q.jobs = append(q.jobs, job)

	// 如果需要按优先级排序
	if q.sortByPriority {
		q.sortJobs()
	}

	utils.Info("job added to queue",
		zap.String("job_id", job.Job.ID),
		zap.String("job_name", job.Job.Name),
		zap.Time("next_run_time", job.NextRunTime))

	return nil
}

// Pop 获取并移除队首任务
func (q *JobQueue) Pop() (*ScheduledJob, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if len(q.jobs) == 0 {
		return nil, fmt.Errorf("job queue is empty")
	}

	// 获取队首任务
	job := q.jobs[0]

	// 从队列中移除
	q.jobs = q.jobs[1:]

	utils.Debug("job popped from queue",
		zap.String("job_id", job.Job.ID),
		zap.String("job_name", job.Job.Name))

	return job, nil
}

// Peek 查看队首任务但不移除
func (q *JobQueue) Peek() (*ScheduledJob, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	if len(q.jobs) == 0 {
		return nil, fmt.Errorf("job queue is empty")
	}

	return q.jobs[0], nil
}

// Remove 从队列中移除指定任务
func (q *JobQueue) Remove(jobID string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	for i, job := range q.jobs {
		if job.Job.ID == jobID {
			// 从切片中移除元素
			q.jobs = append(q.jobs[:i], q.jobs[i+1:]...)

			utils.Info("job removed from queue", zap.String("job_id", jobID))
			return nil
		}
	}

	return fmt.Errorf("job with ID %s not found in queue", jobID)
}

// Update 更新队列中的任务
func (q *JobQueue) Update(job *ScheduledJob) error {
	if job == nil {
		return fmt.Errorf("cannot update nil job")
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	// 查找并更新任务
	for i, existingJob := range q.jobs {
		if existingJob.Job.ID == job.Job.ID {
			q.jobs[i] = job

			// 如果需要按优先级排序
			if q.sortByPriority {
				q.sortJobs()
			}

			utils.Info("job updated in queue",
				zap.String("job_id", job.Job.ID),
				zap.Time("next_run_time", job.NextRunTime))
			return nil
		}
	}

	return fmt.Errorf("job with ID %s not found in queue", job.Job.ID)
}

// Contains 检查队列是否包含指定任务
func (q *JobQueue) Contains(jobID string) bool {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	for _, job := range q.jobs {
		if job.Job.ID == jobID {
			return true
		}
	}

	return false
}

// Size 获取队列中的任务数量
func (q *JobQueue) Size() int {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	return len(q.jobs)
}

// Clear 清空队列
func (q *JobQueue) Clear() {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.jobs = make([]*ScheduledJob, 0)
	utils.Info("job queue cleared")
}

// GetAll 获取所有任务（不移除）
func (q *JobQueue) GetAll() []*ScheduledJob {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	// 创建一个副本，避免对原始切片的修改
	result := make([]*ScheduledJob, len(q.jobs))
	copy(result, q.jobs)

	return result
}

// sortJobs 根据下一次执行时间对任务进行排序
func (q *JobQueue) sortJobs() {
	// 使用sort包的Sort函数，根据NextRunTime进行排序
	sort.Slice(q.jobs, func(i, j int) bool {
		// 如果任务i的NextRunTime为零值，放到最后
		if q.jobs[i].NextRunTime.IsZero() {
			return false
		}

		// 如果任务j的NextRunTime为零值，任务i应该排在前面
		if q.jobs[j].NextRunTime.IsZero() {
			return true
		}

		// 正常情况下，按照NextRunTime升序排序
		return q.jobs[i].NextRunTime.Before(q.jobs[j].NextRunTime)
	})
}

// GetDueJobs 获取已到期需要执行的任务
func (q *JobQueue) GetDueJobs(now time.Time) []*ScheduledJob {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	var dueJobs []*ScheduledJob

	for _, job := range q.jobs {
		// 跳过正在运行的任务
		if job.IsRunning {
			continue
		}

		// 跳过下次执行时间为零值的任务
		if job.NextRunTime.IsZero() {
			continue
		}

		// 如果下次执行时间在当前时间之前或相等，则为到期任务
		if !job.NextRunTime.After(now) {
			dueJobs = append(dueJobs, job)
		}
	}

	return dueJobs
}

// RemoveDueJobs 移除并返回已到期的任务
func (q *JobQueue) RemoveDueJobs(now time.Time) []*ScheduledJob {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	var dueJobs []*ScheduledJob
	var remainingJobs []*ScheduledJob

	for _, job := range q.jobs {
		// 跳过正在运行的任务
		if job.IsRunning {
			remainingJobs = append(remainingJobs, job)
			continue
		}

		// 跳过下次执行时间为零值的任务
		if job.NextRunTime.IsZero() {
			remainingJobs = append(remainingJobs, job)
			continue
		}

		// 如果下次执行时间在当前时间之前或相等，则为到期任务
		if !job.NextRunTime.After(now) {
			dueJobs = append(dueJobs, job)
		} else {
			remainingJobs = append(remainingJobs, job)
		}
	}

	// 更新队列，移除已到期的任务
	q.jobs = remainingJobs

	if len(dueJobs) > 0 {
		utils.Info("removed due jobs from queue", zap.Int("count", len(dueJobs)))
	}

	return dueJobs
}
