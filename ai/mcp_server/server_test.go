package mcp_server

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewServerConstructs(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if s == nil {
		t.Fatal("server nil")
	}
	// 注册三个工具时若 schema 非法会 panic/err；到这里即说明 AddTool 成功
}

func TestArgsToJSONEmpty(t *testing.T) {
	got := argsToJSON(mcp.CallToolRequest{})
	if got != "{}" {
		t.Fatalf("expect {} for empty args, got %s", got)
	}
}

func TestArgsToJSONWithArgs(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "panic", "limit": float64(50)}
	got := argsToJSON(req)
	if got != `{"limit":50,"query":"panic"}` {
		// map 序不保证；放宽为子串检查
		if !contains(got, `"query":"panic"`) || !contains(got, `"limit":50`) {
			t.Fatalf("unexpected args json: %s", got)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
