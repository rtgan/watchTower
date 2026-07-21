package mem

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// HistoryMessage 全量会话历史消息，逐条存储。
// 与 Store 接口正交：Store 管理滑动窗口，本表记录完整历史审计日志。
type HistoryMessage struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	SessionID string    `gorm:"column:session_id;type:varchar(128);not null;index:idx_session_id"`
	UserID    string    `gorm:"column:user_id;type:varchar(128);not null;default:'';index:idx_user_id"`
	Role      string    `gorm:"column:role;type:varchar(32);not null"`
	Content   string    `gorm:"column:content;type:text;not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;index:idx_created_at"`
}

func (HistoryMessage) TableName() string { return "conversation_messages" }

// MemoryExtraction 追踪 Milvus 中存储的记忆，用于去重和清理。
type MemoryExtraction struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	SessionID    string    `gorm:"column:session_id;type:varchar(128);not null;index:idx_session_id"`
	UserID       string    `gorm:"column:user_id;type:varchar(128);not null;default:'';index:idx_user_id"`
	MemoryText   string    `gorm:"column:memory_text;type:text;not null"`
	MilvusID     string    `gorm:"column:milvus_id;type:varchar(256);not null;index:idx_milvus_id"`
	SourceMsgIDs string    `gorm:"column:source_msg_ids;type:json;null"` // JSON array of message IDs
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (MemoryExtraction) TableName() string { return "memory_extractions" }

// MySQLHistoryStore 追加式全量会话历史存储 + Milvus 记忆追踪。
type MySQLHistoryStore struct {
	db *gorm.DB
}

// NewMySQLHistoryStore 创建并自动建表。复用已有的 *gorm.DB 连接池。
func NewMySQLHistoryStore(db *gorm.DB) *MySQLHistoryStore {
	s := &MySQLHistoryStore{db: db}
	// 自动建表，忽略已存在的错误
	_ = db.AutoMigrate(&HistoryMessage{}, &MemoryExtraction{})
	return s
}

// AppendMessages 批量追加多条消息到历史表。
func (s *MySQLHistoryStore) AppendMessages(sessionID, userID string, msgs []*Message) error {
	rows := make([]*HistoryMessage, 0, len(msgs))
	for _, msg := range msgs {
		rows = append(rows, &HistoryMessage{
			SessionID: sessionID,
			UserID:    userID,
			Role:      string(msg.Role),
			Content:   msg.Content,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return s.db.Create(&rows).Error
}

// GetMessages 获取某会话按时间排序的全部历史消息。
func (s *MySQLHistoryStore) GetMessages(sessionID string) ([]*HistoryMessage, error) {
	var rows []*HistoryMessage
	if err := s.db.Where("session_id = ?", sessionID).Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("mysql history get session: %w", err)
	}
	return rows, nil
}

// GetUserMessages 获取某用户跨会话的全部消息（最多 limit 条）。
func (s *MySQLHistoryStore) GetUserMessages(userID string, limit int) ([]*HistoryMessage, error) {
	var rows []*HistoryMessage
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("mysql history get user: %w", err)
	}
	return rows, nil
}

// RecordExtraction 记录一条从 Milvus 提取的记忆，用于追踪。
func (s *MySQLHistoryStore) RecordExtraction(sessionID, userID, memoryText, milvusID string, sourceMsgIDs []int64) error {
	idsJSON := ""
	if len(sourceMsgIDs) > 0 {
		b, _ := json.Marshal(sourceMsgIDs)
		idsJSON = string(b)
	}
	return s.db.Create(&MemoryExtraction{
		SessionID:    sessionID,
		UserID:       userID,
		MemoryText:   memoryText,
		MilvusID:     milvusID,
		SourceMsgIDs: idsJSON,
	}).Error
}