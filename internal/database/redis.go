package database

import (
	"context"
	"fmt"
	"log"

	"ragent-go/internal/config"

	"github.com/redis/go-redis/v9"
)

// InitRedis 初始化 Redis 连接
func InitRedis(cfg *config.Config) (*redis.Client, error) {
	redisCfg := cfg.Redis

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisCfg.Addr(),
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
		PoolSize: redisCfg.PoolSize,
	})

	// 测试连接
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Println("✅ Redis connected successfully")
	return redisClient, nil
}

// CloseRedis 关闭 Redis 连接
func CloseRedis(client *redis.Client) error {
	if client != nil {
		return client.Close()
	}
	return nil
}
