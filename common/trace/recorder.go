package trace

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// spanKey 在 context 中携带当前 span（用于 eino 回调维护嵌套）。
type spanKey struct{}

// Recorder 一次 run 的记录器。请求级隔离：每个 HTTP 请求 / 每个子 agent 各持一个。
type Recorder struct {
	mu   sync.Mutex
	run  *Run
	otel *otelSink // 可选 OTel sink（nil 则不发 OTel span）
}

// NewRecorder 创建一次 run 的记录器。startedAt 由调用方传入（便于测试固定）。
func NewRecorder(query, startedAt string) *Recorder {
	r := &Recorder{otel: globalOTel}
	r.run = &Run{
		ID:        nextRunID(),
		Query:     query,
		StartedAt: startedAt,
		Status:    "running",
		Spans:     []*Span{},
	}
	return r
}

// Run 返回当前 run（只读用途）。
func (r *Recorder) Run() *Run {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.run
}

// StartSpan 开启一个 span，parent 来自 context（eino 适配器维护）；无则挂到 run 根。
// 返回的 span 会写入 context（见 WithSpan），供后续 EndSpan / 嵌套子 span 使用。
func (r *Recorder) StartSpan(ctx context.Context, name, component string, input string) (*Span, context.Context) {
	parent, _ := ctx.Value(spanKey{}).(*Span)
	now := time.Now()
	s := &Span{
		ID:        nextSpanID(),
		Name:      name,
		Component: component,
		StartedAt: now.Format(time.RFC3339Nano),
		startNs:   now.UnixNano(),
		Input:     truncate(input),
		parent:    parent,
	}
	if parent != nil {
		s.ParentID = parent.ID
	}
	r.mu.Lock()
	r.run.Spans = append(r.run.Spans, s)
	r.mu.Unlock()
	if r.otel != nil {
		r.otel.onStart(s)
	}
	return s, context.WithValue(ctx, spanKey{}, s)
}

// EndSpan 结束 span，记录输出 / 错误 / 耗时。
func (r *Recorder) EndSpan(ctx context.Context, s *Span, output string, err error) {
	if s == nil {
		return
	}
	end := time.Now()
	s.EndedAt = end.Format(time.RFC3339Nano)
	s.DurationMs = (end.UnixNano() - s.startNs) / int64(time.Millisecond)
	s.Output = truncate(output)
	if err != nil {
		s.Error = err.Error()
	}
	if r.otel != nil {
		r.otel.onEnd(s, err)
	}
	fireSpanHooks(s)
}

// SpanHook span 结束时的回调（用于 metrics 等下游订阅，避免 trace 反向依赖 metrics）。
type SpanHook func(s *Span)

var (
	hookMu   sync.Mutex
	spanHooks []SpanHook
)

// RegisterSpanHook 注册 span 结束回调（main 启动期由 metrics 调用）。
func RegisterSpanHook(h SpanHook) {
	if h == nil {
		return
	}
	hookMu.Lock()
	spanHooks = append(spanHooks, h)
	hookMu.Unlock()
}

func fireSpanHooks(s *Span) {
	hookMu.Lock()
	hooks := append([]SpanHook(nil), spanHooks...)
	hookMu.Unlock()
	for _, h := range hooks {
		h(s) // 回调内不应 panic；metrics 回调自身是安全的
	}
}

// Event 记录一个瞬时事件（无 duration 的叶子 span），用于 adk 事件流捕获
// （planner 出计划 / executor 调工具 / replanner 决策等离散事件）。
func (r *Recorder) Event(ctx context.Context, name, component string, attrs map[string]string) {
	parent, _ := ctx.Value(spanKey{}).(*Span)
	s := &Span{
		ID:        nextSpanID(),
		Name:      name,
		Component: component,
		StartedAt: time.Now().Format(time.RFC3339Nano),
		Attrs:     attrs,
	}
	if parent != nil {
		s.ParentID = parent.ID
	}
	r.mu.Lock()
	r.run.Spans = append(r.run.Spans, s)
	r.mu.Unlock()
}

// Finalize 结束 run，设置状态并落 store。返回 run 副本（已可序列化）。
func (r *Recorder) Finalize(status string, err error) *Run {
	r.mu.Lock()
	r.run.EndedAt = time.Now().Format(time.RFC3339Nano)
	r.run.Status = status
	if err != nil {
		r.run.Error = err.Error()
	}
	run := r.run
	r.mu.Unlock()
	DefaultStore.Put(run)
	return run
}

// SetOTel 绑定可选 OTel sink（由 InitOTel 后调用）。
func (r *Recorder) SetOTel(s *otelSink) { r.otel = s }

// WithRecorder / FromContext 在 context 中携带 Recorder，便于深层调用取出。
type recorderKey struct{}

func WithRecorder(ctx context.Context, r *Recorder) context.Context {
	return context.WithValue(ctx, recorderKey{}, r)
}

func FromContext(ctx context.Context) *Recorder {
	r, _ := ctx.Value(recorderKey{}).(*Recorder)
	return r
}

// spanFromCtx 取当前 span（eino 适配器内部用）。
func spanFromCtx(ctx context.Context) *Span {
	s, _ := ctx.Value(spanKey{}).(*Span)
	return s
}

// MarshalInput 把任意回调输入序列化为字符串（best-effort，失败返回占位）。
func MarshalInput(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "<unmarshallable>"
	}
	return string(b)
}
