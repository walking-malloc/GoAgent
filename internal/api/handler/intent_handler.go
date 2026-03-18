package handler

import (
	"net/http"
	"ragent-go/internal/service"

	"github.com/gin-gonic/gin"
)

// IntentHandler 意图识别相关接口
type IntentHandler struct {
	intentSvc *service.IntentService
}

// NewIntentHandler 创建 IntentHandler
func NewIntentHandler(intentSvc *service.IntentService) *IntentHandler {
	return &IntentHandler{intentSvc: intentSvc}
}

// IntentClassifyRequest 意图识别请求
type IntentClassifyRequest struct {
	Question string  `json:"question" binding:"required" example:"报销流程怎么走？"` // 用户问题
	TopK     int     `json:"top_k" example:"5"`                              // 返回前几个意图
	MinScore float64 `json:"min_score" example:"0.4"`                        // 最小相似度分数
}

// IntentClassifyResponse 意图识别响应
type IntentClassifyResponse struct {
	Intents []service.IntentScore `json:"intents"` // 识别到的意图列表
}

// ClassifyIntent 意图识别接口
// @Summary 意图识别
// @Description 使用 LLM 对用户问题进行意图识别，返回按分数排序的意图列表
// @Tags Intent
// @Accept json
// @Produce json
// @Param request body IntentClassifyRequest true "意图识别请求"
// @Success 200 {object} IntentClassifyResponse "意图识别结果"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器错误"
// @Router /intent/classify [post]
func (h *IntentHandler) ClassifyIntent(c *gin.Context) {
	if h.intentSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "intent service not initialized"})
		return
	}

	var req IntentClassifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	intents, err := h.intentSvc.Classify(req.Question, req.TopK, req.MinScore)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, IntentClassifyResponse{
		Intents: intents,
	})
}
