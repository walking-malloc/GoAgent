package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"ragent-go/internal/model"
	"ragent-go/internal/pkg/ai"
	"ragent-go/internal/pkg/milvus"
	"ragent-go/internal/pkg/parser"
	"ragent-go/internal/repository"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"
)

// DocumentService 文档服务
type DocumentService struct {
	docRepo        *repository.DocumentRepository
	chunkRepo      *repository.DocumentChunkRepository
	kbRepo         *repository.KnowledgeBaseRepository
	chunkService   *ChunkService
	embeddingSvc   *ai.EmbeddingService
	vectorMgr      *milvus.VectorManager
	uploadBasePath string
}

// NewDocumentService 创建文档服务
func NewDocumentService(
	docRepo *repository.DocumentRepository,
	chunkRepo *repository.DocumentChunkRepository,
	kbRepo *repository.KnowledgeBaseRepository,
	chunkService *ChunkService,
	embeddingSvc *ai.EmbeddingService,
	vectorMgr *milvus.VectorManager,
	uploadBasePath string,
) *DocumentService {
	return &DocumentService{
		docRepo:        docRepo,
		chunkRepo:      chunkRepo,
		kbRepo:         kbRepo,
		chunkService:   chunkService,
		embeddingSvc:   embeddingSvc,
		vectorMgr:      vectorMgr,
		uploadBasePath: uploadBasePath,
	}
}

// UploadDocument 上传文档并开始处理
func (s *DocumentService) UploadDocument(
	kbID, fileName, filePath, fileType string,
	fileSize int64,
	createdBy string,
) (*model.Document, error) {
	// 验证知识库是否存在
	kb, err := s.kbRepo.FindByID(kbID)
	if err != nil {
		return nil, fmt.Errorf("knowledge base not found: %w", err)
	}

	// 创建文档记录
	doc := &model.Document{
		ID:         ulid.Make().String(),
		KBID:       kbID,
		Name:       fileName,
		FileName:   fileName,
		FilePath:   filePath,
		FileType:   strings.ToLower(fileType),
		FileSize:   fileSize,
		Status:     model.DocumentStatusPending,
		CreatedBy:  createdBy,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}

	if err := s.docRepo.Create(doc); err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}

	// 异步处理文档（这里先同步处理，后续可以改为异步）
	go s.processDocument(doc.ID, kb.CollectionName)

	return doc, nil
}

