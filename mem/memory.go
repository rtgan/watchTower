package mem

import (
	"context"
	"sync"
	"time"
)

// SimpleMemory 对外保持与旧 API 兼容的会话记忆句柄：GetMessages / SetMessages。
// 内部委托 Store（InMemory 或 Redis），并负责窗口淘汰 + 可选摘要压缩。
// 新增 longTerm 支持：滑动窗口淘汰的消息异步存入 Milvus，GetMessagesWithContext 可注入语义召回。
type SimpleMemory struct {
	ID        string
	store     Store
	maxWindow int
	longTerm  LongTermMemory // 可选：长期记忆存储（Milvus）
	userID    string         // 用户标识，用于跨会话记忆检索
}

const defaultMaxWindow = 20

// 按会话 ID 加锁，保证同会话的 Load-append-Store 在进程内串行（跨进程需分布式锁，见 RedisStore 文档）。
var (
	locksMu sync.Mutex
	locks   = map[string]*sync.Mutex{}
)

func lockFor(id string) *sync.Mutex {
	locksMu.Lock()
	defer locksMu.Unlock()
	if m, ok := locks[id]; ok {
		return m
	}
	m := &sync.Mutex{}
	locks[id] = m
	return m
}

// GetSimpleMemory 按会话 ID 获取记忆句柄（向后兼容旧 API）。
func GetSimpleMemory(id string) *SimpleMemory {
	return &SimpleMemory{
		ID:        id,
		store:     defaultStore,
		maxWindow: defaultMaxWindow,
	}
}

// GetSimpleMemoryWithUser 按会话 ID + 用户 ID + 长期记忆获取句柄。
// 当 longTerm 非 nil 时，GetMessagesWithContext 会注入语义召回的记忆。
func GetSimpleMemoryWithUser(id, userID string, lt LongTermMemory) *SimpleMemory {
	return &SimpleMemory{
		ID:        id,
		store:     defaultStore,
		maxWindow: defaultMaxWindow,
		longTerm:  lt,
		userID:    userID,
	}
}

// SetMaxWindow 设置窗口大小（测试 / 调试用）。
func (s *SimpleMemory) SetMaxWindow(n int) {
	if n > 0 {
		s.maxWindow = n
	}
}

// GetMessages 取当前会话全部消息。
func (s *SimpleMemory) GetMessages() []*Message {
	msgs, _ := s.store.Load(s.ID)
	return msgs
}

// GetMessagesWithContext 取滑动窗口消息 + 语义召回长期记忆（若 longTerm 已启用）。
// query 为当前用户问题，用于 Milvus 语义检索。
func (s *SimpleMemory) GetMessagesWithContext(ctx context.Context, query string) []*Message {
	msgs, _ := s.store.Load(s.ID)

	// 若 longTerm 可用，注入语义召回的记忆
	if s.longTerm != nil && query != "" {
		topK := 3
		memories, err := s.longTerm.Search(ctx, s.userID, query, topK)
		if err == nil && len(memories) > 0 {
			summary := formatMemoriesForContext(memories)
			if summary != nil {
				// 将长期记忆 prepend 到窗口消息之前
				msgs = append([]*Message{summary}, msgs...)
			}
		}
	}

	return msgs
}

// SetMessages 追加一条消息，并按窗口淘汰 / 可选摘要压缩后持久化。
//
// 摘要压缩：仅当 config.memory.summarize=true 且当前是 assistant 消息且消息数超过 2*maxWindow 时触发，
// 把较早的历史压缩成一条摘要 + 保留最近窗口。默认关闭，关闭时退化为与原逻辑一致的成对窗口淘汰。
func (s *SimpleMemory) SetMessages(ctx context.Context, msg *Message) {
	mu := lockFor(s.ID)
	mu.Lock()
	defer mu.Unlock()

	msgs, _ := s.store.Load(s.ID)
	msgs = append(msgs, msg)

	// 淘汰前记录将被丢弃的消息，异步存入长期记忆
	oldLen := len(msgs)
	if summarizeEnabled() && string(msg.Role) == "assistant" && len(msgs) > 2*s.maxWindow {
		if compacted, err := compactHistory(ctx, msgs, s.maxWindow); err == nil {
			// 摘要压缩成功，淘汰的消息 = 被摘要压缩的旧消息
			evicted := msgs[:oldLen-s.maxWindow]
			s.asyncStoreEvicted(evicted)
			_ = s.store.Store(s.ID, compacted)
			return
		}
		// 摘要失败：降级为普通窗口淘汰，不阻塞写入
	}

	// 普通窗口淘汰：先捕获将被丢弃的消息，再淘汰
	excess := oldLen - s.maxWindow
	if excess > 0 {
		if excess%2 != 0 {
			excess++ // 偶数条，保持配对
		}
		if excess > 0 && excess <= oldLen {
			evicted := make([]*Message, excess)
			copy(evicted, msgs[:excess])
			s.asyncStoreEvicted(evicted)
		}
	}
	msgs = s.evict(msgs)
	_ = s.store.Store(s.ID, msgs)
}

// asyncStoreEvicted 异步将淘汰消息存入长期记忆，不阻塞主流程。
func (s *SimpleMemory) asyncStoreEvicted(evicted []*Message) {
	if s.longTerm == nil || len(evicted) == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.longTerm.StoreEvicted(ctx, s.ID, s.userID, evicted)
	}()
}

// evict 成对丢弃最早的消息，保持 user/assistant 配对（与旧逻辑一致）。
func (s *SimpleMemory) evict(msgs []*Message) []*Message {
	if len(msgs) <= s.maxWindow {
		return msgs
	}
	excess := len(msgs) - s.maxWindow
	if excess%2 != 0 {
		excess++ // 丢弃偶数条，保持配对
	}
	if excess > len(msgs) {
		excess = len(msgs)
	}
	return msgs[excess:]
}
