package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"go.uber.org/zap"
)

// ExecutionReporter 实现任务执行状态报告
type ExecutionReporter struct {
	masterURL     string            // Master节点URL
	workerID      string            // 当前Worker ID
	httpClient    *http.Client      // HTTP客户端
	retryAttempts int               // 重试次数
	retryDelay    time.Duration     // 重试间隔
	outputBuffer  map[string]string // 输出缓冲区
	outputMutex   sync.Mutex        // 缓冲区互斥锁
	flushInterval time.Duration     // 输出刷新间隔
	stopChan      chan struct{}     // 停止信号
	isRunning     bool              // 是否运行中
}

// NewExecutionReporter 创建一个新的执行状态报告器
func NewExecutionReporter(masterURL, workerID string) *ExecutionReporter {
	reporter := &ExecutionReporter{
		masterURL:     masterURL,
		workerID:      workerID,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		retryAttempts: 3,
		retryDelay:    2 * time.Second,
		outputBuffer:  make(map[string]string),
		flushInterval: 5 * time.Second, // 默认5秒刷新一次
		stopChan:      make(chan struct{}),
	}

	return reporter
}

// Start 启动报告器
func (r *ExecutionReporter) Start() {
	r.outputMutex.Lock()
	defer r.outputMutex.Unlock()

	if r.isRunning {
		return
	}

	r.isRunning = true
	r.stopChan = make(chan struct{})

	// 启动输出缓冲区定期刷新
	go r.flushBufferLoop()

	utils.Info("execution reporter started",
		zap.String("worker_id", r.workerID),
		zap.String("master_url", r.masterURL))
}

// Stop 停止报告器
func (r *ExecutionReporter) Stop() {
	r.outputMutex.Lock()
	defer r.outputMutex.Unlock()

	if !r.isRunning {
		return
	}

	r.isRunning = false
	close(r.stopChan)

	// 最后刷新一次确保所有数据发送
	r.flushAllBuffers()

	utils.Info("execution reporter stopped")
}

// ReportStart 报告任务开始执行
func (r *ExecutionReporter) ReportStart(executionID string, pid int) error {
	utils.Info("reporting job start",
		zap.String("execution_id", executionID),
		zap.Int("pid", pid))

	data := map[string]interface{}{
		"execution_id": executionID,
		"worker_id":    r.workerID,
		"pid":          pid,
		"status":       "running",
		"start_time":   time.Now(),
	}

	return r.sendReport("/api/v1/logs/executions/start", data)
}

// ReportOutput 报告命令输出
func (r *ExecutionReporter) ReportOutput(executionID string, output string) error {
	if output == "" {
		return nil
	}

	r.outputMutex.Lock()
	defer r.outputMutex.Unlock()

	// 追加到缓冲区
	r.outputBuffer[executionID] += output

	// 如果输出太大，立即刷新
	if len(r.outputBuffer[executionID]) > 4096 {
		go r.flushOutputBuffer(executionID)
	}

	return nil
}

// ReportCompletion 报告任务完成
func (r *ExecutionReporter) ReportCompletion(executionID string, result *ExecutionResult) error {
	utils.Info("reporting job completion",
		zap.String("execution_id", executionID),
		zap.String("state", string(result.State)),
		zap.Int("exit_code", result.ExitCode))

	// 先刷新所有输出
	r.flushOutputBuffer(executionID)

	// 创建日志对象
	jobLog := &models.JobLog{
		ExecutionID: executionID,
		JobID:       result.ExecutionID, // 这里可能需要从Job获取实际ID
		WorkerID:    r.workerID,
		Status:      string(result.State),
		ExitCode:    result.ExitCode,
		Error:       result.Error,
		StartTime:   result.StartTime,
		EndTime:     result.EndTime,
	}

	// 发送完成报告
	return r.sendReport("/api/v1/logs/executions/complete", jobLog)
}

// ReportError 报告执行错误
func (r *ExecutionReporter) ReportError(executionID string, err error) error {
	if err == nil {
		return nil
	}

	utils.Warn("reporting job error",
		zap.String("execution_id", executionID),
		zap.Error(err))

	data := map[string]interface{}{
		"execution_id": executionID,
		"worker_id":    r.workerID,
		"error":        err.Error(),
		"time":         time.Now(),
	}

	return r.sendReport("/api/v1/logs/executions/error", data)
}

