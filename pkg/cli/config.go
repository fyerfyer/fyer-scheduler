package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config 存储CLI工具的配置信息
type Config struct {
	// 调度系统API地址
	APIBaseURL string `json:"api_base_url"`

	// API密钥，用于认证
	APIKey string `json:"api_key"`

	// 配置文件的路径
	configPath string
}

// 默认配置文件路径
const (
	DefaultConfigFileName = "fyer-cli-config.json"
)

// NewConfig 创建一个新的配置实例
func NewConfig() *Config {
	// 获取用户主目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// 如果无法获取主目录，使用当前目录
		homeDir = "."
	}

	// 配置文件路径
	configPath := filepath.Join(homeDir, ".fyer", DefaultConfigFileName)

	return &Config{
		APIBaseURL: "http://localhost:8080/api/v1",
		APIKey:     "",
		configPath: configPath,
	}
}

// LoadConfig 从文件加载配置
func LoadConfig() (*Config, error) {
	config := NewConfig()

	// 检查配置文件是否存在
	_, err := os.Stat(config.configPath)
	if os.IsNotExist(err) {
		// 配置文件不存在，返回默认配置
		return config, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to check config file: %w", err)
	}

	// 读取配置文件
	data, err := os.ReadFile(config.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析JSON配置
	err = json.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// Save 保存配置到文件
func (c *Config) Save() error {
	// 确保目录存在
	dir := filepath.Dir(c.configPath)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// 序列化配置为JSON
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	// 写入文件
	err = os.WriteFile(c.configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// SetAPIBaseURL 设置API基础URL
func (c *Config) SetAPIBaseURL(url string) {
	c.APIBaseURL = url
}

// SetAPIKey 设置API密钥
func (c *Config) SetAPIKey(key string) {
	c.APIKey = key
}

// GetConfigPath 获取配置文件路径
func (c *Config) GetConfigPath() string {
	return c.configPath
}

// IsConfigured 检查配置是否完成
func (c *Config) IsConfigured() bool {
	return c.APIBaseURL != "" && c.APIKey != ""
}
