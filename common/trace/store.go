package trace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// RunSummary trace 列表项（不含 spans，省带宽）。
type RunSummary struct {
	ID         string `json:"id"`
	Query      string `json:"query"`
	Status     string `json:"status"`
	StartedAt string `json:"started_at"`
	EndedAt    string `json:"ended_at,omitempty"`
	SpanCount  int    `json:"span_count"`
}

// Store trace 存储：内存（带 LRU 上限）+ 文件持久化（可选）。
type Store struct {
	mu    sync.RWMutex
	runs  map[string]*Run
	order []string // 按 Put 顺序，用于 LRU 淘汰
	cap   int
	dir   string // 文件持久化目录；"" 表示不落盘
}

// DefaultStore 默认存储实例。容量 1000，落盘到 traces/（相对进程 cwd）。
// 服务端通常从模块根目录启动，故 traces/ 即 <root>/traces/。
var DefaultStore = NewStore(1000, "traces")

// NewStore 创建存储。cap<=0 表示不限；dir 为空表示不落盘。
func NewStore(cap int, dir string) *Store {
	return &Store{runs: map[string]*Run{}, cap: cap, dir: dir}
}

// SetDir 更新落盘目录（main 启动期从 config 覆盖默认 "traces"）。
func (s *Store) SetDir(dir string) {
	s.mu.Lock()
	s.dir = dir
	s.mu.Unlock()
}

// Put 写入 run（内存 + 文件）。超容量时淘汰最旧。
func (s *Store) Put(r *Run) {
	if r == nil {
		return
	}
	s.mu.Lock()
	s.runs[r.ID] = r
	s.order = append(s.order, r.ID)
	if s.cap > 0 && len(s.order) > s.cap {
		drop := s.order[0]
		s.order = s.order[1:]
		delete(s.runs, drop)
	}
	s.mu.Unlock()
	s.writeFile(r)
}

// Get 取 run：先内存，后文件。
func (s *Store) Get(id string) (*Run, error) {
	s.mu.RLock()
	r, ok := s.runs[id]
	s.mu.RUnlock()
	if ok {
		return r, nil
	}
	return s.readFile(id)
}

// List 返回最近 N 个 run 的摘要（新的在前）。
func (s *Store) List(limit int) []RunSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.order) {
		limit = len(s.order)
	}
	out := make([]RunSummary, 0, limit)
	for i := len(s.order) - 1; i >= 0 && len(out) < limit; i-- {
		r := s.runs[s.order[i]]
		if r == nil {
			continue
		}
		out = append(out, RunSummary{
			ID: r.ID, Query: r.Query, Status: r.Status,
			StartedAt: r.StartedAt, EndedAt: r.EndedAt, SpanCount: len(r.Spans),
		})
	}
	return out
}

func (s *Store) writeFile(r *Run) {
	if s.dir == "" {
		return
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(s.dir, r.ID+".json")
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

func (s *Store) readFile(id string) (*Run, error) {
	if s.dir == "" {
		return nil, fmt.Errorf("trace %s not found", id)
	}
	path := filepath.Join(s.dir, id+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("trace %s not found: %w", id, err)
	}
	var r Run
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("parse trace %s: %w", id, err)
	}
	return &r, nil
}

// sortSpans 按开始时间稳定排序（helper，供展示用）。
func sortSpans(spans []*Span) {
	sort.SliceStable(spans, func(i, j int) bool {
		return spans[i].StartedAt < spans[j].StartedAt
	})
}
