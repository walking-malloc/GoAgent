package parser

import (
	"fmt"
	"strings"

	"github.com/nguyenthenguyen/docx"
)

// WordParser Word 文件解析器
type WordParser struct{}

func (p *WordParser) Parse(filePath string) (string, error) {
	// 只支持 .docx 格式
	if !strings.HasSuffix(strings.ToLower(filePath), ".docx") {
		return "", fmt.Errorf("only .docx format is supported, got: %s", filePath)
	}

	doc, err := docx.ReadDocxFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open document: %w", err)
	}
	defer doc.Close()

	// 提取所有文本内容
	text := doc.Editable().GetContent()

	// 清理文本（移除多余的空行）
	lines := strings.Split(text, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanedLines = append(cleanedLines, trimmed)
		}
	}

	return strings.Join(cleanedLines, "\n\n"), nil
}

func (p *WordParser) Support(fileType string) bool {
	return fileType == "docx" // 只支持 .docx，不支持旧的 .doc 格式
}
