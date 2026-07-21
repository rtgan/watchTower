package trace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// otelSink 把 trace.Span 镜像成 OpenTelemetry span。
// 默认不启用（globalOTel==nil）；调用 InitOTel 后所有新 Recorder 自动拾取。
//
// 设计取舍：OTel span 创建为根 span（不携带 parent context），以保证并发安全。
// 结构化 trace 树（Recorder）保留了完整父子关系；OTel 这一路用于对接外部 collector 的可观测栈。
type otelSink struct {
	tracer oteltrace.Tracer
	spans  sync.Map // spanID -> oteltrace.Span
}

// globalOTel 全局 OTel sink。InitOTel 设置后，NewRecorder 自动拾取。
var globalOTel *otelSink

// InitOTel 初始化 OTel tracer provider，写入 traces/otel/spans.jsonl（自实现 file exporter，
// 无需外部 collector 即可验证 OTel 链路）。返回 shutdown 函数。
//
// 仅当 config.trace.enabled=true 时在 main 启动期调用；否则 globalOTel 保持 nil，零开销。
func InitOTel(serviceName, traceDir string) (func(context.Context) error, error) {
	otelDir := filepath.Join(traceDir, "otel")
	if err := os.MkdirAll(otelDir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(otelDir, "spans.jsonl"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	exp := &fileExporter{f: f}

	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes("", attribute.String("service.name", serviceName)))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp), // 同步导出，简单且足够验证
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	globalOTel = &otelSink{tracer: tp.Tracer("watchTower")}
	return tp.Shutdown, nil
}

// fileExporter 自实现 OTel SpanExporter，把 span 以 JSONL 写入文件。
// 避免引入外部 exporter 包；足以验证 OTel 链路通。
type fileExporter struct {
	mu sync.Mutex
	f  *os.File
}

func (e *fileExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if e.f == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, sp := range spans {
		rec := map[string]any{
			"name":      sp.Name(),
			"start":     sp.StartTime().Format(time.RFC3339Nano),
			"end":       sp.EndTime().Format(time.RFC3339Nano),
			"duration_ms": sp.EndTime().Sub(sp.StartTime()).Milliseconds(),
			"attrs":     attrsToMap(sp.Attributes()),
			"status":    sp.Status().Code.String(),
		}
		b, _ := json.Marshal(rec)
		if _, err := e.f.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func (e *fileExporter) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.f != nil {
		err := e.f.Close()
		e.f = nil
		return err
	}
	return nil
}

func attrsToMap(kvs []attribute.KeyValue) map[string]string {
	if len(kvs) == 0 {
		return nil
	}
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		m[string(kv.Key)] = kv.Value.Emit()
	}
	return m
}

func (o *otelSink) onStart(s *Span) {
	if o == nil || o.tracer == nil {
		return
	}
	_, sp := o.tracer.Start(context.Background(), s.Name)
	o.spans.Store(s.ID, sp)
}

func (o *otelSink) onEnd(s *Span, err error) {
	if o == nil || o.tracer == nil {
		return
	}
	if v, ok := o.spans.LoadAndDelete(s.ID); ok {
		sp := v.(oteltrace.Span)
		if err != nil {
			sp.RecordError(err)
		}
		sp.End()
	}
}
