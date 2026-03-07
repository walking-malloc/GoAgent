package repository

import (
	"ragent-go/internal/model"

	"gorm.io/gorm"
)

type KnowledgeBaseRepository struct {
	db *gorm.DB
}

func NewKnowledgeBaseRepository(db *gorm.DB) *KnowledgeBaseRepository {
	return &KnowledgeBaseRepository{db: db}
}

// Create 创建知识库
func (r *KnowledgeBaseRepository) Create(kb *model.KnowledgeBase) error {
	return r.db.Create(kb).Error
}

// FindByID 根据ID查询知识库
func (r *KnowledgeBaseRepository) FindByID(id string) (*model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	err := r.db.Where("id = ?", id).First(&kb).Error
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

// FindByCollectionName 根据Collection名称查询
func (r *KnowledgeBaseRepository) FindByCollectionName(name string) (*model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	err := r.db.Where("collection_name = ?", name).First(&kb).Error
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

// PageQuery 分页查询知识库列表
func (r *KnowledgeBaseRepository) PageQuery(page, pageSize int, keyword string) ([]model.KnowledgeBase, int64, error) {
	var kbs []model.KnowledgeBase
	var total int64

	query := r.db.Model(&model.KnowledgeBase{})

	// 关键词搜索（搜索名称）
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("create_time DESC").Find(&kbs).Error; err != nil {
		return nil, 0, err
	}

	return kbs, total, nil
}

// Update 更新知识库
func (r *KnowledgeBaseRepository) Update(kb *model.KnowledgeBase) error {
	return r.db.Save(kb).Error
}

// Delete 删除知识库（软删除）
func (r *KnowledgeBaseRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.KnowledgeBase{}).Error
}
