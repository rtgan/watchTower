# Agent 工程化升级变更说明（接手文档）

> 本次更新把 watchTower 从"能跑的 AIOps agent"升级为"可评估、可观测、多智能体、可复用"的工程化项目。
> 按评估体系 → 可观测性 → 记忆/路由/埋点 → 多智能体编排 → MCP 生态 五个阶段推进，每阶段先写 spec 再实现，全部 `go build` + `go vet` + 单测通过。
> 适合接手人快速了解：改了什么、为什么、怎么跑、已知边界。

---

## 一、一句话总览

| 阶段 | 维度 | 做了什么 | 怎么验证 |
|---|---|---|---|
| Phase 1 | 评估体系 #2 | 离线 eval harness：确定性检查 + LLM-as-judge + 轨迹评分 | `go run ./ai/cmd/eval_cmd` |
| Phase 2 | 可观测性 #6 | 结构化 trace 树 + 持久化 + `/api/traces` + 可选 OTel | `GET /api/traces/:id` |
| Phase 3 | 记忆/路由/埋点 #4+#1 | MemoryStore 抽象(+Redis+摘要) / skill 向量路由 / Prometheus 指标 | `GET /metrics` |
| Phase 4 | 多智能体 #3 | supervisor fan-out 并行子 agent + 有界 executor + 错误恢复 | `/api/ai-ops`（默认 supervisor） |
| Phase 5 | MCP 生态 #5 | 把自身告警/日志/文档工具暴露为 MCP server | `go run ./ai/cmd/mcp_server_cmd` |

---

## 二、新增目录与文件

```
docs/specs/                          # 5 个 spec（契约 + 验收标准）
  01-eval-harness.md
  02-trace.md
  03-memory-skill-metrics.md
  04-supervisor.md
  05-mcp-server.md

eval/                                # 评估体系
  scenario.go            # Scenario 结构 + LoadScenarios
  scenarios.go           # 3 个内置场景（从 mock_data 派生）
  mock_tools.go          # MockProvider：确定性 mock 工具
  checkers.go            # 确定性检查器
  judge.go               # LLM-as-judge
  runner.go              # Runner / Scorecard / Summary
  eval_test.go           # 单测（不含 LLM 部分）

common/trace/                        # 可观测性
  model.go               # Run / Span
  recorder.go            # Recorder（context 传播 + span 钩子）
  eino_handler.go        # eino callback 适配器（含 token 抓取）
  store.go               # 内存 + 文件存储（LRU）
  otel.go                # 可选 OTel sink（自实现 file exporter）
  trace_test.go

common/metrics/                     # Prometheus 埋点
  metrics.go             # 指标 + 中间件 + span 钩子
  metrics_test.go

mem/                                 # 会话记忆（重构）
  store.go               # Store 接口 + InMemoryStore
  memory.go              # SimpleMemory（窗口淘汰 + 可选摘要）
  summarizer.go          # LLM 摘要压缩
  redis.go               # RedisStore
  mem_test.go            # 含 miniredis 测试

ai/skills/router.go                  # skill 向量路由（top-K）

ai/agent/supervisor/                 # 多智能体编排
  finding.go             # Finding + 告警解析
  subagent.go            # 单告警子 agent
  recovery.go            # RecoverableLogTool（扩大时间窗重试）
  instrumented.go        # InstrumentedTool（工具调用进 trace）
  synthesizer.go         # 汇总报告
  supervisor.go          # Run + BuildSupervisorAgent + eval 钩子
  supervisor_test.go

ai/mcp_server/                       # MCP server
  server.go              # NewServer / RunStdio / RunSSE
  schemas.go             # 工具 schema
  server_test.go

ai/cmd/eval_cmd/main.go              # eval CLI
ai/cmd/mcp_server_cmd/main.go        # MCP server CLI
```

---

## 三、各阶段详解

### Phase 1：评估体系（eval harness）

**为什么**：原 `mock_data/README.md` 已把 golden 闭环写得清清楚楚，但没有代码自动跑它。加 supervisor、改 memory 这些改进无法验证"是不是真变好了"。eval 把它变成可验证的改进。

