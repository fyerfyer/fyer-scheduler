package repo

import (
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// IJobRepo 定义任务仓库接口
type IJobRepo interface {
	Save(job *models.Job) error
	GetByID(jobID string) (*models.Job, error)
	ListAll() ([]*models.Job, error)
	Delete(jobID string) error
	ListByStatus(status string, page, pageSize int64) ([]*models.Job, int64, error)
	ListByNextRunTime(beforeTime time.Time) ([]*models.Job, error)
	Search(keyword string, page, pageSize int64) ([]*models.Job, int64, error)
	UpdateStatus(jobID, status string) error
	UpdateNextRunTime(jobID string, lastRunTime time.Time) error
	EnableJob(jobID string) error
	DisableJob(jobID string) error
}

// IWorkerRepo 定义Worker仓库接口
type IWorkerRepo interface {
	Register(worker *models.Worker, ttl int64) (clientv3.LeaseID, error)
	Heartbeat(worker *models.Worker) error
	GetByID(workerID string) (*models.Worker, error)
	ListAll() ([]*models.Worker, error)
	ListByStatus(status string, page, pageSize int64) ([]*models.Worker, int64, error)
	ListActive() ([]*models.Worker, error)
	EnableWorker(workerID string) error
	DisableWorker(workerID string) error
	Delete(workerID string) error
	UpdateWorkerResources(worker *models.Worker) error
	UpdateRunningJobs(worker *models.Worker) error
}

// ILogRepo 定义日志仓库接口
type ILogRepo interface {
	Save(log *models.JobLog) error
	GetByExecutionID(executionID string) (*models.JobLog, error)
	GetByJobID(jobID string, page, pageSize int64) ([]*models.JobLog, int64, error)
	GetLatestByJobID(jobID string) (*models.JobLog, error)
	GetByTimeRange(startTime, endTime time.Time, page, pageSize int64) ([]*models.JobLog, int64, error)
	GetByWorkerID(workerID string, page, pageSize int64) ([]*models.JobLog, int64, error)
	GetByStatus(status string, page, pageSize int64) ([]*models.JobLog, int64, error)
	CountByJobStatus(jobID string) (map[string]int64, error)
	GetJobSuccessRate(jobID string, period time.Duration) (float64, error)
	DeleteOldLogs(beforeTime time.Time) (int64, error)
	UpdateLogStatus(executionID, status string, output string) error
	AppendOutput(executionID, newOutput string) error
}
