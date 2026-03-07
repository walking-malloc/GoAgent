package parser

// Parser 文档解析器接口
type Parser interface {
	// Parse 解析文档，返回文本内容
	Parse(filePath string) (string, error)

	// Support 检查是否支持该文件类型
	Support(fileType string) bool
}

// GetParser 根据文件类型获取解析器
func GetParser(fileType string) Parser {
	switch fileType {
	case "pdf":
		return &PDFParser{}
	case "docx", "doc":
		return &WordParser{}
	case "md", "markdown":
		return &MarkdownParser{}
	case "txt":
		return &TextParser{}
	default:
		return &TextParser{} // 默认使用文本解析器
	}
}
