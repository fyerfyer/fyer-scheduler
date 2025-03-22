package scheduler

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// CronParser 提供Cron表达式的解析和验证功能
type CronParser struct {
	parser cron.Parser
}

// NewCronParser 创建一个新的Cron表达式解析器
func NewCronParser() *CronParser {
	// 使用标准的Cron解析器，支持秒级精度
	parser := cron.NewParser(
		cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)

	return &CronParser{
		parser: parser,
	}
}

// Parse 解析Cron表达式，如果格式不正确则返回错误
func (p *CronParser) Parse(expr string) (cron.Schedule, error) {
	if expr == "" {
		return nil, fmt.Errorf("cron expression cannot be empty")
	}

	// 尝试解析表达式
	schedule, err := p.parser.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}

	return schedule, nil
}

// Validate 验证Cron表达式是否有效，返回布尔值结果
func (p *CronParser) Validate(expr string) bool {
	_, err := p.Parse(expr)
	return err == nil
}

// GetNextRunTime 计算给定Cron表达式的下一次执行时间
func (p *CronParser) GetNextRunTime(expr string, from time.Time) (time.Time, error) {
	schedule, err := p.Parse(expr)
	if err != nil {
		return time.Time{}, err
	}

	return schedule.Next(from), nil
}

// GetNextNRunTimes 计算给定Cron表达式的接下来n次执行时间
func (p *CronParser) GetNextNRunTimes(expr string, from time.Time, n int) ([]time.Time, error) {
	if n <= 0 {
		return nil, fmt.Errorf("n must be greater than 0")
	}

	schedule, err := p.Parse(expr)
	if err != nil {
		return nil, err
	}

	times := make([]time.Time, n)
	nextTime := from

	for i := 0; i < n; i++ {
		nextTime = schedule.Next(nextTime)
		times[i] = nextTime
	}

	return times, nil
}

// ParseWithSeconds 解析包含秒的Cron表达式
// 标准格式: 秒 分 时 日 月 周
func (p *CronParser) ParseWithSeconds(expr string) (cron.Schedule, error) {
	return p.Parse(expr)
}

// ParseStandard 解析标准Cron表达式（不含秒）
// 标准格式: 分 时 日 月 周
func (p *CronParser) ParseStandard(expr string) (cron.Schedule, error) {
	// 如果是预定义的特殊表达式，直接解析
	if strings.HasPrefix(expr, "@") {
		return p.Parse(expr)
	}

	// 为不包含秒的表达式添加0秒
	return p.Parse("0 " + expr)
}

// GetDescription 获取Cron表达式的描述性文本
func (p *CronParser) GetDescription(expr string) (string, error) {
	// 处理预定义的特殊表达式
	switch expr {
	case "@yearly", "@annually":
		return "yearly", nil
	case "@monthly":
		return "monthly", nil
	case "@weekly":
		return "weekly", nil
	case "@daily", "@midnight":
		return "daily", nil
	case "@hourly":
		return "hourly", nil
	}

	// 解析表达式以验证其有效性
	_, err := p.Parse(expr)
	if err != nil {
		return "", err
	}

	// 这里可以实现更复杂的描述文本生成，但为了简单起见，我们返回原始表达式
	// 实际应用中可以使用专门的库来生成人类可读的描述
	return expr, nil
}

// IsEveryExpression 检查表达式是否为@every格式
func (p *CronParser) IsEveryExpression(expr string) bool {
	return strings.HasPrefix(expr, "@every ")
}

// ParseDuration 解析@every表达式中的时间间隔
func (p *CronParser) ParseDuration(expr string) (time.Duration, error) {
	if !p.IsEveryExpression(expr) {
		return 0, fmt.Errorf("not an @every expression")
	}

	// 提取时间间隔部分
	durationStr := strings.TrimPrefix(expr, "@every ")
	return time.ParseDuration(durationStr)
}
