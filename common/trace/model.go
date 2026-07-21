package trace

import "sync/atomic"

// Run 一次 agent 执行的完整 trace 树。
type Run struct {
	ID        string  `json:"id"`
	Query     string  `json:"query"`
	StartedAt string  `json:"started_at"`
	EndedAt   string  `json:"ended_at,omitempty"`
	Status    string  `json:"status"` // running | ok | error
	Error     string  `json:"error,omitempty"`
	Spans     []*Span `json:"spans"`
}

// Span trace 中的一个步骤（工具调用 / 模型调用 / planner 决策等）。
type Span struct {
	ID         string            `json:"id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Name       string            `json:"name"`
	Component  string            `json:"component,omitempty"` // ChatModel | Tool | Prompt | Lambda | ...
	Type       string            `json:"type,omitempty"`
	StartedAt  string            `json:"started_at"`
	EndedAt    string            `json:"ended_at,omitempty"`
	DurationMs int64             `json:"duration_ms"`
	Input      string            `json:"input,omitempty"`  // 截断后的输入摘要
	Output     string            `json:"output,omitempty"` // 截断后的输出摘要
	Error      string            `json:"error,omitempty"`
	Attrs      map[string]string `json:"attrs,omitempty"`

	// 运行期父子指针，不序列化。eino 适配器靠 context 链维护嵌套。
	parent *Span
	startNs int64 // unix nano，用于算 duration
}

// runSeq / spanSeq 仅用于生成可读的 ID（不依赖时间），跨进程不保证唯一，进程内唯一。
var runSeq uint64
var spanSeq uint64

func nextRunID() string {
	return "run-" + uintToStr(atomic.AddUint64(&runSeq, 1))
}

func nextSpanID() string {
	return "span-" + uintToStr(atomic.AddUint64(&spanSeq, 1))
}

func uintToStr(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// truncate 截断字符串，避免大输入/输出撑爆 trace 文件。
const maxFieldLen = 2000

func truncate(s string) string {
	if len(s) <= maxFieldLen {
		return s
	}
	return s[:maxFieldLen] + "...(truncated)"
}
