package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type SearchFileInput struct {
	Keyword    string `json:"keyword" jsonschema:"description=文件名中要包含的关键词，例如 '哈尔滨'、'report'、'2026'"`
	SearchPath string `json:"search_path" jsonschema:"description=要搜索的目录路径，例如 '/data/images'。如果不指定则使用默认搜索根目录"`
	Extensions string `json:"extensions,omitempty" jsonschema:"description=按文件扩展名过滤（逗号分隔），例如 'jpg,png,gif' 只搜图片，'pdf,docx' 只搜文档。留空则搜索所有文件类型"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"description=最多返回的结果数量，默认 50"`
}

type FileInfo struct {
	Path    string `json:"path" jsonschema:"description=文件完整路径"`
	Name    string `json:"name" jsonschema:"description=文件名"`
	Size    string `json:"size" jsonschema:"description=文件大小（人类可读格式）"`
	ModTime string `json:"mod_time" jsonschema:"description=最后修改时间"`
	IsDir   bool   `json:"is_dir" jsonschema:"description=是否为目录"`
}

type SearchFileOutput struct {
	Success    bool       `json:"success" jsonschema:"description=搜索是否成功"`
	Files      []FileInfo `json:"files,omitempty" jsonschema:"description=匹配的文件列表"`
	TotalFound int        `json:"total_found" jsonschema:"description=匹配的文件总数"`
	Truncated  bool       `json:"truncated" jsonschema:"description=结果是否被截断（匹配数超过 max_results）"`
	SearchPath string     `json:"search_path" jsonschema:"description=实际搜索的目录路径"`
	Message    string     `json:"message,omitempty" jsonschema:"description=结果说明"`
	Error      string     `json:"error,omitempty" jsonschema:"description=错误信息"`
}

const defaultSearchRoot = "/tmp"
const defaultMaxResults = 50

func NewSearchFileTool() tool.InvokableTool {
	t, err := utils.InferOptionableTool(
		"search_file",
		"Search for files on the server by filename keyword. Supports filtering by file extension "+
			"(e.g. jpg,png for images) and recursive directory traversal. Use this tool when you need to "+
			"find files containing specific keywords in their names, locate images, documents, or any "+
			"files on the server filesystem.",
		func(ctx context.Context, input *SearchFileInput, opts ...tool.Option) (string, error) {
			log.Printf("[search_file] keyword=%q path=%q ext=%q", input.Keyword, input.SearchPath, input.Extensions)

			if input.Keyword == "" {
				out := SearchFileOutput{Success: false, Error: "keyword is required"}
				b, _ := json.MarshalIndent(out, "", "  ")
				return string(b), nil
			}

			searchPath := input.SearchPath
			if searchPath == "" {
				searchPath = defaultSearchRoot
			}
			searchPath = filepath.Clean(searchPath)

			info, err := os.Stat(searchPath)
			if err != nil || !info.IsDir() {
				out := SearchFileOutput{
					Success:    false,
					SearchPath: searchPath,
					Error:      fmt.Sprintf("directory does not exist or is not accessible: %s", searchPath),
				}
				b, _ := json.MarshalIndent(out, "", "  ")
				return string(b), nil
			}

			maxResults := input.MaxResults
			if maxResults <= 0 {
				maxResults = defaultMaxResults
			}

			extSet := parseExtensions(input.Extensions)
			keywordLower := strings.ToLower(input.Keyword)

			var matched []FileInfo
			totalFound := 0

			_ = filepath.WalkDir(searchPath, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				name := d.Name()
				if !strings.Contains(strings.ToLower(name), keywordLower) {
					return nil
				}
				if len(extSet) > 0 && !d.IsDir() {
					ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
					if !extSet[ext] {
						return nil
					}
				}
				totalFound++
				if len(matched) < maxResults {
					fi := FileInfo{
						Path:  path,
						Name:  name,
						IsDir: d.IsDir(),
					}
					if info, err := d.Info(); err == nil {
						fi.Size = humanSize(info.Size())
						fi.ModTime = info.ModTime().Format("2006-01-02 15:04:05")
					}
					matched = append(matched, fi)
				}
				return nil
			})

			out := SearchFileOutput{
				Success:    true,
				Files:      matched,
				TotalFound: totalFound,
				Truncated:  totalFound > len(matched),
				SearchPath: searchPath,
				Message:    fmt.Sprintf("Found %d file(s) matching keyword %q", totalFound, input.Keyword),
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			log.Printf("[search_file] done: %d found, %d returned", totalFound, len(matched))
			return string(b), nil
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	return t
}

func parseExtensions(raw string) map[string]bool {
	if raw == "" {
		return nil
	}
	m := make(map[string]bool)
	for _, e := range strings.Split(raw, ",") {
		e = strings.TrimSpace(strings.ToLower(e))
		e = strings.TrimPrefix(e, ".")
		if e != "" {
			m[e] = true
		}
	}
	return m
}

func humanSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
