package service

import (
	"context"
	"fmt"
	"ragent-go/internal/pkg/ai"
	"ragent-go/internal/repository"
	"strings"

	"github.com/oklog/ulid/v2"
)

// ChatService RAG问答服务
type ChatService struct {
	retrievalSvc *RetrievalService //文档检索
	llmSvc       *ai.LLMService
	kbRepo       *repository.KnowledgeBaseRepository //知识库
	memorySvc    *ConversationMemoryService          //会话记忆
}

// NewChatService 创建RAG问答服务
func NewChatService(
	retrievalSvc *RetrievalService,
	llmSvc *ai.LLMService,
	kbRepo *repository.KnowledgeBaseRepository,
	memorySvc *ConversationMemoryService,
) *ChatService {
	return &ChatService{
		retrievalSvc: retrievalSvc,
		llmSvc:       llmSvc,
		kbRepo:       kbRepo,
		memorySvc:    memorySvc,
	}
}

// ChatRequest 问答请求
type ChatRequest struct {
	Question       string `json:"question"`                  // 用户问题
	KBID           string `json:"kb_id"`                     // 知识库ID（可选）
	TopK           int    `json:"top_k"`                     // 检索Top-K个文档片段（默认5）
	ConversationID string `json:"conversation_id,omitempty"` // 会话ID（可选，用于将来精细化记忆管理）
}

// ChatResponse 问答响应
type ChatResponse struct {
	Answer         string   `json:"answer"`          // AI生成的答案
	Contexts       []string `json:"contexts"`        // 检索到的文档片段
	SourceDocs     []string `json:"source_docs"`     // 来源文档ID列表
	ConversationID string   `json:"conversation_id"` // 会话ID（后端生成或透传）
}

// Chat 执行RAG问答
func (s *ChatService) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if req.Question == "" {
		return nil, fmt.Errorf("question is empty")
	}

	// 如果没有会话ID，则后端自动生成一个
	if req.ConversationID == "" {
		req.ConversationID = ulid.Make().String()
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

	// 2. （可选）从会话记忆中检索相关历史片段
	var memoryContexts []string
	if s.memorySvc != nil {
		if memoryChunks, err := s.memorySvc.SearchRelevantMemory(ctx, req.KBID, req.Question, 3); err == nil {
			memoryContexts = make([]string, 0, len(memoryChunks))
			for _, mc := range memoryChunks {
				if mc.Content != "" {
					memoryContexts = append(memoryContexts, mc.Content)
				}
			}
		}
	}

	// 3. 向量检索（知识库文档）
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
			Answer:         "抱歉，我在知识库中没有找到相关信息。",
			Contexts:       memoryContexts,
			SourceDocs:     []string{},
			ConversationID: req.ConversationID,
		}, nil
	}

	// 4. 提取文档片段和来源
	contexts := make([]string, 0, len(memoryContexts)+len(chunks))
	// 先加入会话记忆片段
	contexts = append(contexts, memoryContexts...)

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

	// 5. 构建Prompt并调用LLM
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

	// 6. 将本轮 Q&A 写入会话向量记忆
	if s.memorySvc != nil {
		// 这里 turnIndex 先用 0，占位即可
		_ = s.memorySvc.AddTurn(ctx, req.KBID, req.ConversationID, req.Question, answer, 0)
	}

	return &ChatResponse{
		Answer:         answer,
		Contexts:       contexts,
		SourceDocs:     sourceDocList,
		ConversationID: req.ConversationID,
	}, nil
}