// processDocument 处理文档（解析、分块、向量化、存储）
func (s *DocumentService) processDocument(docID, collectionName string) {
	// 设置超时上下文（30分钟超时）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// 更新状态为处理中
	_ = s.docRepo.UpdateStatus(docID, model.DocumentStatusProcessing, "")

	// 获取文档信息
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		_ = s.docRepo.UpdateStatus(docID, model.DocumentStatusFailed, fmt.Sprintf("failed to find document: %v", err))
		return
	}

	// 1. 解析文档（带超时检查）
	log.Printf("📄 [%s] 开始解析文档: %s", docID, doc.FileName)

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		log.Printf("❌ [%s] 处理超时或已取消", docID)
		_ = s.docRepo.UpdateStatus(docID, model.DocumentStatusFailed, "processing timeout or cancelled")
		return
	default:
	}

	// 拼接完整文件路径
	fullFilePath := filepath.Join(s.uploadBasePath, doc.FilePath)

	// 记录使用的解析器类型
	log.Printf("🔧 [%s] 使用解析器解析 %s 格式", docID, doc.FileType)

	// 直接调用 Tika 解析器解析文档
	tikaParser := parser.GetTikaParser()
	if tikaParser == nil {
		log.Printf("❌ [%s] Tika 解析器未初始化", docID)
		_ = s.docRepo.UpdateStatus(docID, model.DocumentStatusFailed, "Tika parser is not initialized")
		return
	}

	text, err := tikaParser.Parse(fullFilePath)
	if err != nil {
		log.Printf("❌ [%s] 解析文档失败: %v", docID, err)
		_ = s.docRepo.UpdateStatus(docID, model.DocumentStatusFailed, fmt.Sprintf("failed to parse document: %v", err))
		return
	}

	// 清理和验证文本，确保是有效的UTF-8
	text = sanitizeText(text)

	log.Printf("✅ [%s] 文档解析完成，文本长度: %d 字符", docID, len(text))

	// 检查文本大小，如果太大则警告
	if len(text) > 10*1024*1024 { // 10MB
		log.Printf("⚠️ [%s] 警告：文档文本过大 (%d 字符)，处理可能较慢", docID, len(text))
	}

	if strings.TrimSpace(text) == "" {
		_ = s.docRepo.UpdateStatus(docID, model.DocumentStatusFailed, "document content is empty")
		return
	}

	// 2. 分块（流式处理，避免一次性创建所有对象）
	log.Printf("📦 [%s] 开始分块处理", docID)

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		log.Printf("❌ [%s] 处理超时或已取消", docID)
		_ = s.docRepo.UpdateStatus(docID, model.DocumentStatusFailed, "processing timeout or cancelled")
		return
	default:
	}

	// 3. 流式分块并分批保存到数据库（内存优化版本）
	log.Printf("💾 [%s] 开始分块并保存到数据库，文本长度: %d 字符", docID, len(text))
	saveBatchSize := 50 // 每批保存50个分块
	batchChunks := make([]*model.DocumentChunk, 0, saveBatchSize)
	chunkIndex := 0
	totalChunks := 0

	// 使用流式处理，边分块边保存，避免一次性创建所有chunks
	chunkErr := s.chunkService.ChunkTextStreaming(text, func(chunk Chunk) error {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return fmt.Errorf("处理超时或已取消")
		default:
		}

		// 创建数据库对象
		dbChunk := &model.DocumentChunk{
			ID:            ulid.Make().String(),
			DocID:         docID,
			KBID:          doc.KBID,
			ChunkIndex:    chunkIndex,
			Content:       chunk.Content,
			ContentLength: len(chunk.Content),
			VectorStatus:  model.VectorStatusPending,
			CreateTime:    time.Now(),
			UpdateTime:    time.Now(),
		}

		// 记录元数据信息（可用于日志）
		if len(chunk.Metadata) > 0 {
			if chapter, ok := chunk.Metadata["chapter"]; ok {
				log.Printf("📑 [%s] Chunk %d: 章节=%s", docID, chunkIndex, chapter)
			}
			if section, ok := chunk.Metadata["section"]; ok {
				log.Printf("📄 [%s] Chunk %d: 小节=%s", docID, chunkIndex, section)
			}
		}

		batchChunks = append(batchChunks, dbChunk)
		chunkIndex++
		totalChunks++

		// 达到批次大小时，批量保存
		if len(batchChunks) >= saveBatchSize {
			if err := s.chunkRepo.CreateBatch(batchChunks); err != nil {
				return fmt.Errorf("保存分块失败: %w", err)
			}
			log.Printf("💾 [%s] 已保存 %d 个分块", docID, totalChunks)
			batchChunks = batchChunks[:0] // 清空切片，保留容量
		}

		return nil
	})

	if chunkErr != nil {
		log.Printf("❌ [%s] 分块处理失败: %v", docID, chunkErr)
		_ = s.docRepo.UpdateStatus(docID, model.DocumentStatusFailed, fmt.Sprintf("chunking failed: %v", chunkErr))
		return
	}

	// 保存剩余的分块
	if len(batchChunks) > 0 {
		if err := s.chunkRepo.CreateBatch(batchChunks); err != nil {
			log.Printf("❌ [%s] 保存最后一批分块失败: %v", docID, err)
			_ = s.docRepo.UpdateStatus(docID, model.DocumentStatusFailed, fmt.Sprintf("failed to save final chunks: %v", err))
			return
		}
		log.Printf("💾 [%s] 已保存最后 %d 个分块", docID, len(batchChunks))
	}

	if totalChunks == 0 {
		log.Printf("❌ [%s] 分块失败：未生成分块", docID)
		_ = s.docRepo.UpdateStatus(docID, model.DocumentStatusFailed, "no chunks generated")
		return
	}

	log.Printf("✅ [%s] 分块完成，共 %d 个分块", docID, totalChunks)

	// 更新分块数量
	_ = s.docRepo.UpdateChunkCount(docID, totalChunks)

	// 4. 向量化并插入 Milvus（批量处理，从数据库分批查询）
	log.Printf("🔢 [%s] 开始向量化处理，共 %d 个分块", docID, totalChunks)

	// 从数据库分批查询待向量化分块
	vectorBatchSize := 10 // 每批处理10个分块
	totalBatches := (totalChunks + vectorBatchSize - 1) / vectorBatchSize
	processedCount := 0

	for {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			log.Printf("❌ [%s] 处理超时或已取消", docID)
			_ = s.docRepo.UpdateStatus(docID, model.DocumentStatusFailed, "processing timeout or cancelled")
			return
		default:
		}

		// 查询当前文档的待处理分块（带limit）
		pendingChunks, err := s.chunkRepo.FindPendingVectorsByDocID(docID, vectorBatchSize)
		if err != nil {
			log.Printf("❌ [%s] 查询待向量化分块失败: %v", docID, err)
			break
		}

		if len(pendingChunks) == 0 {
			break // 没有更多待处理的分块
		}

		batchNum := processedCount/vectorBatchSize + 1
		log.Printf("🔄 [%s] 处理批次 %d/%d (%d 个分块)", docID, batchNum, totalBatches, len(pendingChunks))

		if err := s.processChunkBatch(ctx, pendingChunks, collectionName); err != nil {
			log.Printf("❌ [%s] 批次 %d 处理失败: %v", docID, batchNum, err)
			// 记录错误但继续处理
			_ = s.chunkRepo.UpdateVectorStatusBatch(
				getChunkIDs(pendingChunks),
				model.VectorStatusFailed,
			)
		} else {
			// 更新向量化状态
			_ = s.chunkRepo.UpdateVectorStatusBatch(
				getChunkIDs(pendingChunks),
				model.VectorStatusCompleted,
			)
			log.Printf("✅ [%s] 批次 %d/%d 处理完成", docID, batchNum, totalBatches)
		}

		processedCount += len(pendingChunks)

		// 如果已处理完所有分块，退出
		if processedCount >= totalChunks {
			break
		}
	}

	// 5. 更新文档状态为已完成
	log.Printf("🎉 [%s] 文档处理完成", docID)
	_ = s.docRepo.UpdateStatus(docID, model.DocumentStatusCompleted, "")
}

