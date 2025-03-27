package testutils

import (
	"fmt"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/executor"
)

// JobFactory 用于创建测试任务的工厂
type JobFactory struct {
	jobCounter int
}

// NewJobFactory 创建一个新的任务工厂
func NewJobFactory() *JobFactory {
	return &JobFactory{
		jobCounter: 0,
	}
}

// CreateSimpleJob 创建一个简单的测试任务
func (f *JobFactory) CreateSimpleJob() *models.Job {
	f.jobCounter++
	name := fmt.Sprintf("test-job-%d", f.jobCounter)
	job, _ := models.NewJob(name, "echo hello world", "")
	return job
}

// CreateScheduledJob 创建一个定时调度的测试任务
func (f *JobFactory) CreateScheduledJob(cronExpr string) *models.Job {
	f.jobCounter++
	name := fmt.Sprintf("scheduled-job-%d", f.jobCounter)
	job, err := models.NewJob(name, "echo scheduled task", cronExpr)
	if err != nil {
		// 这里可以使用一个更好的错误处理方式
		utils.Error("failed to create scheduled job",
			zap.String("name", name),
			zap.String("cron", cronExpr),
			zap.Error(err))
		return nil
	}
	return job
}

// CreateJobWithCommand 创建指定命令的测试任务
func (f *JobFactory) CreateJobWithCommand(command string) *models.Job {
	f.jobCounter++
	name := fmt.Sprintf("command-job-%d", f.jobCounter)
	job, _ := models.NewJob(name, command, "")
	return job
}

// CreateComplexJob 创建一个复杂的测试任务
func (f *JobFactory) CreateComplexJob(name, command, cronExpr string, timeout int, env map[string]string) *models.Job {
	if name == "" {
		f.jobCounter++
		name = fmt.Sprintf("complex-job-%d", f.jobCounter)
	}

	job, _ := models.NewJob(name, command, cronExpr)
	job.Timeout = timeout
	job.MaxRetry = 3
	job.RetryDelay = 5

	if env != nil {
		job.Env = env
	} else {
		job.Env = map[string]string{
			"TEST_VAR": "test_value",
		}
	}

	return job
}

// CreateJobLog 创建一个测试用的任务执行日志
func (f *JobFactory) CreateJobLog(job *models.Job, workerID, workerIP string) *models.JobLog {
	return models.NewJobLog(job.ID, job.Name, workerID, workerIP, job.Command, false)
}

// CreatePendingJob 创建一个待执行的任务
func (f *JobFactory) CreatePendingJob() *models.Job {
	job := f.CreateSimpleJob()
	job.Status = constants.JobStatusPending
	job.Enabled = true
	return job
}

// CreateRunningJob 创建一个正在运行的任务
func (f *JobFactory) CreateRunningJob() *models.Job {
	job := f.CreateSimpleJob()
	job.Status = constants.JobStatusRunning
	job.Enabled = true
	job.LastRunTime = time.Now()
	return job
}

// CreateCompletedJob 创建一个已完成的任务
func (f *JobFactory) CreateCompletedJob(successful bool) *models.Job {
	job := f.CreateSimpleJob()
	if successful {
		job.Status = constants.JobStatusSucceeded
		job.LastResult = "success"
		job.LastExitCode = 0
	} else {
		job.Status = constants.JobStatusFailed
		job.LastResult = "failed"
		job.LastExitCode = 1
	}
	job.Enabled = true
	job.LastRunTime = time.Now().Add(-5 * time.Minute)
	return job
}

// CreateWorker 创建一个测试用的Worker节点
func CreateWorker(id string, labels map[string]string) *models.Worker {
	if id == "" {
		id = GenerateUniqueID("worker")
	}

	worker := models.NewWorker(id, "test-host-"+id, "127.0.0.1")

	if labels != nil {
		worker.Labels = labels
	} else {
		worker.Labels = map[string]string{
			"env": "test",
		}
	}

	worker.CPUCores = 4
	worker.MemoryTotal = 8192
	worker.MemoryFree = 4096
	worker.DiskTotal = 100
	worker.DiskFree = 50
	worker.LoadAvg = 0.5

	return worker
}

// CreateExecutionContext 创建测试用的执行上下文
func (f *JobFactory) CreateExecutionContext(job *models.Job) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		ExecutionID:   GenerateUniqueID("exec"),
		Job:           job,
		Command:       job.Command,
		Args:          job.Args,
		WorkDir:       job.WorkDir,
		Environment:   job.Env,
		Timeout:       time.Duration(job.Timeout) * time.Second,
		MaxOutputSize: 10 * 1024 * 1024, // 10MB
	}
}
