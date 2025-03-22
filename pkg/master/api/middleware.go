package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// RequestIDKey 是存储请求ID的键名
	RequestIDKey = "X-Request-ID"

	// DefaultAuthKey 是默认的认证头键名
	DefaultAuthKey = "X-API-Key"
)

// AuthConfig 认证中间件配置
type AuthConfig struct {
	// 是否启用认证
	Enabled bool
	// API密钥列表
	APIKeys []string
	// 自定义认证头
	AuthHeader string
	// 需要排除认证的路径前缀
	ExcludePaths []string
}

// DefaultAuthConfig 返回默认认证配置
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		Enabled:      false,
		APIKeys:      []string{},
		AuthHeader:   DefaultAuthKey,
		ExcludePaths: []string{"/api/v1/health"},
	}
}

// RequestLogger 请求日志中间件
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录开始时间
		startTime := time.Now()

		// 获取请求ID
		requestID := c.GetHeader(RequestIDKey)
		if requestID == "" {
			requestID = c.GetString(RequestIDKey)
		}

		// 记录请求信息
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		if query != "" {
			path = path + "?" + query
		}

		// 处理请求
		c.Next()

		// 计算耗时
		latency := time.Since(startTime)

		// 记录请求完成后的信息
		statusCode := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()

		// 根据状态码选择日志级别
		if statusCode >= 500 {
			utils.Error("request completed with server error",
				zap.String("request_id", requestID),
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("status", statusCode),
				zap.Duration("latency", latency),
				zap.String("client_ip", clientIP))
		} else if statusCode >= 400 {
			utils.Warn("request completed with client error",
				zap.String("request_id", requestID),
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("status", statusCode),
				zap.Duration("latency", latency),
				zap.String("client_ip", clientIP))
		} else {
			utils.Info("request completed",
				zap.String("request_id", requestID),
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("status", statusCode),
				zap.Duration("latency", latency),
				zap.String("client_ip", clientIP))
		}
	}
}

// RequestID 请求ID追踪中间件
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查请求头中是否已存在请求ID
		requestID := c.GetHeader(RequestIDKey)
		if requestID == "" {
			// 生成新的请求ID
			requestID = uuid.New().String()
			// 添加到请求头
			c.Request.Header.Set(RequestIDKey, requestID)
		}

		// 将请求ID存储到上下文中
		c.Set(RequestIDKey, requestID)

		// 添加到响应头
		c.Writer.Header().Set(RequestIDKey, requestID)

		c.Next()
	}
}

// Recovery 错误恢复中间件
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 获取请求ID
				requestID := c.GetString(RequestIDKey)

				// 记录错误日志
				utils.Error("recovered from panic",
					zap.String("request_id", requestID),
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path))

				// 返回500响应
				apiErr := NewAPIError(ErrCodeInternalServer, "Internal server error", http.StatusInternalServerError)
				c.AbortWithStatusJSON(apiErr.Status, NewErrorResponse(apiErr, requestID))
			}
		}()

		c.Next()
	}
}

// Auth 认证中间件
func Auth(config *AuthConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultAuthConfig()
	}

	return func(c *gin.Context) {
		// 如果未启用认证，直接通过
		if !config.Enabled {
			c.Next()
			return
		}

		// 检查是否在排除路径列表中
		path := c.Request.URL.Path
		for _, prefix := range config.ExcludePaths {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		// 获取认证头
		apiKey := c.GetHeader(config.AuthHeader)
		if apiKey == "" {
			// 检查查询参数
			apiKey = c.Query("api_key")
		}

		// 验证API密钥
		if apiKey == "" || !contains(config.APIKeys, apiKey) {
			requestID := c.GetString(RequestIDKey)
			utils.Warn("unauthorized access attempt",
				zap.String("request_id", requestID),
				zap.String("path", path),
				zap.String("client_ip", c.ClientIP()))

			apiErr := ErrUnauthorized.WithDetails("Invalid or missing API key")
			c.AbortWithStatusJSON(apiErr.Status, NewErrorResponse(apiErr, requestID))
			return
		}

		c.Next()
	}
}

// CORS 跨域支持中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-API-Key, X-Request-ID")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// APIKeyAuth 简单API密钥认证中间件
func APIKeyAuth(validAPIKeys []string) gin.HandlerFunc {
	return Auth(&AuthConfig{
		Enabled:      true,
		APIKeys:      validAPIKeys,
		AuthHeader:   DefaultAuthKey,
		ExcludePaths: []string{"/api/v1/health", "/api/v1/system/status"},
	})
}

// contains 检查切片中是否包含指定字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
