package model

import (
	"time"

	"gorm.io/gorm"
)

// VectorStatus 向量化状态
type VectorStatus string

const (
	VectorStatusPending   VectorStatus = "pending"   // 待向量化
	VectorStatusCompleted VectorStatus = "completed" // 已完成
	VectorStatusFailed    VectorStatus = "failed"    // 失败
)

// DocumentChunk 文档分块模型
type DocumentChunk struct {
	ID            string         `gorm:"primaryKey;type:char(26)" json:"id"`
	DocID         string         `gorm:"type:char(26);not null;index" json:"doc_id"`
	KBID          string         `gorm:"type:char(26);not null;index" json:"kb_id"`
	ChunkIndex    int            `gorm:"not null" json:"chunk_index"`
	Content       string         `gorm:"type:text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;not null" json:"content"`
	ContentLength int            `gorm:"not null" json:"content_length"`
	VectorStatus  VectorStatus   `gorm:"type:varchar(32);not null;default:'pending';index" json:"vector_status"`
	CreateTime    time.Time      `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime    time.Time      `gorm:"autoUpdateTime" json:"update_time"`
	DeletedAt     gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

// TableName 指定表名
func (DocumentChunk) TableName() string {
	return "t_document_chunk"
}
