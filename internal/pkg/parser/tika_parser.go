package parser

import (
	"fmt"
	"strings"

	"ragent-go/internal/pkg/tika"
)

// TikaParser 通用 Tika 解析器（支持所有 Tika 支持的文件类型）
type TikaParser struct {
	tikaClient *tika.Client
}

// NewTikaParser 创建新的 Tika 解析器
func NewTikaParser(tikaClient *tika.Client) *TikaParser {
	return &TikaParser{
		tikaClient: tikaClient,
	}
}

func (p *TikaParser) Parse(filePath string) (string, error) {
	if p.tikaClient == nil {
		return "", fmt.Errorf("Tika client is not initialized")
	}

	text, err := p.tikaClient.ExtractText(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to extract text from file: %w", err)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("no text content found in file")
	}

	return text, nil
}