// ReportProgress 报告执行进度
func (r *ExecutionReporter) ReportProgress(executionID string, status *ExecutionStatus) error {
	data := map[string]interface{}{
		"execution_id":   executionID,
		"worker_id":      r.workerID,
		"state":          status.State,
		"pid":            status.Pid,
		"cpu":            status.CPU,
		"memory":         status.Memory,
		"update_time":    time.Now(),
		"output_size":    status.OutputSize,
		"current_output": "", // 不在进度报告中发送输出，通过ReportOutput单独发送
	}

	return r.sendReport("/api/v1/logs/executions/progress", data)
}

// flushBufferLoop 定期刷新输出缓冲区
func (r *ExecutionReporter) flushBufferLoop() {
	ticker := time.NewTicker(r.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.flushAllBuffers()
		case <-r.stopChan:
			return
		}
	}
}

// flushAllBuffers 刷新所有输出缓冲区
func (r *ExecutionReporter) flushAllBuffers() {
	r.outputMutex.Lock()
	executionIDs := make([]string, 0, len(r.outputBuffer))
	for id := range r.outputBuffer {
		executionIDs = append(executionIDs, id)
	}
	r.outputMutex.Unlock()

	for _, id := range executionIDs {
		r.flushOutputBuffer(id)
	}
}

// flushOutputBuffer 刷新指定执行ID的输出缓冲区
func (r *ExecutionReporter) flushOutputBuffer(executionID string) {
	r.outputMutex.Lock()
	output, exists := r.outputBuffer[executionID]
	if !exists || output == "" {
		r.outputMutex.Unlock()
		return
	}

	// 清空缓冲区
	r.outputBuffer[executionID] = ""
	r.outputMutex.Unlock()

	// 发送输出数据
	data := map[string]interface{}{
		"execution_id": executionID,
		"worker_id":    r.workerID,
		"output":       output,
		"time":         time.Now(),
	}

	err := r.sendReport("/api/v1/logs/executions/output", data)
	if err != nil {
		utils.Error("failed to flush output buffer",
			zap.String("execution_id", executionID),
			zap.Error(err))
	}
}

// sendReport 发送报告到Master节点
func (r *ExecutionReporter) sendReport(endpoint string, data interface{}) error {
	// 将数据序列化为JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal report data: %w", err)
	}

	// 构建请求URL
	url := fmt.Sprintf("%s%s", r.masterURL, endpoint)

	// 重试逻辑
	var lastErr error
	for attempt := 0; attempt < r.retryAttempts; attempt++ {
		if attempt > 0 {
			utils.Warn("retrying report send",
				zap.String("endpoint", endpoint),
				zap.Int("attempt", attempt+1),
				zap.Error(lastErr))
			time.Sleep(r.retryDelay)
		}

		// 创建请求
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			lastErr = err
			continue
		}

		// 设置请求头
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Worker-ID", r.workerID)

		// 发送请求
		resp, err := r.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// 检查响应状态
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			lastErr = fmt.Errorf("server returned non-success status: %d", resp.StatusCode)
			continue
		}

		// 成功，关闭响应体并返回
		resp.Body.Close()
		return nil
	}

	return fmt.Errorf("failed to send report after %d attempts: %w", r.retryAttempts, lastErr)
}

// SetMasterURL 设置Master节点URL
func (r *ExecutionReporter) SetMasterURL(url string) {
	r.masterURL = url
}

// SetRetryOptions 设置重试选项
func (r *ExecutionReporter) SetRetryOptions(attempts int, delay time.Duration) {
	if attempts > 0 {
		r.retryAttempts = attempts
	}
	if delay > 0 {
		r.retryDelay = delay
	}
}

// SetFlushInterval 设置刷新间隔
func (r *ExecutionReporter) SetFlushInterval(interval time.Duration) {
	if interval > 0 {
		r.flushInterval = interval
	}
}
