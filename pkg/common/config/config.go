package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config 是系统配置的顶层结构
type Config struct {
	// 节点配置
	NodeType string `mapstructure:"node_type"` // "master" 或 "worker"
	NodeID   string `mapstructure:"node_id"`

	// 服务配置
	Server ServerConfig `mapstructure:"server"`

	// etcd配置
	Etcd EtcdConfig `mapstructure:"etcd"`

	// MongoDB配置
	MongoDB MongoDBConfig `mapstructure:"mongodb"`

	// 日志配置
	Log LogConfig `mapstructure:"log"`
}

// ServerConfig 包含HTTP服务器配置
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// EtcdConfig 包含etcd连接配置
type EtcdConfig struct {
	Endpoints []string `mapstructure:"endpoints"`
	Username  string   `mapstructure:"username"`
	Password  string   `mapstructure:"password"`
}

// MongoDBConfig 包含MongoDB连接配置
type MongoDBConfig struct {
	URI      string `mapstructure:"uri"`
	Database string `mapstructure:"database"`
}

// LogConfig 包含日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`
	FilePath   string `mapstructure:"file_path"`
	MaxSize    int    `mapstructure:"max_size"`    // MB
	MaxBackups int    `mapstructure:"max_backups"` // 备份文件数
	MaxAge     int    `mapstructure:"max_age"`     // 天
}

// LoadConfig 从指定路径加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	// 初始化配置
	v := viper.New()
	v.SetConfigFile(configPath)

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file error: %w", err)
	}

	// 解析到结构体
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("parse config file error: %w", err)
	}

	// 支持环境变量替换
	if nodeID := os.Getenv("FYER_NODE_ID"); nodeID != "" {
		config.NodeID = nodeID
	}

	// 简单验证
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// validateConfig 验证配置有效性
func validateConfig(config *Config) error {
	// 验证节点类型
	if config.NodeType != "master" && config.NodeType != "worker" {
		return fmt.Errorf("invalid node type: %s, must be 'master' or 'worker'", config.NodeType)
	}

	// 验证节点ID
	if config.NodeID == "" {
		return fmt.Errorf("node ID cannot be empty")
	}

	// 验证etcd配置
	if len(config.Etcd.Endpoints) == 0 {
		return fmt.Errorf("etcd endpoints cannot be empty")
	}

	// 其他验证可以根据需要添加

	return nil
}

// GetServerAddress 返回服务器地址
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
