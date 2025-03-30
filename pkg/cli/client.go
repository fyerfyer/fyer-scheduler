package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"
)

// 定义API端点常量
const (
	JobsEndpoint    = "/api/v1/jobs"
	WorkersEndpoint = "/api/v1/workers"
	SystemEndpoint  = "/api/v1/system"
	LogsEndpoint    = "/api/v1/logs"
	HealthEndpoint  = "/health"
)

// Client 表示API客户端
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient 创建一个新的API客户端
func NewClient(config *Config) *Client {
	return &Client{
		BaseURL:    config.APIBaseURL,
		APIKey:     config.APIKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Response 通用响应结构
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   *ErrorInfo  `json:"error,omitempty"`
	TraceID string      `json:"trace_id"`
	Time    int64       `json:"time"`
}

// ErrorInfo 错误信息结构
type ErrorInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// PaginationResponse 分页响应
type PaginationResponse struct {
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Items    []interface{} `json:"items"`
}

// Job 任务基本结构
type Job struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Command      string            `json:"command"`
	Args         []string          `json:"args"`
	CronExpr     string            `json:"cron_expr"`
	WorkDir      string            `json:"work_dir"`
	Env          map[string]string `json:"env"`
	Timeout      int               `json:"timeout"`
	MaxRetry     int               `json:"max_retry"`
	RetryDelay   int               `json:"retry_delay"`
	Enabled      bool              `json:"enabled"`
	Status       string            `json:"status"`
	LastRunTime  *time.Time        `json:"last_run_time"`
	NextRunTime  *time.Time        `json:"next_run_time"`
	LastResult   string            `json:"last_result"`
	LastExitCode int               `json:"last_exit_code"`
	CreateTime   time.Time         `json:"create_time"`
	UpdateTime   time.Time         `json:"update_time"`
}

// Worker Worker结构
type Worker struct {
	ID            string            `json:"id"`
	Hostname      string            `json:"hostname"`
	IP            string            `json:"ip"`
	Status        string            `json:"status"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	RunningJobs   []string          `json:"running_jobs"`
	CPUCores      int               `json:"cpu_cores"`
	MemoryTotal   int64             `json:"memory_total"`
	MemoryFree    int64             `json:"memory_free"`
	DiskTotal     int64             `json:"disk_total"`
	DiskFree      int64             `json:"disk_free"`
	LoadAvg       float64           `json:"load_avg"`
	Labels        map[string]string `json:"labels"`
	CreateTime    time.Time         `json:"create_time"`
}

// JobLog 任务日志结构
type JobLog struct {
	ExecutionID  string    `json:"execution_id"`
	JobID        string    `json:"job_id"`
	JobName      string    `json:"job_name"`
	WorkerID     string    `json:"worker_id"`
	WorkerIP     string    `json:"worker_ip"`
	Command      string    `json:"command"`
	Status       string    `json:"status"`
	ExitCode     int       `json:"exit_code"`
	Error        string    `json:"error"`
	Output       string    `json:"output"`
	ScheduleTime time.Time `json:"schedule_time"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	Duration     float64   `json:"duration"`
	IsManual     bool      `json:"is_manual"`
}

// SystemStatus 系统状态结构
type SystemStatus struct {
	TotalJobs        int     `json:"total_jobs"`
	RunningJobs      int     `json:"running_jobs"`
	ActiveWorkers    int     `json:"active_workers"`
	SuccessRate      float64 `json:"success_rate"`
	AvgExecutionTime float64 `json:"avg_execution_time"`
	Version          string  `json:"version"`
	StartTime        string  `json:"start_time"`
}

// 创建任务请求
type CreateJobRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	CronExpr    string            `json:"cron_expr"`
	WorkDir     string            `json:"work_dir"`
	Env         map[string]string `json:"env"`
	Timeout     int               `json:"timeout"`
	MaxRetry    int               `json:"max_retry"`
	RetryDelay  int               `json:"retry_delay"`
	Enabled     bool              `json:"enabled"`
}

// 执行API请求的通用方法
func (c *Client) doRequest(method, endpoint string, queryParams url.Values, body interface{}) (*Response, error) {
	// 构建完整URL
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	u.Path = path.Join(u.Path, endpoint)
	if queryParams != nil {
		u.RawQuery = queryParams.Encode()
	}

	// 准备请求体
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	// 创建请求
	req, err := http.NewRequest(method, u.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	}

	// 发送请求
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(responseBody))
	}

	// 解析响应
	var apiResp Response
	if err := json.Unmarshal(responseBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// 检查API响应状态
	if !apiResp.Success && apiResp.Error != nil {
		return nil, fmt.Errorf("API error %d: %s - %s", apiResp.Error.Code, apiResp.Error.Message, apiResp.Error.Details)
	}

	return &apiResp, nil
}

