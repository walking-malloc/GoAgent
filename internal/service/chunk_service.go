package service

import (
	"regexp"
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

// SimpleChunkText 简单分块方法（用于超大文本，直接按字符分割）
func (s *ChunkService) SimpleChunkText(text string) []string {
	if text == "" {
		return nil
	}
	textLen := len(text)
	if textLen == 0 {
		return nil
	}
	return s.splitByCharactersSimple(text)
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

// ChunkText 将文本按段落分块，返回带元数据的分块列表（优化内存版本）
func (s *ChunkService) ChunkText(text string) []Chunk {
	if text == "" {
		return nil
	}

	// 对于超大文本，使用简单分块避免内存爆炸
	if len(text) > 500*1024 { // 500KB
		return s.chunkTextSimple(text)
	}

	// 预分配chunks切片，避免多次扩容
	// 预估分块数量：文本长度 / chunkSize，但限制最大数量
	estimatedChunks := len(text)/s.chunkSize + 1
	if estimatedChunks > 500 {
		estimatedChunks = 500 // 限制最大预估数量，避免过度分配
	}
	chunks := make([]Chunk, 0, estimatedChunks)

	// 提取标题和章节信息（只提取一次，复用）
	titleContext := s.extractTitleContext(text)

	// 流式处理段落，避免一次性创建所有段落对象
	paragraphIndex := 0
	s.processParagraphsStreamingSync(text, func(para string) {
		para = strings.TrimSpace(para)

		// 过滤过短的段落
		if len(para) < s.minChunkSize {
			return
		}

		// 如果段落小于chunkSize，直接作为一个chunk
		if len(para) <= s.chunkSize {
			chunk := Chunk{
				Content:   para,
				Metadata:  s.buildMetadataLight(para, paragraphIndex, titleContext),
				Paragraph: paragraphIndex,
			}
			chunks = append(chunks, chunk)
			paragraphIndex++
			return
		}

		// 如果段落大于chunkSize，按chunkSize分割
		subChunks := s.splitLongParagraphLight(para, paragraphIndex, titleContext)
		chunks = append(chunks, subChunks...)
		paragraphIndex++
	})

	return chunks
}

// chunkTextSimple 简单分块方法（用于超大文本）
func (s *ChunkService) chunkTextSimple(text string) []Chunk {
	textLen := len(text)
	if textLen == 0 {
		return nil
	}

	actualChunkSize := s.chunkSize - s.chunkOverlap
	if actualChunkSize <= 0 {
		actualChunkSize = s.chunkSize
	}
	chunkCount := (textLen + actualChunkSize - 1) / actualChunkSize
	if chunkCount > 1000 {
		chunkCount = 1000 // 限制最大分块数
	}
	chunks := make([]Chunk, 0, chunkCount)

	start := 0
	chunkIndex := 0
	for start < textLen {
		end := start + s.chunkSize
		if end > textLen {
			end = textLen
		}

		chunkContent := text[start:end]
		if len(strings.TrimSpace(chunkContent)) > 0 {
			chunk := Chunk{
				Content:   chunkContent,
				Metadata:  nil, // 超大文本不提取元数据，节省内存
				Paragraph: chunkIndex,
			}
			chunks = append(chunks, chunk)
			chunkIndex++
		}

		start = end - s.chunkOverlap
		if start < 0 {
			start = end
		}
		if start >= textLen {
			break
		}
	}

	return chunks
}

// processParagraphsStreamingSync 流式处理段落（同步版本，无返回值）
func (s *ChunkService) processParagraphsStreamingSync(text string, callback func(para string)) {
	// 统一换行符
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// 使用双换行符分割，但流式处理
	parts := strings.Split(text, "\n\n")

	// 预编译正则表达式，避免在循环中重复编译
	whitespaceRegex := getWhitespaceRegex()

	for _, part := range parts {
		// 清理段落（移除单换行符，保留段落内容）
		para := strings.ReplaceAll(part, "\n", " ")
		// 压缩多个空格为一个（使用预编译的正则表达式）
		para = whitespaceRegex.ReplaceAllString(para, " ")
		para = strings.TrimSpace(para)

		if para != "" {
			callback(para)
		}
	}
}

// splitIntoParagraphs 将文本分割成段落（保留用于兼容性，但已优化为流式处理）
func (s *ChunkService) splitIntoParagraphs(text string) []string {
	var paragraphs []string
	s.processParagraphsStreamingSync(text, func(para string) {
		paragraphs = append(paragraphs, para)
	})
	return paragraphs
}

// splitLongParagraph 分割过长的段落
func (s *ChunkService) splitLongParagraph(para string, paragraphIndex int, titleContext map[string]string) []Chunk {
	return s.splitLongParagraphLight(para, paragraphIndex, titleContext)
}

// splitLongParagraphLight 分割过长的段落（轻量版本，减少内存占用）
func (s *ChunkService) splitLongParagraphLight(para string, paragraphIndex int, titleContext map[string]string) []Chunk {
	textLen := len(para)
	// 预估子块数量
	estimatedSubChunks := (textLen + s.chunkSize - 1) / s.chunkSize
	if estimatedSubChunks > 100 {
		estimatedSubChunks = 100 // 限制最大数量
	}
	chunks := make([]Chunk, 0, estimatedSubChunks)

	start := 0
	subChunkIndex := 0

	for start < textLen {
		end := start + s.chunkSize
		if end > textLen {
			end = textLen
		}

		// 如果不是最后一块，尝试在句子边界切分
		if end < textLen {
			bestEnd := s.findSentenceBoundary(para, start, end)
			end = bestEnd
		}

		chunkContent := para[start:end]
		if len(strings.TrimSpace(chunkContent)) > 0 {
			// 使用轻量级元数据
			metadata := s.buildMetadataLight(chunkContent, paragraphIndex, titleContext)
			if metadata != nil {
				metadata["sub_chunk_index"] = s.intToString(subChunkIndex)
			}

			chunk := Chunk{
				Content:   chunkContent,
				Metadata:  metadata,
				Paragraph: paragraphIndex,
			}
			chunks = append(chunks, chunk)
			subChunkIndex++
		}

		// 下一个块的起始位置（考虑重叠）
		start = end - s.chunkOverlap
		if start < 0 {
			start = end
		}
		if start >= textLen {
			break
		}
	}

	return chunks
}

// findSentenceBoundary 在指定范围内查找句子边界
func (s *ChunkService) findSentenceBoundary(text string, start, targetEnd int) int {
	textLen := len(text)

	// 向后查找（最多200字符）
	maxSearch := targetEnd + 200
	if maxSearch > textLen {
		maxSearch = textLen
	}

	// 句子结束符：中英文句号、问号、感叹号、换行符
	sentenceEnders := []rune{'.', '。', '!', '！', '?', '？', '\n'}

	// 向后搜索
	for i := targetEnd; i < maxSearch && i < textLen; i++ {
		char := rune(text[i])
		for _, ender := range sentenceEnders {
			if char == ender {
				// 检查下一个字符是否是空格、换行或文本结束
				if i+1 >= textLen {
					return i + 1
				}
				nextChar := rune(text[i+1])
				if nextChar == ' ' || nextChar == '\n' || nextChar == '\r' || nextChar == '\t' {
					return i + 1
				}
			}
		}
	}

	// 向前搜索（在重叠区域内）
	searchStart := targetEnd - s.chunkOverlap
	if searchStart < start {
		searchStart = start
	}

	for i := targetEnd - 1; i >= searchStart; i-- {
		char := rune(text[i])
		for _, ender := range sentenceEnders {
			if char == ender {
				if i+1 >= textLen {
					return i + 1
				}
				nextChar := rune(text[i+1])
				if nextChar == ' ' || nextChar == '\n' || nextChar == '\r' || nextChar == '\t' {
					return i + 1
				}
			}
		}
	}

	// 没找到边界，返回目标位置
	return targetEnd
}

var (
	// 预编译的正则表达式，避免重复编译
	titlePatternsOnce   sync.Once
	titlePatterns       []*regexp.Regexp
	whitespaceRegexOnce sync.Once
	whitespaceRegex     *regexp.Regexp
)

// getTitlePatterns 获取标题匹配模式（延迟初始化）
func getTitlePatterns() []*regexp.Regexp {
	titlePatternsOnce.Do(func() {
		titlePatterns = []*regexp.Regexp{
			regexp.MustCompile(`^第[一二三四五六七八九十\d]+章[：:]\s*(.+)$`),
			regexp.MustCompile(`^第[一二三四五六七八九十\d]+节[：:]\s*(.+)$`),
			regexp.MustCompile(`^[一二三四五六七八九十]+[、．]\s*(.+)$`),
			regexp.MustCompile(`^\d+[、．]\s*(.+)$`),
			regexp.MustCompile(`^第\d+章\s*(.+)$`),
			regexp.MustCompile(`^第\d+节\s*(.+)$`),
		}
	})
	return titlePatterns
}

// getWhitespaceRegex 获取空白字符正则表达式（延迟初始化，避免重复编译）
func getWhitespaceRegex() *regexp.Regexp {
	whitespaceRegexOnce.Do(func() {
		whitespaceRegex = regexp.MustCompile(`\s+`)
	})
	return whitespaceRegex
}

// extractTitleContext 提取标题和章节信息（优化版本，限制扫描范围）
func (s *ChunkService) extractTitleContext(text string) map[string]string {
	metadata := make(map[string]string, 4) // 预分配容量

	// 只扫描前1000行，避免处理超大文件时内存爆炸
	lines := strings.Split(text, "\n")
	maxLines := 1000
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	titlePatterns := getTitlePatterns()

	var lastTitle string
	var lastChapter string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 2 || len(line) > 100 {
			continue
		}

		// 检查是否是标题行
		for i, pattern := range titlePatterns {
			matches := pattern.FindStringSubmatch(line)
			if len(matches) > 1 {
				title := strings.TrimSpace(matches[1])
				if title != "" {
					if i < 2 {
						lastChapter = title
						metadata["chapter"] = title
					} else {
						lastTitle = title
						metadata["section"] = title
					}
					break
				}
			}
		}

		// 如果没有匹配到模式，但行很短且以冒号结尾，可能是标题
		if len(line) < 50 && (strings.HasSuffix(line, "：") || strings.HasSuffix(line, ":")) {
			lastTitle = strings.TrimSuffix(strings.TrimSuffix(line, "："), ":")
			metadata["section"] = lastTitle
		}
	}

	// 保存最后找到的标题和章节
	if lastChapter != "" {
		metadata["last_chapter"] = lastChapter
	}
	if lastTitle != "" {
		metadata["last_section"] = lastTitle
	}

	return metadata
}

// buildMetadata 构建分块的元数据（完整版本）
func (s *ChunkService) buildMetadata(content string, paragraphIndex int, titleContext map[string]string) map[string]string {
	metadata := make(map[string]string)

	// 复制标题上下文
	for k, v := range titleContext {
		metadata[k] = v
	}

	// 添加段落信息
	metadata["paragraph_index"] = s.intToString(paragraphIndex)

	// 尝试从内容开头提取可能的标题
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		if len(firstLine) < 100 && len(firstLine) > 0 {
			// 如果第一行很短，可能是标题
			if len(firstLine) < 50 {
				metadata["content_title"] = firstLine
			}
		}
	}

	// 添加内容长度
	metadata["content_length"] = s.intToString(len(content))

	return metadata
}

