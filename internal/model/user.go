package model

import (
	"time"

	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID         string         `gorm:"primaryKey;type:char(26)" json:"id"`
	Username   string         `gorm:"type:varchar(64);not null" json:"username"`
	Password   string         `gorm:"type:varchar(128);not null" json:"-"` // 密码不序列化
	Role       string         `gorm:"type:varchar(32);not null;default:'user'" json:"role"`
	Avatar     string         `gorm:"type:varchar(128)" json:"avatar"`
	CreateTime time.Time      `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime time.Time      `gorm:"autoUpdateTime" json:"update_time"`
	DeletedAt  gorm.DeletedAt `gorm:"column:deleted_at" json:"-"`
}

// TableName 指定表名
func (User) TableName() string {
	return "t_user"
}

// UserRole 用户角色常量
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// BeforeCreate 创建前自动生成 ULID
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		u.ID = ulid.Make().String()
	}
	return
}
