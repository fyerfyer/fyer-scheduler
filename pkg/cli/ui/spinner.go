package ui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/briandowns/spinner"
)

// SpinnerManager 提供终端加载动画功能
type SpinnerManager struct {
	spinner     *spinner.Spinner // spinner实例
	isActive    bool             // 是否激活状态
	mu          sync.Mutex       // 互斥锁，用于保护并发访问
	charSet     []string         // 字符集
	speed       time.Duration    // 刷新速度
	stopChannel chan struct{}    // 停止通道
}

// NewSpinnerManager 创建一个新的加载动画管理器
func NewSpinnerManager() *SpinnerManager {
	// 使用默认字符集
	charSet := spinner.CharSets[14] // 使用常见的点动画
	s := spinner.New(charSet, 100*time.Millisecond)
	s.Writer = os.Stdout

	return &SpinnerManager{
		spinner:     s,
		isActive:    false,
		charSet:     charSet,
		speed:       100 * time.Millisecond,
		stopChannel: make(chan struct{}),
	}
}

// Start 启动加载动画，显示指定的消息
func (s *SpinnerManager) Start(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isActive {
		// 如果已经在运行，只更新消息
		s.spinner.Suffix = " " + message
		return
	}

	// 设置消息
	s.spinner.Suffix = " " + message
	// 启动spinner
	s.spinner.Start()
	s.isActive = true
}

// Stop 停止加载动画
func (s *SpinnerManager) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isActive {
		return
	}

	s.spinner.Stop()
	s.isActive = false
}

// UpdateMessage 更新加载动画显示的消息
func (s *SpinnerManager) UpdateMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isActive {
		return
	}

	s.spinner.Suffix = " " + message
}

// IsActive 返回加载动画是否处于活动状态
func (s *SpinnerManager) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.isActive
}

// StartWithColor 使用指定颜色启动加载动画
func (s *SpinnerManager) StartWithColor(message string, color string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isActive {
		// 如果已经在运行，只更新消息和颜色
		s.spinner.Suffix = " " + message
		s.spinner.Color(color)
		return
	}

	// 设置消息和颜色
	s.spinner.Suffix = " " + message
	s.spinner.Color(color)

	// 启动spinner
	s.spinner.Start()
	s.isActive = true
}

// SuccessStop 停止加载动画并显示成功消息
func (s *SpinnerManager) SuccessStop(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isActive {
		// 如果没有活动的spinner，直接打印消息
		fmt.Println("✅ " + message)
		return
	}

	// 停止动画
	s.spinner.Stop()
	s.isActive = false

	// 打印成功消息
	fmt.Println("✅ " + message)
}

// ErrorStop 停止加载动画并显示错误消息
func (s *SpinnerManager) ErrorStop(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isActive {
		// 如果没有活动的spinner，直接打印消息
		fmt.Println("❌ " + message)
		return
	}

	// 停止动画
	s.spinner.Stop()
	s.isActive = false

	// 打印错误消息
	fmt.Println("❌ " + message)
}

// StopWithMessage 停止加载动画并显示自定义消息
func (s *SpinnerManager) StopWithMessage(message string, emoji string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isActive {
		// 如果没有活动的spinner，直接打印消息
		fmt.Println(emoji + " " + message)
		return
	}

	// 停止动画
	s.spinner.Stop()
	s.isActive = false

	// 打印自定义消息
	fmt.Println(emoji + " " + message)
}

// SetSpeed 设置加载动画的速度
func (s *SpinnerManager) SetSpeed(speed time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.speed = speed
	s.spinner.UpdateSpeed(speed)
}

// SetCharset 设置加载动画使用的字符集
func (s *SpinnerManager) SetCharset(charsetIndex int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 确保索引有效
	if charsetIndex < 0 || charsetIndex >= len(spinner.CharSets) {
		charsetIndex = 14 // 默认字符集
	}

	s.charSet = spinner.CharSets[charsetIndex]
	s.spinner.UpdateCharSet(s.charSet)
}

// WaitForKeyPress 显示加载动画，并等待按键按下
func (s *SpinnerManager) WaitForKeyPress(message string) {
	// 启动加载动画
	s.Start(message + " (Press any key to continue)")

	// 等待按键
	fmt.Scanln()

	// 停止加载动画
	s.Stop()
}
