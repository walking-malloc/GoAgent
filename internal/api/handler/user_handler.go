package handler

import (
	"strconv"
	"strings"

	"ragent-go/internal/api/handler/request"
	"ragent-go/internal/api/middleware"
	"ragent-go/internal/api/response"
	"ragent-go/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// GetCurrentUser 获取当前用户信息
// @Summary 获取当前用户信息
// @Description 获取当前登录用户的详细信息
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=model.User} "成功"
// @Failure 401 {object} response.Response "未授权"
// @Router /user/current [get]
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		response.Unauthorized(c, "未登录")
		return
	}

	user, err := h.userService.GetCurrentUser(userID)
	if err != nil {
		response.Error(c, 500, "获取用户信息失败")
		return
	}

	response.Success(c, user)
}

// PageQuery 分页查询用户列表
// @Summary 分页查询用户列表
// @Description 分页查询用户列表，支持关键词搜索（仅管理员）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param keyword query string false "搜索关键词"
// @Success 200 {object} response.Response{data=object{list=[]model.User,total=int,page=int,page_size=int}} "成功"
// @Failure 403 {object} response.Response "权限不足"
// @Router /users [get]
func (h *UserHandler) PageQuery(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	keyword := c.Query("keyword")

	users, total, err := h.userService.PageQuery(page, pageSize, keyword)
	if err != nil {
		response.Error(c, 500, "查询用户列表失败")
		return
	}

	response.Success(c, gin.H{
		"list":      users,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Create 创建用户（仅管理员）
// @Summary 创建用户
// @Description 创建新用户（仅管理员）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body request.UserCreateRequest true "创建用户请求"
// @Success 200 {object} response.Response{data=object{id=string}} "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Failure 403 {object} response.Response "权限不足"
// @Router /users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var req request.UserCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	// 去除空格
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.Role = strings.TrimSpace(req.Role)
	req.Avatar = strings.TrimSpace(req.Avatar)

	// 验证必填字段
	if req.Username == "" || req.Password == "" {
		response.BadRequest(c, "用户名和密码不能为空")
		return
	}

	// 不允许创建默认管理员
	if strings.EqualFold(req.Username, "admin") {
		response.Error(c, 400, "默认管理员用户名不可用")
		return
	}

	userID, err := h.userService.Create(req.Username, req.Password, req.Role, req.Avatar)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, gin.H{"id": userID})
}

// Update 更新用户（仅管理员）
// @Summary 更新用户
// @Description 更新用户信息（仅管理员）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户ID"
// @Param request body request.UserUpdateRequest true "更新用户请求"
// @Success 200 {object} response.Response "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Failure 403 {object} response.Response "权限不足"
// @Router /users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		response.BadRequest(c, "用户ID不能为空")
		return
	}

	var req request.UserUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	// 去除空格
	req.Username = strings.TrimSpace(req.Username)
	req.Role = strings.TrimSpace(req.Role)
	req.Avatar = strings.TrimSpace(req.Avatar)

	err := h.userService.Update(userID, req.Username, req.Role, req.Avatar)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, nil)
}

// Delete 删除用户（仅管理员）
// @Summary 删除用户
// @Description 删除用户（仅管理员，软删除）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户ID"
// @Success 200 {object} response.Response "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Failure 403 {object} response.Response "权限不足"
// @Router /users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		response.BadRequest(c, "用户ID不能为空")
		return
	}

	err := h.userService.Delete(userID)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, nil)
}

// ChangePassword 修改当前用户密码
// @Summary 修改密码
// @Description 修改当前登录用户的密码
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body request.ChangePasswordRequest true "修改密码请求"
// @Success 200 {object} response.Response "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Failure 401 {object} response.Response "未授权"
// @Router /user/password [put]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	currentUserID, exists := middleware.GetUserID(c)
	if !exists {
		response.Unauthorized(c, "未登录")
		return
	}

	var req request.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	// 去除空格
	req.CurrentPassword = strings.TrimSpace(req.CurrentPassword)
	req.NewPassword = strings.TrimSpace(req.NewPassword)

	if req.CurrentPassword == "" || req.NewPassword == "" {
		response.BadRequest(c, "当前密码和新密码不能为空")
		return
	}

	err := h.userService.ChangePassword(currentUserID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, nil)
}
