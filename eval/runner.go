package eval

import (
	"context"
	"sync"
	"time"
	"watchTower/ai/agent/plan_execute_replan"
	"watchTower/ai/tools"
)

// RunResult 一次 agent 运行的原始产出（终答 + 轨迹）。
type RunResult struct {
	ScenarioID  string   `json:"scenario_id"`
	FinalReport string   `json:"final_report"`
	Detail      []string `json:"detail"`
	Steps       int      `json:"steps"`        // 轨迹步数（detail 长度）
	StartedAt   string   `json:"started_at"`
	DurationMs  int64    `json:"duration_ms"`
	Error       string   `json:"error,omitempty"`
}

// Scorecard 单场景评分卡。
type Scorecard struct {
	ScenarioID      string       `json:"scenario_id"`
	Strategy        string       `json:"strategy"` // 评估的执行策略：plan_execute | supervisor
	Pass            bool         `json:"pass"`
	Total           float64      `json:"total"`        // 0-100
	ToolCheck       CheckResult  `json:"tool_check"`
	KeywordCheck    CheckResult  `json:"keyword_check"`
	ErrorCodeCheck  CheckResult  `json:"error_code_check"`
	RootCauseCheck  CheckResult  `json:"root_cause_check"`
	TrajectoryScore float64      `json:"trajectory_score"` // 步数效率诊断分（0-100）
	JudgeScore      JudgeResult  `json:"judge_score"`
	Notes           []string     `json:"notes,omitempty"`
}

// Runner 评估运行器。
type Runner struct {
	enableJudge bool
	strategy    string // "plan_execute"（默认）| "supervisor"（Phase 4 接入后可用）
}

type Option func(*Runner)

func WithJudge(b bool) Option          { return func(r *Runner) { r.enableJudge = b } }
func WithStrategy(s string) Option      { return func(r *Runner) { r.strategy = s } }

func NewRunner(opts ...Option) *Runner {
	r := &Runner{enableJudge: true, strategy: "plan_execute"}
	for _, o := range opts {
		o(r)
	}
	return r
}

// 评估用的系统提示（与 controller/chat_v1_ai_ops.go 的生产 prompt 同源，保证评估的是真实行为）。
const evalQuery = `"1. 你是一个智能的服务告警分析助手,首先调用工具query_prometheus_alerts获取所有活跃的告警。"
"2. 分别根据告警的名称调用工具query_internal_docs，获取告警名对应的处理方案。"
"3. 完全遵循内部文档的内容进行查询和分析,不允许使用文档外的任何信息。"
"4. 涉及到时间的参数都需要先通过工具get_current_time获取当前时间,再结合时间要求进行传参。"
"5. 涉及到日志的查询,需要先通过日志工具获取相关日志信息。"
"6. 分别将告警对应查询到的信息进行总结分析,最后生成告警运维分析报告，需包含：活跃告警清单、每个告警的根因分析、处置方案、结论。"`

// Run 跑单个场景：注入 mock 工具 -> 跑 agent -> 确定性检查 + LLM-as-judge -> scorecard。
func (r *Runner) Run(ctx context.Context, s *Scenario) (*RunResult, *Scorecard) {
	startedAt := time.Now().Format(time.RFC3339)
	start := time.Now()

	report, detail, err := runAgent(ctx, r.strategy, s)
	rr := &RunResult{
		ScenarioID:  s.ID,
		FinalReport: report,
		Detail:      detail,
		Steps:       len(detail),
		StartedAt:   startedAt,
		DurationMs:  time.Since(start).Milliseconds(),
	}
	if err != nil {
		rr.Error = err.Error()
	}

	sc := r.score(ctx, s, rr)
	return rr, sc
}

// RunAll 并发跑多个场景。concurrency<=0 时串行。
func (r *Runner) RunAll(ctx context.Context, scenarios []*Scenario, concurrency int) []*Scorecard {
	if concurrency <= 1 {
		var out []*Scorecard
		for _, s := range scenarios {
			_, sc := r.Run(ctx, s)
			out = append(out, sc)
		}
		return out
	}
	if concurrency > len(scenarios) {
		concurrency = len(scenarios)
	}
	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := make([]*Scorecard, len(scenarios))
	for i, s := range scenarios {
		wg.Add(1)
		go func(idx int, sc *Scenario) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			_, card := r.Run(ctx, sc)
			mu.Lock()
			out[idx] = card
			mu.Unlock()
		}(i, s)
	}
	wg.Wait()
	return out
}

