# 腾讯 CLS + Prometheus 真实接入配置指南

本文说明如何把 watchTower 的两条数据链路从"本地 mock"切换到"真实腾讯云 CLS + 真实 Prometheus"，并保证两者通过同一批关键字、同一批告警名彼此配合，让 Agent 能跑通
`告警 → 查内部文档 → 查日志 → 根因分析 → 处置策略` 闭环。

---

## 0. 先看懂两条链路的接入点

| 链路 | Agent 工具 | 代码 | 接入点（配置项） | 数据来源 |
|---|---|---|---|---|
| 告警 | `query_prometheus_alerts` | [ai/tools/query_metric_alerts.go](../ai/tools/query_metric_alerts.go) | `etc/conf.yml` → `prometheus.base_url` | Prometheus HTTP API `GET /api/v1/alerts` |
| 日志 | `query_log` | [ai/tools/query_log.go](../ai/tools/query_log.go) | `cls.*`（直连，**推荐**）或 `mcp_url`（MCP 回退） | 腾讯云 CLS（直连走 SearchLog API；回退走 MCP/SSE） |
| 文档 | `query_internal_docs` | [ai/tools/query_internal_docs.go](../ai/tools/query_internal_docs.go) | `etc/conf.yml` → `milvus` | Milvus 知识库（已索引 knowledge/*.md） |

> **`query_log` 两种接入方式**：
> - **CLS 直连（推荐）**：配 `cls.*`，直接调 CLS SearchLog API。稳定、不依赖易失效的 MCP token，已实跑验证。
> - **MCP 回退**：配 `mcp_url`，走腾讯云 CLS MCP Server（SSE）。需有效 token，入口不稳定、易失效。
> - 两者都没配 → executor 降级跳过日志工具（不阻塞告警+文档链路）。
>
> 两条链路"配合"的核心：**Prometheus 的 `alertname` 必须与知识库《告警处理手册》小节标题里的英文告警名一致**，手册才会告诉 Agent "去搜什么关键字"；而**CLS 里的日志必须真的包含这些关键字**，`query_log` 才能查到。三者缺一不可。

---

## 1. 准备：账号与资源清单

| 项 | 用途 | 获取方式 | 必需？ |
|---|---|---|---|
| 腾讯云账号 + CLS 服务 | 存日志供 Agent 检索 | 控制台开通日志服务 CLS | 是 |
| CLS 日志主题 Topic ID | 日志写入/检索目标 | 新建日志集 + 日志主题 | 是 |
| CLS 地域 + 外网接入域名 | 上传日志用 | 主题详情页（如 `ap-chongqing.cls.tencentcs.com`） | 是 |
| CLS SecretId / SecretKey | 上传日志 + SearchLog 鉴权 | 访问管理 CAM → 新建 API 密钥 | 是（直连） |
| CLS MCP 授权 token | Agent SSE 连接鉴权（仅 MCP 回退用） | CLS 控制台「MCP/大模型接入」页 | 否（直连不需要） |
| Prometheus 实例 | 产出 `/api/v1/alerts` | 自建 / 腾讯云监控 TMP / 云服务器自建 | 是 |

---

## 2. 腾讯 CLS 配置（日志侧）

### 2.1 创建日志主题
1. 控制台进入 **日志服务 CLS → 日志主题**。
2. 新建日志集（如 `watchtower-logs`），在其下新建日志主题（如 `service-log`）。
3. 地域选离你近的（如 `广州 ap-guangzhou`），记下：
   - **Topic ID**（形如 `xxxxxxxx-xxxx-...`）
   - **接入域名 endpoint**（形如 `ap-guangzhou.cls.tencentcs.com`）

### 2.2 配置索引（关键字检索的前提）
CLS 默认**不开索引**，不开索引 `query_log` 搜不到东西。
1. 进主题 → **索引配置** → 开启索引。
2. 开启 **全文索引**（用于 `panic`、`response`、`reconciliation`、`oom` 等关键字检索）。
3. 可选：开键值索引，字段 `service` / `level` / `msg`，便于按服务过滤。

> 关键字必须与手册一致：`panic`、`response`、`error`、`reconciliation`、`memory`、`oom`、`OOMKilled`、`cpu`、`throttle`、`disk`、`no space left`。

### 2.3 配置 Agent 连接 CLS（推荐：直连）

编辑 [etc/conf.yml](../etc/conf.yml) 填 `cls` 块（直连，无需 MCP token）：
```yaml
cls:
  secret_id: "AKIDxxxx"
  secret_key: "xxxx"
  topic_id: "你的TopicID"
  endpoint: "cls.tencentcloudapi.com"   # SearchLog API 域名（固定值，非地域接入域名）
  region: "ap-chongqing"                # 与建主题时地域一致
```

校验：
```bash
go test ./ai/tools/ -run TestGetLogMcpTool -v
# 预期：工具数=1，无 err（走 cls 直连，返回 query_log 工具）
```

> **备选：MCP 回退**。若你更想用 MCP，在 CLS 控制台「MCP/大模型接入」生成 token，填 `mcp_url: "https://mcp-api.tencent-cloud.com/sse/<token>"`，并**留空 `cls` 块**。`query_log` 会自动回退走 MCP。但 MCP token 易失效、入口不稳定，**推荐直连**。

### 2.4 把 mock 日志上传到 CLS（官方 SDK，已实跑验证）

项目提供上传脚本 [scripts_cls/upload_cls_logs.go](../scripts_cls/upload_cls_logs.go)，用腾讯云官方 SDK `tencentcloud-cls-sdk-go` 把 [mock_data/logs/](../mock_data/logs/) 逐行写入 CLS：

```bash
export CLS_SECRET_ID="AKIDxxxx"
export CLS_SECRET_KEY="xxxx"
export CLS_TOPIC_ID="你的主题ID"
export CLS_ENDPOINT="ap-chongqing.cls.tencentcs.com"   # 地域外网接入域名（与 region 对应）
export CLS_LOG_DIR="/Users/你/watchTower/mock_data/logs"
go run ./scripts_cls/upload_cls_logs.go
```

预期输出（实跑）：
```
[OK] mock_data/logs/order-service-panic.log (14 条)
[OK] mock_data/logs/order-create-response.log (11 条)
[OK] mock_data/logs/reconciliation-error.log (11 条)
[OK] mock_data/logs/node-resource.log (15 条)
上传完成: 成功 4 个文件, 失败 0。
```

> **为什么用官方 SDK**：之前手写 TC3-HMAC-SHA256 签名在 CLS `structuredlog` 接口上反复失败（`miss param topic_id` / `Authorization Is Illegal`）——CLS 对该接口的签名规范与通用 TC3 有差异。官方 SDK 内置正确签名 + protobuf，0 失败。
>
> 生产环境：改用业务服务的日志采集 agent（CLS LogListener / Filebeat）把真实服务日志投递到该 Topic，脚本仅用于演示闭环。

### 2.5 校验日志可检索

控制台检索框搜 `panic`，预期命中约 20 条。或用 SearchLog API 验证（`From`/`To` 为**毫秒**级时间戳）。

实跑命中（参考）：`panic`=20、`error`=23、`reconciliation`=7、`memory`=9、`oom OOMKilled`=4。

> 注：`disk` 单独搜可能 0 命中（日志里是 `no space left` 整串，CLS 分词后 `disk` 未必独立命中）。属索引分词问题，不影响主链路。

---

## 3. Prometheus 配置（告警侧）

### 3.1 选形态
- **最简单**：继续用项目自带的 mock（[scripts_mockPrometheus/mock_prometheus.go](../scripts_mockPrometheus/mock_prometheus.go)），`base_url` 指 `http://127.0.0.1:9090`，告警名已与手册对齐。
- **真实**：自建/腾讯云 Prometheus，按 §3.2 部署告警规则。

### 3.2 部署真实告警规则
项目已提供规则文件 [deploy/prometheus/watchtower_alerts.yml](../deploy/prometheus/watchtower_alerts.yml)，6 条规则的 `alert` 名与手册一一对应（ServiceDown / APIHighErrorRate / ReconciliationDiff / HighMemoryUsage / HighCPUUsage / DiskSpaceRunningLow）。

1. 语法检查：
   ```bash
   promtool check rules deploy/prometheus/watchtower_alerts.yml
   ```
2. 放进 Prometheus 容器的 rules 目录，并在 `prometheus.yml` 加载：
   ```yaml
   rule_files:
     - /etc/prometheus/rules/watchtower_alerts.yml
   ```
3. reload：`kill -HUP <prometheus_pid>` 或调 `POST /-/reload`。
4. 验证：`curl http://<prometheus>/api/v1/alerts` 出现上述 6 个 alertname。

> 告警里的 `annotations.description` 也会被 Agent 读到，建议写清"疑似XX"，引导 Agent 去查对应文档。

### 3.3 配置 Agent 连接 Prometheus
编辑 [etc/conf.yml](../etc/conf.yml)：

```yaml
prometheus:
  base_url: "http://<你的Prometheus地址>:9090"   # 不含末尾 /
```

校验：
```bash
go test ./ai/tools/ -run TestQueryPrometheusAlerts -v
```
返回 6 条告警即成功。Docker 部署改 [docker/conf-docker.yml](../docker/conf-docker.yml)，K8s 改 [k8s/deploy/02-namespace-configmap.yml](../k8s/deploy/02-namespace-configmap.yml) 里同名字段。

---

## 4. 配置改完后：重建知识库

知识库内容（告警手册/错误码）有变更时，必须重新索引到 Milvus，否则 `query_internal_docs` 还是旧内容：

```bash
cd ai/cmd/knowledge_cmd && go run main.go   # 自动按 # 切片 + 去重 + 写入 Milvus
```

---

## 5. 端到端联调

```bash
# 1. Prometheus 告警就绪
curl http://127.0.0.1:9090/api/v1/alerts | jq '.data.alerts[].labels.alertname'

# 2. CLS 日志就绪（控制台用关键字 "panic" 搜得到）

# 3. 触发 Agent
cd ai/cmd/ai_ops_cmd && go run main.go
# 或 HTTP: POST http://localhost:6872/api/ai-ops   （连字符，见 router/chatRouter.go）
```

预期 Agent 行为：
1. `query_prometheus_alerts` → 拿到 6 条告警。
2. 逐条 `query_internal_docs(alertname)` → 命中手册对应小节，拿到"搜哪个关键字"。
3. `get_current_time` → 确定最近 1 小时窗口。
4. `query_log(关键字)` → 从 CLS 拿到 [mock_data/logs/](../mock_data/logs/) 的日志。
5. 用《服务错误类型》解读 12003/22003/52002，汇总成报告，对照 [mock_data/error_reports/incident-20260620-order-service.md](../mock_data/error_reports/incident-20260620-order-service.md)。

---

## 6. 两路"配合"的关键检查清单

- [ ] Prometheus `alertname`（6 个）== 手册小节标题英文告警名 == mock_prometheus.go 的 alertname
- [ ] 手册规定的搜索关键字（panic/response/reconciliation/oom/cpu/disk…）== CLS 日志里真实出现的词
- [ ] CLS 主题已开全文索引，否则 `query_log` 搜不到
- [ ] `cls.*` 已配（推荐）或 `mcp_url` token 有效，`query_log` 能连
- [ ] `prometheus.base_url` 指向真实 Prometheus（或 mock）
- [ ] 知识库已重新索引（`knowledge_cmd`）；若检索返回旧内容，drop collection 重建
- [ ] 日志时间在"最近 1 小时"内（上传脚本已把时间散布在过去 1 小时）

任一项不对，闭环就会断：告警名对不上→查不到文档；关键字对不上→查不到日志；没开索引→查不到日志；时间太旧→"最近1小时"捞不到。