// processChunkBatch 批量处理分块（向量化并插入 Milvus）
func (s *DocumentService) processChunkBatch(
	ctx context.Context,
	chunks []*model.DocumentChunk,
	collectionName string,
) error {
	// 提取文本内容
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	// 批量向量化
	vectors, err := s.embeddingSvc.EmbedTexts(texts)
	if err != nil {
		return fmt.Errorf("failed to embed texts: %w", err)
	}

	if len(vectors) != len(chunks) {
		return fmt.Errorf("vector count mismatch")
	}

	// 准备元数据
	ids := make([]string, len(chunks))
	metadatas := make([]map[string]interface{}, len(chunks))
	for i, chunk := range chunks {
		ids[i] = chunk.ID
		metadatas[i] = map[string]interface{}{
			"chunk_id": int64(chunk.ChunkIndex),
			"doc_id":   chunk.DocID,
			"kb_id":    chunk.KBID,
			"content":  chunk.Content,
		}
	}

	// 插入 Milvus
	if err := s.vectorMgr.InsertVectors(ctx, collectionName, vectors, ids, metadatas); err != nil {
		return fmt.Errorf("failed to insert vectors: %w", err)
	}

	return nil
}

// getChunkIDs 获取分块ID列表
func getChunkIDs(chunks []*model.DocumentChunk) []string {
	ids := make([]string, len(chunks))
	for i, chunk := range chunks {
		ids[i] = chunk.ID
	}
	return ids
}

// GetDocumentByID 根据ID获取文档
func (s *DocumentService) GetDocumentByID(id string) (*model.Document, error) {
	return s.docRepo.FindByID(id)
}

