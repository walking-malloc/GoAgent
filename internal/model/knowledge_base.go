package model

import (
	"time"

	"gorm.io/gorm"
)

// KnowledgeBase 知识库模型
type KnowledgeBase struct {
	ID             string         `gorm:"primaryKey;type:char(26)" json:"id"`
	Name           string         `gorm:"type:varchar(128);not null" json:"name"`
	EmbeddingModel string         `gorm:"type:varchar(64)" json:"embedding_model"`
	CollectionName string         `gorm:"type:varchar(128);uniqueIndex;not null" json:"collection_name"`
	CreatedBy      string         `gorm:"type:varchar(64)" json:"created_by"`
	UpdatedBy      string         `gorm:"type:varchar(64)" json:"updated_by"`
	CreateTime     time.Time      `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime     time.Time      `gorm:"autoUpdateTime" json:"update_time"`
	DeletedAt      gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

// TableName 指定表名
func (KnowledgeBase) TableName() string {
	return "t_knowledge_base"
}
