package mem

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

// LongTermMemory 跨会话长期记忆的存储与检索接口。
// 与 Store 接口正交：Store 管理单会话的滑动窗口，LongTermMemory 管理跨会话的语义记忆。
type LongTermMemory interface {
	// StoreEvicted 将滑动窗口淘汰的消息存入长期记忆（嵌入 + 写入 Milvus）。
	// userID 为空时仅按 sessionID 存储，不关联用户。
	StoreEvicted(ctx context.Context, sessionID, userID string, evicted []*Message) error

	// Search 根据查询文本搜索相关长期记忆，返回按相似度降序排列。
	Search(ctx context.Context, userID, query string, topK int) ([]*MemoryRecord, error)
}

// MemoryRecord 一条检索到的长期记忆。
type MemoryRecord struct {
	ID        string  `json:"id"`
	Content   string  `json:"content"`
	UserID    string  `json:"user_id"`
	SessionID string  `json:"session_id"`
	Score     float64 `json:"score"`
}

// formatMemoriesForContext 将检索到的长期记忆格式化为一条系统消息。
// 格式：每条记忆带时间戳，行首用 "-" 标记。
func formatMemoriesForContext(memories []*MemoryRecord) *schema.Message {
	if len(memories) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("【相关历史记忆】\n")
	for _, m := range memories {
		ts := time.Now().Format("2006-01-02")
		// 合并同一 session 的连续记忆，简化注入
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", ts, m.Content))
	}
	return schema.SystemMessage(sb.String())
}

// defaultLongTerm 进程级默认长期记忆存储。nil 表示未启用。
var defaultLongTerm LongTermMemory

// SetDefaultLongTermMemory main 启动期按 config 设置长期记忆存储。
func SetDefaultLongTermMemory(lt LongTermMemory) {
	if lt != nil {
		defaultLongTerm = lt
	}
}

// GetDefaultLongTermMemory 获取当前长期记忆存储（可能为 nil）。
func GetDefaultLongTermMemory() LongTermMemory {
	return defaultLongTerm
}

// defaultHistoryStore 进程级默认全量历史存储。nil 表示未启用。
var defaultHistoryStore *MySQLHistoryStore

// SetDefaultHistoryStore main 启动期设置全量历史存储。
func SetDefaultHistoryStore(hs *MySQLHistoryStore) {
	if hs != nil {
		defaultHistoryStore = hs
	}
}

// GetDefaultHistoryStore 获取当前全量历史存储（可能为 nil）。
func GetDefaultHistoryStore() *MySQLHistoryStore {
	return defaultHistoryStore
}