// ============ Job 相关方法 ============

// ListJobs 获取任务列表
func (c *Client) ListJobs(page, pageSize int) ([]Job, int64, error) {
	// 设置查询参数
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("page_size", fmt.Sprintf("%d", pageSize))

	// 发送请求
	resp, err := c.doRequest(http.MethodGet, JobsEndpoint, params, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list jobs: %w", err)
	}

	// 解析分页响应
	var pagination map[string]interface{}
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to process response data: %w", err)
	}

	if err := json.Unmarshal(data, &pagination); err != nil {
		return nil, 0, fmt.Errorf("failed to parse pagination: %w", err)
	}

	// 解析任务列表
	itemsData, err := json.Marshal(pagination["items"])
	if err != nil {
		return nil, 0, fmt.Errorf("failed to process items data: %w", err)
	}

	var jobs []Job
	if err := json.Unmarshal(itemsData, &jobs); err != nil {
		return nil, 0, fmt.Errorf("failed to parse jobs: %w", err)
	}

	total := int64(pagination["total"].(float64))
	return jobs, total, nil
}

// GetJob 获取单个任务的详情
func (c *Client) GetJob(id string) (*Job, error) {
	endpoint := fmt.Sprintf("%s/%s", JobsEndpoint, id)
	resp, err := c.doRequest(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// 解析任务
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to process response data: %w", err)
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("failed to parse job: %w", err)
	}

	return &job, nil
}

// CreateJob 创建新任务
func (c *Client) CreateJob(req *CreateJobRequest) (*Job, error) {
	resp, err := c.doRequest(http.MethodPost, JobsEndpoint, nil, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	// 解析创建的任务
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to process response data: %w", err)
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("failed to parse job: %w", err)
	}

	return &job, nil
}

// UpdateJob 更新任务
func (c *Client) UpdateJob(id string, req *CreateJobRequest) (*Job, error) {
	endpoint := fmt.Sprintf("%s/%s", JobsEndpoint, id)
	resp, err := c.doRequest(http.MethodPut, endpoint, nil, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update job: %w", err)
	}

	// 解析更新后的任务
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to process response data: %w", err)
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("failed to parse job: %w", err)
	}

	return &job, nil
}

// DeleteJob 删除任务
func (c *Client) DeleteJob(id string) error {
	endpoint := fmt.Sprintf("%s/%s", JobsEndpoint, id)
	_, err := c.doRequest(http.MethodDelete, endpoint, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}
	return nil
}

// TriggerJob 立即触发执行任务
func (c *Client) TriggerJob(id string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s/run", JobsEndpoint, id)
	resp, err := c.doRequest(http.MethodPost, endpoint, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to trigger job: %w", err)
	}

	// 解析执行ID
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return "", fmt.Errorf("failed to process response data: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to parse result: %w", err)
	}

	return result["execution_id"].(string), nil
}

// KillJob 终止执行中的任务
func (c *Client) KillJob(id string) error {
	endpoint := fmt.Sprintf("%s/%s/kill", JobsEndpoint, id)
	_, err := c.doRequest(http.MethodPost, endpoint, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to kill job: %w", err)
	}
	return nil
}

// EnableJob 启用任务
func (c *Client) EnableJob(id string) error {
	endpoint := fmt.Sprintf("%s/%s/enable", JobsEndpoint, id)
	_, err := c.doRequest(http.MethodPut, endpoint, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to enable job: %w", err)
	}
	return nil
}

// DisableJob 禁用任务
func (c *Client) DisableJob(id string) error {
	endpoint := fmt.Sprintf("%s/%s/disable", JobsEndpoint, id)
	_, err := c.doRequest(http.MethodPut, endpoint, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to disable job: %w", err)
	}
	return nil
}

// ============ Worker 相关方法 ============

// ListWorkers 获取Worker列表
func (c *Client) ListWorkers(page, pageSize int) ([]Worker, int64, error) {
	// 设置查询参数
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("page_size", fmt.Sprintf("%d", pageSize))

	// 发送请求
	resp, err := c.doRequest(http.MethodGet, WorkersEndpoint, params, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list workers: %w", err)
	}

	// 解析分页响应
	var pagination map[string]interface{}
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to process response data: %w", err)
	}

	if err := json.Unmarshal(data, &pagination); err != nil {
		return nil, 0, fmt.Errorf("failed to parse pagination: %w", err)
	}

	// 解析Worker列表
	itemsData, err := json.Marshal(pagination["items"])
	if err != nil {
		return nil, 0, fmt.Errorf("failed to process items data: %w", err)
	}

	var workers []Worker
	if err := json.Unmarshal(itemsData, &workers); err != nil {
		return nil, 0, fmt.Errorf("failed to parse workers: %w", err)
	}

	total := int64(pagination["total"].(float64))
	return workers, total, nil
}

