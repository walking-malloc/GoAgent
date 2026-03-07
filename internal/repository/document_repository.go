package repository

import (
	"ragent-go/internal/model"

	"gorm.io/gorm"
)

type DocumentRepository struct {
	db *gorm.DB
}

func NewDocumentRepository(db *gorm.DB) *DocumentRepository {
	return &DocumentRepository{db: db}
}

// Create 创建文档
func (r *DocumentRepository) Create(doc *model.Document) error {
	return r.db.Create(doc).Error
}

// FindByID 根据ID查询文档
func (r *DocumentRepository) FindByID(id string) (*model.Document, error) {
	var doc model.Document
	err := r.db.Where("id = ?", id).First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// FindByKBID 根据知识库ID查询文档列表
func (r *DocumentRepository) FindByKBID(kbID string, page, pageSize int) ([]*model.Document, int64, error) {
	var docs []*model.Document
	var total int64

	query := r.db.Model(&model.Document{}).Where("kb_id = ?", kbID)

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Order("create_time DESC").Offset(offset).Limit(pageSize).Find(&docs).Error; err != nil {
		return nil, 0, err
	}

	return docs, total, nil
}

// Update 更新文档
func (r *DocumentRepository) Update(doc *model.Document) error {
	return r.db.Save(doc).Error
}

// UpdateStatus 更新文档状态
func (r *DocumentRepository) UpdateStatus(id string, status model.DocumentStatus, errorMsg string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}
	return r.db.Model(&model.Document{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateChunkCount 更新分块数量
func (r *DocumentRepository) UpdateChunkCount(id string, count int) error {
	return r.db.Model(&model.Document{}).Where("id = ?", id).Update("chunk_count", count).Error
}

// Delete 删除文档（软删除）
func (r *DocumentRepository) Delete(id string) error {
	return r.db.Delete(&model.Document{}, "id = ?", id).Error
}