// GetDocumentProgress 获取文档处理进度
func (s *DocumentService) GetDocumentProgress(docID string) (map[string]interface{}, error) {
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return nil, err
	}

	// 统计向量化进度
	totalChunks, _ := s.chunkRepo.CountByDocID(docID)
	completedChunks, _ := s.chunkRepo.CountByVectorStatus(docID, model.VectorStatusCompleted)
	failedChunks, _ := s.chunkRepo.CountByVectorStatus(docID, model.VectorStatusFailed)
	pendingChunks, _ := s.chunkRepo.CountByVectorStatus(docID, model.VectorStatusPending)

	// 计算进度百分比
	var progressPercent float64
	if totalChunks > 0 {
		progressPercent = float64(completedChunks) / float64(totalChunks) * 100
	}

	return map[string]interface{}{
		"doc_id":           docID,
		"status":           doc.Status,
		"chunk_count":      totalChunks,
		"completed_count":  completedChunks,
		"failed_count":     failedChunks,
		"pending_count":    pendingChunks,
		"progress_percent": progressPercent,
		"error_message":    doc.ErrorMessage,
	}, nil
}

// ListDocuments 分页查询文档列表
func (s *DocumentService) ListDocuments(kbID string, page, pageSize int) ([]*model.Document, int64, error) {
	return s.docRepo.FindByKBID(kbID, page, pageSize)
}

// DeleteDocument 删除文档
func (s *DocumentService) DeleteDocument(docID string) error {
	ctx := context.Background()

	// 获取文档信息
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return err
	}

	// 获取知识库信息
	kb, err := s.kbRepo.FindByID(doc.KBID)
	if err != nil {
		return err
	}

	// 删除 Milvus 中的向量
	if err := s.vectorMgr.DeleteVectorsByDocID(ctx, kb.CollectionName, docID); err != nil {
		// 记录错误但不阻止删除
		fmt.Printf("Warning: failed to delete vectors from Milvus: %v\n", err)
	}

	// 删除数据库中的分块
	if err := s.chunkRepo.DeleteByDocID(docID); err != nil {
		return fmt.Errorf("failed to delete chunks: %w", err)
	}

	// 删除文档（软删除）
	if err := s.docRepo.Delete(docID); err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	// 删除文件
	if doc.FilePath != "" {
		fullPath := filepath.Join(s.uploadBasePath, doc.FilePath)
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: failed to delete file: %v\n", err)
		}
	}

	return nil
}

// SaveUploadedFile 保存上传的文件
func (s *DocumentService) SaveUploadedFile(fileData []byte, fileName string) (string, error) {
	// 创建上传目录（按日期）
	dateDir := time.Now().Format("2006/01/02")
	uploadDir := filepath.Join(s.uploadBasePath, dateDir)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	// 生成唯一文件名
	fileExt := filepath.Ext(fileName)
	fileBaseName := strings.TrimSuffix(fileName, fileExt)
	uniqueFileName := fmt.Sprintf("%s_%s%s", fileBaseName, ulid.Make().String(), fileExt)
	filePath := filepath.Join(dateDir, uniqueFileName)
	fullPath := filepath.Join(s.uploadBasePath, filePath)

	// 保存文件
	if err := os.WriteFile(fullPath, fileData, 0644); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	return filePath, nil
}

// GetFileType 根据文件名获取文件类型
func (s *DocumentService) GetFileType(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	if len(ext) > 0 {
		ext = ext[1:] // 去掉点号
	}
	return ext
}

// sanitizeText 清理和验证文本，确保是有效的UTF-8
func sanitizeText(text string) string {
	if text == "" {
		return text
	}

	// 检查是否是有效的UTF-8
	if utf8.ValidString(text) {
		return text
	}

	// 如果不是有效的UTF-8，尝试修复
	var result strings.Builder
	result.Grow(len(text))

	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		if r == utf8.RuneError && size == 1 {
			// 跳过无效的字节，替换为空格
			result.WriteRune(' ')
			text = text[1:]
			continue
		}
		result.WriteRune(r)
		text = text[size:]
	}

	return result.String()
}