**怎么工作**：
1. 每个 `Scenario` 内联固定数据：告警快照 + 日志（按 keyword）+ 文档（按 alertname）+ 固定时间 + 期望判定。
2. `MockProvider` 把这些数据注入为 mock 工具（`query_prometheus_alerts`/`query_log`/`query_internal_docs`/`get_current_time`），**工具离线确定性**，但 **LLM 调用是真实的**（被评估对象）。
3. agent 跑完后分层打分：
   - 确定性检查：必调工具 / 关键字 / 错误码 / 根因是否命中。
   - LLM-as-judge（DsThink）：根因正确性 / 完整性 / 幻觉 / 处置可执行性。系统提示明确要求"完全遵循内部文档"，幻觉检查是天然靶点。
   - 轨迹评分：步数效率（超出基线扣分）。
4. 总分 100，通过判定：根因 + 必调工具命中 且 总分 ≥ 75。

**怎么跑**：
```bash
go run ./ai/cmd/eval_cmd                          # 跑内置 3 场景（含 LLM judge）
go run ./ai/cmd/eval_cmd --no-judge               # 仅确定性检查（不调 LLM）
go run ./ai/cmd/eval_cmd --concurrency 3           # 并发
go run ./ai/cmd/eval_cmd --strategy supervisor     # 评估 supervisor 策略
go run ./ai/cmd/eval_cmd --strict                  # CI 回归：任一不过则退出码 1
```
结果落 `eval/results/<时间戳>.json`。**需 `.env` 配 `ARK_API_KEY`**（LLM 是被评估对象，不 mock）。

**接缝设计**：`ai/tools/provider.go` 抽出 `ToolProvider` 接口 + `ToolSet`。真实链路用 `RealProvider`，eval 用 `MockProvider`。`plan_execute_replan.BuildPlanExecuteReplanAgentWithProvider(ctx, query, provider)` 是注入点（旧 `BuildPlanExecuteReplanAgent` 不变，内部转调 `RealProvider`）。

**已知边界**：
- judge 是 LLM，有随机性，同场景两次分数可能 ±5 抖动；确定性检查分数应一致。
- `ExpectedKeywords` 靠在 `detail`（adk 事件流）里子串匹配，较宽松；Phase 2 的结构化 trace 上线后可精确到工具入参。

---

### Phase 2：可观测性 trace

**为什么**：原 `log_callback` 只是 `fmt.Printf` 打 stdout，无法定位"agent 为什么给出错误根因"。trace 让每次运行产出一棵 span 树（planner/executor/工具/模型），落盘可查。

**怎么工作**：
- `trace.Recorder` 请求级隔离（经 context 传播）。`trace.EinoHandler(rec)` 把 eino callback 适配成 span：`OnStart` 开 span、`OnEnd` 结束。**嵌套靠 context 链维护**（OnStart 把 span 写入 ctx，OnEnd 恢复父 span），eino 对并行分支 fork ctx，故每个分支独立维护栈，不串扰。
- `Tool` span 记工具入参/出参/耗时/错误；`ChatModel` span 抓 token 用量（`model.ConvCallbackOutput`）写入 attrs。
- 持久化：内存 LRU（默认 1000）+ 文件 `traces/<runID>.json`。
- `plan_execute_replan` 的事件循环把 adk 事件记入 trace（`rec.Event`）。
- 可选 OTel sink：`trace.enabled=true` 时，每个 span 镜像成 OTel span，自实现 file exporter 写 `traces/otel/spans.jsonl`（无需外部 collector 即可验证 OTel 链路）。

**怎么跑**：
```bash
# 正常启动服务后触发一次诊断
curl -X POST http://localhost:6872/api/ai-ops
# 查 trace
curl http://localhost:6872/api/traces         # 列表
curl http://localhost:6872/api/traces/<id>     # 完整 span 树（响应里的 trace_id）
```
`/api/ai-ops` 响应新增 `trace_id` 字段，对应 `/api/traces/:id`。

**已知边界**：
- OTel sink 的 span 创建为根 span（不携带 parent context），以保证并发安全；结构化 trace 树（Recorder）保留了完整父子关系。
- `chat_workflow` 仍全量注入 skill（query 在模板构建期未知）；skill 路由目前只在 `plan_execute` / supervisor 路径生效。

---

### Phase 3：记忆 + skill 路由 + 埋点

