# Video Exporter

基于 Go 和 joy5 库的视频流监控导出系统，用于实时监控直播流的健康状况和质量指标。通过 HTTP-FLV 拉流，使用纯 Go 实现，无需外部依赖。

## 功能特性

- ✅ 实时流监控（多协程并发）
- ✅ 深度质量分析（码率、帧率、分辨率、GOP等）
- ✅ **网络指标采集**（HTTP 响应时间、TTFB、读取吞吐、读阻塞统计）
- ✅ 健康评估系统（可播放性、质量等级）
- ✅ **全链路监控**（支持项目 → 线路角色 → 流的三层结构）
- ✅ **自定义标签**（支持按店铺、商品类别等打标签）
- ✅ 自动重连机制（指数退避）
- ✅ 超时控制（避免上游卡死）
- ✅ 支持 HTTP-FLV 流格式（基于 joy5 库，纯 Go 实现）
- ✅ Prometheus 指标导出
- ✅ 结构化日志输出

## 快速开始

### 1. 安装 Go

确保已安装 Go 1.24 或更高版本：

**macOS:**
```bash
brew install go
```

**Ubuntu/Debian:**
```bash
sudo apt-get install -y golang-go
```

**验证安装：**
```bash
go version
```

### 2. 配置流地址

编辑 `config.yml`：

```yaml
exporter:
  check_interval: 30    # 检查间隔（秒）
  sample_duration: 10   # 采样时长（秒）
  min_keyframes: 2      # 最小关键帧数
  max_concurrent: 1000  # 最大并发数
  max_retries: 3        # 最大重试次数（指数退避）
  stall_threshold_ms: 200  # 读阻塞阈值（毫秒），超过此时间的单次读取视为阻塞
  listen_addr: "8080"   # Prometheus 监听端口
  log_level: "info"     # 日志级别：debug, info, warn, error

# 三层结构：项目 -> 线路角色 -> 流列表
streams:
  G01:                  # 项目/店铺 ID
    SOURCE:             # 线路角色（视频源 -> 视频服务）
      - url: http://srs-source/live/room01.flv
        id: store-01
        tags:
          table: store-01
          biz: electronics

    CDN:                # 线路角色（CDN -> 用户）
      - url: http://srs-cdn/live/room01.flv
        id: store-01-cdn
        tags:
          table: store-01
          biz: electronics
          isp: ct
```

**配置说明：**
- **第一层**（项目）：项目/店铺 ID，映射为 Prometheus label `project`
- **第二层**（线路角色）：SOURCE / SERVICE / CDN 等，映射为 label `line`（小写）
- **第三层**（流配置）：`url`（流地址）、`id`（流ID）、`tags`（自定义标签）
- **自定义标签**：支持 `table`（店铺）、`desk`（柜台）、`biz`（商品类别）、`isp`（运营商）、`role`（角色/用途标识）等业务标签（白名单控制）

### 3. 运行

```bash
# 方式1: 直接运行（推荐）
go run .

# 方式2: 编译后运行
go build -o video-exporter
./video-exporter
```

**说明**：
- `go run .` 会编译整个包（当前目录下所有 .go 文件），这是 Go 的标准做法
- `go run main.go` 只会编译 main.go，但 main.go 依赖其他文件中的函数，会报错
- `go run *.go` 虽然可以工作，但不是推荐做法

## 项目结构

```
video-exporter/
├── main.go                 # 程序入口
├── config.go               # 配置加载/结构体
├── logger.go               # 日志系统
├── exporter.go             # Prometheus 指标导出
├── scheduler.go            # 调度与并发检查
├── stream.go               # 核心流检查逻辑
├── config.yml              # 配置文件（挂载到容器 /app/config.yml）
├── Dockerfile              # 多阶段构建镜像
├── docker-compose.yml      # 本地/服务器编排与配置挂载
├── grafana-dashboard.json  # Grafana 仪表盘（按项目过滤）
├── Makefile                # 常用命令
├── run.sh                  # 运行脚本
├── go.mod
└── go.sum
```

## 监控输出

### 控制台输出
```
检查 #001 stream-01 stream-01 (https://...)
可播放: true | 质量: good | 响应: 150ms
视频包: 1234 | 关键帧: 45
码率: 2500.5kbps (平均: 2480.3kbps) | 稳定性: stable
帧率: 25.0fps | 分辨率: 1920x1080
编码: H.264 | GOP: 75帧
```

### Prometheus 指标
访问 `http://localhost:8080/metrics` 查看所有指标：

**基础指标示例：**
```
video_stream_up{project="G01",line="source",id="store-01",table="store-01",biz="electronics"} 1
video_stream_bitrate_bps{project="G01",line="cdn",id="store-01-cdn",table="store-01",isp="ct"} 1500000.0
video_stream_framerate{project="G01",line="source",id="store-01"} 25.0
```

