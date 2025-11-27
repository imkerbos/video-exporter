package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	// 设置时区为上海
	os.Setenv("TZ", "Asia/Shanghai")
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err == nil {
		time.Local = loc
	}

	// 初始化日志
	InitLogger()
	log := GetLogger()

	log.Info("启动 Video Stream Exporter")

	// 加载配置
	cfg, err := LoadConfig("config.yml")
	if err != nil {
		log.Error("加载配置失败", "错误", err)
		os.Exit(1)
	}

	// 设置日志级别
	SetLogLevel(cfg.Exporter.LogLevel)

	// 设置全局配置
	SetGlobalConfig(cfg)

	// 创建调度器
	scheduler := NewScheduler(cfg)

	// 添加所有流（三层结构：项目 -> 线路角色 -> 流列表）
	totalStreams := 0
	for projectID, groups := range cfg.Streams {
		for groupName, streamList := range groups {
			line := strings.ToLower(groupName) // 线路角色转为小写（source / cdn / service）

			for _, sc := range streamList {
				// 构造标签 map
				tags := make(map[string]string)

				// 合并 tags map（推荐方式）
				for k, v := range sc.Tags {
					tags[k] = v
				}

				// 兼容 tag 字段（简单写法）
				if sc.Tag != "" {
					if _, exists := tags["tag"]; !exists {
						tags["tag"] = sc.Tag
					}
				}

				// 系统固定标签
				tags["project"] = projectID
				tags["line"] = line
				tags["id"] = sc.ID

				// 创建 StreamChecker 并注册到 Scheduler
				scheduler.AddStream(sc.ID, sc.URL, projectID, line, tags)
				totalStreams++

				log.Debug("加载流配置",
					"项目", projectID,
					"线路", line,
					"流ID", sc.ID,
					"URL", sc.URL,
					"标签数", len(tags))
			}
		}
	}

	log.Info("已加载流", "总数", totalStreams)

	// 启动调度器
	go scheduler.Start()

	// 创建并启动 Prometheus exporter
	exporter := NewExporter(scheduler)

	listenAddr := cfg.Exporter.ListenAddr
	if listenAddr == "" {
		listenAddr = ":8080"
	}
	if listenAddr[0] != ':' {
		listenAddr = ":" + listenAddr
	}

	// 启动 HTTP 服务器
	go func() {
		if err := exporter.StartHTTPServer(listenAddr); err != nil {
			log.Error("HTTP 服务器错误", "错误", err)
		}
	}()

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("服务已启动，按 Ctrl+C 停止")

	<-sigChan
	log.Info("收到停止信号")

	// 停止调度器
	scheduler.Stop()

	log.Info("服务已停止")
}
