package utils

import (
	"os"
	"sync"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/fyerfyer/fyer-scheduler/pkg/common/config"
)

var (
	logger *zap.Logger
	once   sync.Once
)

// InitLogger 初始化日志系统
func InitLogger(cfg *config.LogConfig) {
	once.Do(func() {
		// 设置日志级别
		var level zapcore.Level
		switch cfg.Level {
		case "debug":
			level = zapcore.DebugLevel
		case "info":
			level = zapcore.InfoLevel
		case "warn":
			level = zapcore.WarnLevel
		case "error":
			level = zapcore.ErrorLevel
		default:
			level = zapcore.InfoLevel
		}

		// 配置编码器
		encoderConfig := zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		// 输出到控制台
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		consoleCore := zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			level,
		)

		cores := []zapcore.Core{consoleCore}

		// 如果配置了文件路径，同时输出到文件
		if cfg.FilePath != "" {
			// 配置日志滚动
			fileWriter := &lumberjack.Logger{
				Filename:   cfg.FilePath,
				MaxSize:    cfg.MaxSize, // MB
				MaxBackups: cfg.MaxBackups,
				MaxAge:     cfg.MaxAge, // 天
				Compress:   true,       // 压缩轮转的日志文件
				LocalTime:  true,
			}

			fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
			fileCore := zapcore.NewCore(
				fileEncoder,
				zapcore.AddSync(fileWriter),
				level,
			)

			cores = append(cores, fileCore)
		}

		// 合并所有输出
		core := zapcore.NewTee(cores...)

		// 创建Logger
		logger = zap.New(
			core,
			zap.AddCaller(),                       // 添加调用者信息
			zap.AddCallerSkip(1),                  // 跳过helper方法
			zap.AddStacktrace(zapcore.ErrorLevel), // 错误级别以上添加堆栈跟踪
		)
	})
}

// GetLogger 返回已初始化的logger实例
func GetLogger() *zap.Logger {
	// 确保logger已经初始化
	if logger == nil {
		// 如果未初始化，使用默认配置初始化
		defaultCfg := &config.LogConfig{
			Level:      "info",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     7,
		}
		InitLogger(defaultCfg)
	}
	return logger
}

// Logger简便方法

// Debug 输出Debug级别日志
func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}

// Info 输出Info级别日志
func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

// Warn 输出Warn级别日志
func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

// Error 输出Error级别日志
func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

// Fatal 输出Fatal级别日志
func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}

// 关闭日志
func CloseLogger() {
	if logger != nil {
		_ = logger.Sync()
	}
}
