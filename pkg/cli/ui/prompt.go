package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
)

// PromptManager 提供交互式命令行界面所需的提示功能
type PromptManager struct {
}

// NewPromptManager 创建一个新的提示管理器
func NewPromptManager() *PromptManager {
	return &PromptManager{}
}

// Select 显示单选菜单并返回选择的选项
func (p *PromptManager) Select(label string, items []string) (string, error) {
	prompt := promptui.Select{
		Label: label,
		Items: items,
	}

	_, result, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			fmt.Println("Operation cancelled")
			return "", err
		}
		return "", fmt.Errorf("failed to select option: %w", err)
	}

	return result, nil
}

// SelectWithIndex 显示单选菜单并返回选择的索引和选项
func (p *PromptManager) SelectWithIndex(label string, items []string) (int, string, error) {
	prompt := promptui.Select{
		Label: label,
		Items: items,
	}

	index, result, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			fmt.Println("Operation cancelled")
			return -1, "", err
		}
		return -1, "", fmt.Errorf("failed to select option: %w", err)
	}

	return index, result, nil
}

// Confirm 显示确认提示，返回用户是否确认
func (p *PromptManager) Confirm(label string) (bool, error) {
	prompt := promptui.Select{
		Label: label,
		Items: []string{"Yes", "No"},
	}

	_, result, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			fmt.Println("Operation cancelled")
			return false, err
		}
		return false, fmt.Errorf("failed to confirm: %w", err)
	}

	return result == "Yes", nil
}

// Input 获取单行文本输入
func (p *PromptManager) Input(label string, defaultValue string, validate func(string) error) (string, error) {
	prompt := promptui.Prompt{
		Label:     label,
		Default:   defaultValue,
		Validate:  validate,
		AllowEdit: true,
	}

	result, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			fmt.Println("Operation cancelled")
			return "", err
		}
		return "", fmt.Errorf("failed to get input: %w", err)
	}

	return result, nil
}

// Password 获取密码输入（不显示输入内容）
func (p *PromptManager) Password(label string) (string, error) {
	prompt := promptui.Prompt{
		Label:    label,
		Mask:     '*',
		Validate: nil,
	}

	result, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			fmt.Println("Operation cancelled")
			return "", err
		}
		return "", fmt.Errorf("failed to get password: %w", err)
	}

	return result, nil
}

// MultiSelect 显示多选菜单
func (p *PromptManager) MultiSelect(label string, items []string) ([]string, error) {
	selections := make([]string, 0)

	fmt.Println(label)
	fmt.Println("(Use space to select/deselect, enter to confirm, ctrl+c to cancel)")
	fmt.Println()

	// 为每个选项创建选择状态
	selected := make(map[string]bool)

	for {
		for i, item := range items {
			mark := "[ ]"
			if selected[item] {
				mark = "[✓]"
			}
			fmt.Printf("%d. %s %s\n", i+1, mark, item)
		}

		fmt.Println("\nSelected items:", len(selections))
		fmt.Print("\nEnter item number to toggle, or 'done' to finish: ")

		var input string
		fmt.Scanln(&input)

		if strings.ToLower(input) == "done" {
			break
		}

		if idx, err := processInput(input, len(items)); err == nil {
			item := items[idx]
			// 切换选择状态
			selected[item] = !selected[item]

			// 更新选择列表
			selections = make([]string, 0)
			for _, item := range items {
				if selected[item] {
					selections = append(selections, item)
				}
			}

			// 清屏以便重新显示列表
			clearScreen()
			fmt.Println(label)
			fmt.Println("(Use space to select/deselect, enter to confirm, ctrl+c to cancel)")
			fmt.Println()
		}
	}

	return selections, nil
}

// 处理输入的索引值
func processInput(input string, maxLen int) (int, error) {
	var idx int
	_, err := fmt.Sscanf(input, "%d", &idx)
	if err != nil || idx < 1 || idx > maxLen {
		return -1, fmt.Errorf("invalid input")
	}
	return idx - 1, nil
}

// 清屏
func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

// 常用的验证函数

// ValidateRequired 验证必填项
func ValidateRequired(input string) error {
	if strings.TrimSpace(input) == "" {
		return fmt.Errorf("this field is required")
	}
	return nil
}

// ValidateInt 验证整数输入
func ValidateInt(input string) error {
	var num int
	_, err := fmt.Sscanf(input, "%d", &num)
	if err != nil {
		return fmt.Errorf("please enter a valid integer")
	}
	return nil
}

// ValidatePositiveInt 验证正整数输入
func ValidatePositiveInt(input string) error {
	var num int
	_, err := fmt.Sscanf(input, "%d", &num)
	if err != nil || num <= 0 {
		return fmt.Errorf("please enter a positive integer")
	}
	return nil
}

// ValidateYesNo 验证Yes/No输入
func ValidateYesNo(input string) error {
	input = strings.ToLower(input)
	if input != "y" && input != "n" && input != "yes" && input != "no" {
		return fmt.Errorf("please enter y/n or yes/no")
	}
	return nil
}

// IsInterruptError 检查是否是中断错误
func IsInterruptError(err error) bool {
	return err == promptui.ErrInterrupt
}

// Exit 退出程序
func Exit(code int) {
	os.Exit(code)
}
