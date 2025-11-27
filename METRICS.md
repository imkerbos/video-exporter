# 指标实现逻辑与含义说明

本文档详细说明每个 Prometheus 指标的实现逻辑、计算方式和业务含义。

## 指标分类

### 1. 基础状态指标

#### `video_stream_up`
- **类型**: Gauge
- **含义**: 流是否在线（1=在线，0=离线）
- **实现逻辑**:
  ```go
  if m.Healthy {
      upValue = 1.0
  } else {
      upValue = 0.0
  }
  ```
- **更新时机**: 每次采样周期结束时
- **业务价值**: 快速判断流是否可用

#### `video_stream_healthy`
- **类型**: Gauge
- **含义**: 流健康状态（1=健康，0=不健康）
- **实现逻辑**:
  ```go
  if m.Healthy && m.ConsecutiveFails == 0 {
      healthValue = 1.0
  } else {
      healthValue = 0.0
  }
  ```
- **更新时机**: 每次采样周期结束时
- **业务价值**: 区分"在线但有问题"和"完全离线"

#### `video_stream_playable`
- **类型**: Gauge
- **含义**: 流是否可播放（1=可播放，0=不可播放）
- **实现逻辑**:
  ```go
  // 在 stream.go 的 Check() 方法中
  sc.playable = keyframeCount >= 2 && videoCount > 10
  ```
- **判断标准**:
  - 至少收到 2 个关键帧
  - 至少收到 10 个视频包
- **业务价值**: 判断流是否真正可用（有足够数据播放）

---

### 2. 数据包统计指标

#### `video_stream_total_packets`
- **类型**: Gauge
- **含义**: 本次采样周期内收到的总包数
- **实现逻辑**:
  ```go
  // 在读取循环中累加
  packetCount++
  sc.totalPackets = int64(packetCount)
  ```
- **更新时机**: 每次采样周期结束时
- **业务价值**: 监控数据包接收情况

#### `video_stream_video_packets`
- **类型**: Gauge
- **含义**: 本次采样周期内收到的视频包数
- **实现逻辑**:
  ```go
  // 在 stream.go 的 Check() 方法中
  switch pkt.Type {
  case av.H264:
      videoCount++
  }
  sc.videoPackets = int64(videoCount)
  ```
- **业务价值**: 监控视频数据包接收情况

#### `video_stream_audio_packets`
- **类型**: Gauge
- **含义**: 本次采样周期内收到的音频包数
- **实现逻辑**: 类似 video_packets，统计 `av.AAC` 类型的包
- **业务价值**: 监控音频数据包接收情况

#### `video_stream_keyframes`
- **类型**: Gauge
- **含义**: 本次采样周期内收到的关键帧数
- **实现逻辑**:
  ```go
  if pkt.IsKeyFrame {
      keyframeCount++
  }
  sc.keyframes = int64(keyframeCount)
  ```
- **业务价值**: 关键帧数量影响播放体验，数量不足会导致卡顿

---

### 3. 视频质量指标

#### `video_stream_bitrate_bps`
- **类型**: Gauge
- **含义**: 当前流的实时码率（bits per second）
- **实现逻辑**:
  ```go
  // 优先使用 DTS 时间计算（更准确）
  if lastDTS > firstDTS {
      dtsElapsed := float64(lastDTS-firstDTS) / 1e9 // 纳秒转秒
      sc.currentBitrate = (float64(totalBytes) * 8) / dtsElapsed
  } else {
      // 如果没有 DTS，使用实际耗时
      sc.currentBitrate = (float64(totalBytes) * 8) / duration.Seconds()
  }
  ```
- **计算方式**: `总字节数 * 8 / 时间（秒）`
- **业务价值**: 码率过低会导致画质差，过高可能浪费带宽

#### `video_stream_avg_bitrate_bps`
- **类型**: Gauge
- **含义**: 平均码率（基于最近 10 次采样的历史数据）
- **实现逻辑**:
  ```go
  // 维护码率历史（最多 10 个值）
  sc.bitrateHistory = append(sc.bitrateHistory, sc.currentBitrate)
  if len(sc.bitrateHistory) > 10 {
      sc.bitrateHistory = sc.bitrateHistory[1:]
  }
  
  // 计算平均值
  sum := 0.0
  for i := 0; i < len(sc.bitrateHistory); i++ {
      sum += sc.bitrateHistory[i]
  }
  sc.avgBitrate = sum / float64(len(sc.bitrateHistory))
  ```
- **业务价值**: 平滑码率波动，反映长期趋势