**A. 记忆**
- `mem.Store` 接口（`InMemoryStore` 默认 / `RedisStore` 可选）。`config.memory.driver=redis` 时 main 切 Redis。
- `SimpleMemory` 保持旧 API（`GetMessages`/`SetMessages`），但签名改为 `SetMessages(ctx, msg)`（4 处调用已更新）。
- 可选摘要压缩：`config.memory.summarize=true` 且历史超 `2*窗口` 时，用 DsQuick 把旧消息压成摘要（增加延迟，默认关）。
- 单测用 `miniredis` 验证 Redis 路径，不依赖真 Redis。

**B. skill 路由**
- `ai/skills/router.go`：`RouteForPrompt(ctx, query)` 用 doubao embedder 把 skill description 预算向量，按 query 余弦相似度取 top-K（默认 2）注入，**不再全量塞 prompt**（实现原 `loader.go` 的 TODO）。
- embedder 不可用时降级全量 `FormatForPrompt()`。

**C. 埋点**
- `common/metrics`：`agent_run_duration_seconds`（latency）/ `agent_run_total` / `tool_call_total` / `llm_tokens_total` / `deflection_observed`。
- `GET /metrics` 供 Prometheus 抓取。`metrics.Middleware()` 记每路由 latency + run 计数。
- tool/token 计数经 trace span 钩子订阅（`trace.RegisterSpanHook`），避免 trace 反向依赖 metrics。
- `RecordDeflection("resolved"|"escalated")` 供 eval / 人工标注调用。

**已知边界**：
- `llm_tokens_total` 的 `model` 标签取 span.Name（eino 节点名），非精确模型名；token 来自 eino `TokenUsage`，部分模型可能不回填。
- `tool_call_total` 依赖 eino 回调把工具 span 的 Component 标为 "Tool"（已确认 eino v0.8.8 该常量为 "Tool"）。

---

### Phase 4：supervisor 并行子 agent + 有界 executor

**为什么**：原 `plan_execute_replan` 用单 Executor 串行处理所有告警；且 `executor.go` 的 `MaxIterations: 999999`（实际无界）。supervisor 把"分别处理每条告警"变成真正并行。

**怎么工作**：
1. **Triage**：调 `query_prometheus_alerts` 拿告警清单，解析去重。
2. **Fan-out**：每条告警起一个 `react.NewAgent` 子 agent（专属 system prompt + 工具子集 `query_log`/`query_internal_docs`/`get_current_time`，**不给 alerts** 避免重复抓全量），并发上限 4（`errgroup` 思路 + semaphore）。
3. **Synthesize**：DsThink 把所有 finding 汇总成告警分析报告。
4. 每个子 agent 自带子 recorder（独立 trace），父 recorder 记 fan-out 结构。
5. **有界**：`executor.go` `MaxIterations` 改 `999999 → 10`；子 agent `MaxStep=12`。
6. **错误恢复**：`RecoverableLogTool` 包裹 query_log，首次空/失败时扩大时间窗到最近 6h 重试一次；仍失败则 finding 标 `需人工介入`。`InstrumentedTool` 包裹所有子 agent 工具，调用进子 trace + 触发 `tool_call_total`。

**策略切换**：`config.agent.strategy`：
- `supervisor`（默认）：`/api/ai-ops` 走 `supervisor.BuildSupervisorAgent`。
- `plan_execute`：回退原 plan-execute-replan（对照）。

**怎么跑**：
```bash
# 默认 supervisor
go run ./ai/cmd/ai_ops_cmd
# 回退 plan_execute
go run ./ai/cmd/ai_ops_cmd --plan_execute
# 评估两种策略
go run ./ai/cmd/eval_cmd --strategy plan_execute
go run ./ai/cmd/eval_cmd --strategy supervisor
```

**已知边界**：
- 子 agent 工具调用进 trace 靠 `InstrumentedTool` 包装（非 eino 全局 callback），覆盖了 log/docs/time；未覆盖的流式细节依赖 eino 回调。
- 并发上限 4 写死（`defaultConcurrency`）；跨进程的一致性需 Redis + 分布式锁（记忆已支持 Redis，锁未提供）。

---

### Phase 5：暴露 watchTower 为 MCP server

**为什么**：已用 MCP 作 CLS 客户端（回退路径）；本项目把自身能力**反向暴露**成 MCP server，供别的 agent / IDE（Cursor、Claude Desktop）复用。2026 企业落地标配姿态。

