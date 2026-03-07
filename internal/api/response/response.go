package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
// @Description 统一响应格式
type Response struct {
	Code    int         `json:"code" example:"0"`          // 状态码，0表示成功，非0表示失败
	Message string      `json:"message" example:"success"` // 响应消息
	Data    interface{} `json:"data,omitempty"`            // 响应数据（可选）
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// SuccessWithMessage 成功响应（带消息）
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: message,
		Data:    data,
	})
}

// Error 错误响应
func Error(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
	})
}

// ErrorWithData 错误响应（带数据）
func ErrorWithData(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// BadRequest 400 错误
func BadRequest(c *gin.Context, message string) {
	if message == "" {
		message = "Bad Request"
	}
	c.JSON(http.StatusBadRequest, Response{
		Code:    400,
		Message: message,
	})
}

// Unauthorized 401 错误
func Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = "Unauthorized"
	}
	c.JSON(http.StatusUnauthorized, Response{
		Code:    401,
		Message: message,
	})
}

// Forbidden 403 错误
func Forbidden(c *gin.Context, message string) {
	if message == "" {
		message = "Forbidden"
	}
	c.JSON(http.StatusForbidden, Response{
		Code:    403,
		Message: message,
	})
}

// NotFound 404 错误
func NotFound(c *gin.Context, message string) {
	if message == "" {
		message = "Not Found"
	}
	c.JSON(http.StatusNotFound, Response{
		Code:    404,
		Message: message,
	})
}

// InternalServerError 500 错误
func InternalServerError(c *gin.Context, message string) {
	if message == "" {
		message = "Internal Server Error"
	}
	c.JSON(http.StatusInternalServerError, Response{
		Code:    500,
		Message: message,
	})
}
