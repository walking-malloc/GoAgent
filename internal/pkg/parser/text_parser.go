package parser

import (
	"os"
)

// TextParser 文本文件解析器
type TextParser struct{}

func (p *TextParser) Parse(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (p *TextParser) Support(fileType string) bool {
	return fileType == "txt"
}
