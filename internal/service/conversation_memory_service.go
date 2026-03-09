package service

import (
	"context"
	"fmt"
	"sync"

	"ragent-go/internal/pkg/ai"
	"ragent-go/internal/pkg/milvus"

	"github.com/oklog/ulid/v2"
)

// ConversationMemoryService 会话向量记忆服务：
// - 每轮对话结束，把「问+答」作为一条记忆向量写入 Milvus
// - 新问题到来时，可以先从记忆向量库检索相关历史片段
// - 内部带一个简单的 Embedding 缓存，减少重复向量化
type ConversationMemoryService struct {
	embeddingSvc   *ai.EmbeddingService
	vectorMgr      *milvus.VectorManager
	collectionName string

	mu         sync.RWMutex
	embedCache map[string][]float32
}

// NewConversationMemoryService 创建会话记忆服务
// collectionName 通常使用独立的 Collection，例如 "chat_memory"
func NewConversationMemoryService(
	embeddingSvc *ai.EmbeddingService,
	vectorMgr *milvus.VectorManager,
	collectionName string,
) *ConversationMemoryService {
	if embeddingSvc == nil || vectorMgr == nil || collectionName == "" {
		// 依赖不完整时，不启用会话记忆
		return nil
	}

	return &ConversationMemoryService{
		embeddingSvc:   embeddingSvc,
		vectorMgr:      vectorMgr,
		collectionName: collectionName,
		embedCache:     make(map[string][]float32),
	}
}

// AddTurn 在一轮问答完成后，将 Q&A 写入向量库
// - kbID: 当前知识库 ID，可为空
// - conversationID: 会话 ID，可为空（用于将来做更细粒度过滤）
// - turnIndex: 会话内轮次索引，用作 chunk_id 元数据（可选，传 0 也可以）
func (s *ConversationMemoryService) AddTurn(
	ctx context.Context,
	kbID string,
	conversationID string,
	question string,
	answer string,
	turnIndex int64,
) error {
	if s == nil {
		return nil
	}
	if question == "" && answer == "" {
		return nil
	}

	text := fmt.Sprintf("Q: %s\nA: %s", question, answer)

	vec, err := s.getOrEmbed(text)
	if err != nil {
		return fmt.Errorf("failed to embed conversation turn: %w", err)
	}

	id := ulid.Make().String()
	vectors := [][]float32{vec}
	ids := []string{id}

	meta := map[string]interface{}{
		// 这里沿用文档向量的 schema：
		// - chunk_id: 轮次索引
		// - doc_id:   会话 ID（如果有）
		// - kb_id:    知识库 ID（如果有）
		// - content:  Q+A 文本
		"chunk_id": turnIndex,
		"doc_id":   conversationID,
		"kb_id":    kbID,
		"content":  text,
	}

	if err := s.vectorMgr.InsertVectors(ctx, s.collectionName, vectors, ids, []map[string]interface{}{meta}); err != nil {
		return fmt.Errorf("failed to insert conversation memory vector: %w", err)
	}

	return nil
}

// SearchRelevantMemory 检索与当前问题相似的历史对话片段
// - 目前按 kbID 过滤（可选），不强制限定在某个 conversation 内
func (s *ConversationMemoryService) SearchRelevantMemory(
	ctx context.Context,
	kbID string,
	question string,
	topK int,
) ([]milvus.RetrievedChunk, error) {
	if s == nil {
		return nil, nil
	}
	if question == "" {
		return nil, nil
	}

	if topK <= 0 {
		topK = 3
	}

	vec, err := s.getOrEmbed(question)
	if err != nil {
		return nil, fmt.Errorf("failed to embed question for memory search: %w", err)
	}

	chunks, err := s.vectorMgr.SearchVectors(ctx, s.collectionName, vec, topK, kbID)
	if err != nil {
		return nil, fmt.Errorf("failed to search conversation memory: %w", err)
	}

	return chunks, nil
}

// getOrEmbed 简单的 Embedding 缓存，避免对相同文本重复向量化
func (s *ConversationMemoryService) getOrEmbed(text string) ([]float32, error) {
	s.mu.RLock()
	if vec, ok := s.embedCache[text]; ok {
		s.mu.RUnlock()
		return vec, nil
	}
	s.mu.RUnlock()

	vec, err := s.embeddingSvc.EmbedText(text)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.embedCache[text] = vec
	s.mu.Unlock()
	return vec, nil
}
