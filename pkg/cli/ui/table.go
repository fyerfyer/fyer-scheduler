package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
)

// TableManager 提供终端表格显示功能
type TableManager struct {
}

// NewTableManager 创建一个新的表格管理器
func NewTableManager() *TableManager {
	return &TableManager{}
}

// DisplayTable 显示基本表格，支持自定义表头和表格数据
func (t *TableManager) DisplayTable(headers []string, data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.AppendBulk(data)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(true)
	table.Render()
}

// DisplayColoredTable 显示带有颜色的表格，支持颜色单元格
func (t *TableManager) DisplayColoredTable(headers []string, data [][]string, coloredColumns []int) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(true)

	// 添加彩色行
	for _, row := range data {
		coloredRow := make([]string, len(row))
		copy(coloredRow, row)

		// 对指定的列应用颜色
		for _, colIdx := range coloredColumns {
			if colIdx >= 0 && colIdx < len(row) {
				// 提取状态文本
				status := row[colIdx]
				// 获取相应的颜色代码
				colorCode := getColorForStatus(status)
				// 应用颜色
				coloredRow[colIdx] = fmt.Sprintf("%s%s%s", colorCode, status, ResetColor())
			}
		}

		table.Append(coloredRow)
	}

	table.Render()
}

// DisplayStatusTable 显示状态表格，自动为状态列添加颜色和emoji
func (t *TableManager) DisplayStatusTable(headers []string, data [][]string, statusColumnIndex int) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(true)

	// 添加带状态的行
	for _, row := range data {
		if statusColumnIndex >= 0 && statusColumnIndex < len(row) {
			// 获取状态文本
			status := row[statusColumnIndex]
			// 获取相应的颜色和emoji
			colorCode := getColorForStatus(status)
			emoji := getEmojiForStatus(status)

			// 添加emoji和颜色到状态文本
			row[statusColumnIndex] = fmt.Sprintf("%s%s %s%s",
				colorCode, emoji, status, ResetColor())
		}

		table.Append(row)
	}

	table.Render()
}

// DisplayCompactTable 显示紧凑型表格，无边框和分隔线
func (t *TableManager) DisplayCompactTable(headers []string, data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(false)
	table.SetColumnSeparator(" ")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.AppendBulk(data)
	table.Render()
}

// DisplayTableWithFooter 显示带有页脚的表格
func (t *TableManager) DisplayTableWithFooter(headers []string, data [][]string, footer []string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(true)
	table.AppendBulk(data)

	// 添加页脚
	if len(footer) > 0 {
		table.SetFooter(footer)
	}

	table.Render()
}

// DisplayKeyValueTable 显示键值对表格
func (t *TableManager) DisplayKeyValueTable(data map[string]string) {
	// 转换为二维数组
	rows := make([][]string, 0, len(data))
	for key, value := range data {
		rows = append(rows, []string{key, value})
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Key", "Value"})
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(true)
	table.AppendBulk(rows)
	table.Render()
}

// DisplayPaginatedTable 显示分页表格
func (t *TableManager) DisplayPaginatedTable(headers []string, data [][]string, page, pageSize, total int) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(true)
	table.AppendBulk(data)

	// 计算总页数
	totalPages := (total + pageSize - 1) / pageSize

	// 添加分页信息作为页脚
	footer := []string{
		fmt.Sprintf("Page %d of %d", page, totalPages),
		fmt.Sprintf("Showing %d-%d of %d items",
			(page-1)*pageSize+1,
			min((page)*pageSize, total),
			total),
	}

	// 填充页脚以匹配标题长度
	for len(footer) < len(headers) {
		footer = append(footer, "")
	}

	table.SetFooter(footer)
	table.Render()
}

// 获取状态的颜色代码
func getColorForStatus(status string) string {
	// 统一转换为小写，以处理大小写不敏感
	loweredStatus := strings.ToLower(status)
	switch {
	case strings.Contains(loweredStatus, "online") ||
		strings.Contains(loweredStatus, "success") ||
		strings.Contains(loweredStatus, "enabled") ||
		strings.Contains(loweredStatus, "active") ||
		strings.Contains(loweredStatus, "healthy"):
		return "\033[32m" // 绿色
	case strings.Contains(loweredStatus, "running") ||
		strings.Contains(loweredStatus, "pending") ||
		strings.Contains(loweredStatus, "in progress"):
		return "\033[34m" // 蓝色
	case strings.Contains(loweredStatus, "warning") ||
		strings.Contains(loweredStatus, "busy"):
		return "\033[33m" // 黄色
	case strings.Contains(loweredStatus, "error") ||
		strings.Contains(loweredStatus, "failed") ||
		strings.Contains(loweredStatus, "offline") ||
		strings.Contains(loweredStatus, "unhealthy"):
		return "\033[31m" // 红色
	case strings.Contains(loweredStatus, "disabled") ||
		strings.Contains(loweredStatus, "cancelled"):
		return "\033[90m" // 灰色
	default:
		return "\033[0m" // 默认颜色
	}
}

// 获取状态的emoji
func getEmojiForStatus(status string) string {
	// 统一转换为小写，以处理大小写不敏感
	loweredStatus := strings.ToLower(status)
	switch {
	case strings.Contains(loweredStatus, "online") ||
		strings.Contains(loweredStatus, "success") ||
		strings.Contains(loweredStatus, "enabled") ||
		strings.Contains(loweredStatus, "active") ||
		strings.Contains(loweredStatus, "healthy"):
		return "✅"
	case strings.Contains(loweredStatus, "running"):
		return "🔄"
	case strings.Contains(loweredStatus, "pending") ||
		strings.Contains(loweredStatus, "in progress"):
		return "⏳"
	case strings.Contains(loweredStatus, "warning") ||
		strings.Contains(loweredStatus, "busy"):
		return "⚠️"
	case strings.Contains(loweredStatus, "error") ||
		strings.Contains(loweredStatus, "failed") ||
		strings.Contains(loweredStatus, "offline") ||
		strings.Contains(loweredStatus, "unhealthy"):
		return "❌"
	case strings.Contains(loweredStatus, "disabled"):
		return "⚫"
	case strings.Contains(loweredStatus, "cancelled"):
		return "🛑"
	default:
		return "❓"
	}
}

// ResetColor 返回重置颜色的ANSI转义序列
func ResetColor() string {
	return "\033[0m"
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
