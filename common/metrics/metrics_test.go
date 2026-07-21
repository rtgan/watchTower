package metrics

import (
	"testing"
	"watchTower/common/trace"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestSpanHookToolCounter(t *testing.T) {
	// 注册一次（测试环境）

	spanHook(&trace.Span{Component: "Tool", Name: "query_log"})
	spanHook(&trace.Span{Component: "Tool", Name: "query_log"})
	spanHook(&trace.Span{Component: "Tool", Name: "query_log", Error: "boom"})

	success := testutil.ToFloat64(ToolCallTotal.WithLabelValues("query_log", "success"))
	fail := testutil.ToFloat64(ToolCallTotal.WithLabelValues("query_log", "fail"))
	if success != 2 {
		t.Fatalf("expect 2 success tool calls, got %v", success)
	}
	if fail != 1 {
		t.Fatalf("expect 1 fail tool call, got %v", fail)
	}
}

func TestSpanHookTokenCounter(t *testing.T) {
	spanHook(&trace.Span{
		Component: "ChatModel",
		Name:      "deepseek",
		Attrs: map[string]string{
			"model":              "deepseek",
			"prompt_tokens":      "120",
			"completion_tokens":  "80",
		},
	})
	prompt := testutil.ToFloat64(LLMTokensTotal.WithLabelValues("deepseek", "prompt"))
	comp := testutil.ToFloat64(LLMTokensTotal.WithLabelValues("deepseek", "completion"))
	if prompt != 120 {
		t.Fatalf("expect 120 prompt tokens, got %v", prompt)
	}
	if comp != 80 {
		t.Fatalf("expect 80 completion tokens, got %v", comp)
	}
}

func TestSpanHookIgnoresIrrelevant(t *testing.T) {
	// 非 Tool/ChatModel 的 span 不应影响计数
	before := testutil.ToFloat64(ToolCallTotal.WithLabelValues("unknown", "success"))
	spanHook(&trace.Span{Component: "Retriever", Name: "milvus"})
	after := testutil.ToFloat64(ToolCallTotal.WithLabelValues("unknown", "success"))
	if before != after {
		t.Fatalf("retriever span should not affect tool counter: %v -> %v", before, after)
	}
}

func TestRecordDeflection(t *testing.T) {
	RecordDeflection("resolved")
	RecordDeflection("resolved")
	RecordDeflection("escalated")
	r := testutil.ToFloat64(DeflectionObserved.WithLabelValues("resolved"))
	e := testutil.ToFloat64(DeflectionObserved.WithLabelValues("escalated"))
	if r < 2 || e < 1 {
		t.Fatalf("deflection counts wrong: resolved=%v escalated=%v", r, e)
	}
}
