package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// TokenBlacklistPrefix Token 黑名单前缀
	TokenBlacklistPrefix = "token:blacklist:"
	// TokenBlacklistTTL Token 黑名单过期时间（7天，与 JWT 过期时间一致）
	TokenBlacklistTTL = 7 * 24 * time.Hour
)

// TokenBlacklist Token 黑名单管理
type TokenBlacklist struct {
	client *redis.Client
}

// NewTokenBlacklist 创建 Token 黑名单管理器
func NewTokenBlacklist(client *redis.Client) *TokenBlacklist {
	return &TokenBlacklist{
		client: client,
	}
}

// Add 添加 Token 到黑名单
func (t *TokenBlacklist) Add(ctx context.Context, token string) error {
	key := TokenBlacklistPrefix + token
	return t.client.Set(ctx, key, "1", TokenBlacklistTTL).Err()
}

// IsBlacklisted 检查 Token 是否在黑名单中
func (t *TokenBlacklist) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	key := TokenBlacklistPrefix + token
	exists, err := t.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// Remove 从黑名单中移除 Token（可选，通常不需要）
func (t *TokenBlacklist) Remove(ctx context.Context, token string) error {
	key := TokenBlacklistPrefix + token
	return t.client.Del(ctx, key).Err()
}
