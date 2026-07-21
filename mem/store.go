package mem

import (
	"sync"

	"github.com/cloudwego/eino/schema"
)

// Message 会话消息的别名，便于 mem 包内统一引用。
type Message = schema.Message

// Store 会话记忆存储抽象。
//
// 设计动机：原 mem 是进程内 map（自标"生产不推荐，分布式一致性问题"）。
// 抽象出 Store 后，单机用 InMemoryStore，分布式部署切 RedisStore（config.memory.driver=redis），
// 上层 SimpleMemory 的窗口淘汰 / 摘要压缩逻辑不变。
type Store interface {
	// Load 取某会话的全部消息。不存在返回 (nil, nil)。
	Load(id string) ([]*Message, error)
	// Store 覆盖写某会话的全部消息。
	Store(id string, msgs []*Message) error
}

// InMemoryStore 进程内 map 存储（默认）。
type InMemoryStore struct {
	mu sync.Mutex
	m  map[string][]*Message
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{m: map[string][]*Message{}}
}

func (s *InMemoryStore) Load(id string) ([]*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[id], nil
}

func (s *InMemoryStore) Store(id string, msgs []*Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[id] = msgs
	return nil
}

// defaultStore 进程级默认存储。可由 main 按 config 切换为 RedisStore。
var defaultStore Store = NewInMemoryStore()

// SetDefaultStore main 启动期按 config.memory.driver 切换默认存储（如 RedisStore）。
func SetDefaultStore(s Store) {
	if s != nil {
		defaultStore = s
	}
}
