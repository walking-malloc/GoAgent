package parser

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/lukasjarosch/go-docx"
)

var (
	// 预编译正则表达式，避免重复编译
	paraRegexOnce       sync.Once
	paraRegex           *regexp.Regexp
	textRegexOnce       sync.Once
	textRegex           *regexp.Regexp
	whitespaceRegexOnce sync.Once
	whitespaceRegex     *regexp.Regexp
)

const MinParagraphLength = 400

func getParaRegex() *regexp.Regexp {
	paraRegexOnce.Do(func() {
		paraRegex = regexp.MustCompile(`(?s)<w:p[^>]*>(.*?)</w:p>`)
	})
	return paraRegex
}

func getTextRegex() *regexp.Regexp {
	textRegexOnce.Do(func() {
		textRegex = regexp.MustCompile(`(?s)<w:t[^>]*>(.*?)</w:t>`)
	})
	return textRegex
}

func getWhitespaceRegex() *regexp.Regexp {
	whitespaceRegexOnce.Do(func() {
		whitespaceRegex = regexp.MustCompile(`\s+`)
	})
	return whitespaceRegex
}

// WordParser Word 文件解析器
type WordParser struct{}

func (p *WordParser) Parse(filePath string) (string, error) {
	// 只支持 .docx 格式
	if !strings.HasSuffix(strings.ToLower(filePath), ".docx") {
		return "", fmt.Errorf("only .docx format is supported, got: %s", filePath)
	}

	// 使用docxgo打开文档
	doc, err := docx.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open document: %w", err)
	}
	defer doc.Close()

	// 获取文档XML内容
	docBytes := doc.GetFile(docx.DocumentXml)
	if len(docBytes) == 0 {
		return "", fmt.Errorf("failed to get document XML")
	}

	// 直接从XML中提取段落（<w:p>标签）
	paragraphs := extractParagraphsFromXML(docBytes)

	if len(paragraphs) == 0 {
		return "", fmt.Errorf("no text content found in document")
	}

	return strings.Join(paragraphs, "\n\n"), nil
}

// extractParagraphsFromXML 从XML中提取段落（按<w:p>标签分割）
func extractParagraphsFromXML(xml []byte) []string {
	content := string(xml)
	var rawParagraphs []string

	// 匹配 <w:p>...</w:p> 标签对（段落）
	// 使用预编译的正则表达式
	paraRegex := getParaRegex()
	matches := paraRegex.FindAllStringSubmatch(content, -1)

	// 先提取所有段落
	for _, match := range matches {
		if len(match) > 1 {
			// 从段落XML中提取所有文本节点 <w:t>...</w:t>
			paraText := extractTextFromParagraph(match[1])
			trimmed := strings.TrimSpace(paraText)
			if trimmed != "" {
				rawParagraphs = append(rawParagraphs, trimmed)
			}
		}
	}

	// 合并过短的段落（避免Word文档产生太多小chunks）
	// Word文档中，每个<w:p>标签可能对应一个很短的段落（如标题、空行等）
	// 我们将小于100字符的段落合并到相邻段落中

	var mergedParagraphs []string
	var currentPara strings.Builder

	for _, para := range rawParagraphs {
		paraLen := len(para)

		// 如果段落足够长，直接添加
		if paraLen >= MinParagraphLength {
			// 如果之前有累积的短段落，先添加累积的内容
			if currentPara.Len() > 0 {
				mergedParagraphs = append(mergedParagraphs, currentPara.String())
				currentPara.Reset()
			}
			mergedParagraphs = append(mergedParagraphs, para)
		} else {
			// 段落太短，累积起来
			if currentPara.Len() > 0 {
				currentPara.WriteString(" ")
			}
			currentPara.WriteString(para)

			// 如果累积后达到最小长度，保存并重置
			if currentPara.Len() >= MinParagraphLength {
				mergedParagraphs = append(mergedParagraphs, currentPara.String())
				currentPara.Reset()
			}
		}
	}

	// 添加最后累积的段落（即使不够长也添加，避免丢失内容）
	if currentPara.Len() > 0 {
		mergedParagraphs = append(mergedParagraphs, currentPara.String())
	}

	// 如果没有合并任何段落，返回原始段落
	if len(mergedParagraphs) == 0 {
		return rawParagraphs
	}

	return mergedParagraphs
}

// extractTextFromParagraph 从段落XML中提取文本
func extractTextFromParagraph(paraXML string) string {
	var text strings.Builder

	// 查找段落中所有 <w:t>...</w:t> 标签中的文本（支持跨行）
	textRegex := getTextRegex()
	matches := textRegex.FindAllStringSubmatch(paraXML, -1)

	whitespaceRegex := getWhitespaceRegex()

	for _, match := range matches {
		if len(match) > 1 {
			textContent := match[1]
			// 解码XML实体（如 &lt; &gt; &amp; 等）
			textContent = decodeXMLEntities(textContent)
			// 移除XML中的换行和多余空格，但保留单词之间的空格
			textContent = strings.ReplaceAll(textContent, "\n", " ")
			textContent = strings.ReplaceAll(textContent, "\r", " ")
			textContent = whitespaceRegex.ReplaceAllString(textContent, " ")
			text.WriteString(textContent)
		}
	}

	return text.String()
}

// decodeXMLEntities 解码XML实体
func decodeXMLEntities(text string) string {
	// 常见的XML实体
	replacements := map[string]string{
		"&lt;":   "<",
		"&gt;":   ">",
		"&amp;":  "&",
		"&apos;": "'",
		"&quot;": "\"",
	}

	result := text
	for entity, char := range replacements {
		result = strings.ReplaceAll(result, entity, char)
	}

	return result
}

func (p *WordParser) Support(fileType string) bool {
	return fileType == "docx" // 只支持 .docx，不支持旧的 .doc 格式
}
