package trace

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
)

func TestRecorderNestedSpans(t *testing.T) {
	rec := NewRecorder("test query", "2026-07-14T10:00:00Z")
	ctx := context.Background()

	root, ctx := rec.StartSpan(ctx, "agent", "Lambda", "input")
	child, ctx2 := rec.StartSpan(ctx, "tool:query_log", "Tool", `{"query":"panic"}`)
	rec.EndSpan(ctx2, child, `{"total":4}`, nil)
	rec.EndSpan(ctx, root, "done", nil)

	run := rec.Finalize("ok", nil)
	if len(run.Spans) != 2 {
		t.Fatalf("expect 2 spans, got %d", len(run.Spans))
	}
	if run.Spans[0].ID != run.Spans[1].ParentID {
		t.Fatalf("child parent_id should match root id: root=%s child.parent=%s",
			run.Spans[0].ID, run.Spans[1].ParentID)
	}
	if run.Spans[1].Component != "Tool" {
		t.Fatalf("child component should be Tool, got %s", run.Spans[1].Component)
	}
}

func TestEinoHandler(t *testing.T) {
	rec := NewRecorder("q", time.Now().Format(time.RFC3339Nano))
	h := EinoHandler(rec)
	ctx := context.Background()

	info := &callbacks.RunInfo{Name: "query_log", Type: "Invokable", Component: components.Component("Tool")}
	ctx = h.OnStart(ctx, info, `{"query":"panic"}`)
	ctx = h.OnEnd(ctx, info, `{"total":4}`)

	run := rec.Run()
	if len(run.Spans) != 1 {
		t.Fatalf("expect 1 span from eino handler, got %d", len(run.Spans))
	}
	if run.Spans[0].Name != "query_log" || run.Spans[0].Component != "Tool" {
		t.Fatalf("span mismatch: %+v", run.Spans[0])
	}
	if run.Spans[0].Output == "" {
		t.Fatal("span output should be recorded")
	}
}

func TestEinoHandlerNil(t *testing.T) {
	// nil recorder 应返回 no-op handler，不 panic
	h := EinoHandler(nil)
	ctx := h.OnStart(context.Background(), &callbacks.RunInfo{Name: "x"}, nil)
	h.OnEnd(ctx, &callbacks.RunInfo{Name: "x"}, nil)
}

func TestStorePutGetList(t *testing.T) {
	s := NewStore(10, t.TempDir())
	r := &Run{ID: "run-test-1", Query: "q", Status: "ok", StartedAt: "t1", Spans: []*Span{{ID: "s1"}}}
	s.Put(r)

	got, err := s.Get("run-test-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Query != "q" {
		t.Fatalf("query mismatch: %s", got.Query)
	}

	list := s.List(10)
	if len(list) != 1 {
		t.Fatalf("expect 1 in list, got %d", len(list))
	}
	if list[0].SpanCount != 1 {
		t.Fatalf("span count mismatch: %d", list[0].SpanCount)
	}
}

func TestStoreLRUEviction(t *testing.T) {
	s := NewStore(2, "")
	for i := 0; i < 5; i++ {
		s.Put(&Run{ID: "r" + string(rune('0'+i)), Query: "q"})
	}
	if len(s.List(10)) != 2 {
		t.Fatalf("expect 2 after LRU eviction, got %d", len(s.List(10)))
	}
}

func TestOTelSink(t *testing.T) {
	dir := t.TempDir()
	shutdown, err := InitOTel("watchTower-test", dir)
	if err != nil {
		t.Fatalf("InitOTel: %v", err)
	}
	defer func() {
		_ = shutdown(context.Background())
		globalOTel = nil // 还原全局，避免污染其他测试
	}()

	// 新 Recorder 应自动拾取 globalOTel
	rec := NewRecorder("otel q", "2026-07-14T10:00:00Z")
	ctx := context.Background()
	s, ctx2 := rec.StartSpan(ctx, "otel-span", "Tool", "in")
	rec.EndSpan(ctx2, s, "out", nil)
	rec.Finalize("ok", nil)

	// WithSyncer 同步导出：EndSpan 后文件应已写入
	_ = shutdown(context.Background()) // 触发 flush
	b, rerr := readFileIfExists(dir + "/otel/spans.jsonl")
	if rerr != nil {
		t.Fatalf("read otel file: %v", rerr)
	}
	if len(b) == 0 {
		t.Fatal("otel spans file empty; expected at least one span")
	}
}

func readFileIfExists(path string) ([]byte, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}
