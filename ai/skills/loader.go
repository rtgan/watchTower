package skills

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Content     string `yaml:"-"`
}

var (
	loadedSkills []*Skill
	loadOnce     sync.Once
)

// LoadSkills scans the ai/skills/ directory for .md files with YAML frontmatter,
// parses them into Skill structs, and caches the result.
func LoadSkills() []*Skill {
	loadOnce.Do(func() {
		dir := findSkillsDir()
		if dir == "" {
			log.Println("[skills] skills directory not found, skipping")
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			log.Printf("[skills] read dir %s: %v", dir, err)
			return
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			skill, err := parseSkillFile(filepath.Join(dir, e.Name()))
			if err != nil {
				log.Printf("[skills] skip %s: %v", e.Name(), err)
				continue
			}
			loadedSkills = append(loadedSkills, skill)
			log.Printf("[skills] loaded: %s (%s)", skill.Name, e.Name())
		}
		log.Printf("[skills] total %d skill(s) loaded", len(loadedSkills))
	})
	return loadedSkills
}

// FormatForPrompt renders all loaded skills into a text block suitable for
// injection into a system prompt.（所有skill都会读取下来并注入到目标的system prompt中）
func FormatForPrompt() string {
	list := LoadSkills()
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## 已加载的 Skills（领域知识）\n")
	for i, skill := range list {
		b.WriteString(fmt.Sprintf("\n### Skill %d: %s\n", i+1, skill.Name))
		if skill.Description != "" {
			b.WriteString(fmt.Sprintf("说明: %s\n", skill.Description))
		}
		b.WriteString(skill.Content)
		b.WriteString("\n")
	}
	return b.String()
}

// parseSkillFile reads a .md file, splits YAML frontmatter (between --- lines)
// from the markdown body, and returns a Skill.
func parseSkillFile(path string) (*Skill, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(raw)

	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	skill := &Skill{}
	if err := yaml.Unmarshal([]byte(frontmatter), skill); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	if skill.Name == "" {
		skill.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	}
	skill.Content = strings.TrimSpace(body)
	return skill, nil
}

func splitFrontmatter(content string) (frontmatter, body string, err error) {
	const sep = "---"
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, sep) {
		return "", trimmed, nil
	}
	rest := trimmed[len(sep):]
	idx := strings.Index(rest, sep)
	if idx < 0 {
		return "", trimmed, nil
	}
	return strings.TrimSpace(rest[:idx]), strings.TrimSpace(rest[idx+len(sep):]), nil
}

func findSkillsDir() string {
	wd, err := os.Getwd() //获取当前进程的工作目录(运行所在目录)
	if err != nil {
		return ""
	}
	dir := wd
	for { //从当前工作目录开始，逐级向上查找 go.mod 文件，直到找到 ai/skills 目录
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			candidate := filepath.Join(dir, "ai", "skills")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
