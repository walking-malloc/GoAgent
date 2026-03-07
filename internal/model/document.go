package model

import (
	"time"

	"gorm.io/gorm"
)

// DocumentStatus 文档状态
type DocumentStatus string

const (
	DocumentStatusPending    DocumentStatus = "pending"    // 待处理
	DocumentStatusProcessing DocumentStatus = "processing" // 处理中
	DocumentStatusCompleted  DocumentStatus = "completed"  // 已完成
	DocumentStatusFailed     DocumentStatus = "failed"     // 失败
)

// Document 文档模型
type Document struct {
	ID           string         `gorm:"primaryKey;type:char(26)" json:"id"`
	KBID         string         `gorm:"type:char(26);not null;index" json:"kb_id"`
	Name         string         `gorm:"type:varchar(255);not null" json:"name"`
	FileName     string         `gorm:"type:varchar(255);not null" json:"file_name"`
	FilePath     string         `gorm:"type:varchar(512);not null" json:"file_path"`
	FileType     string         `gorm:"type:varchar(32);not null" json:"file_type"`
	FileSize     int64          `gorm:"not null" json:"file_size"`
	Status       DocumentStatus `gorm:"type:varchar(32);not null;default:'pending';index" json:"status"`
	ChunkCount   int            `gorm:"default:0" json:"chunk_count"`
	ErrorMessage string         `gorm:"type:text" json:"error_message,omitempty"`
	CreatedBy    string         `gorm:"type:varchar(64)" json:"created_by"`
	CreateTime   time.Time      `gorm:"autoCreateTime;index" json:"create_time"`
	UpdateTime   time.Time      `gorm:"autoUpdateTime" json:"update_time"`
	DeletedAt    gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

// TableName 指定表名
func (Document) TableName() string {
	return "t_document"
}
