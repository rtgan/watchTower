# Spec: 评估体系 (Eval Harness)

> 对应维度 #2。把"感觉更好了"变成"可验证的改进"。已有 golden 数据 (`mock_data/`)，起步成本低。

## 目标
对 AIOps agent 做端到端评估：给定一个告警场景，跑 agent，对**终答 + 轨迹**自动打分，输出 scorecard。支持回归（CI 可跑）。

## 设计原则
- **离线 + 确定性**：eval 不依赖真实 Prometheus/CLS/Milvus，用 mock 工具注入固定数据。同 seed 多次跑取分布。
- **分层评分**：确定性检查（工具是否调用、关键字是否正确、根因是否命中）+ LLM-as-judge（幻觉/完整度/可执行性）。
- **轨迹评分**：不只看终答，看工具调用序列（是否浪费步数、是否重试失败工具）。

## 契约

### `eval/scenario.go`
```go
type Scenario struct {
    ID          string                 // 如 "incident-20260620-order-service"
    Description string
    // mock 数据源（注入给 agent 的工具）
    AlertsJSON  string                 // query_prometheus_alerts 的返回体
    Logs        map[string]string      // keyword -> 日志文本（query_log 按 keyword 命中）
    Docs        map[string]string      // alertname -> 告警处理手册小节（query_internal_docs 返回）
    FixedNow    string                 // get_current_time 固定返回，保证时间相关参数确定性

    // 期望（用于打分）
    ExpectedRootCause   string         // 期望命中的根因关键词，如 "order/service.go:128"
    ExpectedKeywords     []string       // 期望 query_log 使用的 keyword，如 ["panic","response","reconciliation"]
    ExpectedErrorCodes   []string       // 期望解读的错误码，如 ["12003","52002","22003"]
    MustCallTools        []string       // 期望必须调用的工具，如 ["query_prometheus_alerts","query_internal_docs"]
    GoldenReport         string         // 期望报告要点（judge 对照用，非逐字比对）
}

func LoadScenarios(dir string) ([]*Scenario, error)   // 读 eval/scenarios/*.json
```

### `eval/runner.go`
```go
type RunResult struct {
    ScenarioID  string
    FinalReport string
    Detail      []string   // agent 事件流（轨迹）
    StartedAt   time.Time
    Duration    time.Duration
    Error       string
}

type Scorecard struct {
    ScenarioID       string
    Pass             bool
    Total            float64       // 0~100
    ToolCheck        CheckResult   // MustCallTools 是否都调用
    KeywordCheck     CheckResult   // ExpectedKeywords 是否出现在轨迹
    ErrorCodeCheck   CheckResult   // ExpectedErrorCodes 是否出现在报告
    RootCauseCheck  CheckResult   // ExpectedRootCause 是否出现在报告
    TrajectoryScore  float64       // 步数效率分（越少越好，相对基线）
    JudgeScore       JudgeResult   // LLM-as-judge
    Notes            []string
}

type Runner struct { ... }
func NewRunner(opts ...Option) *Runner
func (r *Runner) Run(ctx context.Context, s *Scenario) (*RunResult, *Scorecard, error)
func (r *Runner) RunAll(ctx context.Context, scenarios []*Scenario) ([]*Scorecard, error)
```

### `eval/checkers.go` - 确定性检查器
对 `RunResult.Detail + FinalReport` 做子串/正则匹配，输出 `CheckResult{Pass, Hit, Missed, Detail}`。

### `eval/judge.go` - LLM-as-judge
用 DsThink 模型，给定报告 + 期望，按 rubric 打分：
- 根因正确性 (0-30)
- 完整性 (0-30)
- **幻觉**（是否使用文档外信息）(0-20，扣分项)
- 处置可执行性 (0-20)
返回 `JudgeResult{Score, Rubric, Reason}`。系统提示明确要求"完全遵循内部文档"，幻觉检查是天然靶点。

### `eval/mock_tools.go` - 确定性工具注入
基于 `utils.InferOptionableTool` 构造 mock 版 `query_prometheus_alerts`/`query_log`/`query_internal_docs`/`get_current_time`，返回 scenario 固定数据。供 runner 注入给 agent。

### `eval/cmd` (=`ai/cmd/eval_cmd/main.go`)
- 读 `eval/scenarios/*.json`
- 跑所有场景（默认串行；`--concurrency N` 并行）
- 打印每个 scorecard + 汇总（pass rate / 均分）
- 写 `eval/results/<timestamp>.json`

## 与 agent 的接缝
eval 需要把 mock 工具注入给 agent。为此在 `plan_execute_replan` 暴露工具注入点：
```go
// 新增：可注入工具的构造入口（旧 BuildPlanExecuteReplanAgent 保持不变，内部转调）
func BuildPlanExecuteReplanAgentWithTools(ctx, query, toolProvider ToolProvider) (resp string, detail []string, err error)
```
`ToolProvider` 默认实现 = 现有行为（真实工具 + 降级）；eval 传入 mock provider。

## 验收
- [ ] `go run ./ai/cmd/eval_cmd` 可离线跑通 `incident-20260620` 场景并输出 scorecard。
- [ ] 至少 3 个场景（含 1 个根因命中、1 个单告警、1 个日志无命中边界）。
- [ ] 同 seed 跑两次，确定性检查分数一致；JudgeScore 在 ±5 内。
- [ ] scorecard 落 `eval/results/<ts>.json`。

## 文件
- 新增：`eval/*.go`、`eval/scenarios/*.json`、`ai/cmd/eval_cmd/main.go`
- 修改：`ai/agent/plan_execute_replan/`（暴露 ToolProvider 注入点，不改旧入口行为）
