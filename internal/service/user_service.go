package service

import (
	"errors"
	"ragent-go/internal/model"
	"ragent-go/internal/repository"
)

type UserService struct {
	userRepo *repository.UserRepository
}

func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{
		userRepo: userRepo,
	}
}

// GetCurrentUser 获取当前用户信息
func (s *UserService) GetCurrentUser(userID string) (*model.User, error) {
	return s.userRepo.FindByID(userID)
}

// PageQuery 分页查询用户
func (s *UserService) PageQuery(page, pageSize int, keyword string) ([]model.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.userRepo.PageQuery(page, pageSize, keyword)
}

// Create 创建用户
func (s *UserService) Create(username, password, role, avatar string) (string, error) {
	// 验证必填字段
	if username == "" || password == "" {
		return "", errors.New("用户名和密码不能为空")
	}

	// 不允许创建默认管理员
	if username == "admin" {
		return "", errors.New("默认管理员用户名不可用")
	}

	// 检查用户名是否已存在
	existing, _ := s.userRepo.FindByUsername(username)
	if existing != nil {
		return "", errors.New("用户名已存在")
	}

	// 验证角色
	if role != model.RoleAdmin && role != model.RoleUser {
		role = model.RoleUser
	}

	user := &model.User{
		Username: username,
		Password: password, // 原项目是明文密码，这里保持一致
		Role:     role,
		Avatar:   avatar,
	}

	if err := s.userRepo.Create(user); err != nil {
		return "", errors.New("创建用户失败")
	}

	return user.ID, nil
}

// Update 更新用户
func (s *UserService) Update(userID, username, role, avatar string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	// 不允许修改默认管理员
	if user.Username == "admin" {
		return errors.New("默认管理员不允许修改")
	}

	if username != "" {
		// 不允许使用默认管理员用户名
		if username == "admin" {
			return errors.New("默认管理员用户名不可用")
		}
		// 检查用户名是否被其他用户使用
		existing, _ := s.userRepo.FindByUsername(username)
		if existing != nil && existing.ID != userID {
			return errors.New("用户名已被使用")
		}
		user.Username = username
	}

	if role != "" {
		if role != model.RoleAdmin && role != model.RoleUser {
			return errors.New("无效的角色")
		}
		user.Role = role
	}

	if avatar != "" {
		user.Avatar = avatar
	}

	return s.userRepo.Update(user)
}

// ChangePassword 修改当前用户密码
func (s *UserService) ChangePassword(userID, currentPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	// 验证当前密码
	if user.Password != currentPassword {
		return errors.New("当前密码不正确")
	}

	if newPassword == currentPassword {
		return errors.New("新密码与当前密码相同")
	}

	// 更新密码
	user.Password = newPassword
	return s.userRepo.Update(user)
}

// Delete 删除用户
func (s *UserService) Delete(userID string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	// 不允许删除默认管理员
	if user.Username == "admin" {
		return errors.New("默认管理员不允许删除")
	}

	return s.userRepo.Delete(userID)
}
