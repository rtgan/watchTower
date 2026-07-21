package mem

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

// RedisStore 基于 Redis 的会话记忆存储，解决 InMemoryStore 在分布式部署下的一致性问题。
//
// 用法：config.memory.driver="redis" 时，main 启动期调用 mem.NewRedisStore 并 SetDefaultStore。
// 单进程内仍用 mem 的 per-conversation 锁串行化；跨进程的并发一致性需上层加分布式锁（本实现不提供，
// 大多数会话路由到同一实例或容忍最终一致即可）。
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisStore 创建并 ping Redis。db 为 Redis 逻辑库序号。
func NewRedisStore(addr, password string, db int) (*RedisStore, error) {
	cli := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := cli.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisStore{client: cli, ttl: 24 * time.Hour}, nil
}

func redisKey(id string) string { return "watchtower:mem:" + id }

func (s *RedisStore) Load(id string) ([]*Message, error) {
	b, err := s.client.Get(context.Background(), redisKey(id)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var msgs []*schema.Message
	if err := json.Unmarshal(b, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func (s *RedisStore) Store(id string, msgs []*Message) error {
	b, err := json.Marshal(msgs)
	if err != nil {
		return err
	}
	return s.client.Set(context.Background(), redisKey(id), b, s.ttl).Err()
}
