package worker

import (
	"fmt"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/executor"
	"github.com/fyerfyer/fyer-scheduler/pkg/worker/jobmgr"
	"sync"
	"testing"
)

// testIntegrationExecutionReporter 是一个用于测试的执行报告器
type testIntegrationExecutionReporter struct {
	executionResults map[string]*executor.ExecutionResult
	executionLogs    map[string]string
	mutex            *sync.Mutex
}

func (r *testIntegrationExecutionReporter) ReportStart(executionID string, pid int) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.executionLogs[executionID] = fmt.Sprintf("Process started with PID %d\n", pid)
	return nil
}

func (r *testIntegrationExecutionReporter) ReportOutput(executionID string, output string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if log, exists := r.executionLogs[executionID]; exists {
		r.executionLogs[executionID] = log + output
	} else {
		r.executionLogs[executionID] = output
	}
	return nil
}

func (r *testIntegrationExecutionReporter) ReportCompletion(executionID string, result *executor.ExecutionResult) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.executionResults[executionID] = result
	return nil
}

func (r *testIntegrationExecutionReporter) ReportError(executionID string, err error) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if log, exists := r.executionLogs[executionID]; exists {
		r.executionLogs[executionID] = log + fmt.Sprintf("ERROR: %v\n", err)
	} else {
		r.executionLogs[executionID] = fmt.Sprintf("ERROR: %v\n", err)
	}
	return nil
}

func (r *testIntegrationExecutionReporter) ReportProgress(executionID string, status *executor.ExecutionStatus) error {
	// 简单实现，只记录进度
	return nil
}

// 测试用的任务事件处理器
type testJobEventHandler struct {
	events []*jobmgr.JobEvent
	t      *testing.T
}

func newTestJobEventHandler(t *testing.T) *testJobEventHandler {
	return &testJobEventHandler{
		events: make([]*jobmgr.JobEvent, 0),
		t:      t,
	}
}

// HandleJobEvent 实现IJobEventHandler接口
func (h *testJobEventHandler) HandleJobEvent(event *jobmgr.JobEvent) {
	h.t.Logf("Received job event: type=%v, job=%s", event.Type, event.Job.ID)
	h.events = append(h.events, event)
}
