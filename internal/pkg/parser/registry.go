package parser

import (
	"fmt"
	"strings"
	"sync"
)

// Registry 解析器注册表
type Registry struct {
	parsers map[string]Parser
	mu      sync.RWMutex
}

var globalRegistry *Registry
var once sync.Once

// GetRegistry 获取全局解析器注册表
func GetRegistry() *Registry {
	once.Do(func() {
		globalRegistry = &Registry{
			parsers: make(map[string]Parser),
		}
		// 注册默认解析器
		globalRegistry.RegisterDefaultParsers()
	})
	return globalRegistry
}

// RegisterDefaultParsers 注册默认解析器
func (r *Registry) RegisterDefaultParsers() {
	r.Register("pdf", &PDFParser{})
	r.Register("docx", &WordParser{})
	r.Register("doc", &WordParser{})
	r.Register("md", &MarkdownParser{})
	r.Register("markdown", &MarkdownParser{})
	r.Register("txt", &TextParser{})
}

// Register 注册解析器
func (r *Registry) Register(fileType string, parser Parser) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fileTypeLower := strings.ToLower(fileType)
	r.parsers[fileTypeLower] = parser
}

// Get 获取解析器
func (r *Registry) Get(fileType string) (Parser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fileTypeLower := strings.ToLower(fileType)
	parser, ok := r.parsers[fileTypeLower]
	if !ok {
		return nil, fmt.Errorf("未找到文件类型 %s 的解析器", fileType)
	}
	return parser, nil
}

// GetWithFallback 获取解析器，如果不存在则返回默认解析器
func (r *Registry) GetWithFallback(fileType string) Parser {
	parser, err := r.Get(fileType)
	if err != nil {
		// 返回文本解析器作为默认
		return &TextParser{}
	}
	return parser
}

// ListSupportedTypes 列出所有支持的文件类型
func (r *Registry) ListSupportedTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.parsers))
	for fileType := range r.parsers {
		types = append(types, fileType)
	}
	return types
}

// Unregister 取消注册解析器
func (r *Registry) Unregister(fileType string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fileTypeLower := strings.ToLower(fileType)
	delete(r.parsers, fileTypeLower)
}
