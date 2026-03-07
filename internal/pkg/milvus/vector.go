package milvus

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// VectorManager Milvus 向量管理器
type VectorManager struct {
	client client.Client
}

// NewVectorManager 创建向量管理器
func NewVectorManager(milvusClient client.Client) *VectorManager {
	return &VectorManager{
		client: milvusClient,
	}
}

// InsertVectors 插入向量数据
// collectionName: Collection 名称
// vectors: 向量数据（二维数组）
// ids: 向量ID（chunk_id）
// metadatas: 元数据（doc_id, kb_id, content等）
func (m *VectorManager) InsertVectors(
	ctx context.Context,
	collectionName string,
	vectors [][]float32,
	ids []string,
	metadatas []map[string]interface{},
) error {
	if len(vectors) == 0 {
		return fmt.Errorf("vectors is empty")
	}
	if len(vectors) != len(ids) {
		return fmt.Errorf("vectors and ids length mismatch")
	}
	if len(vectors) != len(metadatas) {
		return fmt.Errorf("vectors and metadatas length mismatch")
	}

	// 准备数据
	dim := len(vectors[0])
	vectorData := make([][]float32, len(vectors))
	for i, v := range vectors {
		if len(v) != dim {
			return fmt.Errorf("vector dimension mismatch at index %d", i)
		}
		vectorData[i] = v
	}

	// 构建插入数据
	insertData := make([]entity.Column, 0, 6)

	// ID 列
	idColumn := entity.NewColumnVarChar("id", ids)
	insertData = append(insertData, idColumn)

	// Vector 列
	vectorColumn := entity.NewColumnFloatVector("vector", dim, vectorData)
	insertData = append(insertData, vectorColumn)

	// 元数据列
	chunkIDs := make([]int64, len(ids))
	docIDs := make([]string, len(ids))
	kbIDs := make([]string, len(ids))
	contents := make([]string, len(ids))

	for i, metadata := range metadatas {
		if chunkID, ok := metadata["chunk_id"].(int64); ok {
			chunkIDs[i] = chunkID
		}
		if docID, ok := metadata["doc_id"].(string); ok {
			docIDs[i] = docID
		}
		if kbID, ok := metadata["kb_id"].(string); ok {
			kbIDs[i] = kbID
		}
		if content, ok := metadata["content"].(string); ok {
			contents[i] = content
		}
	}

	insertData = append(insertData, entity.NewColumnInt64("chunk_id", chunkIDs))
	insertData = append(insertData, entity.NewColumnVarChar("doc_id", docIDs))
	insertData = append(insertData, entity.NewColumnVarChar("kb_id", kbIDs))
	insertData = append(insertData, entity.NewColumnVarChar("content", contents))

	// 插入数据
	_, err := m.client.Insert(ctx, collectionName, "", insertData...)
	if err != nil {
		return fmt.Errorf("failed to insert vectors: %w", err)
	}

	return nil
}

// DeleteVectorsByDocID 根据文档ID删除向量
func (m *VectorManager) DeleteVectorsByDocID(ctx context.Context, collectionName string, docID string) error {
	expr := fmt.Sprintf("doc_id == '%s'", docID)
	return m.client.Delete(ctx, collectionName, "", expr)
}

// DeleteVectorsByKBID 根据知识库ID删除向量
func (m *VectorManager) DeleteVectorsByKBID(ctx context.Context, collectionName string, kbID string) error {
	expr := fmt.Sprintf("kb_id == '%s'", kbID)
	return m.client.Delete(ctx, collectionName, "", expr)
}

// RetrievedChunk 检索到的文档分块
type RetrievedChunk struct {
	ID         string  // 分块ID
	ChunkIndex int64   // 分块序号
	DocID      string  // 文档ID
	KBID       string  // 知识库ID
	Content    string  // 分块内容
	Score      float32 // 相似度分数
}

// SearchVectors 向量检索
// collectionName: Collection 名称
// queryVector: 查询向量
// topK: 返回Top-K个结果
// kbID: 可选的知识库ID过滤
func (m *VectorManager) SearchVectors(
	ctx context.Context,
	collectionName string,
	queryVector []float32,
	topK int,
	kbID string,
) ([]RetrievedChunk, error) {
	if len(queryVector) == 0 {
		return nil, fmt.Errorf("query vector is empty")
	}
	if topK <= 0 {
		topK = 5 // 默认返回5个结果
	}

	// 构建搜索参数（使用AUTOINDEX搜索参数）
	searchParams, err := entity.NewIndexAUTOINDEXSearchParam(1) // level=1
	if err != nil {
		return nil, fmt.Errorf("failed to create search param: %w", err)
	}

	// 构建过滤表达式（如果指定了知识库ID）
	var expr string
	if kbID != "" {
		expr = fmt.Sprintf("kb_id == '%s'", kbID)
	}

	// 构建查询向量（转换为entity.Vector类型）
	queryVectors := []entity.Vector{
		entity.FloatVector(queryVector),
	}

	// 执行搜索
	searchResults, err := m.client.Search(
		ctx,
		collectionName,
		[]string{}, // 分区列表（空表示所有分区）
		expr,       // 过滤表达式
		[]string{"id", "chunk_id", "doc_id", "kb_id", "content"}, // 输出字段
		queryVectors,
		"vector",  // 向量字段名
		entity.L2, // 距离类型
		topK,
		searchParams,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search vectors: %w", err)
	}

	// 解析搜索结果
	if len(searchResults) == 0 {
		return []RetrievedChunk{}, nil
	}

	// 取第一个查询的结果（因为我们只查询了一个向量）
	result := searchResults[0]
	if result.Err != nil {
		return nil, fmt.Errorf("search error: %w", result.Err)
	}

	// 获取结果数量
	resultCount := result.ResultCount
	if resultCount == 0 {
		return []RetrievedChunk{}, nil
	}

	chunks := make([]RetrievedChunk, 0, resultCount)

	// 解析IDs列
	idColumn := result.IDs
	idValues := make([]string, resultCount)
	if idColumn != nil {
		for i := 0; i < resultCount; i++ {
			id, err := idColumn.GetAsString(i)
			if err == nil {
				idValues[i] = id
			}
		}
	}

	// 解析Scores
	scores := result.Scores

	// 解析Fields（输出字段）
	fields := result.Fields
	if fields == nil {
		return []RetrievedChunk{}, nil
	}

	// 获取各个字段的列
	chunkIDColumn := fields.GetColumn("chunk_id")
	docIDColumn := fields.GetColumn("doc_id")
	kbIDColumn := fields.GetColumn("kb_id")
	contentColumn := fields.GetColumn("content")

	// 提取各个字段的值
	for i := 0; i < resultCount; i++ {
		chunk := RetrievedChunk{
			ID:    idValues[i],
			Score: scores[i],
		}

		// 提取chunk_id
		if chunkIDColumn != nil {
			if chunkID, err := chunkIDColumn.GetAsInt64(i); err == nil {
				chunk.ChunkIndex = chunkID
			}
		}

		// 提取doc_id
		if docIDColumn != nil {
			if docID, err := docIDColumn.GetAsString(i); err == nil {
				chunk.DocID = docID
			}
		}

		// 提取kb_id
		if kbIDColumn != nil {
			if kbID, err := kbIDColumn.GetAsString(i); err == nil {
				chunk.KBID = kbID
			}
		}

		// 提取content
		if contentColumn != nil {
			if content, err := contentColumn.GetAsString(i); err == nil {
				chunk.Content = content
			}
		}

		chunks = append(chunks, chunk)
	}

	return chunks, nil
}
