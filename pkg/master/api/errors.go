package api

import (
	"fmt"
	"net/http"
)

// 常见错误码定义
const (
	// 客户端错误 4xx
	ErrCodeBadRequest       = 400000 // 请求格式错误
	ErrCodeUnauthorized     = 401000 // 未授权
	ErrCodeForbidden        = 403000 // 禁止访问
	ErrCodeNotFound         = 404000 // 资源不存在
	ErrCodeMethodNotAllowed = 405000 // 方法不允许
	ErrCodeConflict         = 409000 // 资源冲突
	ErrCodeRequestTimeout   = 408000 // 请求超时
	ErrCodeTooManyRequests  = 429000 // 请求过多

	// 具体业务错误码 - 任务相关 41xxxx
	ErrCodeJobNotFound       = 410001 // 任务不存在
	ErrCodeJobAlreadyExists  = 410002 // 任务已存在
	ErrCodeJobInvalidCron    = 410003 // 无效的Cron表达式
	ErrCodeJobInvalidName    = 410004 // 无效的任务名称
	ErrCodeJobInvalidCommand = 410005 // 无效的命令
	ErrCodeJobRunning        = 410006 // 任务正在运行
	ErrCodeJobNotRunning     = 410007 // 任务未运行
	ErrCodeJobDisabled       = 410008 // 任务被禁用

	// 具体业务错误码 - Worker相关 42xxxx
	ErrCodeWorkerNotFound    = 420001 // Worker不存在
	ErrCodeWorkerOffline     = 420002 // Worker离线
	ErrCodeWorkerDisabled    = 420003 // Worker被禁用
	ErrCodeNoAvailableWorker = 420004 // 没有可用的Worker

	// 具体业务错误码 - 日志相关 43xxxx
	ErrCodeLogNotFound       = 430001 // 日志不存在
	ErrCodeExecutionNotFound = 430002 // 执行记录不存在
	ErrCodeInvalidTimeRange  = 430003 // 无效的时间范围

	// 服务端错误 5xx
	ErrCodeInternalServer     = 500000 // 内部服务器错误
	ErrCodeNotImplemented     = 501000 // 功能未实现
	ErrCodeServiceUnavailable = 503000 // 服务不可用
	ErrCodeDatabaseError      = 500001 // 数据库错误
	ErrCodeETCDError          = 500002 // ETCD错误
)

// APIError 表示API错误
type APIError struct {
	Code    int    // 错误码
	Message string // 错误消息
	Details string // 详细错误信息
	Status  int    // HTTP状态码
}

// Error 实现error接口
func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Details)
	}
	return e.Message
}

// WithDetails 添加详细错误信息
func (e *APIError) WithDetails(details string) *APIError {
	e.Details = details
	return e
}

// WithDetailf 添加格式化的详细错误信息
func (e *APIError) WithDetailf(format string, args ...interface{}) *APIError {
	e.Details = fmt.Sprintf(format, args...)
	return e
}

