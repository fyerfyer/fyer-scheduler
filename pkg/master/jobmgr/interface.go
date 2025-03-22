package jobmgr

import (
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
)

// IWorkerSelector 定义Worker选择器接口
type IWorkerSelector interface {
	SelectWorker(job *models.Job) (*models.Worker, error)
	SelectWorkerWithLabels(job *models.Job, requiredLabels map[string]string) (*models.Worker, error)
	GetStrategy() SelectionStrategy
	SetStrategy(strategy SelectionStrategy)
}

// IScheduler 定义调度器接口
type IScheduler interface {
	Start() error
	Stop() error
	TriggerImmediateJob(jobID string) error
	GetSchedulerStatus() map[string]interface{}
}
