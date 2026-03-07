package service

import (
	"context"
	"errors"
	"ragent-go/internal/model"
	"ragent-go/internal/pkg/jwt"
	"ragent-go/internal/pkg/redis"
	"ragent-go/internal/repository"
)

type AuthService struct {
	userRepo       *repository.UserRepository
	tokenBlacklist *redis.TokenBlacklist
}

func NewAuthService(userRepo *repository.UserRepository, tokenBlacklist *redis.TokenBlacklist) *AuthService {
	return &AuthService{
		userRepo:       userRepo,
		tokenBlacklist: tokenBlacklist,
	}
}

// LoginVO 登录响应
type LoginVO struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Token  string `json:"token"`
	Avatar string `json:"avatar"`
}

const DefaultAvatarURL = "https://avatars.githubusercontent.com/u/583231?v=4"

// Login 用户登录
func (s *AuthService) Login(username, password string) (*LoginVO, error) {
	// 参数验证
	if username == "" || password == "" {
		return nil, errors.New("用户名或密码不能为空")
	}

	// 查找用户
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	// 验证密码（原项目是明文密码，这里保持一致性）
	if user.Password != password {
		return nil, errors.New("用户名或密码错误")
	}

	// 生成 Token
	token, err := jwt.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, errors.New("生成Token失败")
	}

	// 设置默认头像
	avatar := user.Avatar
	if avatar == "" {
		avatar = DefaultAvatarURL
	}

	return &LoginVO{
		UserID: user.ID,
		Role:   user.Role,
		Token:  token,
		Avatar: avatar,
	}, nil
}

// Logout 用户登出
func (s *AuthService) Logout(ctx context.Context, token string) error {
	// 将 Token 添加到黑名单
	if s.tokenBlacklist != nil {
		return s.tokenBlacklist.Add(ctx, token)
	}
	return nil
}

// GetCurrentUser 获取当前用户信息
func (s *AuthService) GetCurrentUser(userID string) (*model.User, error) {
	return s.userRepo.FindByID(userID)
}