**怎么工作**：
- `ai/mcp_server/server.go`：用 `mark3labs/mcp-go` 的 server 端注册三个工具：`query_prometheus_alerts` / `query_log` / `query_internal_docs`。
- 工具实现**直接复用 `ai/tools/*`** 的构造函数，保证 MCP 暴露的与 agent 内部用的是同一份逻辑。
- handler 把 MCP 入参序列化为 JSON 交给 eino 工具的 `InvokableRun`。
- `query_log` 在 CLS 未配置时仍注册（可被 ListTools 发现），调用时返回明确错误，不 crash server。
- 两种 transport：stdio（本地/IDE）、SSE（远程）。

**怎么跑**：
```bash
go run ./ai/cmd/mcp_server_cmd              # stdio（IDE 接入）
go run ./ai/cmd/mcp_server_cmd --sse :9100   # SSE
```
Cursor / Claude Desktop 配置（stdio 示例）：
```json
{ "mcpServers": { "watchTower": { "command": "go", "args": ["run", "./ai/cmd/mcp_server_cmd"] } } }
```

---

## 四、配置变更（etc/conf.yml）

新增三段（均有默认值，不配也能跑）：
```yaml
trace:        # Phase 2
  enabled: false        # true 额外开 OTel sink（结构化 trace 总是开）
  dir: "traces"
  service_name: "watchTower"

memory:       # Phase 3
  driver: "memory"     # memory | redis
  redis_addr: "localhost:6379"
  summarize: false     # true 开摘要压缩（增延迟）

agent:        # Phase 4
  strategy: "supervisor"   # supervisor | plan_execute
```

---

## 五、新增依赖（已在依赖图里，go mod tidy 自动整理）

```
github.com/prometheus/client_golang   # 埋点
github.com/redis/go-redis/v9          # Redis 记忆
github.com/alicebob/miniredis/v2      # Redis 测试
go.opentelemetry.io/otel (+sdk,trace) # OTel sink
github.com/mark3labs/mcp-go           # MCP server（原已用于客户端）
```

---

## 六、测试与验证

```bash
go build ./... && go vet ./...                              # 全量编译 + vet
go test ./eval/... ./common/trace/... ./common/metrics/...  # 全绿
go test ./mem/... ./ai/agent/supervisor/... ./ai/mcp_server/...
```

单测覆盖（均离线、不依赖 LLM/Prometheus/Milvus/Redis）：
- eval：场景结构 / mock 工具命中 / 检查器 / judge 解析 / 轨迹分。
- trace：嵌套 span / eino 适配器 / nil 防御 / 存储 LRU / OTel sink 落盘。
- metrics：工具计数 / token 计数 / deflection。
- mem：in-memory / Redis(miniredis) / 窗口淘汰。
- supervisor：告警解析 / finding 解析 / 时间窗扩大 / 空失败判定。
- mcp_server：构造 / args 序列化。

**未跑的**：含真实 LLM 的端到端（eval 全跑、/api/ai-ops、MCP tool 调用）需 `ARK_API_KEY` + 网络，会消耗 token，留给使用者按需执行。

---

## 七、给接手人的建议

1. **先跑 eval**（`go run ./ai/cmd/eval_cmd --no-judge` 先看确定性部分）——这是验证后续一切改进的基线。
2. **改任何 agent 逻辑后**：`go run ./ai/cmd/eval_cmd --strict` 跑回归，看分数是否回退。
3. **排查 agent 失败**：看 `/api/traces/<trace_id>`，定位到具体工具 span 的 error / input。
4. **扩展场景**：在 `eval/scenarios/` 加 JSON（结构同 `Scenario`），或改 `eval/scenarios.go` 的内置场景。
5. **加新工具**：在 `ai/tools/` 加构造函数 + 在 `tools.ToolSet` 分组 + 在 supervisor/MCP server 注册 + 在 eval `MockProvider`/场景里补 mock 数据。
6. **接 OTel 到真 collector**：`trace.enabled=true` 后，把 `common/trace/otel.go` 的 file exporter 换成 `otlptracehttp` 即可（一处改动）。

## 八、spec 文档

5 个 spec 在 `docs/specs/`，含每个阶段的契约、验收标准、文件清单——是比本文件更精确的实现依据。
