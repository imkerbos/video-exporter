package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// allowedTagKeys 允许进入 Prometheus label 的自定义标签键（白名单）
// 语义说明：
//   - table: 店铺编号
//   - desk: 柜台编号（可选，与 table 类似）
//   - biz: 商品类别（electronics/clothing/food等）
//   - isp: 运营商（ct/cm/cu）
//   - role: 角色/用途标识（例如 test/prod）
//
// 注意：line_type 已移除，如需区分线路类型（电信/联通/移动/国际线路），可使用 isp 标签
var allowedTagKeys = map[string]struct{}{
	"table": {},
	"desk":  {},
	"biz":   {},
	"isp":   {},
	"role":  {},
}

// getPrometheusLabels 从 StreamMetrics.Labels 中提取 Prometheus 标签
// 返回：基础标签（project, line, id）+ 白名单中的自定义标签
func getPrometheusLabels(m StreamMetrics) map[string]string {
	labels := make(map[string]string)

	// 基础标签（必选）
	labels["project"] = m.Project
	labels["line"] = m.Line
	labels["id"] = m.ID

	// 从 Labels 中筛选白名单标签
	for k, v := range m.Labels {
		// 跳过基础标签（已在上面设置）
		if k == "project" || k == "line" || k == "id" {
			continue
		}
		// 只添加白名单中的标签
		if _, allowed := allowedTagKeys[k]; allowed {
			labels[k] = v
		}
	}

	return labels
}

// Exporter Prometheus 导出器
type Exporter struct {
	streamUp       *prometheus.GaugeVec
	streamHealthy  *prometheus.GaugeVec
	streamPlayable *prometheus.GaugeVec
	totalPackets   *prometheus.GaugeVec
	videoPackets   *prometheus.GaugeVec
	audioPackets   *prometheus.GaugeVec
	keyframes      *prometheus.GaugeVec
	currentBitrate *prometheus.GaugeVec
	avgBitrate     *prometheus.GaugeVec
	framerate      *prometheus.GaugeVec
	responseTime   *prometheus.GaugeVec
	gopSize        *prometheus.GaugeVec
	qualityScore   *prometheus.GaugeVec
	stabilityScore *prometheus.GaugeVec
	overallScore   *prometheus.GaugeVec // 综合评分（综合考虑质量和稳定性）

	// 网络指标
	// 注意：connect_latency_ms 已移除，语义与 response_ms 重复
	// response_ms: HTTP 响应头返回时间（在 responseTime 指标中）
	ttfb           *prometheus.GaugeVec
	readThroughput *prometheus.GaugeVec
	readStallCount *prometheus.GaugeVec
	readStallMax   *prometheus.GaugeVec
	readStallTotal *prometheus.GaugeVec
	readStallRatio *prometheus.GaugeVec

	scheduler *Scheduler
	log       *slog.Logger
}

