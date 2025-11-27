package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 配置结构
type Config struct {
	Exporter ExporterConfig                       `yaml:"exporter"`
	Streams  map[string]map[string][]StreamConfig `yaml:"streams"` // project -> line -> streams
	// 第一层 key: 项目/店铺 ID，例如 "G01"
	// 第二层 key: 线路角色/分组，例如 "SOURCE" / "CDN" / "SERVICE"
	// 第三层: 流列表
}

// ExporterConfig 导出器配置
type ExporterConfig struct {
	CheckInterval    int    `yaml:"check_interval"`  // 检查间隔（秒）
	SampleDuration   int    `yaml:"sample_duration"` // 采样时长（秒），默认10秒
	MinKeyframes     int    `yaml:"min_keyframes"`   // 最小关键帧数，默认2
	MaxConcurrent    int    `yaml:"max_concurrent"`
	MaxRetries       int    `yaml:"max_retries"`
	StallThresholdMs int    `yaml:"stall_threshold_ms"` // 读阻塞阈值（毫秒），默认200ms
	ListenAddr       string `yaml:"listen_addr"`        // Prometheus exporter 监听地址
	LogLevel         string `yaml:"log_level"`          // 日志级别
}

// StreamConfig 流配置
type StreamConfig struct {
	URL  string            `yaml:"url"`            // 流地址
	ID   string            `yaml:"id"`             // 流/店铺 ID
	Tag  string            `yaml:"tag,omitempty"`  // 简单 tag 写法（向后兼容）
	Tags map[string]string `yaml:"tags,omitempty"` // 自定义标签 map（推荐使用）
}

// LoadConfig 加载配置文件
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// 全局配置
var globalConfig *Config

func SetGlobalConfig(cfg *Config) {
	globalConfig = cfg
}
