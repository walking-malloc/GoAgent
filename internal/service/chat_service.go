package service

import (
	"context"
	"fmt"
	"ragent-go/internal/pkg/ai"
	"ragent-go/internal/repository"
	"strings"
)

// ChatService RAG问答服务
type ChatService struct {
	retrievalSvc *RetrievalService
	llmSvc       *ai.LLMService
	kbRepo       *repository.KnowledgeBaseRepository
}

// NewChatService 创建RAG问答服务
func NewChatService(
	retrievalSvc *RetrievalService,
	llmSvc *ai.LLMService,
	kbRepo *repository.KnowledgeBaseRepository,
) *ChatService {
	return &ChatService{
		retrievalSvc: retrievalSvc,
		llmSvc:       llmSvc,
		kbRepo:       kbRepo,
	}
}

// ChatRequest 问答请求
type ChatRequest struct {
	Question string `json:"question"` // 用户问题
	KBID     string `json:"kb_id"`    // 知识库ID（可选）
	TopK     int    `json:"top_k"`    // 检索Top-K个文档片段（默认5）
}

// ChatResponse 问答响应
type ChatResponse struct {
	Answer     string   `json:"answer"`      // AI生成的答案
	Contexts   []string `json:"contexts"`    // 检索到的文档片段
	SourceDocs []string `json:"source_docs"` // 来源文档ID列表
}

// Chat 执行RAG问答
func (s *ChatService) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if req.Question == "" {
		return nil, fmt.Errorf("question is empty")
	}

	// 默认TopK
	if req.TopK <= 0 {
		req.TopK = 5
	}

	// 1. 获取知识库信息（确定Collection名称）
	var collectionName string
	if req.KBID != "" {
		kb, err := s.kbRepo.FindByID(req.KBID)
		if err != nil {
			return nil, fmt.Errorf("knowledge base not found: %w", err)
		}
		collectionName = kb.CollectionName
	} else {
		// 如果没有指定知识库，需要从配置或默认知识库获取
		return nil, fmt.Errorf("kb_id is required")
	}

	// 2. 向量检索
	retrieveReq := RetrieveRequest{
		Query:          req.Question,
		CollectionName: collectionName,
		TopK:           req.TopK,
		KBID:           req.KBID,
	}

	chunks, err := s.retrievalSvc.Retrieve(ctx, retrieveReq)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve: %w", err)
	}

	if len(chunks) == 0 {
		return &ChatResponse{
			Answer:     "抱歉，我在知识库中没有找到相关信息。",
			Contexts:   []string{},
			SourceDocs: []string{},
		}, nil
	}

	// 3. 提取文档片段和来源
	contexts := make([]string, 0, len(chunks))
	sourceDocs := make(map[string]bool)

	for _, chunk := range chunks {
		contexts = append(contexts, chunk.Content)
		if chunk.DocID != "" {
			sourceDocs[chunk.DocID] = true //去重
		}
	}

	// 转换为列表
	sourceDocList := make([]string, 0, len(sourceDocs))
	for docID := range sourceDocs {
		sourceDocList = append(sourceDocList, docID)
	}

	// 4. 构建Prompt并调用LLM
	messages := ai.BuildRAGPrompt(req.Question, contexts)
	llmReq := ai.ChatRequest{
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	answer, err := s.llmSvc.Chat(llmReq)
	if err != nil {
		return nil, fmt.Errorf("failed to generate answer: %w", err)
	}

	// 清理答案（移除多余的空白）
	answer = strings.TrimSpace(answer)

	return &ChatResponse{
		Answer:     answer,
		Contexts:   contexts,
		SourceDocs: sourceDocList,
	}, nil
}
