# Spec: 可观测性 Trace

> 对应维度 #6。eino callback 已存在，扩成结构化 trace 树 + 持久化 + HTTP 查看。既喂 eval（轨迹评分）又喂 debug agent 失败，一处投入三处收益。

## 目标
agent 每次运行产出一棵结构化 trace 树（run -> spans：planner step / tool call / replanner decision），可持久化、可 HTTP 查询、可回放。让"agent 为什么给出错误根因"可定位到具体工具调用/步。

## 设计原则
- **解耦 eino 内部**：核心是 `Recorder`，显式 API（StartRun/StartSpan/EndSpan）+ 一个 eino callback 适配器（best-effort 用 RunInfo 映射）。即使适配器漏事件，工具/supervisor 的显式埋点也能补全。
- **请求级隔离**：Recorder 经 context 传播，多请求不串。
- **OTel 可选**：结构化 JSON 是主产出（无外部依赖）；OTel span 作为可选 sink（仅当配置开启）。

## 契约

### `common/trace/model.go`
```go
type Run struct {
    ID        string        // runID（生成，不依赖 Date.now——由调用方传入或用计数）
    Query     string
    StartedAt string        // 调用方传入时间戳
    Status    string        // running | ok | error
    Error     string
    Spans     []*Span
    root      *Span
}

type Span struct {
    ID        string
    ParentID  string
    Name      string        // 节点名/工具名
    Component string        // ChatModel | Tool | Prompt | ...
    Type      string
    StartedAt string
    DurationMs int64
    Input     string        // 截断后的输入摘要
    Output    string        // 截断后的输出摘要
    Error     string
    Attrs     map[string]string
}
```

### `common/trace/recorder.go`
```go
type Recorder struct { ... }
func NewRecorder(query, startedAt string) *Recorder
func (r *Recorder) StartSpan(name, component string, opts ...SpanOpt) *Span
func (r *Recorder) EndSpan(s *Span, output string, err error)
func (r *Recorder) Finalize(status string, err error) *Run
func (r *Recorder) Run() *Run

// context 传播
func WithRecorder(ctx, r) context.Context
func FromContext(ctx) (*Recorder, bool)
```

### `common/trace/eino_handler.go` - eino callback 适配器
```go
func EinoHandler(r *Recorder) callbacks.Handler
```
OnStart -> StartSpan(Info.Name, Info.Component)；OnEnd -> EndSpan；OnError -> 记 error。用 parent 栈维护树形（eino 回调是嵌套的）。

### `common/trace/adk_capture.go` - adk 事件捕获
`plan_execute_replan` 的事件循环里，把 `prints.Event(event)` 的事件结构化记入 Recorder（planner 出计划 / executor 调工具 / replanner 决策）。这是 plan-execute 链路最丰富的信号。

### `common/trace/store.go` - 持久化
- 内存：`sync.Map[runID]*Run`（带 LRU 上限，默认 1000）
- 文件：`traces/<runID>.json`
- `Get(runID) (*Run, error)`、`List(limit) []*Run`

### `common/trace/http.go` - HTTP 端点
- `GET /api/traces` -> 列出最近 N 个 run（id/query/status/started/duration/spans数）
- `GET /api/traces/:id` -> 完整 run 树（JSON）
挂到 router。

### `common/trace/otel.go`（可选 sink）
- `InitOTel(serviceName string, opts ...OTelOption) (shutdown func(), err error)`
- 用 `go.opentelemetry.io/otel` + `otel/sdk/trace`；默认 no-op tracer（零开销）。
- Recorder 可注册 OTel sink：每个 span 同步发一个 OTel span。仅当 `config.trace.enabled=true` 时启用。
- 自带一个 file SpanExporter（写 `traces/otel/<runID>.json`），无需外部 collector 即可验证 OTel 链路通。

## 集成点
- `controller/chat_v1_chat.go` / `chatStream.go`：把 `logcallback.LogCallback(nil)` 升级为 `trace.EinoHandler(recorder)`（或叠加）。run 完 Finalize 落 store。
- `controller/chat_v1_ai_ops.go`：同理，并把 `detail` 与 trace 关联。
- `plan_execute_replan`：事件循环接 adk_capture。

## 与 eval 的关系
`RunResult.Detail` 来自 `detail`；新增 `RunResult.Trace *trace.Run`。eval 轨迹评分优先用 `Trace`（结构化，精确到工具），无则降级用 `Detail`。

## 验收
- [ ] 一次 `/api/ai-ops` 调用后，`GET /api/traces/:id` 返回带 planner/executor/tool span 的树。
- [ ] span 能定位到"哪个工具被调用、入参/出参摘要、耗时、是否报错"。
- [ ] `traces/<runID>.json` 落盘。
- [ ] OTel sink 开启时，`traces/otel/<runID>.json` 有等价 span（验证 OTel 链路）。
- [ ] 不开启 trace 时，agent 行为与现状一致（零回退）。

## 文件
- 新增：`common/trace/*.go`
- 修改：`controller/chat/*`、`ai/agent/plan_execute_replan/*`、`router/*.go`、`etc/conf.yml`(+trace 配置)
