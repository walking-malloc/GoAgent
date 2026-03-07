package service

import (
	"context"
	"fmt"
	"ragent-go/internal/pkg/ai"
	"ragent-go/internal/pkg/milvus"
)

// RetrievalService 检索服务
type RetrievalService struct {
	embeddingSvc *ai.EmbeddingService
	vectorMgr    *milvus.VectorManager
}

// NewRetrievalService 创建检索服务
func NewRetrievalService(
	embeddingSvc *ai.EmbeddingService,
	vectorMgr *milvus.VectorManager,
) *RetrievalService {
	return &RetrievalService{
		embeddingSvc: embeddingSvc,
		vectorMgr:    vectorMgr,
	}
}

// RetrieveRequest 检索请求
type RetrieveRequest struct {
	Query          string // 查询文本
	CollectionName string // Collection名称
	TopK           int    // 返回Top-K个结果
	KBID           string // 可选的知识库ID过滤
}

// Retrieve 检索相关文档片段
func (s *RetrievalService) Retrieve(ctx context.Context, req RetrieveRequest) ([]milvus.RetrievedChunk, error) {
	if req.Query == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if req.CollectionName == "" {
		return nil, fmt.Errorf("collection name is empty")
	}
	if req.TopK <= 0 {
		req.TopK = 5 // 默认返回5个结果
	}

	// 1. 将查询文本向量化
	queryVector, err := s.embeddingSvc.EmbedText(req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// 2. 在Milvus中检索相似向量
	chunks, err := s.vectorMgr.SearchVectors(ctx, req.CollectionName, queryVector, req.TopK, req.KBID)
	if err != nil {
		return nil, fmt.Errorf("failed to search vectors: %w", err)
	}

	return chunks, nil
}
