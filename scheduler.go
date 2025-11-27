package main

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Scheduler 调度器
type Scheduler struct {
	checkers map[string]*StreamChecker
	config   *Config
	mu       sync.RWMutex
	stopChan chan struct{}
	log      *slog.Logger
}

// NewScheduler 创建调度器
func NewScheduler(config *Config) *Scheduler {
	return &Scheduler{
		checkers: make(map[string]*StreamChecker),
		config:   config,
		stopChan: make(chan struct{}),
		log:      GetLogger(),
	}
}

// AddStream 添加流
func (s *Scheduler) AddStream(id, url, project, line string, labels map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s::%s::%s", project, line, url)
	checker := NewStreamChecker(id, url, project, line, labels)
	s.checkers[key] = checker

	s.log.Info("添加流", "流ID", id, "URL", url, "项目", project, "线路", line)
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.log.Info("启动调度器",
		"流数量", len(s.checkers),
		"检查间隔秒", s.config.Exporter.CheckInterval,
		"最大并发", s.config.Exporter.MaxConcurrent,
		"最大重试", s.config.Exporter.MaxRetries)

	// 立即执行第一次检查（在后台）
	go s.runCheckCycle()

	// 定时执行检查
	ticker := time.NewTicker(time.Duration(s.config.Exporter.CheckInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			s.log.Info("调度器已停止")
			return
		case <-ticker.C:
			go s.runCheckCycle()
		}
	}
}

// runCheckCycle 执行一轮检查
func (s *Scheduler) runCheckCycle() {
	cycleStartTime := time.Now()

	s.mu.RLock()
	checkers := make([]*StreamChecker, 0, len(s.checkers))
	for _, checker := range s.checkers {
		checkers = append(checkers, checker)
	}
	s.mu.RUnlock()

	s.log.Info("开始检查周期",
		"流数量", len(checkers),
		"最大并发", s.config.Exporter.MaxConcurrent,
		"采样时长秒", s.config.Exporter.SampleDuration)

	// 使用信号量控制并发
	semaphore := make(chan struct{}, s.config.Exporter.MaxConcurrent)
	var wg sync.WaitGroup
	var successCount, failCount int64
	var mu sync.Mutex

	for _, checker := range checkers {
		wg.Add(1)
		go func(c *StreamChecker) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 执行检查，带重试
			err := s.checkWithRetry(c)

			// 统计成功/失败数量
			mu.Lock()
			if err == nil {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}(checker)
	}

	wg.Wait()

	cycleDuration := time.Since(cycleStartTime)
	s.log.Info("检查周期完成",
		"总耗时秒", fmt.Sprintf("%.2f", cycleDuration.Seconds()),
		"成功", successCount,
		"失败", failCount,
		"总数", len(checkers))
}

// checkWithRetry 带重试的检查
// 返回 nil 表示成功，返回 error 表示所有重试都失败
func (s *Scheduler) checkWithRetry(checker *StreamChecker) error {
	// 超时时间：采样时间 + 网络缓冲(5秒)
	sampleDurationSec := 10
	if s.config.Exporter.SampleDuration > 0 {
		sampleDurationSec = s.config.Exporter.SampleDuration
	}
	timeout := time.Duration(sampleDurationSec+5) * time.Second

	// 如果检查间隔很长，可以给更多时间
	if s.config.Exporter.CheckInterval > 20 {
		timeout = time.Duration(s.config.Exporter.CheckInterval-5) * time.Second
	}

	var lastErr error
	for attempt := 0; attempt <= s.config.Exporter.MaxRetries; attempt++ {
		if attempt > 0 {
			// 指数退避：2^attempt 秒（2s, 4s, 8s, ...）
			retryDelay := time.Duration(1<<uint(attempt)) * time.Second
			s.log.Info("等待重试", "流ID", checker.id, "尝试次数", attempt, "延迟秒", retryDelay.Seconds())
			time.Sleep(retryDelay)
		}

		err := checker.Check(timeout)
		if err == nil {
			// 成功
			return nil
		}

		lastErr = err
		s.log.Warn("检查失败", "流ID", checker.id, "尝试次数", attempt+1, "最大重试", s.config.Exporter.MaxRetries+1, "错误", err)
	}

	// 所有重试都失败
	checker.MarkFailed()
	s.log.Error("达到最大重试次数，标记为失败",
		"流ID", checker.id,
		"总尝试次数", s.config.Exporter.MaxRetries+1,
		"最后错误", lastErr)
	return lastErr
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	close(s.stopChan)
}

// GetAllMetrics 获取所有流的指标
func (s *Scheduler) GetAllMetrics() []StreamMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := make([]StreamMetrics, 0, len(s.checkers))
	for _, checker := range s.checkers {
		metrics = append(metrics, checker.GetMetrics())
	}

	return metrics
}
