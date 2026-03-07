package handler

import (
	"ragent-go/internal/api/handler/request"
	"ragent-go/internal/api/middleware"
	"ragent-go/internal/api/response"
	"ragent-go/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

type KnowledgeBaseHandler struct {
	kbService *service.KnowledgeBaseService
}

func NewKnowledgeBaseHandler(kbService *service.KnowledgeBaseService) *KnowledgeBaseHandler {
	return &KnowledgeBaseHandler{
		kbService: kbService,
	}
}

// Create 创建知识库
// @Summary 创建知识库
// @Description 创建新知识库，自动创建 Milvus Collection
// @Tags 知识库
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body request.KnowledgeBaseCreateRequest true "创建知识库请求"
// @Success 200 {object} response.Response{data=model.KnowledgeBase} "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Router /knowledge-base [post]
func (h *KnowledgeBaseHandler) Create(c *gin.Context) {
	var req request.KnowledgeBaseCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	// 获取当前用户ID（如果已登录）
	userID := ""
	if middleware.IsAuthenticated(c) {
		userID, _ = middleware.GetUserID(c)
	}

	kb, err := h.kbService.Create(req.Name, req.EmbeddingModel, userID)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, kb)
}

// GetByID 获取知识库详情
// @Summary 获取知识库详情
// @Description 根据ID获取知识库详细信息
// @Tags 知识库
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Success 200 {object} response.Response{data=model.KnowledgeBase} "成功"
// @Failure 404 {object} response.Response "知识库不存在"
// @Router /knowledge-base/{id} [get]
func (h *KnowledgeBaseHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "知识库ID不能为空")
		return
	}

	kb, err := h.kbService.GetByID(id)
	if err != nil {
		response.NotFound(c, "知识库不存在")
		return
	}

	response.Success(c, kb)
}

// PageQuery 分页查询知识库列表
// @Summary 分页查询知识库列表
// @Description 分页查询知识库列表，支持关键词搜索
// @Tags 知识库
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param keyword query string false "搜索关键词"
// @Success 200 {object} response.Response{data=object{list=[]model.KnowledgeBase,total=int,page=int,page_size=int}} "成功"
// @Router /knowledge-base [get]
func (h *KnowledgeBaseHandler) PageQuery(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	keyword := c.Query("keyword")

	kbs, total, err := h.kbService.PageQuery(page, pageSize, keyword)
	if err != nil {
		response.Error(c, 500, "查询知识库列表失败")
		return
	}

	response.Success(c, gin.H{
		"list":      kbs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Update 更新知识库
// @Summary 更新知识库
// @Description 更新知识库名称（只能更新名称）
// @Tags 知识库
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Param request body request.KnowledgeBaseUpdateRequest true "更新知识库请求"
// @Success 200 {object} response.Response "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Router /knowledge-base/{id} [put]
func (h *KnowledgeBaseHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "知识库ID不能为空")
		return
	}

	var req request.KnowledgeBaseUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	// 获取当前用户ID（如果已登录）
	userID := ""
	if middleware.IsAuthenticated(c) {
		userID, _ = middleware.GetUserID(c)
	}

	if err := h.kbService.Update(id, req.Name, userID); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, nil)
}

// Delete 删除知识库
// @Summary 删除知识库
// @Description 删除知识库，同时删除 Milvus Collection
// @Tags 知识库
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "知识库ID"
// @Success 200 {object} response.Response "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Router /knowledge-base/{id} [delete]
func (h *KnowledgeBaseHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "知识库ID不能为空")
		return
	}

	if err := h.kbService.Delete(id); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, nil)
}
