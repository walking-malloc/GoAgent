package milvus

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// CollectionManager Milvus Collection 管理器接口
type CollectionManager interface {
	// CreateCollection 创建 Milvus Collection
	CreateCollection(ctx context.Context, name string, dim int) error
	// DeleteCollection 删除 Milvus Collection
	DeleteCollection(ctx context.Context, name string) error
	// CollectionExists 检查 Collection 是否存在
	CollectionExists(ctx context.Context, name string) (bool, error)
	// ListCollections 列出所有 Collection
	ListCollections(ctx context.Context) ([]string, error)
}

// RealCollectionManager 真实的 Milvus Collection 管理器实现
type RealCollectionManager struct {
	client client.Client
}

// NewRealCollectionManager 创建真实的 Collection 管理器
func NewRealCollectionManager(milvusClient client.Client) *RealCollectionManager {
	return &RealCollectionManager{
		client: milvusClient,
	}
}

// CreateCollection 创建 Milvus Collection
func (m *RealCollectionManager) CreateCollection(ctx context.Context, name string, dim int) error {
	// 检查 Collection 是否已存在
	exists, err := m.CollectionExists(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to check collection existence: %w", err)
	}
	if exists {
		return fmt.Errorf("collection %s already exists", name)
	}

	// 定义 Schema
	schema := &entity.Schema{
		CollectionName: name,
		Description:    fmt.Sprintf("Knowledge base collection: %s", name),
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": "64"},
				PrimaryKey: true,
				AutoID:     false,
			},
			{
				Name:     "vector",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", dim),
				},
			},
			{
				Name:     "chunk_id",
				DataType: entity.FieldTypeInt64,
			},
			{
				Name:       "doc_id",
				DataType:   entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": "64"},
			},
			{
				Name:       "kb_id",
				DataType:   entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": "64"},
			},
			{
				Name:       "content",
				DataType:   entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": "65535"},
			},
		},
	}

	// 创建 Collection
	err = m.client.CreateCollection(ctx, schema, entity.DefaultShardNumber)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	// 创建索引（使用 AUTOINDEX，Milvus 会自动选择索引类型）
	idx, err := entity.NewIndexAUTOINDEX(entity.L2)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	err = m.client.CreateIndex(ctx, name, "vector", idx, false)
	if err != nil {
		// 如果索引创建失败，尝试删除 Collection
		_ = m.client.DropCollection(ctx, name)
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

// DeleteCollection 删除 Milvus Collection
func (m *RealCollectionManager) DeleteCollection(ctx context.Context, name string) error {
	return m.client.DropCollection(ctx, name)
}

// CollectionExists 检查 Collection 是否存在
func (m *RealCollectionManager) CollectionExists(ctx context.Context, name string) (bool, error) {
	return m.client.HasCollection(ctx, name)
}

// ListCollections 列出所有 Collection
func (m *RealCollectionManager) ListCollections(ctx context.Context) ([]string, error) {
	collections, err := m.client.ListCollections(ctx)
	if err != nil {
		return nil, err
	}

	// 转换为字符串数组
	result := make([]string, len(collections))
	for i, col := range collections {
		result[i] = col.Name
	}
	return result, nil
}