**网络指标示例：**
```
video_stream_response_ms{project="G01",line="source",id="store-01"} 624.0
video_stream_ttfb_ms{project="G01",line="source",id="store-01"} 624.5
video_stream_read_throughput_bps{project="G01",line="cdn",id="store-01-cdn"} 1800000.0
video_stream_read_stall_count{project="G01",line="cdn",id="store-01-cdn"} 2
video_stream_read_stall_max_ms{project="G01",line="cdn",id="store-01-cdn"} 350.0
video_stream_read_stall_ratio{project="G01",line="cdn",id="store-01-cdn"} 0.15
```

## 监控指标

> 📖 **详细指标说明**: 查看 [METRICS.md](./METRICS.md) 了解每个指标的实现逻辑、计算方式和业务含义。

### 视频质量指标
- **基础统计**: 总包数、视频包数、音频包数、关键帧数量
- **码率**: 实时码率、平均码率、码率稳定性
- **帧率**: 实时帧率计算（基于 DTS 时间）
- **GOP**: 关键帧间隔分析
- **编码**: 视频编码格式（H.264/H.265等）
- **质量评分** (`video_stream_quality_score`): good/fair/poor（基于帧率和码率）
  - 0=poor, 1=fair, 2=good
- **稳定性评分** (`video_stream_stability_score`): stable/moderate/unstable（基于码率变异系数）
  - 0=unstable, 1=moderate, 2=stable
- **综合评分** (`video_stream_overall_score`): 综合考虑视频质量、稳定性和网络卡顿情况
  - **硬性卡顿判定**：如果 `read_stall_ratio > 0.5`（阻塞时间占比超过 50%），无论质量和稳定性如何，强制为 0（poor）
  - 在未触发硬性卡顿判定的情况下，使用离散等级 + 显式映射：
    - `(2,2)` → 2 (excellent: 质量好且稳定)
    - `(2,1)` → 1 (good: 质量好但稳定性中等)
    - `(2,0)` → 0 (poor: 质量好但不稳定，网络抖动严重)
    - `(1,2)` → 1 (good: 质量中等但稳定)
    - `(1,1)` → 1 (fair: 质量和稳定性都是中等)
    - `(0,*)` → 0 (poor: 质量差，无论稳定性如何)
  - 范围：0-2，越高越好

### 网络指标（新增）
- **HTTP 响应时间** (`video_stream_response_ms`): HTTP 响应头返回时间，单位：毫秒
  - 从发起请求到收到 HTTP 响应头的时间（包含 TCP/TLS 连接建立）
  - 过高可能表示网络或服务端响应慢
- **首字节时间** (`video_stream_ttfb_ms`): Time To First Byte，单位：毫秒
  - 从发起请求到读取第一个数据包的时间
  - 与 `response_ms` 的差值可以反映服务器开始传输数据的时间
  - **建议告警**：TTFB > 500ms 的流占比，用于判断服务侧响应慢或网络问题
- **读取吞吐** (`video_stream_read_throughput_bps`): 采样期间平均读取吞吐，单位：bps
  - 与视频码率差异可反映网络瓶颈
- **读阻塞统计**:
  - `video_stream_read_stall_count`: 单次读取阻塞超过阈值的次数（默认阈值 200ms，可通过 `stall_threshold_ms` 配置）
  - `video_stream_read_stall_max_ms`: 最长一次阻塞的时长（毫秒）
  - `video_stream_read_stall_total_ms`: 所有阻塞的合计时长（毫秒）
  - `video_stream_read_stall_ratio`: 阻塞时间占总采样时长的比例（0~1），值越高表示网络抖动越严重
    - **告警建议**：`read_stall_ratio > 0.5` 表示超过 50% 时间在阻塞，严重影响体验
    - 在 `overall_score` 计算中，如果 `read_stall_ratio > 0.5`，会强制将综合评分设为 0（poor）
  - 常用于判定网络抖动引起的卡顿

### 健康评估
- **可播放性**: 基于关键帧数和视频包数判断
- **健康状态**: 结合连续失败次数评估
- **响应时长**: FLV HTTP 请求响应时间（单位：ms）

### 指标类型说明
所有 `video_stream_*` 指标均为 **Gauge** 类型，表示：
- **每次采样周期结束时写入当前周期的值**（不是 lifetime 累加）
- 例如：`read_stall_count=19` 表示本次采样周期内发生了 19 次读阻塞
- 可以使用 PromQL 的 `avg_over_time()`、`sum_over_time()` 等函数进行二次计算
- 这种设计便于在 Grafana 中展示趋势，也便于做告警规则

### 全链路分析
通过三层配置结构（项目 → 线路角色 → 流），可以：
- **快速定位问题段**: 对比 SOURCE / SERVICE / CDN 各段的指标
- **区分问题类型**: 网络指标异常 → 网络问题；视频指标异常 → 视频本身问题
- **按业务维度聚合**: 使用自定义标签（table店铺、desk柜台、biz商品类别 等）进行业务分析

## 配置说明

