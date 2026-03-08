package parser

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

// Manager 解析器管理器
type Manager struct {
	registry *Registry
}

// NewManager 创建解析器管理器
func NewManager() *Manager {
	return &Manager{
		registry: GetRegistry(),
	}
}

// Parse 解析文档
func (m *Manager) Parse(filePath string) (*ParseResult, error) {
	// 从文件路径提取文件类型
	fileType := m.detectFileType(filePath)

	// 获取解析器
	parser := m.registry.GetWithFallback(fileType)

	// 执行解析
	text, err := parser.Parse(filePath)
	if err != nil {
		return Failure(err), err
	}

	// 检查解析结果
	if strings.TrimSpace(text) == "" {
		warning := fmt.Sprintf("文件 %s 解析结果为空", filePath)
		log.Printf("⚠️  %s", warning)
		result := Success(text)
		result.AddWarning(warning)
		return result, nil
	}

	return Success(text), nil
}

// ParseWithType 使用指定的文件类型解析
func (m *Manager) ParseWithType(filePath, fileType string) (*ParseResult, error) {
	parser := m.registry.GetWithFallback(fileType)

	text, err := parser.Parse(filePath)
	if err != nil {
		return Failure(err), err
	}

	return Success(text), nil
}

// RegisterParser 注册自定义解析器
func (m *Manager) RegisterParser(fileType string, parser Parser) {
	m.registry.Register(fileType, parser)
	log.Printf("✅ 已注册解析器: %s", fileType)
}

// GetSupportedTypes 获取支持的文件类型列表
func (m *Manager) GetSupportedTypes() []string {
	return m.registry.ListSupportedTypes()
}

// detectFileType 从文件路径检测文件类型
func (m *Manager) detectFileType(filePath string) string {
	ext := filepath.Ext(filePath)
	if len(ext) > 0 {
		return strings.ToLower(ext[1:]) // 去掉点号
	}
	return ""
}