func (r *Runner) score(ctx context.Context, s *Scenario, rr *RunResult) *Scorecard {
	sc := &Scorecard{ScenarioID: s.ID, Strategy: r.strategy}
	report := rr.FinalReport

	// 确定性检查
	sc.ToolCheck = CheckTools(rr.Detail, s.Expected.MustCallTools)
	sc.KeywordCheck = CheckKeywords(rr.Detail, report, s.Expected.Keywords)
	sc.ErrorCodeCheck = CheckErrorCodes(report, s.Expected.ErrorCodes)
	sc.RootCauseCheck = CheckRootCause(report, s.Expected.RootCause)

	// LLM-as-judge（可关；失败不致命）
	if r.enableJudge && rr.Error == "" {
		sc.JudgeScore = Judge(ctx, report, s)
	}

	// 轨迹步数效率：以基线步数为参照，超出则扣分。计入 Total（权重 10，见下方加权公式）。
	sc.TrajectoryScore = trajectoryScore(rr.Steps)

	// 加权：工具15 + 关键字15 + 错误码10 + 根因20 + judge30 + 轨迹10 = 100
	total := 15*frac(sc.ToolCheck) + 15*frac(sc.KeywordCheck) +
		10*frac(sc.ErrorCodeCheck) + 20*frac(sc.RootCauseCheck) +
		30*(sc.JudgeScore.Score/100.0) + 10*(sc.TrajectoryScore/100.0)
	sc.Total = round1(total)

	// 通过判定：根因 + 必调工具必须命中，且总分达标
	sc.Pass = sc.RootCauseCheck.Pass && sc.ToolCheck.Pass && sc.Total >= 75
	if rr.Error != "" {
		sc.Notes = append(sc.Notes, "agent run error: "+rr.Error)
	}
	if sc.JudgeScore.Error != "" {
		sc.Notes = append(sc.Notes, "judge skipped: "+sc.JudgeScore.Error)
	}
	return sc
}

// frac 命中比例：无期望时记满分；否则 hit/total。
func frac(c CheckResult) float64 {
	want := len(c.Hit) + len(c.Missed)
	if want == 0 {
		return 1.0
	}
	return float64(len(c.Hit)) / float64(want)
}

// trajectoryBaseline 期望步数基线（6 告警场景约需 ~15 步）。超出基线按每步 4 分扣，最低 50。
const trajectoryBaseline = 15

func trajectoryScore(steps int) float64 {
	over := steps - trajectoryBaseline
	score := 100.0
	if over > 0 {
		score -= float64(over) * 4
	}
	if score < 50 {
		score = 50
	}
	if score > 100 {
		score = 100
	}
	return score
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

// Summary 汇总多个 scorecard。
type Summary struct {
	Total    int           `json:"total"`
	Strategy string        `json:"strategy"` // 本次评估的执行策略
	Passed   int           `json:"passed"`
	PassRate float64       `json:"pass_rate"`
	AvgScore float64       `json:"avg_score"`
	Cards    []*Scorecard  `json:"cards"`
}

func Summarize(cards []*Scorecard) Summary {
	sum := Summary{Cards: cards, Total: len(cards)}
	if len(cards) > 0 {
		sum.Strategy = cards[0].Strategy // 同一次 run 策略一致，取首张
	}
	var acc float64
	for _, c := range cards {
		if c.Pass {
			sum.Passed++
		}
		acc += c.Total
	}
	if sum.Total > 0 {
		sum.PassRate = round1(float64(sum.Passed) / float64(sum.Total) * 100)
		sum.AvgScore = round1(acc / float64(sum.Total))
	}
	return sum
}

func runAgent(ctx context.Context, strategy string, s *Scenario) (string, []string, error) {
	// supervisor 钩子由 ai/agent/supervisor 包通过 init() 注入；未注入则回退 plan_execute。
	if strategy == "supervisor" && supervisorRunFn != nil {
		return supervisorRunFn(ctx, evalQuery, MockProvider{s})
	}
	return plan_execute_replan.BuildPlanExecuteReplanAgentWithProvider(ctx, evalQuery, MockProvider{s})
}

// SetSupervisorRunFn 由 ai/agent/supervisor 包在 init() 中调用，注入 supervisor 执行钩子。
func SetSupervisorRunFn(fn func(ctx context.Context, query string, provider tools.ToolProvider) (string, []string, error)) {
	supervisorRunFn = fn
}

// supervisorRunFn 由 ai/agent/supervisor 包注入（Phase 4）。未注入时 eval 自动回退 plan_execute。
var supervisorRunFn func(ctx context.Context, query string, provider tools.ToolProvider) (string, []string, error)
