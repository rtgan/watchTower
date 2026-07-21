# Spec: 记忆 + Skill 路由 + 埋点

> 对应维度 #4（长上下文/记忆、成本）+ #1（真实指标）。代码里已有 TODO（skill 向量匹配），边界清晰。

## 目标
1. **记忆**：`mem` 抽象成 `MemoryStore` 接口，加摘要压缩（长会话不爆 context），可选 Redis（分布式一致）。现状进程内 map 已自标"生产不推荐"。
2. **Skill 路由**：实现 `skills/loader.go` 的 TODO——按 query 向量相似度取 top-K skill，不再全量塞 prompt（避免 prompt 线性膨胀）。
3. **埋点**：给 agent 自身装 Prometheus 指标（latency / tool calls / tokens / deflection），现状把 Prometheus 当数据源读但自身零指标。

---

## A. 记忆

### `mem/store.go` - 抽象接口
```go
type Message = schema.Message

type MemoryStore interface {
    Get(id string) (*Conversation, error)
    Append(id string, msg *Message) error
    // 滚动窗口 + 超阈值摘要压缩
    SetMaxWindow(id string, n int)
}
```
`Conversation{ID, Messages, MaxWindow}`。

### 实现
- `mem/in_memory.go`：现有 `SimpleMemory` 重构为 `InMemoryStore`（默认，零依赖）。
- `mem/redis.go`：`RedisStore`（用 `redis/go-redis/v9`，依赖已在图）。仅当 `config.memory.driver=="redis"` 时启用；否则 InMemory。
- `mem/summarizer.go`：消息数超阈值时，用 DsQuick 把旧消息摘要成一条 system 摘要，保留最近窗口。压缩阈值可配（默认触发于 > 2*MaxWindow）。

### 向后兼容
保留 `GetSimpleMemory(id)` 旧 API（内部转新接口），现有 `controller/chat/*` 不破坏。

---

## B. Skill 路由

### `ai/skills/router.go`
```go
type Router struct {
    embedder embedding.Embedder
    skills   []*Skill
    vecs     [][]float32  // 预计算每个 skill 的 description 向量
}
func NewRouter(ctx) (*Router, error)            // 用 doubao embedder 预算 skill 向量
func (r *Router) TopK(ctx, query string, k int) ([]*Skill, error)  // 余弦相似度 top-K
func (r *Router) FormatForPrompt(skills []*Skill) string  // 复用现有渲染
```
- 兼容降级：embedder 不可用 / Router 未初始化时，退回 `FormatForPrompt()`（全量，现状）。
- `workAgent_planExecuteReplan.go` 与 `chat_workflow/prompt.go` 改用 `skills.RouteForPrompt(query)`。

---

## C. 埋点

### `common/metrics/metrics.go`
用 `prometheus/client_golang`（依赖已在图）注册：
```go
RunDurationSeconds  *HistogramVec  // {endpoint}   /api/ai-ops、/api/chat、/api/chat-stream
ToolCallTotal       *CounterVec    // {tool,status=success|fail|timeout}
LLMTokensTotal      *CounterVec    // {model,type=prompt|completion}
AgentRunTotal       *CounterVec    // {endpoint,status=ok|err}
DeflectionObserved *CounterVec    // {result=resolved|escalated}  // 由 eval/标注写
```
- `MetricsMiddleware` 包 gin 路由记 latency。
- 工具埋点：在 `trace.EinoHandler` 里识别 Tool span -> 发 `ToolCallTotal`；或在 supervisor/executor 工具调用处发。
- Token：从 chat model 回调的 usage 取（best-effort，无则不发）。

### 端点
- `GET /metrics`（promhttp.Handler），供 Prometheus 抓取。
- `etc/conf.yml`：`metrics.enabled`（默认 true）、可选 `metrics.path`。

### 验收
- [ ] InMemory 跑通旧 chat 链路；Redis 配置时切 Redis（用 miniredis 写单测验证，不依赖真 Redis）。
- [ ] 摘要压缩：构造 > 2*MaxWindow 消息后，Get 返回的 messages 含摘要 + 最近窗口。
- [ ] Skill 路由：query="ServiceDown 怎么排查" 命中 alert_handling skill 且不全量注入。
- [ ] `GET /metrics` 含上述指标；一次 ai-ops 后 `agent_run_total{...}` + `tool_call_total{...}` 递增。

## 文件
- 新增：`mem/{store,in_memory,redis,summarizer}.go`、`ai/skills/router.go`、`common/metrics/metrics.go`
- 修改：`controller/chat/*`、`ai/skills/loader.go`、`ai/agent/plan_execute_replan/*`、`ai/agent/chat_workflow/prompt.go`、`router/*.go`、`etc/conf.yml`
