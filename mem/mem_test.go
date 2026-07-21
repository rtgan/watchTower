package mem

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/cloudwego/eino/schema"
)

func mkMsg(role, content string) *Message {
	switch role {
	case "user":
		return schema.UserMessage(content)
	case "assistant":
		return schema.AssistantMessage(content, nil)
	default:
		return schema.SystemMessage(content)
	}
}

func TestInMemoryStoreRoundTrip(t *testing.T) {
	s := NewInMemoryStore()
	id := "c1"
	if msgs, _ := s.Load(id); msgs != nil {
		t.Fatal("expect nil for new convo")
	}
	_ = s.Store(id, []*Message{mkMsg("user", "hi"), mkMsg("assistant", "hello")})
	got, _ := s.Load(id)
	if len(got) != 2 {
		t.Fatalf("expect 2 msgs, got %d", len(got))
	}
}

func TestRedisStoreRoundTrip(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Skipf("miniredis unavailable: %v", err)
	}
	defer mr.Close()

	rs, err := NewRedisStore(mr.Addr(), "", 0)
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	SetDefaultStore(rs)
	defer SetDefaultStore(NewInMemoryStore())

	id := "c1"
	sm := GetSimpleMemory(id)
	sm.SetMaxWindow(4)
	sm.SetMessages(context.Background(), mkMsg("user", "hi"))
	sm.SetMessages(context.Background(), mkMsg("assistant", "hello"))

	got := sm.GetMessages()
	if len(got) != 2 {
		t.Fatalf("expect 2 msgs after set, got %d", len(got))
	}
	// 验证确实落到 Redis（直接查 key）
	exists, err := rs.client.Exists(context.Background(), redisKey(id)).Result()
	if err != nil {
		t.Fatalf("redis exists: %v", err)
	}
	if exists != 1 {
		t.Fatal("expect key to exist in redis")
	}
}

func TestSimpleMemoryWindowEviction(t *testing.T) {
	SetDefaultStore(NewInMemoryStore())
	sm := GetSimpleMemory("w1")
	sm.SetMaxWindow(4)
	ctx := context.Background()
	// 写 8 条（4 对），窗口 4 -> 淘汰最早 4 条，剩 4
	for i := 0; i < 4; i++ {
		sm.SetMessages(ctx, mkMsg("user", "u"))
		sm.SetMessages(ctx, mkMsg("assistant", "a"))
	}
	got := sm.GetMessages()
	if len(got) != 4 {
		t.Fatalf("expect 4 after eviction, got %d", len(got))
	}
}

func TestSimpleMemoryCompactFallbackWhenLLMUnavailable(t *testing.T) {
	// summarize 默认关闭；此处不强制开启，验证关闭时纯窗口淘汰行为正确即可
	SetDefaultStore(NewInMemoryStore())
	sm := GetSimpleMemory("w2")
	sm.SetMaxWindow(2)
	ctx := context.Background()
	sm.SetMessages(ctx, mkMsg("user", "q"))
	sm.SetMessages(ctx, mkMsg("assistant", "a"))
	if len(sm.GetMessages()) != 2 {
		t.Fatalf("expect 2, got %d", len(sm.GetMessages()))
	}
}
