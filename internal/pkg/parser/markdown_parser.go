package parser

import (
	"os"
)

// MarkdownParser Markdown 文件解析器
type MarkdownParser struct{}

func (p *MarkdownParser) Parse(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (p *MarkdownParser) Support(fileType string) bool {
	return fileType == "md" || fileType == "markdown"
}
