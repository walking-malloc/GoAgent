package handler

import (
	"net/http"
	"ragent-go/internal/service"

	"github.com/gin-gonic/gin"
)

// ChatHandler 问答处理器
type ChatHandler struct {
	chatService *service.ChatService
}

// NewChatHandler 创建问答处理器
func NewChatHandler(chatService *service.ChatService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
	}
}

// ChatRequest 问答请求
type ChatRequest struct {
	Question string `json:"question" binding:"required" example:"什么是RAG？"` // 用户问题
	KBID     string `json:"kb_id" example:"01KK4BRK0P0HEBSNSBB6MY8QGW"`    // 知识库ID（可选）
	TopK     int    `json:"top_k" example:"5"`                             // 检索Top-K个文档片段（默认5）
}

// ChatResponse 问答响应
type ChatResponse struct {
	Answer     string   `json:"answer" example:"RAG是检索增强生成..."`                     // AI生成的答案
	Contexts   []string `json:"contexts" example:"[\"文档片段1\", \"文档片段2\"]"`          // 检索到的文档片段
	SourceDocs []string `json:"source_docs" example:"[\"doc_id_1\", \"doc_id_2\"]"` // 来源文档ID列表
}

// Chat RAG问答
// @Summary RAG智能问答
// @Description 基于知识库进行智能问答，自动检索相关文档片段并生成答案
// @Tags Chat
// @Accept json
// @Produce json
// @Param request body ChatRequest true "问答请求"
// @Success 200 {object} ChatResponse "问答响应"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /chat [post]
func (h *ChatHandler) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 调用服务
	resp, err := h.chatService.Chat(c.Request.Context(), service.ChatRequest{
		Question: req.Question,
		KBID:     req.KBID,
		TopK:     req.TopK,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ChatResponse{
		Answer:     resp.Answer,
		Contexts:   resp.Contexts,
		SourceDocs: resp.SourceDocs,
	})
}
