package jobmgr

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/constants"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/repo"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// SelectionStrategy 定义Worker选择策略类型
type SelectionStrategy string

const (
	// Worker选择策略
	LeastJobsStrategy   SelectionStrategy = "least_jobs"   // 最少任务优先
	LoadBalanceStrategy SelectionStrategy = "load_balance" // 最低负载优先
	ResourceStrategy    SelectionStrategy = "resource"     // 资源最充足优先
	RandomStrategy      SelectionStrategy = "random"       // 随机选择
)

// WorkerSelector 提供选择合适Worker的功能
type WorkerSelector struct {
	workerRepo repo.IWorkerRepo
	strategy   SelectionStrategy
}

// NewWorkerSelector 创建一个新的Worker选择器
func NewWorkerSelector(workerRepo repo.IWorkerRepo, strategy SelectionStrategy) IWorkerSelector {
	// 如果没有指定策略，默认使用最少任务策略
	if strategy == "" {
		strategy = LeastJobsStrategy
	}

	return &WorkerSelector{
		workerRepo: workerRepo,
		strategy:   strategy,
	}
}

// SelectWorker 根据任务需求选择合适的Worker
func (s *WorkerSelector) SelectWorker(job *models.Job) (*models.Worker, error) {
	// 获取所有活跃Worker
	workers, err := s.workerRepo.ListActive()
	if err != nil {
		return nil, fmt.Errorf("failed to list active workers: %w", err)
	}

	// 如果没有可用Worker，直接返回错误
	if len(workers) == 0 {
		return nil, fmt.Errorf("no active workers available")
	}

	// 过滤出可用的Worker（非禁用状态）
	var availableWorkers []*models.Worker
	for _, worker := range workers {
		if worker.Status != constants.WorkerStatusDisabled {
			availableWorkers = append(availableWorkers, worker)
		}
	}

	// 如果没有可用Worker，返回错误
	if len(availableWorkers) == 0 {
		return nil, fmt.Errorf("no enabled workers available")
	}

	// 按照策略选择Worker
	var selectedWorker *models.Worker
	switch s.strategy {
	case LeastJobsStrategy:
		selectedWorker = s.selectWorkerByLeastJobs(availableWorkers)
	case LoadBalanceStrategy:
		selectedWorker = s.selectWorkerByLowestLoad(availableWorkers)
	case ResourceStrategy:
		selectedWorker = s.selectWorkerByResource(availableWorkers, job)
	case RandomStrategy:
		selectedWorker = s.selectWorkerRandomly(availableWorkers)
	default:
		// 默认使用最少任务策略
		selectedWorker = s.selectWorkerByLeastJobs(availableWorkers)
	}

	// 记录选择结果
	if selectedWorker != nil {
		utils.Info("worker selected",
			zap.String("strategy", string(s.strategy)),
			zap.String("worker_id", selectedWorker.ID),
			zap.String("worker_ip", selectedWorker.IP),
			zap.Int("running_jobs", len(selectedWorker.RunningJobs)))
	}

	return selectedWorker, nil
}

// selectWorkerByLeastJobs 选择运行任务最少的Worker
func (s *WorkerSelector) selectWorkerByLeastJobs(workers []*models.Worker) *models.Worker {
	if len(workers) == 0 {
		return nil
	}

	// 根据运行任务数排序
	sort.Slice(workers, func(i, j int) bool {
		return len(workers[i].RunningJobs) < len(workers[j].RunningJobs)
	})

	return workers[0]
}

// selectWorkerByLowestLoad 选择负载最低的Worker
func (s *WorkerSelector) selectWorkerByLowestLoad(workers []*models.Worker) *models.Worker {
	if len(workers) == 0 {
		return nil
	}

	// 根据负载平均值排序
	sort.Slice(workers, func(i, j int) bool {
		return workers[i].LoadAvg < workers[j].LoadAvg
	})

	return workers[0]
}

// selectWorkerByResource 选择资源最充足的Worker
func (s *WorkerSelector) selectWorkerByResource(workers []*models.Worker, job *models.Job) *models.Worker {
	if len(workers) == 0 {
		return nil
	}

	// 简单地按可用内存排序（实际中可能需要更复杂的资源评估）
	sort.Slice(workers, func(i, j int) bool {
		return workers[i].MemoryFree > workers[j].MemoryFree
	})

	return workers[0]
}

// selectWorkerRandomly 随机选择一个Worker
func (s *WorkerSelector) selectWorkerRandomly(workers []*models.Worker) *models.Worker {
	if len(workers) == 0 {
		return nil
	}

	// 初始化随机数生成器
	rand.New(rand.NewSource(time.Now().UnixNano()))

	// 随机选择一个Worker
	randomIndex := rand.Intn(len(workers))
	return workers[randomIndex]
}

// SelectWorkerWithLabels 根据标签选择Worker
func (s *WorkerSelector) SelectWorkerWithLabels(job *models.Job, requiredLabels map[string]string) (*models.Worker, error) {
	// 获取所有活跃Worker
	workers, err := s.workerRepo.ListActive()
	if err != nil {
		return nil, fmt.Errorf("failed to list active workers: %w", err)
	}

	// 过滤符合标签要求的Worker
	var matchingWorkers []*models.Worker
	for _, worker := range workers {
		if worker.Status == constants.WorkerStatusDisabled {
			continue
		}

		// 检查是否满足所有标签要求
		matches := true
		for k, v := range requiredLabels {
			workerValue, exists := worker.Labels[k]
			if !exists || workerValue != v {
				matches = false
				break
			}
		}

		if matches {
			matchingWorkers = append(matchingWorkers, worker)
		}
	}

	// 如果没有匹配的Worker，返回错误
	if len(matchingWorkers) == 0 {
		return nil, fmt.Errorf("no workers match the required labels")
	}

	// 使用常规策略从匹配的Worker中选择一个
	return s.selectWorkerByStrategy(matchingWorkers, job)
}

// selectWorkerByStrategy 基于当前策略选择Worker
func (s *WorkerSelector) selectWorkerByStrategy(workers []*models.Worker, job *models.Job) (*models.Worker, error) {
	if len(workers) == 0 {
		return nil, fmt.Errorf("no workers to select from")
	}

	var selectedWorker *models.Worker
	switch s.strategy {
	case LeastJobsStrategy:
		selectedWorker = s.selectWorkerByLeastJobs(workers)
	case LoadBalanceStrategy:
		selectedWorker = s.selectWorkerByLowestLoad(workers)
	case ResourceStrategy:
		selectedWorker = s.selectWorkerByResource(workers, job)
	case RandomStrategy:
		selectedWorker = s.selectWorkerRandomly(workers)
	default:
		selectedWorker = s.selectWorkerByLeastJobs(workers)
	}

	if selectedWorker == nil {
		return nil, fmt.Errorf("failed to select a worker")
	}

	return selectedWorker, nil
}

// GetStrategy 获取当前的选择策略
func (s *WorkerSelector) GetStrategy() SelectionStrategy {
	return s.strategy
}

// SetStrategy 设置选择策略
func (s *WorkerSelector) SetStrategy(strategy SelectionStrategy) {
	s.strategy = strategy
	utils.Info("worker selection strategy changed", zap.String("strategy", string(strategy)))
}
