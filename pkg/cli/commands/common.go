package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fyerfyer/fyer-scheduler/pkg/cli"
	"github.com/fyerfyer/fyer-scheduler/pkg/cli/models"
	"github.com/fyerfyer/fyer-scheduler/pkg/cli/ui"
)

// CommandManager 管理CLI命令的通用功能
type CommandManager struct {
	// 客户端实例
	Client *cli.Client

	// UI组件
	Prompt  *ui.PromptManager
	Table   *ui.TableManager
	Spinner *ui.SpinnerManager
}

// NewCommandManager 创建一个新的命令管理器
func NewCommandManager(client *cli.Client) *CommandManager {
	return &CommandManager{
		Client:  client,
		Prompt:  ui.NewPromptManager(),
		Table:   ui.NewTableManager(),
		Spinner: ui.NewSpinnerManager(),
	}
}

// 分页参数结构
type PaginationParams struct {
	Page     int
	PageSize int
}

// 默认分页参数
const (
	DefaultPage     = 1
	DefaultPageSize = 10
)

// NewPaginationParams 创建默认分页参数
func NewPaginationParams() PaginationParams {
	return PaginationParams{
		Page:     DefaultPage,
		PageSize: DefaultPageSize,
	}
}

// 解析命令行参数的常用函数

// ParseInt 安全地将字符串转换为整数，提供默认值
func ParseInt(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}

	return value
}

// ParseBool 安全地将字符串转换为布尔值，提供默认值
func ParseBool(s string, defaultValue bool) bool {
	if s == "" {
		return defaultValue
	}

	s = strings.ToLower(s)
	if s == "true" || s == "yes" || s == "y" || s == "1" {
		return true
	}

	if s == "false" || s == "no" || s == "n" || s == "0" {
		return false
	}

	return defaultValue
}

// FormatTime 格式化时间为人类可读形式
func FormatTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "Never"
	}

	return t.Format("2006-01-02 15:04:05")
}

// FormatTimeAgo 格式化时间为"多久之前"的形式
func FormatTimeAgo(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "Never"
	}

	return models.FormatTimeAgo(*t)
}

// ConfirmAction 请求用户确认操作
func (m *CommandManager) ConfirmAction(message string) bool {
	confirm, err := m.Prompt.Confirm(message)
	if err != nil {
		return false
	}
	return confirm
}

// 错误处理函数

// HandleError 处理错误并输出友好消息
func (m *CommandManager) HandleError(err error, message string) {
	if err != nil {
		fmt.Printf("Error: %s - %v\n", message, err)
	}
}

// DisplayError 显示错误消息，带有彩色格式
func (m *CommandManager) DisplayError(message string) {
	fmt.Printf("\033[31mError: %s\033[0m\n", message)
}

// DisplayWarning 显示警告消息，带有彩色格式
func (m *CommandManager) DisplayWarning(message string) {
	fmt.Printf("\033[33mWarning: %s\033[0m\n", message)
}

// DisplaySuccess 显示成功消息，带有彩色格式
func (m *CommandManager) DisplaySuccess(message string) {
	fmt.Printf("\033[32m✓ %s\033[0m\n", message)
}

// DisplayInfo 显示信息消息，带有彩色格式
func (m *CommandManager) DisplayInfo(message string) {
	fmt.Printf("\033[34mℹ %s\033[0m\n", message)
}

// 列表相关辅助函数

// GetPaginationFooter 获取分页页脚信息
func GetPaginationFooter(page, pageSize int, total int64) []string {
	startItem := (page-1)*pageSize + 1
	endItem := page * pageSize

	if total == 0 {
		return []string{"No items found", ""}
	}

	if total < int64(endItem) {
		endItem = int(total)
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	return []string{
		fmt.Sprintf("Page %d of %d", page, totalPages),
		fmt.Sprintf("Showing %d-%d of %d items", startItem, endItem, total),
	}
}

// AskForPagination 询问用户分页选项
func (m *CommandManager) AskForPagination() (PaginationParams, error) {
	params := NewPaginationParams()

	// 询问页码
	pageStr, err := m.Prompt.Input("Page number", strconv.Itoa(params.Page), ui.ValidatePositiveInt)
	if err != nil {
		return params, err
	}
	params.Page = ParseInt(pageStr, DefaultPage)

	// 询问每页数量
	pageSizeStr, err := m.Prompt.Input("Items per page", strconv.Itoa(params.PageSize), ui.ValidatePositiveInt)
	if err != nil {
		return params, err
	}
	params.PageSize = ParseInt(pageSizeStr, DefaultPageSize)

	return params, nil
}

// 命令共享的任务处理函数

// GetStatusDisplay 获取状态的带颜色显示
func GetStatusDisplay(status string) string {
	color := models.GetStatusColor(status)
	emoji := models.GetStatusEmoji(status)
	return fmt.Sprintf("%s%s %s%s", color, emoji, status, models.ResetColor())
}

// ShortenString 截断过长字符串
func ShortenString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// EnvMapToString 将环境变量映射转换为字符串
func EnvMapToString(env map[string]string) string {
	if len(env) == 0 {
		return "None"
	}

	parts := make([]string, 0, len(env))
	for k, v := range env {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(parts, ", ")
}

// ArgsToString 将参数数组转换为字符串
func ArgsToString(args []string) string {
	if len(args) == 0 {
		return "None"
	}

	return strings.Join(args, " ")
}

// 输入验证函数

// ValidateCronExpression 验证cron表达式
func ValidateCronExpression(input string) error {
	if input == "" {
		return nil // 允许空表达式
	}

	// 简单验证，只检查基本格式
	parts := strings.Fields(input)
	if len(parts) != 5 && len(parts) != 6 {
		return fmt.Errorf("invalid cron expression format (expected 5 or 6 fields)")
	}

	return nil
}

// ValidateTimeout 验证超时值
func ValidateTimeout(input string) error {
	if input == "" {
		return nil
	}

	timeout, err := strconv.Atoi(input)
	if err != nil {
		return fmt.Errorf("timeout must be a valid integer")
	}

	if timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}

	return nil
}

// ValidateRetry 验证重试次数
func ValidateRetry(input string) error {
	if input == "" {
		return nil
	}

	retry, err := strconv.Atoi(input)
	if err != nil {
		return fmt.Errorf("retry count must be a valid integer")
	}

	if retry < 0 {
		return fmt.Errorf("retry count cannot be negative")
	}

	return nil
}

// ValidateJobName 验证任务名称
func ValidateJobName(input string) error {
	if input == "" {
		return fmt.Errorf("job name cannot be empty")
	}

	if len(input) > 128 {
		return fmt.Errorf("job name cannot exceed 128 characters")
	}

	// 简单的字符验证，可以根据需要调整
	for _, c := range input {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == ' ') {
			return fmt.Errorf("job name contains invalid characters (use letters, numbers, spaces, hyphens, underscores, and dots)")
		}
	}

	return nil
}

// ValidateCommand 验证命令
func ValidateCommand(input string) error {
	if input == "" {
		return fmt.Errorf("command cannot be empty")
	}

	return nil
}

// NoValidation 无验证函数，始终返回nil
func NoValidation(input string) error {
	return nil
}
