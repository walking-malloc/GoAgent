package middleware

import (
	"context"
	"net/http"
	"strings"

	"ragent-go/internal/api/response"
	"ragent-go/internal/pkg/jwt"
	"ragent-go/internal/pkg/redis"

	"github.com/gin-gonic/gin"
)

var tokenBlacklist *redis.TokenBlacklist

// SetTokenBlacklist 设置 Token 黑名单管理器（在 main.go 中调用）
func SetTokenBlacklist(blacklist *redis.TokenBlacklist) {
	tokenBlacklist = blacklist
}

// Auth 认证中间件
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取 Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "未提供认证Token")
			c.Abort()
			return
		}

		// 检查 Bearer 前缀
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "Token格式错误")
			c.Abort()
			return
		}

		token := parts[1]

		// 检查 Token 是否在黑名单中
		if tokenBlacklist != nil {
			blacklisted, err := tokenBlacklist.IsBlacklisted(context.Background(), token)
			if err == nil && blacklisted {
				response.Unauthorized(c, "Token已失效")
				c.Abort()
				return
			}
		}

		// 解析 Token
		claims, err := jwt.ParseToken(token)
		if err != nil {
			response.Unauthorized(c, "Token无效或已过期")
			c.Abort()
			return
		}

		// 将用户信息存储到上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// RequireRole 要求特定角色的中间件
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			response.Unauthorized(c, "未登录")
			c.Abort()
			return
		}

		userRole := role.(string)
		for _, r := range roles {
			if userRole == r {
				c.Next()
				return
			}
		}

		response.Error(c, http.StatusForbidden, "权限不足")
		c.Abort()
	}
}

// OptionalAuth 可选认证中间件（登录或未登录都能访问）
// 如果提供了有效的 Token，会设置用户信息到上下文；如果没有提供或无效，不报错继续执行
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取 Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// 没有 Token，继续执行（不设置用户信息）
			c.Next()
			return
		}

		// 检查 Bearer 前缀
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			// Token 格式错误，继续执行（不设置用户信息）
			c.Next()
			return
		}

		token := parts[1]

		// 检查 Token 是否在黑名单中
		if tokenBlacklist != nil {
			blacklisted, err := tokenBlacklist.IsBlacklisted(context.Background(), token)
			if err == nil && blacklisted {
				// Token 已失效，继续执行（不设置用户信息）
				c.Next()
				return
			}
		}

		// 解析 Token
		claims, err := jwt.ParseToken(token)
		if err != nil {
			// Token 无效或已过期，继续执行（不设置用户信息）
			c.Next()
			return
		}

		// Token 有效，将用户信息存储到上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// GetUserID 从上下文获取用户ID
func GetUserID(c *gin.Context) (string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", false
	}

	return userID.(string), true
}

// GetUsername 从上下文获取用户名
func GetUsername(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}

	return username.(string), true
}

// GetRole 从上下文获取用户角色
func GetRole(c *gin.Context) (string, bool) {
	role, exists := c.Get("role")
	if !exists {
		return "", false
	}

	return role.(string), true
}

// IsAuthenticated 检查用户是否已登录
func IsAuthenticated(c *gin.Context) bool {
	_, exists := c.Get("user_id")
	return exists
}