// NewExporter 创建导出器
// 注意：由于 Prometheus 标签必须固定，我们使用基础标签（project, line, id）+ 可选标签（table, desk, biz, isp, role）
// 如果某个流没有某个可选标签，则使用空字符串
// 语义说明：
//   - project: 项目/店铺 ID
//   - line: 线路角色（SOURCE/SERVICE/CDN，拓扑节点）
//   - id: 流/店铺 ID
//   - table: 店铺编号（可选）
//   - desk: 柜台编号（可选）
//   - biz: 商品类别（可选）
//   - isp: 运营商（可选）
//   - role: 角色/用途标识（可选）
func NewExporter(scheduler *Scheduler) *Exporter {
	// 定义标签列表：基础标签 + 可选标签
	labelNames := []string{"project", "line", "id", "table", "desk", "biz", "isp", "role"}

	exporter := &Exporter{
		scheduler: scheduler,
		log:       GetLogger(),

		streamUp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_up",
				Help: "Stream is up (1) or down (0)",
			},
			labelNames,
		),

		streamHealthy: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_healthy",
				Help: "Stream health status (1=healthy, 0=unhealthy)",
			},
			labelNames,
		),

		streamPlayable: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_playable",
				Help: "Stream is playable (1=yes, 0=no)",
			},
			labelNames,
		),

		totalPackets: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_total_packets",
				Help: "Total packets received",
			},
			labelNames,
		),

		videoPackets: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_video_packets",
				Help: "Video packets received",
			},
			labelNames,
		),

		audioPackets: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_audio_packets",
				Help: "Audio packets received",
			},
			labelNames,
		),

		keyframes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_keyframes",
				Help: "Keyframes received",
			},
			labelNames,
		),

		currentBitrate: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_bitrate_bps",
				Help: "Current stream bitrate in bits per second",
			},
			labelNames,
		),

		avgBitrate: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_avg_bitrate_bps",
				Help: "Average stream bitrate in bits per second",
			},
			labelNames,
		),

		framerate: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_framerate",
				Help: "Stream framerate in fps",
			},
			labelNames,
		),

		responseTime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_response_ms",
				Help: "FLV HTTP request response time in milliseconds",
			},
			labelNames,
		),

		gopSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_gop_size",
				Help: "GOP size in frames",
			},
			labelNames,
		),

		qualityScore: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_quality_score",
				Help: "Stream quality score (0=poor, 1=fair, 2=good)",
			},
			labelNames,
		),

		stabilityScore: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_stability_score",
				Help: "Bitrate stability score (0=unstable, 1=moderate, 2=stable)",
			},
			labelNames,
		),

		overallScore: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_overall_score",
				Help: "Overall quality score considering both video quality and network stability (0=poor, 1=good/fair, 2=excellent)",
			},
			labelNames,
		),

		// 网络指标
		// 注意：connect_latency_ms 已移除，语义与 response_ms 重复
		// response_ms: HTTP 响应头返回时间（在 responseTime 指标中）
		ttfb: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_ttfb_ms",
				Help: "Time to first byte (TTFB) in milliseconds",
			},
			labelNames,
		),

		readThroughput: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_read_throughput_bps",
				Help: "Average read throughput during sampling period in bits per second",
			},
			labelNames,
		),

		readStallCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_read_stall_count",
				Help: "Number of read stalls (reads taking longer than 200ms)",
			},
			labelNames,
		),

		readStallMax: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_read_stall_max_ms",
				Help: "Maximum read stall duration in milliseconds",
			},
			labelNames,
		),

		readStallTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_read_stall_total_ms",
				Help: "Total read stall duration in milliseconds",
			},
			labelNames,
		),

		readStallRatio: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "video_stream_read_stall_ratio",
				Help: "Ratio of read stall time to total sampling duration (0~1). Higher values indicate more network jitter",
			},
			labelNames,
		),
	}

	// 注册指标
	prometheus.MustRegister(
		exporter.streamUp,
		exporter.streamHealthy,
		exporter.streamPlayable,
		exporter.totalPackets,
		exporter.videoPackets,
		exporter.audioPackets,
		exporter.keyframes,
		exporter.currentBitrate,
		exporter.avgBitrate,
		exporter.framerate,
		exporter.responseTime,
		exporter.gopSize,
		exporter.qualityScore,
		exporter.stabilityScore,
		exporter.overallScore,
		exporter.ttfb,
		exporter.readThroughput,
		exporter.readStallCount,
		exporter.readStallMax,
		exporter.readStallTotal,
		exporter.readStallRatio,
	)

	return exporter
}

