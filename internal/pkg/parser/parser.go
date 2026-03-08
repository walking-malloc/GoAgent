package parser

import (
	"log"
	"ragent-go/internal/config"
	"strings"
)

var globalManager *Manager

// InitParser 初始化解析器（需要在应用启动时调用）
func InitParser(cfg *config.Config) {
	globalManager = NewManager()
	log.Println("✅ 文档解析器已初始化")
	log.Printf("📋 支持的文件类型: %s", strings.Join(globalManager.GetSupportedTypes(), ", "))
}

// Parser 文档解析器接口
type Parser interface {
	// Parse 解析文档，返回文本内容
	Parse(filePath string) (string, error)

	// Support 检查是否支持该文件类型
	Support(fileType string) bool
}

// GetParser 根据文件类型获取解析器（兼容旧接口）
func GetParser(fileType string) Parser {
	if globalManager == nil {
		globalManager = NewManager()
	}
	return globalManager.registry.GetWithFallback(fileType)
}

// GetManager 获取解析器管理器
func GetManager() *Manager {
	if globalManager == nil {
		globalManager = NewManager()
	}
	return globalManager
}