// buildMetadataLight 构建分块的元数据（轻量版本，减少内存占用）
func (s *ChunkService) buildMetadataLight(content string, paragraphIndex int, titleContext map[string]string) map[string]string {
	// 只在有标题上下文时才创建map，否则返回nil
	if len(titleContext) == 0 {
		return nil // 返回nil而不是空map，节省内存
	}

	metadata := make(map[string]string, len(titleContext)+2) // 预分配容量

	// 只复制重要的标题上下文
	if chapter, ok := titleContext["chapter"]; ok {
		metadata["chapter"] = chapter
	}
	if section, ok := titleContext["section"]; ok {
		metadata["section"] = section
	}

	// 添加段落信息
	metadata["paragraph_index"] = s.intToString(paragraphIndex)

	return metadata
}

// intToString 将整数转换为字符串
func (s *ChunkService) intToString(n int) string {
	if n == 0 {
		return "0"
	}
	var result []byte
	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}
	return string(result)
}

// ChunkTextStreaming 流式处理分块，通过回调函数处理每个chunk（内存优化版本）
func (s *ChunkService) ChunkTextStreaming(text string, callback func(chunk Chunk) error) error {
	if text == "" {
		return nil
	}

	//对于超大文本，使用简单分块
	if len(text) > 500*1024 {
		return s.chunkTextSimpleStreaming(text, callback)
	}

	// 提取标题和章节信息（只提取一次，复用）
	titleContext := s.extractTitleContext(text)

	// 流式处理段落，边处理边回调，不保存所有chunks
	// 简化逻辑：只按段落分割，不再进一步分割长段落
	paragraphIndex := 0
	return s.processParagraphsStreaming(text, func(para string) error {
		para = strings.TrimSpace(para)

		// 过滤空段落和过短的段落
		if para == "" || len(para) < s.minChunkSize {
			return nil
		}

		// 每个段落直接作为一个chunk，不再检查chunkSize
		chunk := Chunk{
			Content:   para,
			Metadata:  s.buildMetadataLight(para, paragraphIndex, titleContext),
			Paragraph: paragraphIndex,
		}
		if callback != nil {
			if err := callback(chunk); err != nil {
				return err
			}
		}
		paragraphIndex++
		return nil
	})
}

