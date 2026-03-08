package parser

// ParseResult 解析结果
type ParseResult struct {
	Text     string            // 提取的文本内容
	Metadata map[string]string // 文档元数据（标题、作者、创建时间等）
	Pages    int               // 页数（如果适用）
	Error    error             // 解析错误（如果有）
	Warnings []string          // 警告信息
}

// Success 创建成功的解析结果
func Success(text string) *ParseResult {
	return &ParseResult{
		Text:     text,
		Metadata: make(map[string]string),
		Warnings: make([]string, 0),
	}
}

// Failure 创建失败的解析结果
func Failure(err error) *ParseResult {
	return &ParseResult{
		Error:    err,
		Metadata: make(map[string]string),
		Warnings: make([]string, 0),
	}
}

// AddWarning 添加警告信息
func (r *ParseResult) AddWarning(warning string) {
	r.Warnings = append(r.Warnings, warning)
}

// AddMetadata 添加元数据
func (r *ParseResult) AddMetadata(key, value string) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
	r.Metadata[key] = value
}

// IsSuccess 检查解析是否成功
func (r *ParseResult) IsSuccess() bool {
	return r.Error == nil && r.Text != ""
}
