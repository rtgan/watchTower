package supervisor

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"watchTower/common/trace"
)

// InstrumentedTool 透明包裹一个 InvokableTool，每次调用记录一个 trace span（component=Tool）。
//
// 作用：子 agent（react）内部工具调用默认不会进 trace；通过包装每个工具，
// 在 InvokableRun 前后 StartSpan/EndSpan，把"调了哪个工具、入参/出参/耗时/是否报错"记入子 trace，
// 同时经 trace span 钩子触发 tool_call_total 指标。并发安全（各子 agent 闭包各自的 recorder）。
type InstrumentedTool struct {
	inner tool.InvokableTool
	rec   *trace.Recorder
}

func NewInstrumentedTool(inner tool.InvokableTool, rec *trace.Recorder) *InstrumentedTool {
	return &InstrumentedTool{inner: inner, rec: rec}
}

func (t *InstrumentedTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.inner.Info(ctx)
}

func (t *InstrumentedTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	name := "tool"
	if info, err := t.inner.Info(ctx); err == nil && info != nil {
		name = info.Name
	}
	s, ctx2 := t.rec.StartSpan(ctx, name, "Tool", args)
	out, err := t.inner.InvokableRun(ctx2, args, opts...)
	t.rec.EndSpan(ctx2, s, out, err)
	return out, err
}

// instrument 批量包装工具（仅 InvokableTool 会被包装；其余原样返回）。
func instrument(tools []tool.BaseTool, rec *trace.Recorder) []tool.BaseTool {
	if rec == nil {
		return tools
	}
	out := make([]tool.BaseTool, 0, len(tools))
	for _, bt := range tools {
		if it, ok := bt.(tool.InvokableTool); ok {
			out = append(out, NewInstrumentedTool(it, rec))
		} else {
			out = append(out, bt)
		}
	}
	return out
}