#### `video_stream_framerate`
- **类型**: Gauge
- **含义**: 实时帧率（frames per second）
- **实现逻辑**:
  ```go
  if lastDTS > firstDTS {
      dtsElapsed := float64(lastDTS-firstDTS) / 1e9
      sc.framerate = float64(videoCount) / dtsElapsed
  }
  ```
- **计算方式**: `视频包数 / DTS 时间跨度（秒）`
- **业务价值**: 帧率过低会导致画面不流畅

#### `video_stream_gop_size`
- **类型**: Gauge
- **含义**: GOP 大小（关键帧间隔的帧数）
- **实现逻辑**:
  ```go
  if keyframeCount > 1 {
      keyframeInterval = videoCount / keyframeCount
  } else if keyframeCount == 1 {
      keyframeInterval = videoCount
  } else {
      keyframeInterval = 0
  }
  sc.gopSize = keyframeInterval
  ```
- **业务价值**: GOP 过大可能导致切换频道时等待时间长

#### `video_stream_quality_score`
- **类型**: Gauge
- **含义**: 视频质量评分（0=poor, 1=fair, 2=good）
- **实现逻辑**:
  ```go
  // 在 stream.go 的 Check() 方法中评估
  if sc.playable {
      if sc.framerate >= 25 && sc.currentBitrate >= 600000 {
          sc.quality = "good"  // 高质量：帧率>=25fps，码率>=600kbps
      } else if sc.framerate >= 20 && sc.currentBitrate >= 400000 {
          sc.quality = "fair"  // 中等质量：帧率>=20fps，码率>=400kbps
      } else {
          sc.quality = "poor"  // 低质量
      }
  } else {
      sc.quality = "poor"
  }
  
  // 在 exporter.go 中映射为数值
  switch m.Quality {
  case "good":  qualityScore = 2.0
  case "fair":  qualityScore = 1.0
  case "poor":  qualityScore = 0.0
  }
  ```
- **评估标准**:
  - **good**: 帧率 >= 25fps 且码率 >= 600kbps
  - **fair**: 帧率 >= 20fps 且码率 >= 400kbps
  - **poor**: 其他情况或不可播放
- **业务价值**: 快速判断视频质量等级

#### `video_stream_stability_score`
- **类型**: Gauge
- **含义**: 码率稳定性评分（0=unstable, 1=moderate, 2=stable）
- **实现逻辑**:
  ```go
  // 计算码率历史的标准差和变异系数（CV）
  variance := 0.0
  for i := 0; i < historyLen; i++ {
      diff := sc.bitrateHistory[i] - sc.avgBitrate
      variance += diff * diff
  }
  variance /= float64(historyLen)
  stdDev := math.Sqrt(variance)
  cv := stdDev / sc.avgBitrate  // 变异系数
  
  // 根据 CV 评估稳定性
  if cv < 0.15 {
      sc.bitrateStability = "stable"     // CV < 15% = 稳定
  } else if cv < 0.30 {
      sc.bitrateStability = "moderate"    // CV < 30% = 中等
  } else {
      sc.bitrateStability = "unstable"    // CV >= 30% = 不稳定
  }
  
  // 在 exporter.go 中映射为数值
  switch m.BitrateStability {
  case "stable":    stabilityScore = 2.0
  case "moderate":  stabilityScore = 1.0
  case "unstable":  stabilityScore = 0.0
  }
  ```
- **评估标准**:
  - **stable**: 变异系数（CV）< 15%
  - **moderate**: 15% <= CV < 30%
  - **unstable**: CV >= 30%
- **业务价值**: 判断码率波动情况，稳定性差可能导致播放卡顿

#### `video_stream_overall_score`
- **类型**: Gauge
- **含义**: 综合评分（综合考虑视频质量和网络稳定性）
- **实现逻辑**:
  ```go
  // 硬性卡顿判定：如果 read_stall_ratio > 0.5，强制为 poor（0）
  // 理由：高卡顿占比即使码率/帧率再好，也是不良体验
  var overallScore float64
  if m.ReadStallRatio > 0.5 {
      // 阻塞时间占比超过 50%，强制判定为 poor
      overallScore = 0.0
  } else {
      // 使用离散等级 + 显式映射
      switch {
      case qualityScore == 2.0 && stabilityScore == 2.0:
          overallScore = 2.0  // excellent: 质量好且稳定
      case qualityScore == 2.0 && stabilityScore == 1.0:
          overallScore = 1.0  // good: 质量好但稳定性中等
      case qualityScore == 2.0 && stabilityScore == 0.0:
          overallScore = 0.0  // poor: 质量好但不稳定，网络抖动严重
      case qualityScore == 1.0 && stabilityScore == 2.0:
          overallScore = 1.0  // good: 质量中等但稳定
      case qualityScore == 1.0 && stabilityScore == 1.0:
          overallScore = 1.0  // fair
      case qualityScore == 1.0 && stabilityScore == 0.0:
          overallScore = 0.0  // poor
      default:  // qualityScore == 0.0 的情况
          overallScore = 0.0  // poor: 质量差，无论稳定性如何
      }
  }
  ```
