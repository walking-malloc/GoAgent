package service

import (
	"context"
	"fmt"
	"ragent-go/internal/pkg/ai"
	"ragent-go/internal/repository"
	"strings"
	"sync"

	"github.com/oklog/ulid/v2"
)

// ChatService RAG问答服务
type ChatService struct {
	retrievalSvc *RetrievalService //文档检索
	llmSvc       *ai.LLMService
	kbRepo       *repository.KnowledgeBaseRepository //知识库
	memorySvc    *ConversationMemoryService          //会话记忆
	intentSvc    *IntentService                      //意图识别（自动选择知识库）
	rewriteSvc   *QueryRewriteService                //问题重写（基于最近几轮对话）

	historyMu sync.RWMutex
	// 每个会话最近的问答记录（按时间顺序），元素示例："Q: xxx\nA: yyy"
	conversationHistory map[string][]string
}

// NewChatService 创建RAG问答服务
func NewChatService(
	retrievalSvc *RetrievalService,
	llmSvc *ai.LLMService,
	kbRepo *repository.KnowledgeBaseRepository,
	memorySvc *ConversationMemoryService,
	intentSvc *IntentService,
	rewriteSvc *QueryRewriteService,
) *ChatService {
	return &ChatService{
		retrievalSvc:        retrievalSvc,
		llmSvc:              llmSvc,
		kbRepo:              kbRepo,
		memorySvc:           memorySvc,
		intentSvc:           intentSvc,
		rewriteSvc:          rewriteSvc,
		conversationHistory: make(map[string][]string),
	}
}

// ChatRequest 问答请求
type ChatRequest struct {
	Question       string `json:"question"`                  // 用户问题
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

	// 预设一个最终用于检索/意图识别的问题，默认使用原始问题
	finalQuestion := strings.TrimSpace(req.Question)

	// 0. 基于“当前会话最近几轮原始 Q+A”进行问题重写（可选，不再使用向量相似度）
	if s.rewriteSvc != nil {
		recentQAPairs := s.getRecentHistory(req.ConversationID, 5)
		if len(recentQAPairs) > 0 {
			if rw, err := s.rewriteSvc.RewriteQuestion(recentQAPairs, req.Question); err == nil {
				if q := strings.TrimSpace(rw.RewrittenQuestion); q != "" {
					finalQuestion = q
				}
				// SubQuestions 目前先不展开多路检索，后续需要时可以在这里扩展
			}
		}
	}

	// 1. 获取知识库信息（确定Collection名称）
	var collectionName string

	// 如果没有指定知识库，尝试通过意图识别自动选择知识库
	if s.intentSvc == nil {
		return nil, fmt.Errorf("kb_id is required and intent service is not available")
	}

	intents, err := s.intentSvc.Classify(finalQuestion, 1, 0.4)
	if err != nil {
		return nil, fmt.Errorf("failed to classify intent: %w", err)
	}
	if len(intents) == 0 || intents[0].KBID == "" {
		return nil, fmt.Errorf("no suitable knowledge base found by intent, please specify kb_id")
	}

	kb, err := s.kbRepo.FindByID(intents[0].KBID)
	if err != nil {
		return nil, fmt.Errorf("knowledge base not found (from intent): %w", err)
	}
	collectionName = kb.CollectionName

	// 2. （可选）从会话记忆中检索相关历史片段
	var memoryContexts []string
	if s.memorySvc != nil {
		if memoryChunks, err := s.memorySvc.SearchRelevantMemory(ctx, kb.ID, finalQuestion, 3); err == nil {
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
		Query:          finalQuestion,
		CollectionName: collectionName,
		TopK:           req.TopK,
		KBID:           kb.ID,
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

	// 5. 构建Prompt并调用LLM（这里仍然把原始问题交给模型，以便回答贴近用户问法）
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
		_ = s.memorySvc.AddTurn(ctx, kb.ID, req.ConversationID, req.Question, answer, 0)
	}
	// 同时在进程内维护一份简易的按会话有序历史，用于后续问题重写
	s.appendHistory(req.ConversationID, req.Question, answer)

	return &ChatResponse{
		Answer:         answer,
		Contexts:       contexts,
		SourceDocs:     sourceDocList,
		ConversationID: req.ConversationID,
	}, nil
}

// getRecentHistory 返回指定会话最近 limit 条 Q+A 文本（按时间顺序）
func (s *ChatService) getRecentHistory(conversationID string, limit int) []string {
	if conversationID == "" || limit <= 0 {
		return nil
	}
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()
	history := s.conversationHistory[conversationID]
	if len(history) == 0 {
		return nil
	}
	if len(history) <= limit {
		// 返回一个拷贝，避免外部修改内部切片
		cp := make([]string, len(history))
		copy(cp, history)
		return cp
	}
	cp := make([]string, limit)
	copy(cp, history[len(history)-limit:])
	return cp
}

// appendHistory 将一轮新的 Q+A 追加到指定会话历史中
func (s *ChatService) appendHistory(conversationID, question, answer string) {
	if conversationID == "" {
		return
	}
	text := fmt.Sprintf("Q: %s\nA: %s", strings.TrimSpace(question), strings.TrimSpace(answer))
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	s.conversationHistory[conversationID] = append(s.conversationHistory[conversationID], text)
}