// GetWorker 获取单个Worker的详情
func (c *Client) GetWorker(id string) (*Worker, error) {
	endpoint := fmt.Sprintf("%s/%s", WorkersEndpoint, id)
	resp, err := c.doRequest(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get worker: %w", err)
	}

	// 解析Worker
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to process response data: %w", err)
	}

	var worker Worker
	if err := json.Unmarshal(data, &worker); err != nil {
		return nil, fmt.Errorf("failed to parse worker: %w", err)
	}

	return &worker, nil
}

// GetActiveWorkers 获取活跃的Worker列表
func (c *Client) GetActiveWorkers() ([]Worker, error) {
	endpoint := fmt.Sprintf("%s/active", WorkersEndpoint)
	resp, err := c.doRequest(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get active workers: %w", err)
	}

	// 解析Worker列表
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to process response data: %w", err)
	}

	var workers []Worker
	if err := json.Unmarshal(data, &workers); err != nil {
		return nil, fmt.Errorf("failed to parse workers: %w", err)
	}

	return workers, nil
}

// EnableWorker 启用Worker
func (c *Client) EnableWorker(id string) error {
	endpoint := fmt.Sprintf("%s/%s/enable", WorkersEndpoint, id)
	_, err := c.doRequest(http.MethodPut, endpoint, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to enable worker: %w", err)
	}
	return nil
}

// DisableWorker 禁用Worker
func (c *Client) DisableWorker(id string) error {
	endpoint := fmt.Sprintf("%s/%s/disable", WorkersEndpoint, id)
	_, err := c.doRequest(http.MethodPut, endpoint, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to disable worker: %w", err)
	}
	return nil
}

// CheckWorkerHealth 检查Worker的健康状态
func (c *Client) CheckWorkerHealth(id string) (bool, error) {
	endpoint := fmt.Sprintf("%s/%s/health", WorkersEndpoint, id)
	resp, err := c.doRequest(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check worker health: %w", err)
	}

	// 解析健康状态
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return false, fmt.Errorf("failed to process response data: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return false, fmt.Errorf("failed to parse result: %w", err)
	}

	return result["healthy"].(bool), nil
}

// ============ System 相关方法 ============

// GetSystemStatus 获取系统状态
func (c *Client) GetSystemStatus() (*SystemStatus, error) {
	endpoint := fmt.Sprintf("%s/status", SystemEndpoint)
	resp, err := c.doRequest(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get system status: %w", err)
	}

	// 解析系统状态
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to process response data: %w", err)
	}

	var status SystemStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("failed to parse system status: %w", err)
	}

	return &status, nil
}

// CheckHealth 检查系统健康状态
func (c *Client) CheckHealth() (bool, error) {
	resp, err := c.doRequest(http.MethodGet, HealthEndpoint, nil, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check system health: %w", err)
	}

	// 只要能得到正确响应，就认为系统健康
	return resp.Success, nil
}

// ============ Logs 相关方法 ============

// GetJobLogs 获取任务的执行日志
func (c *Client) GetJobLogs(jobID string, page, pageSize int) ([]JobLog, int64, error) {
	// 设置查询参数
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("page_size", fmt.Sprintf("%d", pageSize))

	endpoint := fmt.Sprintf("%s/jobs/%s", LogsEndpoint, jobID)
	resp, err := c.doRequest(http.MethodGet, endpoint, params, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get job logs: %w", err)
	}

	// 解析分页响应
	var pagination map[string]interface{}
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to process response data: %w", err)
	}

	if err := json.Unmarshal(data, &pagination); err != nil {
		return nil, 0, fmt.Errorf("failed to parse pagination: %w", err)
	}

	// 解析日志列表
	itemsData, err := json.Marshal(pagination["items"])
	if err != nil {
		return nil, 0, fmt.Errorf("failed to process items data: %w", err)
	}

	var logs []JobLog
	if err := json.Unmarshal(itemsData, &logs); err != nil {
		return nil, 0, fmt.Errorf("failed to parse logs: %w", err)
	}

	total := int64(pagination["total"].(float64))
	return logs, total, nil
}

// GetExecutionLog 获取特定执行的日志详情
func (c *Client) GetExecutionLog(executionID string) (*JobLog, error) {
	endpoint := fmt.Sprintf("%s/executions/%s", LogsEndpoint, executionID)
	resp, err := c.doRequest(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution log: %w", err)
	}

	// 解析日志
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to process response data: %w", err)
	}

	var log JobLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, fmt.Errorf("failed to parse log: %w", err)
	}

	return &log, nil
}
