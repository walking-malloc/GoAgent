package request

// KnowledgeBaseCreateRequest 创建知识库请求
type KnowledgeBaseCreateRequest struct {
	Name           string `json:"name" binding:"required" example:"产品文档库"`           // 知识库名称
	EmbeddingModel string `json:"embedding_model" example:"qwen3-embedding:8b-fp16"` // 嵌入模型标识
}

// KnowledgeBaseUpdateRequest 更新知识库请求
type KnowledgeBaseUpdateRequest struct {
	Name string `json:"name" binding:"required" example:"产品文档库（更新）"` // 知识库名称
}
