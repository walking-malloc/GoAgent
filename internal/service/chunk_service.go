package service

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// ChunkService 文档分块服务（简化版本，避免卡死）
type ChunkService struct {
	chunkSize    int // 分块大小（字符数）
	chunkOverlap int // 重叠大小（字符数）
	minChunkSize int // 最小分块大小（过滤过短段落）
}

// Chunk 分块结构，包含内容和元数据
type Chunk struct {
	Content   string            // 分块内容
	Metadata  map[string]string // 元数据（标题、章节等）
	Paragraph int               // 段落序号
}

const (
	DefaultChunkSize    = 500 // 默认分块大小
	DefaultChunkOverlap = 50  // 默认重叠大小
	MinChunkSize        = 50  // 最小分块大小（过滤过短段落）
)

// NewChunkService 创建分块服务
func NewChunkService(chunkSize, chunkOverlap int) *ChunkService {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if chunkOverlap < 0 {
		chunkOverlap = DefaultChunkOverlap
	}
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 5 // 最多20%重叠
	}
	return &ChunkService{
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
		minChunkSize: MinChunkSize,
	}
}

var (
	// 预编译的正则表达式，避免重复编译
	whitespaceRegexOnce sync.Once
	whitespaceRegex     *regexp.Regexp
)

// getWhitespaceRegex 获取空白字符正则表达式（延迟初始化，避免重复编译）
func getWhitespaceRegex() *regexp.Regexp {
	whitespaceRegexOnce.Do(func() {
		whitespaceRegex = regexp.MustCompile(`\s+`)
	})
	return whitespaceRegex
}

// buildMetadataLight 构建分块的元数据（轻量版本，仅保留段落索引）
func (s *ChunkService) buildMetadataLight(paragraphIndex int) map[string]string {
	metadata := make(map[string]string, 1)
	metadata["paragraph_index"] = strconv.Itoa(paragraphIndex)
	return metadata
}

func (s *ChunkService) ChunkTextStreaming(text string, callback func(chunk Chunk) error) error {
	if text == "" {
		return nil
	}

	// 按段落分割；段落过长则进一步按固定长度 + overlap 切分
	paragraphIndex := 0
	return s.processParagraphs(text, func(para string) error {
		para = strings.TrimSpace(para)

		// 过滤空段落和过短的段落
		if para == "" || len(para) < s.minChunkSize {
			return nil
		}

		// 预先构造元数据，只保留段落索引
		meta := s.buildMetadataLight(paragraphIndex)

		// 如果段落长度小于等于 chunkSize，直接作为一个块
		if len(para) <= s.chunkSize {
			chunk := Chunk{
				Content:   para,
				Metadata:  meta,
				Paragraph: paragraphIndex,
			}
			if callback != nil {
				if err := callback(chunk); err != nil {
					return err
				}
			}
			paragraphIndex++
			return nil
		}

		// 段落过长：按固定长度 + overlap 继续切分
		textLen := len(para)
		start := 0
		for start < textLen {
			end := start + s.chunkSize
			if end > textLen {
				end = textLen
			}

			part := strings.TrimSpace(para[start:end])
			if part != "" {
				chunk := Chunk{
					Content:   part,
					Metadata:  meta,
					Paragraph: paragraphIndex,
				}
				if callback != nil {
					if err := callback(chunk); err != nil {
						return err
					}
				}
			}

			if end == textLen {
				break
			}

			// 计算下一个块的起始位置（考虑重叠）
			nextStart := end - s.chunkOverlap
			if nextStart <= start {
				// 至少前进 1 个字符，避免无限循环
				nextStart = start + 1
			}
			start = nextStart
		}

		paragraphIndex++
		return nil
	})
}

// processParagraphs 按段落切分文本（不识别标题），段落之间以空行分隔
func (s *ChunkService) processParagraphs(text string, callback func(para string) error) error {
	// 统一换行符
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// 使用双换行切分段落
	parts := strings.Split(text, "\n\n")
	whitespaceRegex := getWhitespaceRegex()

	for _, part := range parts {
		// 段内的单换行视为空格
		para := strings.ReplaceAll(part, "\n", " ")
		// 压缩多个空白为一个空格
		para = whitespaceRegex.ReplaceAllString(para, " ")
		para = strings.TrimSpace(para)

		if para == "" {
			continue
		}

		if err := callback(para); err != nil {
			return err
		}
	}

	return nil
}