// chunkTextSimpleStreaming 简单分块流式处理
func (s *ChunkService) chunkTextSimpleStreaming(text string, callback func(chunk Chunk) error) error {
	textLen := len(text)
	if textLen == 0 {
		return nil
	}

	start := 0
	chunkIndex := 0
	for start < textLen {
		end := start + s.chunkSize
		if end > textLen {
			end = textLen
		}

		chunkContent := text[start:end]
		trimmedContent := strings.TrimSpace(chunkContent)
		if len(trimmedContent) > 0 {
			chunk := Chunk{
				Content:   trimmedContent, // 使用trimmed后的内容
				Metadata:  nil,
				Paragraph: chunkIndex,
			}
			if callback != nil {
				if err := callback(chunk); err != nil {
					return err
				}
			}
			chunkIndex++
		}

		// 计算下一个块的起始位置（考虑重叠）
		nextStart := end - s.chunkOverlap
		if nextStart <= start {
			// 如果重叠导致没有前进，至少前进1个字符，避免无限循环
			nextStart = start + 1
		}
		start = nextStart

		if start >= textLen {
			break
		}
	}

	return nil
}

// splitLongParagraphStreaming 流式分割过长段落
func (s *ChunkService) splitLongParagraphStreaming(para string, paragraphIndex int, titleContext map[string]string, callback func(chunk Chunk) error) error {
	start := 0
	textLen := len(para)
	subChunkIndex := 0

	for start < textLen {
		end := start + s.chunkSize
		if end > textLen {
			end = textLen
		}

		if end < textLen {
			bestEnd := s.findSentenceBoundary(para, start, end)
			end = bestEnd
		}

		chunkContent := para[start:end]
		trimmedContent := strings.TrimSpace(chunkContent)
		if len(trimmedContent) > 0 {
			metadata := s.buildMetadataLight(trimmedContent, paragraphIndex, titleContext)
			if metadata != nil {
				metadata["sub_chunk_index"] = s.intToString(subChunkIndex)
			}

			chunk := Chunk{
				Content:   trimmedContent, // 使用trimmed后的内容
				Metadata:  metadata,
				Paragraph: paragraphIndex,
			}
			if callback != nil {
				if err := callback(chunk); err != nil {
					return err
				}
			}
			subChunkIndex++
		}

		// 计算下一个块的起始位置（考虑重叠）
		nextStart := end - s.chunkOverlap
		if nextStart <= start {
			// 如果重叠导致没有前进，至少前进1个字符，避免无限循环
			nextStart = start + 1
		}
		start = nextStart

		if start >= textLen {
			break
		}
	}

	return nil
}

// processParagraphsStreaming 流式处理段落（异步版本，返回error）
func (s *ChunkService) processParagraphsStreaming(text string, callback func(para string) error) error {
	// 统一换行符
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// 使用双换行符分割，但流式处理
	parts := strings.Split(text, "\n\n")

	// 预编译正则表达式，避免在循环中重复编译
	whitespaceRegex := getWhitespaceRegex()

	for _, part := range parts {
		// 清理段落（移除单换行符，保留段落内容）
		para := strings.ReplaceAll(part, "\n", " ")
		// 压缩多个空格为一个（使用预编译的正则表达式）
		para = whitespaceRegex.ReplaceAllString(para, " ")
		para = strings.TrimSpace(para)

		// 过滤空段落和过短的段落（避免重复处理）
		if para != "" && len(para) >= 10 { // 至少10个字符才处理
			if err := callback(para); err != nil {
				return err
			}
		}
	}

	return nil
}

// ChunkTextLegacy 兼容旧接口，返回字符串数组
func (s *ChunkService) ChunkTextLegacy(text string) []string {
	chunks := s.ChunkText(text)
	result := make([]string, len(chunks))
	for i, chunk := range chunks {
		result[i] = chunk.Content
	}
	return result
}