- **硬性卡顿判定**: 如果 `read_stall_ratio > 0.5`（阻塞时间占比超过 50%），无论质量和稳定性如何，`overall_score` 强制为 0（poor）
- **映射规则**（在未触发硬性卡顿判定的情况下）:
  | quality_score | stability_score | overall_score | 说明 |
  |--------------|-----------------|---------------|------|
  | 2 (good)     | 2 (stable)      | 2 (excellent) | 质量好且稳定 |
  | 2 (good)     | 1 (moderate)    | 1 (good)     | 质量好但稳定性中等 |
  | 2 (good)     | 0 (unstable)    | 0 (poor)     | 质量好但不稳定，网络抖动严重 |
  | 1 (fair)     | 2 (stable)      | 1 (good)     | 质量中等但稳定 |
  | 1 (fair)     | 1 (moderate)    | 1 (fair)     | 质量和稳定性都是中等 |
  | 1 (fair)     | 0 (unstable)    | 0 (poor)     | 质量中等且不稳定 |
  | 0 (poor)     | *               | 0 (poor)     | 质量差，无论稳定性如何 |
- **业务价值**: 给"老板看的那条总分"，综合考虑质量、稳定性和网络卡顿情况

---

### 4. 网络指标

#### `video_stream_response_ms`
- **类型**: Gauge
- **含义**: HTTP 响应头返回时间（毫秒）
- **实现逻辑**:
  ```go
  reqStart := time.Now()
  resp, err := globalHTTPClient.Do(req)
  responseHeaderTime := time.Since(reqStart)
  sc.response = responseHeaderTime.Milliseconds()
  ```
- **测量点**: 从发起 HTTP 请求到收到响应头的时间
- **包含内容**: TCP/TLS 连接建立 + HTTP 请求 + 响应头返回
- **业务价值**: 判断服务端响应速度，过高可能表示网络或服务端问题

#### `video_stream_ttfb_ms`
- **类型**: Gauge
- **含义**: 首字节时间（Time To First Byte，毫秒）
- **实现逻辑**:
  ```go
  // 在 stallTrackingReader 的 Read() 方法中记录第一次真正读到数据的时间
  // 必须在 n > 0 时才记录，确保是真正读取到数据
  if n > 0 && !r.firstReadDone {
      *r.firstReadTime = readStart
      r.firstReadDone = true
  }
  
  // 在 Check() 方法中计算
  if !firstReadTime.IsZero() {
      ttfbMs = firstReadTime.Sub(reqStart).Seconds() * 1000
  }
  ```
- **测量点**: 从发起 HTTP 请求到真正读取到第一个数据包（n > 0）的时间
- **与 response_ms 的关系**: `ttfb_ms - response_ms` 可以反映服务器开始传输数据的延迟
- **业务价值**: 判断首包到达时间，建议告警：TTFB > 500ms 的流占比

#### `video_stream_read_throughput_bps`
- **类型**: Gauge
- **含义**: 采样期间平均读取吞吐（bits per second）
- **实现逻辑**:
  ```go
  // 在 stallTrackingReader 的 Read() 方法中累加字节数
  if n > 0 {
      *r.totalBytes += int64(n)
  }
  
  // 在 Check() 方法中计算
  sampleDurationSeconds := duration.Seconds()
  if sampleDurationSeconds > 0 {
      readThroughputBps = float64(totalBytes*8) / sampleDurationSeconds
  }
  ```
- **计算方式**: `总读取字节数 * 8 / 采样时长（秒）`
- **业务价值**: 与视频码率对比，如果读取吞吐 < 码率，可能存在网络瓶颈

#### `video_stream_read_stall_count`
- **类型**: Gauge
- **含义**: 单次读取阻塞超过阈值的次数
- **实现逻辑**:
  ```go
  // 在 stallTrackingReader 的 Read() 方法中
  readStart := time.Now()
  n, err := r.reader.Read(p)
  elapsed := time.Since(readStart)
  
  if elapsed > r.stallThreshold {
      *r.stallCount++
  }
  ```
