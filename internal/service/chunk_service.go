package service

import (
	"strings"
)

// ChunkService 文档分块服务（简化版本，避免卡死）
type ChunkService struct {
	chunkSize    int // 分块大小（字符数）
	chunkOverlap int // 重叠大小（字符数）
}

// NewChunkService 创建分块服务
func NewChunkService(chunkSize, chunkOverlap int) *ChunkService {
	if chunkSize <= 0 {
		chunkSize = 1000 // 默认1000字符
	}
	if chunkOverlap < 0 {
		chunkOverlap = 200 // 默认200字符重叠
	}
	// 确保重叠不超过分块大小
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 5 // 最多20%重叠
	}
	return &ChunkService{
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
	}
}

// ChunkText 将文本分块（极简版本，直接按字符分割，避免复杂逻辑导致卡死）
func (s *ChunkService) ChunkText(text string) []string {
	if text == "" {
		return nil
	}

	textLen := len(text)
	if textLen == 0 {
		return nil
	}

	// 对于超大文本（超过500KB），直接按字符分割，不做任何复杂处理
	if textLen > 500*1024 {
		return s.splitByCharactersSimple(text)
	}

	// 尝试在句号、换行符等位置分割，但如果找不到合适位置，直接按字符分割
	return s.splitWithBoundaries(text)
}

// splitByCharactersSimple 最简单的按字符分割（用于超大文本，优化性能）
func (s *ChunkService) splitByCharactersSimple(text string) []string {
	textLen := len(text)
	if textLen == 0 {
		return nil
	}

	// 预估分块数量（考虑重叠）
	// 每个块实际前进: chunkSize - overlap
	actualChunkSize := s.chunkSize - s.chunkOverlap
	if actualChunkSize <= 0 {
		actualChunkSize = s.chunkSize
	}
	chunkCount := (textLen + actualChunkSize - 1) / actualChunkSize
	chunks := make([]string, 0, chunkCount)

	start := 0
	for start < textLen {
		end := start + s.chunkSize
		if end > textLen {
			end = textLen
		}

		// 直接使用字符串切片，Go会自动处理内存
		chunks = append(chunks, text[start:end])

		// 下一个块的起始位置（考虑重叠）
		start = end - s.chunkOverlap
		if start < 0 {
			start = end
		}
		// 防止无限循环
		if start >= textLen {
			break
		}
	}

	return chunks
}

// splitWithBoundaries 尝试在边界处分割，但失败时回退到简单分割
func (s *ChunkService) splitWithBoundaries(text string) []string {
	textLen := len(text)
	if textLen == 0 {
		return nil
	}

	// 如果文本很短，直接返回
	if textLen <= s.chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < textLen {
		end := start + s.chunkSize
		if end > textLen {
			end = textLen
		}

		// 如果已经是最后一块，直接添加
		if end >= textLen {
			chunk := text[start:end]
			if len(strings.TrimSpace(chunk)) > 0 {
				chunks = append(chunks, chunk)
			}
			break
		}

		// 尝试在句号、问号、感叹号、换行符处分割
		bestEnd := end
		searchStart := end - s.chunkOverlap
		if searchStart < start {
			searchStart = start
		}

		// 向后查找边界字符（最多查找200个字符）
		maxSearch := end + 200
		if maxSearch > textLen {
			maxSearch = textLen
		}

		found := false
		for i := end; i < maxSearch && i < textLen; i++ {
			char := text[i]
			// 检查是否是句子结束符或换行符
			if char == '.' || char == '!' || char == '?' || char == '\n' {
				// 检查下一个字符是否是空格或换行
				if i+1 >= textLen || text[i+1] == ' ' || text[i+1] == '\n' || text[i+1] == '\r' {
					bestEnd = i + 1
					found = true
					break
				}
			}
		}

		// 如果没找到，尝试向前查找
		if !found {
			for i := end - 1; i >= searchStart; i-- {
				char := text[i]
				if char == '.' || char == '!' || char == '?' || char == '\n' {
					// 检查下一个字符是否是空格或换行
					if i+1 >= textLen || text[i+1] == ' ' || text[i+1] == '\n' || text[i+1] == '\r' {
						bestEnd = i + 1
						found = true
						break
					}
				}
			}
		}

		// 如果还是没找到合适的边界，直接按原位置分割
		if !found {
			bestEnd = end
		}

		chunk := text[start:bestEnd]
		if len(strings.TrimSpace(chunk)) > 0 {
			chunks = append(chunks, chunk)
		}

		// 下一个块的起始位置（考虑重叠）
		start = bestEnd - s.chunkOverlap
		if start < 0 {
			start = bestEnd
		}
		if start >= textLen {
			break
		}
	}

	return chunks
}
