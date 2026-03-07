package request

// UserCreateRequest 创建用户请求
type UserCreateRequest struct {
	Username string `json:"username" binding:"required" example:"testuser"`    // 用户名（不能为"admin"）
	Password string `json:"password" binding:"required" example:"password123"` // 密码
	Role     string `json:"role" example:"user"`                               // 角色（admin/user），默认为user
	Avatar   string `json:"avatar" example:"https://example.com/avatar.jpg"`   // 头像URL
}

// UserUpdateRequest 更新用户请求
type UserUpdateRequest struct {
	Username string `json:"username" example:"newusername"`                      // 用户名（不能为"admin"）
	Role     string `json:"role" example:"admin"`                                // 角色（admin/user）
	Avatar   string `json:"avatar" example:"https://example.com/new-avatar.jpg"` // 头像URL
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required" example:"oldpassword"` // 当前密码
	NewPassword     string `json:"new_password" binding:"required" example:"newpassword123"`  // 新密码
}