| 参数 | 说明 | 默认值 |
|------|------|--------|
| check_interval | 健康检查间隔（秒） | 30 |
| sample_duration | 采样时长（秒） | 10 |
| min_keyframes | 最小关键帧数 | 2 |
| max_concurrent | 最大并发监控数 | 1000 |
| max_retries | 连接失败最大重试次数 | 3 |
| stall_threshold_ms | 读阻塞阈值（毫秒） | 200 |
| listen_addr | Prometheus 监听端口 | 8080 |
| log_level | 日志级别（debug/info/warn/error） | info |

## 支持的流格式

- **HTTP-FLV**（主要支持）：通过 HTTP 拉取 FLV 流，使用 joy5 库解析
- **其他格式**：当前版本主要支持 HTTP-FLV 格式。如需支持其他格式（RTMP、HLS、RTSP 等），需要扩展代码实现相应的协议解析

## 性能

| 流数量 | 内存占用 | CPU占用 |
|--------|----------|---------|
| 1路    | ~10MB    | <1%     |
| 10路   | ~30MB    | ~5%     |
| 100路  | ~200MB   | ~20%    |


## 编译与运行

### 开发环境运行
```bash
# 直接运行（推荐）
go run .

# 或者编译后运行
go build -o video-exporter
./video-exporter
```

### 生产环境编译
```bash
# 本地编译
go build -o video-exporter

# Linux
GOOS=linux GOARCH=amd64 go build -o video-exporter-linux

# Windows
GOOS=windows GOARCH=amd64 go build -o video-exporter.exe

# macOS
GOOS=darwin GOARCH=amd64 go build -o video-exporter-mac
```

**为什么用 `go run .` 而不是 `go run main.go`？**
- `go run .` 会编译整个包（当前目录下所有 .go 文件），这是 Go 的标准做法
- `go run main.go` 只会编译 main.go，但 main.go 依赖其他文件（logger.go, config.go 等）中的函数，会报错
- `go run *.go` 虽然可以工作，但不是推荐做法

## 部署

### 后台运行
```bash
nohup ./video-exporter > monitor.log 2>&1 &
```

### Systemd 服务
```ini
[Unit]
Description=Video Exporter
After=network.target

[Service]
Type=simple
User=nobody
WorkingDirectory=/opt/video-exporter
ExecStart=/opt/video-exporter/video-exporter
Restart=always

[Install]
WantedBy=multi-user.target
```

## Prometheus 集成

### 访问指标
```bash
# 查看所有指标
curl http://localhost:8080/metrics

# 在浏览器中访问
http://localhost:8080/metrics
```

### Prometheus 配置
```yaml
scrape_configs:
  - job_name: 'video-exporter'
    static_configs:
      - targets: ['localhost:8080']
    scrape_interval: 15s
```

### PromQL 查询示例

**按线路角色统计某项目过去 1 小时的平均质量评分：**
```promql
avg_over_time(video_stream_quality_score{project="G01"}[1h]) by (line)
```

**按店铺和线路角色统计 read stall 次数（累计）：**
```promql
sum_over_time(video_stream_read_stall_count{project="G01", table="store-01"}[1h]) by (line)
```

**统计 TTFB > 500ms 的流占比（用于判断服务侧响应慢）：**
```promql
count(video_stream_ttfb_ms{project="G01"} > 500) / count(video_stream_ttfb_ms{project="G01"})
```

**按业务类型统计综合评分：**
```promql
avg_over_time(video_stream_overall_score{project="G01"}[1h]) by (biz, line)
```

**对比各段链路的响应时间：**
```promql
avg_over_time(video_stream_response_ms{project="G01"}[5m]) by (line)
```

**找出读阻塞最严重的流：**
```promql
topk(10, video_stream_read_stall_count)
```

**网络吞吐与视频码率对比（判断网络瓶颈）：**
```promql
video_stream_read_throughput_bps / video_stream_bitrate_bps
# 如果比值 < 1.2，可能存在网络瓶颈
```

### 告警示例
```yaml
# 流离线告警
- alert: StreamDown
  expr: video_stream_up == 0
  for: 1m

# 低码率告警
- alert: LowBitrate
  expr: video_stream_bitrate_bps < 500000
  for: 2m

# 网络阻塞告警
- alert: HighReadStall
  expr: video_stream_read_stall_count > 5
  for: 1m

# 首字节时间过长告警
- alert: HighTTFB
  expr: video_stream_ttfb_ms > 1000
  for: 2m

# 阻塞占比过高告警
- alert: HighStallRatio
  expr: video_stream_read_stall_ratio > 0.5
  for: 1m

# 响应过慢告警
- alert: SlowResponse
  expr: video_stream_response_ms > 2000
  for: 1m
```


## 常见问题

### Q: 连接失败
A: 检查流地址是否正确，网络是否可达

### Q: 码率为0
A: 等待1-2个检查周期，让系统收集足够数据

### Q: 如何查看 Prometheus 指标
A: 访问 http://localhost:8080/metrics

### Q: 响应时间显示 N/A
A: 需要成功完成 HTTP 连接才会产生响应时间

## 许可证

MIT License
