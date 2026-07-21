package skills

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino/components/embedding"
)

// defaultTopK 路由注入的 skill 数上限（实现 TODO：向量相似度匹配对应 skill，避免全量塞 prompt）。
const defaultTopK = 2

// Router 按 query 向量相似度路由 skill，替代 FormatForPrompt 的全量注入。
//
// 单例懒加载：首次调用 GetRouter 时用 doubao embedder 预算每个 skill description 的向量。
// embedder 不可用（如未配 ARK_API_KEY）时返回 error，调用方降级到全量 FormatForPrompt。
type Router struct {
	embedder embedding.Embedder
	skills   []*Skill
	vecs     [][]float64 // 与 skills 一一对应
}

var (
	defaultRouter *Router
	routerOnce    sync.Once
	routerErr     error
)

// GetRouter 单例（懒加载 + 预算 skill 向量）。
func GetRouter() (*Router, error) {
	routerOnce.Do(func() {
		creator := model.GetGlobalFactory().GetModelCreator(model.DoubaoEmbedderType)
		if creator == nil {
			routerErr = fmt.Errorf("embedder creator not found")
			return
		}
		eb, ok := creator(context.Background(), config.Conf).(*model.DoubaoEmbedder)
		if !ok || eb == nil || eb.Embedder == nil {
			routerErr = fmt.Errorf("embedder nil")
			return
		}
		r := &Router{embedder: eb.Embedder, skills: LoadSkills()}
		if len(r.skills) == 0 {
			defaultRouter = r // 无 skill：路由返回空，等价于不注入
			return
		}
		descs := make([]string, len(r.skills))
		for i, s := range r.skills {
			descs[i] = s.Name + " " + s.Description
		}
		vecs, err := r.embedder.EmbedStrings(context.Background(), descs)
		if err != nil {
			routerErr = fmt.Errorf("embed skills: %w", err)
			return
		}
		r.vecs = vecs
		defaultRouter = r
	})
	return defaultRouter, routerErr
}

// TopK 返回与 query 最相似的 k 个 skill（余弦相似度降序）。embedder 失败时降级全量。
func (r *Router) TopK(ctx context.Context, query string, k int) ([]*Skill, error) {
	all := LoadSkills()
	if r == nil || r.embedder == nil || len(r.skills) == 0 || len(r.vecs) != len(r.skills) {
		return all, nil // 降级全量
	}
	if k <= 0 || k > len(r.skills) {
		k = len(r.skills)
	}
	qvecs, err := r.embedder.EmbedStrings(ctx, []string{query})
	if err != nil || len(qvecs) == 0 {
		return all, nil // 降级全量
	}
	qv := qvecs[0]
	type scored struct {
		idx int
		s   float64
	}
	ss := make([]scored, len(r.skills))
	for i, sv := range r.vecs {
		ss[i] = scored{i, cosine(qv, sv)}
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].s > ss[j].s })
	out := make([]*Skill, 0, k)
	for i := 0; i < k && i < len(ss); i++ {
		out = append(out, r.skills[ss[i].idx])
	}
	return out, nil
}

// RouteForPrompt 按 query 相关性渲染 top-K skill（embedder 不可用则降级全量）。
// 这是 plan_execute_replan 注入 skill 的推荐入口（替代 FormatForPrompt）。
func RouteForPrompt(ctx context.Context, query string) string {
	r, err := GetRouter()
	if err != nil || r == nil {
		return FormatForPrompt() // 降级全量
	}
	top, err := r.TopK(ctx, query, defaultTopK)
	if err != nil || len(top) == 0 {
		return FormatForPrompt()
	}
	return formatSkills(top)
}

// formatSkills 渲染筛选后的 skill（标注"按相关性筛选"以便 prompt 可读）。
func formatSkills(skills []*Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## 已加载的 Skills（领域知识，按相关性筛选）\n")
	for i, s := range skills {
		b.WriteString(fmt.Sprintf("\n### Skill %d: %s\n", i+1, s.Name))
		if s.Description != "" {
			b.WriteString(fmt.Sprintf("说明: %s\n", s.Description))
		}
		b.WriteString(s.Content)
		b.WriteString("\n")
	}
	return b.String()
}

func cosine(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
