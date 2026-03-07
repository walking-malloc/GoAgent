package handler

import (
	"fmt"
	"ragent-go/internal/api/middleware"
	"ragent-go/internal/api/response"
	"ragent-go/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

type DocumentHandler struct {
	docService *service.DocumentService
}

func NewDocumentHandler(docService *service.DocumentService) *DocumentHandler {
	return &DocumentHandler{
		docService: docService,
	}
}

// UploadDocument 上传文档
// @Summary 上传文档
// @Description 上传文档到知识库，自动进行解析、分块、向量化
// @Tags 文档
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param kb_id formData string true "知识库ID"
// @Param file formData file true "文档文件"
// @Success 200 {object} response.Response{data=model.Document} "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Failure 500 {object} response.Response "上传失败"
// @Router /documents [post]
func (h *DocumentHandler) UploadDocument(c *gin.Context) {
	// 获取知识库ID
	kbID := c.PostForm("kb_id")
	if kbID == "" {
		response.Error(c, 400, "知识库ID不能为空")
		return
	}

	// 获取文件
	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, 400, fmt.Sprintf("获取文件失败: %v", err))
		return
	}

	// 读取文件内容
	src, err := file.Open()
	if err != nil {
		response.Error(c, 500, fmt.Sprintf("打开文件失败: %v", err))
		return
	}
	defer src.Close()

	fileData := make([]byte, file.Size)
	if _, err := src.Read(fileData); err != nil {
		response.Error(c, 500, fmt.Sprintf("读取文件失败: %v", err))
		return
	}

	// 获取文件类型
	fileType := h.docService.GetFileType(file.Filename)

	// 保存文件
	filePath, err := h.docService.SaveUploadedFile(fileData, file.Filename)
	if err != nil {
		response.Error(c, 500, fmt.Sprintf("保存文件失败: %v", err))
		return
	}

	// 获取当前用户
	createdBy := ""
	if middleware.IsAuthenticated(c) {
		createdBy, _ = middleware.GetUsername(c)
	}

	// 创建文档记录并开始处理
	doc, err := h.docService.UploadDocument(
		kbID,
		file.Filename,
		filePath,
		fileType,
		file.Size,
		createdBy,
	)
	if err != nil {
		response.Error(c, 500, fmt.Sprintf("上传文档失败: %v", err))
		return
	}

	response.Success(c, doc)
}

// GetDocumentByID 获取文档详情
// @Summary 获取文档详情
// @Description 根据ID获取文档详细信息
// @Tags 文档
// @Produce json
// @Security BearerAuth
// @Param id path string true "文档ID"
// @Success 200 {object} response.Response{data=model.Document} "成功"
// @Failure 404 {object} response.Response "文档不存在"
// @Router /documents/{id} [get]
func (h *DocumentHandler) GetDocumentByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Error(c, 400, "文档ID不能为空")
		return
	}

	doc, err := h.docService.GetDocumentByID(id)
	if err != nil {
		response.Error(c, 404, "文档不存在")
		return
	}

	response.Success(c, doc)
}

// GetDocumentProgress 获取文档处理进度
// @Summary 获取文档处理进度
// @Description 获取文档的处理进度信息（状态、分块数量、向量化进度等）
// @Tags 文档
// @Produce json
// @Security BearerAuth
// @Param id path string true "文档ID"
// @Success 200 {object} response.Response{data=map[string]interface{}} "成功"
// @Failure 404 {object} response.Response "文档不存在"
// @Router /documents/{id}/progress [get]
func (h *DocumentHandler) GetDocumentProgress(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Error(c, 400, "文档ID不能为空")
		return
	}

	progress, err := h.docService.GetDocumentProgress(id)
	if err != nil {
		response.Error(c, 404, "文档不存在")
		return
	}

	response.Success(c, progress)
}

// ListDocuments 分页查询文档列表
// @Summary 分页查询文档列表
// @Description 根据知识库ID分页查询文档列表
// @Tags 文档
// @Produce json
// @Security BearerAuth
// @Param kb_id query string true "知识库ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} response.Response{data=map[string]interface{}} "成功"
// @Router /documents [get]
func (h *DocumentHandler) ListDocuments(c *gin.Context) {
	kbID := c.Query("kb_id")
	if kbID == "" {
		response.Error(c, 400, "知识库ID不能为空")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	docs, total, err := h.docService.ListDocuments(kbID, page, pageSize)
	if err != nil {
		response.Error(c, 500, fmt.Sprintf("查询文档列表失败: %v", err))
		return
	}

	response.Success(c, gin.H{
		"list":      docs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// DeleteDocument 删除文档
// @Summary 删除文档
// @Description 删除文档及其所有分块和向量数据
// @Tags 文档
// @Produce json
// @Security BearerAuth
// @Param id path string true "文档ID"
// @Success 200 {object} response.Response "成功"
// @Failure 404 {object} response.Response "文档不存在"
// @Router /documents/{id} [delete]
func (h *DocumentHandler) DeleteDocument(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Error(c, 400, "文档ID不能为空")
		return
	}

	if err := h.docService.DeleteDocument(id); err != nil {
		response.Error(c, 500, fmt.Sprintf("删除文档失败: %v", err))
		return
	}

	response.Success(c, nil)
}
