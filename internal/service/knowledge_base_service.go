package service

import (
	"context"
	"errors"
	"fmt"
	"ragent-go/internal/model"
	"ragent-go/internal/pkg/milvus"
	"ragent-go/internal/repository"

	"github.com/oklog/ulid/v2"
)

type KnowledgeBaseService struct {
	repo          *repository.KnowledgeBaseRepository
	collectionMgr milvus.CollectionManager
	defaultDim    int // 默认向量维度
}

func NewKnowledgeBaseService(
	repo *repository.KnowledgeBaseRepository,
	collectionMgr milvus.CollectionManager,
	defaultDim int,
) *KnowledgeBaseService {
	return &KnowledgeBaseService{
		repo:          repo,
		collectionMgr: collectionMgr,
		defaultDim:    defaultDim,
	}
}

// Create 创建知识库
func (s *KnowledgeBaseService) Create(name, embeddingModel, createdBy string) (*model.KnowledgeBase, error) {
	// 参数验证
	if name == "" {
		return nil, errors.New("知识库名称不能为空")
	}

	// 生成 ULID
	id := ulid.Make().String()

	// 生成唯一的 Collection 名称（格式：kb_{ULID}）
	collectionName := fmt.Sprintf("kb_%s", id)

	// 检查 Collection 名称是否已存在（理论上不会，因为 ULID 唯一）
	exists, err := s.collectionMgr.CollectionExists(context.Background(), collectionName)
	if err != nil {
		return nil, fmt.Errorf("检查 Collection 是否存在失败: %w", err)
	}
	if exists {
		return nil, errors.New("Collection 名称已存在")
	}

	// 确定向量维度（根据 embedding 模型，这里简化处理，使用默认值）
	dim := s.defaultDim
	if dim == 0 {
		dim = 1024 // 默认维度
	}

	// 创建 Milvus Collection
	if err := s.collectionMgr.CreateCollection(context.Background(), collectionName, dim); err != nil {
		return nil, fmt.Errorf("创建 Milvus Collection 失败: %w", err)
	}

	// 创建知识库记录
	kb := &model.KnowledgeBase{
		ID:             id,
		Name:           name,
		EmbeddingModel: embeddingModel,
		CollectionName: collectionName,
		CreatedBy:      createdBy,
	}

	if err := s.repo.Create(kb); err != nil {
		// 如果数据库创建失败，尝试删除已创建的 Collection（回滚）
		_ = s.collectionMgr.DeleteCollection(context.Background(), collectionName)
		return nil, fmt.Errorf("创建知识库失败: %w", err)
	}

	return kb, nil
}

// GetByID 获取知识库详情
func (s *KnowledgeBaseService) GetByID(id string) (*model.KnowledgeBase, error) {
	return s.repo.FindByID(id)
}

// PageQuery 分页查询知识库列表
func (s *KnowledgeBaseService) PageQuery(page, pageSize int, keyword string) ([]model.KnowledgeBase, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repo.PageQuery(page, pageSize, keyword)
}

// Update 更新知识库（只能更新名称）
func (s *KnowledgeBaseService) Update(id, name, updatedBy string) error {
	kb, err := s.repo.FindByID(id)
	if err != nil {
		return errors.New("知识库不存在")
	}

	if name == "" {
		return errors.New("知识库名称不能为空")
	}

	kb.Name = name
	kb.UpdatedBy = updatedBy

	return s.repo.Update(kb)
}

// Delete 删除知识库
func (s *KnowledgeBaseService) Delete(id string) error {
	kb, err := s.repo.FindByID(id)
	if err != nil {
		return errors.New("知识库不存在")
	}

	// TODO: 检查是否有文档（后续实现文档模块时添加）
	// if hasDocuments, err := s.checkHasDocuments(id); err != nil {
	//     return err
	// } else if hasDocuments {
	//     return errors.New("知识库存在文档，无法删除")
	// }

	// 删除 Milvus Collection
	if err := s.collectionMgr.DeleteCollection(context.Background(), kb.CollectionName); err != nil {
		// 记录错误但不阻止删除（Collection 可能已经被删除）
		// log.Printf("删除 Milvus Collection 失败: %v", err)
	}

	// 软删除数据库记录
	return s.repo.Delete(id)
}
