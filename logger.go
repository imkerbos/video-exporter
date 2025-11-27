package main

import (
	"log/slog"
	"os"
	"strings"
)

var logger *slog.Logger
var levelVar slog.LevelVar

// InitLogger 初始化日志
func InitLogger() {
	levelVar.Set(slog.LevelInfo)
	// 只输出到标准输出，适合 Docker 环境
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: &levelVar,
	}))
}

// GetLogger 获取日志实例
func GetLogger() *slog.Logger {
	if logger == nil {
		InitLogger()
	}
	return logger
}

// SetLogLevel 设置日志级别
func SetLogLevel(level string) {
	if level == "" {
		level = "info"
	}

	switch strings.ToLower(level) {
	case "debug":
		levelVar.Set(slog.LevelDebug)
	case "info":
		levelVar.Set(slog.LevelInfo)
	case "warn", "warning":
		levelVar.Set(slog.LevelWarn)
	case "error":
		levelVar.Set(slog.LevelError)
	default:
		levelVar.Set(slog.LevelInfo)
	}

	if logger != nil {
		logger.Info("日志级别已更新", "level", levelVar.Level())
	}
}
