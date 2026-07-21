package supervisor

import (
	"context"
	"encoding/json"
	"time"
	"watchTower/ai/tools"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// RecoverableLogTool 包裹 query_log，在首次返回空/失败时按"扩大时间窗到最近 6h"重试一次。
//
// 体现"工具可靠性 / 错误恢复"：原 executor 对 query_log 失败是静默降级跳过，
// 这里改为显式重试；仍失败则返回原始结果，由上层 finding.Error 标注需人工介入。
type RecoverableLogTool struct {
	inner tool.InvokableTool
}

func NewRecoverableLogTool(inner tool.InvokableTool) *RecoverableLogTool {
	return &RecoverableLogTool{inner: inner}
}

func (t *RecoverableLogTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.inner.Info(ctx)
}

func (t *RecoverableLogTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	out, err := t.inner.InvokableRun(ctx, args, opts...)
	if err == nil && !isEmptyOrFailed(out) {
		return out, nil
	}
	// 恢复：扩大时间窗重试一次
	widened := widenTimeWindow(args)
	out2, err2 := t.inner.InvokableRun(ctx, widened, opts...)
	if err2 == nil && !isEmptyOrFailed(out2) {
		// 标注本次结果是恢复重试得到
		return annotateRecovered(out2), nil
	}
	// 仍失败：返回原始结果
	return out, err
}

// isEmptyOrFailed 判断 query_log 返回是否为空或失败（success=false 或 total=0）。
func isEmptyOrFailed(s string) bool {
	var probe struct {
		Success bool `json:"success"`
		Total   int  `json:"total"`
	}
	if err := json.Unmarshal([]byte(s), &probe); err != nil {
		return false
	}
	return !probe.Success || probe.Total == 0
}

// widenTimeWindow 把 QueryLogInput 的时间窗扩大到最近 6 小时（保留 query/limit）。
func widenTimeWindow(args string) string {
	var in tools.QueryLogInput
	if err := json.Unmarshal([]byte(args), &in); err != nil {
		return args
	}
	now := time.Now()
	in.StartTime = now.Add(-6 * time.Hour).Format("2006-01-02 15:04:05")
	in.EndTime = now.Format("2006-01-02 15:04:05")
	b, err := json.Marshal(in)
	if err != nil {
		return args
	}
	return string(b)
}

// annotateRecovered 在返回 JSON 里加一个标记，便于上层识别这是恢复重试的结果。
func annotateRecovered(s string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return s
	}
	m["recovered"] = true
	b, err := json.Marshal(m)
	if err != nil {
		return s
	}
	return string(b)
}
