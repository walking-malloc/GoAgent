package handler

import (
	"ragent-go/internal/api/response"
	"ragent-go/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"admin"`    // 用户名
	Password string `json:"password" binding:"required" example:"admin123"` // 密码
}

// Login 用户登录
// @Summary 用户登录
// @Description 用户登录，获取访问Token
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body LoginRequest true "登录请求"
// @Success 200 {object} response.Response{data=service.LoginVO} "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "用户名或密码不能为空")
		return
	}

	result, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, result)
}

// Logout 用户登出
// @Summary 用户登出
// @Description 用户登出，将Token加入黑名单
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response "成功"
// @Failure 401 {object} response.Response "未授权"
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// 从请求头获取 Token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.Unauthorized(c, "未提供认证Token")
		return
	}

	// 提取 Token（去掉 "Bearer " 前缀）
	token := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}

	// 将 Token 加入黑名单
	if err := h.authService.Logout(c.Request.Context(), token); err != nil {
		response.Error(c, 500, "登出失败")
		return
	}

	response.Success(c, nil)
}
