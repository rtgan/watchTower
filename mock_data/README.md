# watchTower Mock 数据集

本目录提供一批**互相关联**的 mock 数据，用于驱动 watchTower 的 AI 运维 Agent（`plan_execute_replan`）完成
"告警 → 查内部文档 → 查日志 → 根因分析 → 处置策略" 的完整排查闭环。

所有数据围绕**同一个故障 incident-20260620 order-service 雪崩事件**，构成一条单一根因链，确保 Agent 能正确定位根因并给出策略。

## 一、根因链（6 条告警同源）

```
05:55 发布 order-service v1.8.2 引入 nil pointer bug (order/service.go:128)
        │
        ├─► panic 反复重启 ───────────────────► ServiceDown        (搜 "panic")
        │     ├─► 内存泄漏 OOMKilled ─────────► HighMemoryUsage    (搜 "memory"/"oom")
        │     ├─► 重启风暴 + GC ──────────────► HighCPUUsage       (搜 "cpu"/"throttle")
        │     └─► panic 堆栈日志刷爆 /data ───► DiskSpaceRunningLow(搜 "disk"/"no space left")
        │
        ├─► 重启间隙调用下游失败 code=12003 ─► APIHighErrorRate    (搜 "response" + 接口名)
        │
        └─► 失败订单未同步下游 ──────────────► ReconciliationDiff  (搜 "error"+"reconciliation", code=52002)
```

## 二、目录结构与对应关系

| 文件 | 作用 | 对应 Agent 工具 / 环节 |
|---|---|---|
| `prometheus_alerts.json` | 6 条活跃告警快照 | `query_prometheus_alerts` 的返回 |
| `scripts_mockPrometheus/mock_prometheus.go` | 上述告警的 HTTP mock 源 | `query_prometheus_alerts` 实际调用 |
| `logs/order-service-panic.log` | panic 堆栈日志 | `query_log(keyword="panic")` |
| `logs/order-create-response.log` | 接口失败日志(12003/22003) | `query_log(keyword="response", 接口="/api/order/create")` |
| `logs/reconciliation-error.log` | 对账差异日志(52002) | `query_log(keyword=["error","reconciliation"])` |
| `logs/node-resource.log` | 内存/CPU/磁盘日志 | `query_log(keyword="memory"/"oom"/"cpu"/"disk")` |
| `error_reports/incident-20260620-order-service.md` | 故障报告(报错+根因+处置) | Agent 最终产出对照 |
| `../docs/告警处理手册.md` | 各告警的排查步骤与搜索关键字 | `query_internal_docs`（RAG 知识库） |
| `../docs/服务错误类型.md` | 多服务错误码释义 | `query_internal_docs`（RAG 知识库） |

## 三、Agent 闭环如何跑通

Agent 系统提示（见 `controller/chat/chat_v1_ai_ops.go`）要求：
1. `query_prometheus_alerts` → 拿到 6 条告警（alertname）。
2. 对每条告警名 `query_internal_docs` → 检索到《告警处理手册》对应小节，拿到"搜什么关键字"。
3. `get_current_time` → 确定时间范围。
4. `query_log` 按关键字搜索 → 拿到本目录 `logs/*.log` 中的日志。
5. 用《服务错误类型》解读错误码（12003/22003/52002）。
6. 汇总成告警分析报告，对照 `error_reports/incident-20260620-order-service.md`。

## 四、关键设计点（保证 Agent 能正确排查）

1. **告警名 ↔ 文档标题对齐**：Prometheus alertname 是英文（如 `ServiceDown`），《告警处理手册》每个 `#` 小节标题都同时含英文 alertname 与中文名。知识库按 `#` 切片、检索 TopK=1，所以 `query_internal_docs("ServiceDown")` 能命中对应小节。
2. **文档规定的搜索关键字 ↔ 日志内容对齐**：手册说 ServiceDown 搜 `"panic"`，则 `logs/order-service-panic.log` 里确实有 `panic: runtime error: invalid memory address or nil pointer dereference`。每条告警的搜索关键字在对应日志中都能命中。
3. **错误码跨文档对齐**：日志里的 `12003`/`22003`/`52002` 在《服务错误类型》中均有释义，且 `22003` 明确"常伴随 12003"，`52002` 明确"需先恢复下游服务"，形成跨服务因果链。
4. **单一根因**：6 条告警最终汇聚到 `order/service.go:128` 空指针这一根因，Agent 应能识别这是雪崩而非 6 个独立问题。

## 五、使用方式

```bash
# 1. 启动 mock Prometheus（提供告警数据）
go run scripts_mockPrometheus/mock_prometheus.go   # 监听 :9090

# 2. 把 logs/ 下的日志上传到腾讯 CLS（或用其 mock），供 query_log 检索
#    日志工具走 CLS MCP（etc/conf.yml 的 mcp_url）

# 3. 重建知识库（索引 docs/*.md 到 Milvus）
cd ai/cmd/knowledge_cmd && go run main.go

# 4. 触发 Agent 排查
cd ai/cmd/ai_ops_cmd && go run main.go
# 或调用 HTTP 接口：POST /v1/ai_ops
```

> 注：日志查询工具 `query_log` 走腾讯 CLS 的 MCP，本目录 `logs/*.log` 是该工具预期返回的日志内容。
> 如需本地闭环测试，可将其导入 CLS 主题，或在 CLS mock 中按关键字返回对应文件内容。
