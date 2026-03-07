package database

import (
	"context"
	"fmt"
	"log"
	"ragent-go/internal/config"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

// InitMilvus 初始化 Milvus 连接
func InitMilvus(cfg *config.Config) (client.Client, error) {
	ctx := context.Background()

	// 创建 Milvus 客户端配置
	milvusCfg := client.Config{
		Address: cfg.Milvus.Addr(),
	}

	// 如果有用户名和密码，添加认证
	if cfg.Milvus.Username != "" {
		milvusCfg.Username = cfg.Milvus.Username
		milvusCfg.Password = cfg.Milvus.Password
	}

	// 创建 Milvus 客户端（NewClient 会自动连接）
	milvusClient, err := client.NewClient(ctx, milvusCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Milvus client: %w", err)
	}

	// 测试连接（通过 ListCollections 测试）
	_, err = milvusClient.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Milvus: %w", err)
	}

	log.Println("✅ Milvus connected successfully")
	return milvusClient, nil
}

// CloseMilvus 关闭 Milvus 连接
func CloseMilvus(milvusClient client.Client) error {
	if milvusClient != nil {
		return milvusClient.Close()
	}
	return nil
}