// updateMetrics 更新指标
func (e *Exporter) updateMetrics() {
	e.log.Debug("开始更新指标")
	metrics := e.scheduler.GetAllMetrics()
	e.log.Debug("获取到指标", "数量", len(metrics))

	for _, m := range metrics {
		// 获取 Prometheus 标签（基础标签 + 白名单标签）
		promLabels := getPrometheusLabels(m)

		// 构建标签值列表（按固定顺序：project, line, id, table, desk, biz, isp, role）
		labelValues := []string{
			promLabels["project"],
			promLabels["line"],
			promLabels["id"],
			promLabels["table"], // 可能为空
			promLabels["desk"],  // 可能为空
			promLabels["biz"],   // 可能为空
			promLabels["isp"],   // 可能为空
			promLabels["role"],  // 可能为空
		}

		// 流状态
		upValue := 0.0
		if m.Healthy {
			upValue = 1.0
		}
		e.streamUp.WithLabelValues(labelValues...).Set(upValue)

		// 健康状态
		healthValue := 0.0
		if m.Healthy && m.ConsecutiveFails == 0 {
			healthValue = 1.0
		}
		e.streamHealthy.WithLabelValues(labelValues...).Set(healthValue)

		// 可播放状态
		playableValue := 0.0
		if m.Playable {
			playableValue = 1.0
		}
		e.streamPlayable.WithLabelValues(labelValues...).Set(playableValue)

		// 数据包统计
		e.totalPackets.WithLabelValues(labelValues...).Set(float64(m.TotalPackets))
		e.videoPackets.WithLabelValues(labelValues...).Set(float64(m.VideoPackets))
		e.audioPackets.WithLabelValues(labelValues...).Set(float64(m.AudioPackets))
		e.keyframes.WithLabelValues(labelValues...).Set(float64(m.Keyframes))

		// 码率指标
		e.currentBitrate.WithLabelValues(labelValues...).Set(m.CurrentBitrate)
		e.avgBitrate.WithLabelValues(labelValues...).Set(m.AvgBitrate)

		// 其他质量指标
		e.framerate.WithLabelValues(labelValues...).Set(m.Framerate)
		e.responseTime.WithLabelValues(labelValues...).Set(float64(m.Response))
		e.gopSize.WithLabelValues(labelValues...).Set(float64(m.GOPSize))

		// 质量评分
		qualityScore := 0.0
		switch m.Quality {
		case "good":
			qualityScore = 2.0
		case "fair":
			qualityScore = 1.0
		case "poor":
			qualityScore = 0.0
		}
		e.qualityScore.WithLabelValues(labelValues...).Set(qualityScore)

		// 稳定性评分
		stabilityScore := 0.0
		switch m.BitrateStability {
		case "stable":
			stabilityScore = 2.0
		case "moderate":
			stabilityScore = 1.0
		case "unstable":
			stabilityScore = 0.0
		}
		e.stabilityScore.WithLabelValues(labelValues...).Set(stabilityScore)

		// 综合评分：综合考虑视频质量和网络稳定性
		// 硬性卡顿判定：如果 read_stall_ratio > 0.5，强制为 poor（0）
		// 理由：高卡顿占比即使码率/帧率再好，也是不良体验
		var overallScore float64
		if m.ReadStallRatio > 0.5 {
			// 阻塞时间占比超过 50%，强制判定为 poor
			overallScore = 0.0
		} else {
			// 使用离散等级 + 显式映射，更清晰直观
			// 映射规则：
			//   - (2,2) -> 2 (excellent: 质量好且稳定)
			//   - (2,1) -> 1 (good: 质量好但稳定性中等)
			//   - (2,0) -> 0 (poor: 质量好但不稳定，网络抖动严重)
			//   - (1,2) -> 1 (good: 质量中等但稳定)
			//   - (1,1) -> 1 (fair: 质量和稳定性都是中等)
			//   - (1,0) -> 0 (poor: 质量中等且不稳定)
			//   - (0,2) -> 0 (poor: 质量差，即使稳定也没用)
			//   - (0,1) -> 0 (poor: 质量差)
			//   - (0,0) -> 0 (poor: 质量差且不稳定)
			switch {
			case qualityScore == 2.0 && stabilityScore == 2.0:
				overallScore = 2.0 // excellent
			case qualityScore == 2.0 && stabilityScore == 1.0:
				overallScore = 1.0 // good (质量好但稳定性中等)
			case qualityScore == 2.0 && stabilityScore == 0.0:
				overallScore = 0.0 // poor (质量好但不稳定，网络抖动严重)
			case qualityScore == 1.0 && stabilityScore == 2.0:
				overallScore = 1.0 // good (质量中等但稳定)
			case qualityScore == 1.0 && stabilityScore == 1.0:
				overallScore = 1.0 // fair
			case qualityScore == 1.0 && stabilityScore == 0.0:
				overallScore = 0.0 // poor
			default: // qualityScore == 0.0 的情况
				overallScore = 0.0 // poor (质量差，无论稳定性如何)
			}
		}
		e.overallScore.WithLabelValues(labelValues...).Set(overallScore)

		// 网络指标
		// response_ms: HTTP 响应头返回时间（在 responseTime 指标中，已在上方设置）
		// ttfb_ms: 首字节时间（从请求开始到第一个数据包读取的时间）
		e.ttfb.WithLabelValues(labelValues...).Set(m.TTFBMs)
		e.readThroughput.WithLabelValues(labelValues...).Set(m.ReadThroughputBps)
		e.readStallCount.WithLabelValues(labelValues...).Set(float64(m.ReadStallCount))
		e.readStallMax.WithLabelValues(labelValues...).Set(m.ReadStallMaxMs)
		e.readStallTotal.WithLabelValues(labelValues...).Set(m.ReadStallTotalMs)
		e.readStallRatio.WithLabelValues(labelValues...).Set(m.ReadStallRatio)
	}

	e.log.Debug("指标更新完成")
}

// StartHTTPServer 启动 HTTP 服务器
func (e *Exporter) StartHTTPServer(addr string) error {
	mux := http.NewServeMux()

	// Prometheus metrics endpoint - 每次请求时更新指标
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		e.log.Debug("收到 metrics 请求")
		e.updateMetrics()
		e.log.Debug("指标更新完成")
		promhttp.Handler().ServeHTTP(w, r)
	})

	// 首页
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html>
<head><title>Video Stream Exporter</title></head>
<body>
<h1>Video Stream Exporter</h1>
<p><a href="/metrics">Metrics</a></p>
</body>
</html>`)
	})

	e.log.Info("Prometheus exporter 启动", "地址", addr)
	e.log.Info("访问指标", "URL", fmt.Sprintf("http://localhost%s/metrics", addr))

	return http.ListenAndServe(addr, mux)
}