- **阈值**: 默认 200ms，可通过配置 `stall_threshold_ms` 调整
- **业务价值**: 统计网络抖动次数，次数多表示网络不稳定

#### `video_stream_read_stall_max_ms`
- **类型**: Gauge
- **含义**: 最长一次读阻塞的时长（毫秒）
- **实现逻辑**:
  ```go
  if elapsed > stallThreshold {
      if elapsed > *r.maxStall {
          *r.maxStall = elapsed
      }
  }
  sc.readStallMaxMs = maxStall.Seconds() * 1000
  ```
- **业务价值**: 判断最严重的网络阻塞情况

#### `video_stream_read_stall_total_ms`
- **类型**: Gauge
- **含义**: 所有读阻塞的合计时长（毫秒）
- **实现逻辑**:
  ```go
  if elapsed > stallThreshold {
      *r.totalStall += elapsed
  }
  sc.readStallTotalMs = totalStall.Seconds() * 1000
  ```
- **业务价值**: 判断采样周期内总阻塞时间，例如：7.6 秒阻塞 / 10 秒采样 = 76% 时间在阻塞

#### `video_stream_read_stall_ratio`
- **类型**: Gauge
- **含义**: 阻塞时间占总采样时长的比例（0~1）
- **实现逻辑**:
  ```go
  // 计算阻塞时间占比（阻塞时间 / 采样时长）
  readStallRatio := 0.0
  if sampleDurationSeconds > 0 {
      totalStallSeconds := totalStall.Seconds()
      readStallRatio = totalStallSeconds / sampleDurationSeconds
      // 限制在 0~1 范围内
      if readStallRatio > 1.0 {
          readStallRatio = 1.0
      }
  }
  sc.readStallRatio = readStallRatio
  ```
- **计算方式**: `总阻塞时长（秒） / 采样时长（秒）`
- **业务价值**: 
  - 直观反映网络卡顿占比，值越高表示网络抖动越严重
  - 可用于告警：`read_stall_ratio > 0.5` 表示超过 50% 时间在阻塞，严重影响体验
  - 在 `overall_score` 计算中，如果 `read_stall_ratio > 0.5`，会强制将综合评分设为 0（poor）
- **示例**:
  - `read_stall_ratio = 0.0`：无阻塞，网络流畅
  - `read_stall_ratio = 0.3`：30% 时间在阻塞，存在轻微网络抖动
  - `read_stall_ratio = 0.76`：76% 时间在阻塞，网络严重不稳定
  - `read_stall_ratio = 1.0`：整个采样周期都在阻塞，网络几乎不可用

---

## 指标更新机制

### 采样周期
- **触发**: 根据 `check_interval` 配置（默认 60 秒）
- **采样时长**: 根据 `sample_duration` 配置（默认 10 秒）
- **提前退出条件**: 
  - 达到采样时长且收集到足够关键帧（`min_keyframes`，默认 2 个）
  - 或超过采样时长的 2 倍（避免长时间阻塞）

### 指标类型
- **所有指标均为 Gauge 类型**
- **每次采样周期结束时写入当前周期的值**（不是 lifetime 累加）
- 可以使用 PromQL 的 `avg_over_time()`、`sum_over_time()` 等函数进行二次计算

### 示例
- `read_stall_count=19` 表示本次采样周期内发生了 19 次读阻塞
- `read_stall_total_ms=7643` 表示本次采样周期内总阻塞时间为 7.6 秒
- 使用 `sum_over_time(video_stream_read_stall_count[1h])` 可以统计过去 1 小时的累计阻塞次数

---

## 业务场景示例

### 场景 1: 判断网络抖动
```
quality_score = 2 (good)
stability_score = 0 (unstable)
read_stall_count = 19
read_stall_total_ms = 7643
overall_score = 0 (poor)
```
**结论**: 视频本身质量好，但网络抖动严重（7.6 秒阻塞），导致综合评分低。

### 场景 2: 判断服务端响应慢
```
response_ms = 840
ttfb_ms = 840
```
**结论**: response_ms 和 ttfb_ms 几乎一样，说明响应头返回后立即开始传输数据，但整体响应时间较长（840ms），可能是服务端处理慢或网络延迟高。

### 场景 3: 判断网络瓶颈
```
bitrate_bps = 2.67Mbps
read_throughput_bps = 2.43Mbps
```
**结论**: 读取吞吐略低于视频码率，可能存在轻微的网络瓶颈。

