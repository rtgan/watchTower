package mem

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ConversationWindow MySQL 会话窗口消息表，实现 Store 接口。
// 一行对应一个会话，messages 为 JSON 序列化的 []*schema.Message。
type ConversationWindow struct {
	SessionID string    `gorm:"column:session_id;primaryKey;type:varchar(128)"`
	Messages  string    `gorm:"column:messages;type:json;not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (ConversationWindow) TableName() string { return "conversation_windows" }

// MySQLStore 实现 Store 接口，将滑动窗口消息持久化到 MySQL。
// 无 TTL，消息永久保留，解决 RedisStore 24h 过期问题。
type MySQLStore struct {
	db *gorm.DB
}

// NewMySQLStore 创建 MySQLStore，自动建表并验证连通性。
func NewMySQLStore(dsn string) (*MySQLStore, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("mysql open: %w", err)
	}
	if err := db.AutoMigrate(&ConversationWindow{}); err != nil {
		return nil, fmt.Errorf("mysql migrate conversation_windows: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("mysql get sql.DB: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	return &MySQLStore{db: db}, nil
}

// DB 暴露底层 *gorm.DB，供 MySQLHistoryStore 复用同一连接池。
func (s *MySQLStore) DB() *gorm.DB { return s.db }

// Load 从 MySQL 读取会话消息。不存在返回 (nil, nil)。
func (s *MySQLStore) Load(id string) ([]*Message, error) {
	var row ConversationWindow
	if err := s.db.Where("session_id = ?", id).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("mysql load: %w", err)
	}
	var msgs []*schema.Message
	if err := json.Unmarshal([]byte(row.Messages), &msgs); err != nil {
		return nil, fmt.Errorf("mysql load unmarshal: %w", err)
	}
	return msgs, nil
}

// Store 覆盖写会话消息到 MySQL（upsert 语义）。
func (s *MySQLStore) Store(id string, msgs []*Message) error {
	b, err := json.Marshal(msgs)
	if err != nil {
		return fmt.Errorf("mysql store marshal: %w", err)
	}
	row := &ConversationWindow{
		SessionID: id,
		Messages:  string(b),
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "session_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"messages"}),
	}).Create(row).Error
}