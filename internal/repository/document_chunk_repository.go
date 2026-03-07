package repository

import (
	"ragent-go/internal/model"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
)

type DocumentChunkRepository struct {
	db *gorm.DB
}

func NewDocumentChunkRepository(db *gorm.DB) *DocumentChunkRepository {
	return &DocumentChunkRepository{db: db}
}

// Create 创建文档分块
func (r *DocumentChunkRepository) Create(chunk *model.DocumentChunk) error {
	return r.db.Create(chunk).Error
}

// CreateBatch 批量创建文档分块
func (r *DocumentChunkRepository) CreateBatch(chunks []*model.DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// 清理和验证文本内容，确保是有效的UTF-8
	for i, chunk := range chunks {
		cleaned := sanitizeUTF8(chunk.Content)
		if cleaned != chunk.Content {
			chunks[i].Content = cleaned
		}
	}

	// 使用事务确保字符集设置生效
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 在事务中设置字符集
		if err := tx.Exec("SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci").Error; err != nil {
			return err
		}
		if err := tx.Exec("SET CHARACTER SET utf8mb4").Error; err != nil {
			return err
		}

		// 减小批次大小，避免长时间锁定数据库
		return tx.CreateInBatches(chunks, 50).Error //每批处理50个分块
	})
}

// sanitizeUTF8 清理和验证UTF-8文本，移除无效字符
func sanitizeUTF8(text string) string {
	if text == "" {
		return text
	}

	// 检查是否是有效的UTF-8
	if utf8.ValidString(text) {
		return text
	}

	// 如果不是有效的UTF-8，尝试修复
	var result strings.Builder
	result.Grow(len(text))

	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		if r == utf8.RuneError && size == 1 {
			// 跳过无效的字节
			text = text[1:]
			continue
		}
		result.WriteRune(r)
		text = text[size:]
	}

	return result.String()
}

// FindByDocID 根据文档ID查询分块列表
func (r *DocumentChunkRepository) FindByDocID(docID string) ([]*model.DocumentChunk, error) {
	var chunks []*model.DocumentChunk
	err := r.db.Where("doc_id = ?", docID).Order("chunk_index ASC").Find(&chunks).Error
	return chunks, err
}

// FindPendingVectors 查询待向量化的分块
func (r *DocumentChunkRepository) FindPendingVectors(kbID string, limit int) ([]*model.DocumentChunk, error) {
	var chunks []*model.DocumentChunk
	query := r.db.Where("kb_id = ? AND vector_status = ?", kbID, model.VectorStatusPending)
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Order("create_time ASC").Find(&chunks).Error
	return chunks, err
}

// FindPendingVectorsByDocID 查询指定文档的待向量化分块
func (r *DocumentChunkRepository) FindPendingVectorsByDocID(docID string, limit int) ([]*model.DocumentChunk, error) {
	var chunks []*model.DocumentChunk
	query := r.db.Where("doc_id = ? AND vector_status = ?", docID, model.VectorStatusPending)
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Order("chunk_index ASC").Find(&chunks).Error
	return chunks, err
}

// UpdateVectorStatus 更新向量化状态
func (r *DocumentChunkRepository) UpdateVectorStatus(id string, status model.VectorStatus) error {
	return r.db.Model(&model.DocumentChunk{}).Where("id = ?", id).Update("vector_status", status).Error
}

// UpdateVectorStatusBatch 批量更新向量化状态
func (r *DocumentChunkRepository) UpdateVectorStatusBatch(ids []string, status model.VectorStatus) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Model(&model.DocumentChunk{}).Where("id IN ?", ids).Update("vector_status", status).Error
}

// DeleteByDocID 根据文档ID删除分块（软删除）
func (r *DocumentChunkRepository) DeleteByDocID(docID string) error {
	return r.db.Where("doc_id = ?", docID).Delete(&model.DocumentChunk{}).Error
}

// CountByDocID 统计文档的分块数量
func (r *DocumentChunkRepository) CountByDocID(docID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.DocumentChunk{}).Where("doc_id = ?", docID).Count(&count).Error
	return count, err
}

// CountByVectorStatus 根据向量化状态统计分块数量
func (r *DocumentChunkRepository) CountByVectorStatus(docID string, status model.VectorStatus) (int64, error) {
	var count int64
	err := r.db.Model(&model.DocumentChunk{}).
		Where("doc_id = ? AND vector_status = ?", docID, status).
		Count(&count).Error
	return count, err
}
