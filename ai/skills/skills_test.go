package skills

import (
	"strings"
	"testing"
)

// TestLoadSkills 验证 skill 文件能被 loader 解析（frontmatter + body），
// 并保证 alert_handling 速查路由的关键映射存在。改 skill 文件后跑此测试防回归。
func TestLoadSkills(t *testing.T) {
	list := LoadSkills()
	if len(list) == 0 {
		t.Skip("no skills loaded（可能 cwd 找不到 go.mod/ai/skills）")
	}
	var ah *Skill
	for _, s := range list {
		if strings.Contains(s.Name, "告警处理") || strings.Contains(s.Name, "告警") {
			ah = s
		}
	}
	if ah == nil {
		t.Fatalf("告警处理 skill 未加载；got %+v", names(list))
	}
	if ah.Description == "" {
		t.Fatal("description 为空")
	}
	// 速查路由应含告警名->关键字映射，且明确指向知识库为单一事实源
	body := ah.Content
	for _, want := range []string{"ServiceDown", "panic", "ReconciliationDiff", "query_internal_docs"} {
		if !strings.Contains(body, want) {
			t.Fatalf("alert_handling skill 缺少 %q；body: %s", want, body[:min(200, len(body))])
		}
	}
}

func names(list []*Skill) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		out = append(out, s.Name)
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
