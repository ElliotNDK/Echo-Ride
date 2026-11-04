package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Blacklist interface {
	Add(ctx context.Context, jti string, ttl time.Duration) error
	Exists(ctx context.Context, jti string) (bool, error)
}

type RedisBlacklist struct {
	client *redis.Client
	prefix string
}

func NewRedisClient(addr, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func NewBlacklist(client *redis.Client) *RedisBlacklist {
	return &RedisBlacklist{client: client, prefix: "jti:blacklist:"}
}

func (b *RedisBlacklist) key(jti string) string {
	return b.prefix + jti
}

func (b *RedisBlacklist) Add(ctx context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	if err := b.client.Set(ctx, b.key(jti), "1", ttl).Err(); err != nil {
		return fmt.Errorf("failed to add jti to blacklist: %w", err)
	}
	return nil
}

func (b *RedisBlacklist) Exists(ctx context.Context, jti string) (bool, error) {
	res, err := b.client.Exists(ctx, b.key(jti)).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check blacklist: %w", err)
	}
	return res > 0, nil
}