// 预定义常见错误
var (
	ErrBadRequest         = NewAPIError(ErrCodeBadRequest, "Invalid request", http.StatusBadRequest)
	ErrUnauthorized       = NewAPIError(ErrCodeUnauthorized, "Unauthorized", http.StatusUnauthorized)
	ErrForbidden          = NewAPIError(ErrCodeForbidden, "Access forbidden", http.StatusForbidden)
	ErrNotFound           = NewAPIError(ErrCodeNotFound, "Resource not found", http.StatusNotFound)
	ErrMethodNotAllowed   = NewAPIError(ErrCodeMethodNotAllowed, "Method not allowed", http.StatusMethodNotAllowed)
	ErrConflict           = NewAPIError(ErrCodeConflict, "Resource conflict", http.StatusConflict)
	ErrTooManyRequests    = NewAPIError(ErrCodeTooManyRequests, "Too many requests", http.StatusTooManyRequests)
	ErrInternalServer     = NewAPIError(ErrCodeInternalServer, "Internal server error", http.StatusInternalServerError)
	ErrNotImplemented     = NewAPIError(ErrCodeNotImplemented, "Not implemented", http.StatusNotImplemented)
	ErrServiceUnavailable = NewAPIError(ErrCodeServiceUnavailable, "Service unavailable", http.StatusServiceUnavailable)

	// 任务相关错误
	ErrJobNotFound       = NewAPIError(ErrCodeJobNotFound, "Job not found", http.StatusNotFound)
	ErrJobAlreadyExists  = NewAPIError(ErrCodeJobAlreadyExists, "Job already exists", http.StatusConflict)
	ErrJobInvalidCron    = NewAPIError(ErrCodeJobInvalidCron, "Invalid cron expression", http.StatusBadRequest)
	ErrJobInvalidName    = NewAPIError(ErrCodeJobInvalidName, "Invalid job name", http.StatusBadRequest)
	ErrJobInvalidCommand = NewAPIError(ErrCodeJobInvalidCommand, "Invalid command", http.StatusBadRequest)
	ErrJobRunning        = NewAPIError(ErrCodeJobRunning, "Job is already running", http.StatusConflict)
	ErrJobNotRunning     = NewAPIError(ErrCodeJobNotRunning, "Job is not running", http.StatusBadRequest)
	ErrJobDisabled       = NewAPIError(ErrCodeJobDisabled, "Job is disabled", http.StatusBadRequest)

	// Worker相关错误
	ErrWorkerNotFound    = NewAPIError(ErrCodeWorkerNotFound, "Worker not found", http.StatusNotFound)
	ErrWorkerOffline     = NewAPIError(ErrCodeWorkerOffline, "Worker is offline", http.StatusServiceUnavailable)
	ErrWorkerDisabled    = NewAPIError(ErrCodeWorkerDisabled, "Worker is disabled", http.StatusForbidden)
	ErrNoAvailableWorker = NewAPIError(ErrCodeNoAvailableWorker, "No available worker", http.StatusServiceUnavailable)

	// 日志相关错误
	ErrLogNotFound       = NewAPIError(ErrCodeLogNotFound, "Log not found", http.StatusNotFound)
	ErrExecutionNotFound = NewAPIError(ErrCodeExecutionNotFound, "Execution not found", http.StatusNotFound)
	ErrInvalidTimeRange  = NewAPIError(ErrCodeInvalidTimeRange, "Invalid time range", http.StatusBadRequest)

	// 服务端错误
	ErrDatabaseError = NewAPIError(ErrCodeDatabaseError, "Database error", http.StatusInternalServerError)
	ErrETCDError     = NewAPIError(ErrCodeETCDError, "ETCD error", http.StatusInternalServerError)
)

// NewAPIError 创建新的API错误
func NewAPIError(code int, message string, status int) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}

// NewValidationError 创建参数验证错误
func NewValidationError(details string) *APIError {
	return ErrBadRequest.WithDetails(details)
}

// NewNotFoundError 创建资源不存在错误
func NewNotFoundError(resourceType, resourceID string) *APIError {
	return ErrNotFound.WithDetailf("%s with ID '%s' not found", resourceType, resourceID)
}

// WrapError 将普通错误包装为APIError
func WrapError(err error, defaultError *APIError) *APIError {
	if err == nil {
		return nil
	}

	// 检查是否已经是API错误
	if apiErr, ok := err.(*APIError); ok {
		return apiErr
	}

	// 包装为API错误
	return defaultError.WithDetails(err.Error())
}

// IsNotFoundError 检查是否是资源不存在错误
func IsNotFoundError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Status == http.StatusNotFound
	}
	return false
}

// IsConflictError 检查是否是资源冲突错误
func IsConflictError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Status == http.StatusConflict
	}
	return false
}

// IsBadRequestError 检查是否是请求参数错误
func IsBadRequestError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Status == http.StatusBadRequest
	}
	return false
}
