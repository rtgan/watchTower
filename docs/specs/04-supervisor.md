# Spec: Supervisor 并行子 Agent + 有界 Executor

> 对应维度 #3。把多智能体从"有"变"强"：现有 `plan_execute_replan` 是单 Executor 串行处理所有告警；改为 supervisor fan-out 每告警并行子 agent，再加有界 executor + 错误恢复。

## 目标
1. **Supervisor 模式**：拿告警清单 -> 每条告警派一个独立子 agent（专属工具子集 + 专属 prompt）并行排查 -> 收集结构化 finding -> synthesizer 汇总成报告。
2. **有界 Executor**：把 `executor.go` 的 `MaxIterations: 999999` 改为真实上界 + 早停。
3. **错误恢复**：工具失败 N 次 -> 触发 recovery（改写 query / 扩大时间窗 / 标记需人工），而非静默 skip。

---

## A. Supervisor

### `ai/agent/supervisor/supervisor.go`
```go
type Finding struct {
    AlertName   string
    RootCause   string
    Evidence    []string   // 命中日志/错误码
    Remediation string
    Error       string     // 子 agent 失败时填
    Trace       *trace.Run // 子 agent 的 trace
}

type Supervisor struct { ... }
func NewSupervisor(ctx) (*Supervisor, error)
func (s *Supervisor) Run(ctx, query string) (report string, findings []*Finding, trace *trace.Run, err error)
```

### 流程
1. **Triage**：调 `query_prometheus_alerts` 拿告警清单（仍走真实/注入工具，受 ToolProvider 控制）。
2. **Fan-out**：每条告警起一个 `react.NewAgent` 子 agent（eino ReAct），并发（`errgroup` + 并发上限默认 4）：
   - 子 agent 专属 system prompt：注入"该告警名 -> 应搜什么关键字"（来自 skill/文档）。
   - 工具子集：`query_log`、`query_internal_docs`、`get_current_time`（不给 prometheus，避免它再去抓全量告警）。
   - 输出结构化 `Finding`（用工具调用 + 最后一步让模型输出 JSON，或解析 detail）。
3. **Synthesize**：一个 DsThink 调用，喂所有 findings + 系统 prompt，生成最终告警分析报告（沿用现有报告格式）。
4. **Trace**：supervisor 自己一个 Run；每个子 agent 一个子 span + 子 Run。

### 与 plan_execute_replan 的关系
- 新增 `supervisor` 作为 `/api/ai-ops` 的默认执行器（更贴合"分别处理每条告警"的 prompt 意图，且并行更快）。
- `plan_execute_replan` 保留，作为对照/单线程回退（`config.agent.strategy = "plan_execute" | "supervisor"`，默认 supervisor）。
- 两者都实现统一签名 `Run(ctx, query) (report, detail, trace, err)`，便于 eval 复用。

---

## B. 有界 Executor

### `ai/agent/plan_execute_replan/executor.go`
- `MaxIterations: 999999` -> `MaxIterations: 10`（每步内部 ReAct 循环上限；外层 planexecute.New 的 `MaxIterations: 20` 不变）。
- 加收敛 guard：replanner 连续 2 次判定"无进展"则提前结束，避免空转烧 token。

---

## C. 错误恢复

### `ai/agent/supervisor/recovery.go`
- 子 agent 工具调用失败计数：同一工具失败 >= `maxToolFailures`（默认 2）-> recovery 策略：
  1. 对 `query_log`：扩大时间窗（1h -> 6h）+ 放宽 keyword 重试 1 次。
  2. 仍失败：finding.Error = "需人工介入"，synthesizer 在报告里标注。
- 不再静默 skip（现状 executor 的 log.Printf 降级跳过保留，但 supervisor 层有显式 recovery 记录到 trace + finding）。

---

## 验收
- [ ] `/api/ai-ops`（默认 supervisor）对 6 告警场景：6 个子 agent 并发，trace 能看到 6 条并行子 span，总耗时显著低于串行 plan_execute。
- [ ] executor MaxIterations=10，构造一个需要 >10 步的场景时能优雅终止而非无限循环。
- [ ] mock 一个 query_log 持续失败的场景，finding.Error 被标注、报告里提示需人工、trace 里能看到 recovery 尝试。
- [ ] `config.agent.strategy=plan_execute` 时回退到旧链路，行为不变。
- [ ] eval 对 supervisor 和 plan_execute 两种策略都能跑并出 scorecard。

## 文件
- 新增：`ai/agent/supervisor/{supervisor,finding,recovery,synthesizer}.go`
- 修改：`ai/agent/plan_execute_replan/executor.go`（有界）、`controller/chat_v1_ai_ops.go`（策略选择）、`etc/conf.yml`（agent 策略配置）
