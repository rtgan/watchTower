package mem

import (
	"sync"

	"github.com/cloudwego/eino/schema"
)

var (
	SimpleMemoryMap = make(map[string]*SimpleMemory) //内存级存储：「会话ID → 这段对话的短期记忆」
	mu              sync.Mutex
)

// 按会话ID获取对应的短期会话历史；若没有就创建一个
func GetSimpleMemory(id string) *SimpleMemory {
	mu.Lock()
	defer mu.Unlock()
	// 如果存在就返回，不存在就创建
	if mem, ok := SimpleMemoryMap[id]; ok {
		return mem
	} else {
		newMem := &SimpleMemory{
			ID:            id,
			Messages:      []*schema.Message{},
			MaxWindowSize: 20,
		}
		SimpleMemoryMap[id] = newMem
		return newMem
	}
}

// 按会话ID保存的多轮对话短期记忆
type SimpleMemory struct {
	ID            string            `json:"id"`
	Messages      []*schema.Message `json:"messages"`
	MaxWindowSize int
	smu           sync.RWMutex
}

// 保存消息到短期记忆
func (s *SimpleMemory) SetMessages(msg *schema.Message) {
	s.smu.Lock()
	defer s.smu.Unlock()
	s.Messages = append(s.Messages, msg)
	if len(s.Messages) > s.MaxWindowSize {
		// 确保成对丢弃消息，保持对话配对关系
		// 计算需要丢弃的消息数量（必须是偶数）
		excess := len(s.Messages) - s.MaxWindowSize
		if excess%2 != 0 {
			excess++ // 确保丢弃偶数条消息
		}
		// 丢弃前面的消息，保持对话配对
		s.Messages = s.Messages[excess:]
	}
}

// 获取短期记忆中的消息列表
func (s *SimpleMemory) GetMessages() []*schema.Message {
	s.smu.RLock()
	defer s.smu.RUnlock()
	return s.Messages
}
