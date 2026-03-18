package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"ragent-go/internal/pkg/ai"
	"ragent-go/internal/pkg/milvus"
	"ragent-go/internal/repository"
)

// RetrievalService 检索服务（支持向量 + 关键词召回、RRF 融合）
type RetrievalService struct {
	embeddingSvc *ai.EmbeddingService
	vectorMgr    *milvus.VectorManager
	chunkRepo    *repository.DocumentChunkRepository
	rerankSvc    *ai.RerankService
}

// NewRetrievalService 创建检索服务
func NewRetrievalService(
	embeddingSvc *ai.EmbeddingService,
	vectorMgr *milvus.VectorManager,
	chunkRepo *repository.DocumentChunkRepository,
	rerankSvc *ai.RerankService,
) *RetrievalService {
	return &RetrievalService{
		embeddingSvc: embeddingSvc,
		vectorMgr:    vectorMgr,
		chunkRepo:    chunkRepo,
		rerankSvc:    rerankSvc,
	}
}

// RetrieveRequest 检索请求
type RetrieveRequest struct {
	Query          string // 查询文本
	CollectionName string // Collection名称
	TopK           int    // 返回Top-K个结果
	KBID           string // 可选的知识库ID过滤
}

// Retrieve 检索相关文档片段
func (s *RetrievalService) Retrieve(ctx context.Context, req RetrieveRequest) ([]milvus.RetrievedChunk, error) {
	if req.Query == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if req.CollectionName == "" {
		return nil, fmt.Errorf("collection name is empty")
	}
	if req.TopK <= 0 {
		req.TopK = 5 // 默认返回5个结果
	}

	// 为了给 RRF 融合更多候选，这里适当放大召回数量
	vecTopK := req.TopK * 3
	kwTopK := req.TopK * 3
	if vecTopK < req.TopK {
		vecTopK = req.TopK
	}
	if kwTopK < req.TopK {
		kwTopK = req.TopK
	}

	// 1. 向量召回
	queryVector, err := s.embeddingSvc.EmbedText(req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}
	vectorChunks, err := s.vectorMgr.SearchVectors(ctx, req.CollectionName, queryVector, vecTopK, req.KBID)
	if err != nil {
		return nil, fmt.Errorf("failed to search vectors: %w", err)
	}

	// 2. 关键词召回（基于 MySQL 文本 LIKE，作为一个简单的 BM25 近似）
	var keywordChunks []milvus.RetrievedChunk
	if s.chunkRepo != nil {
		if chunks, err := s.chunkRepo.SearchByKeyword(req.KBID, req.Query, kwTopK); err == nil {
			keywordChunks = make([]milvus.RetrievedChunk, 0, len(chunks))
			for _, c := range chunks {
				keywordChunks = append(keywordChunks, milvus.RetrievedChunk{
					ID:         c.ID,
					ChunkIndex: int64(c.ChunkIndex),
					DocID:      c.DocID,
					KBID:       c.KBID,
					Content:    c.Content,
					// 关键词通道的原始分数先留空，后续在 RRF 中按排名折算
					Score: 0,
				})
			}
		}
	}

	// 3. 使用 RRF 融合多通道召回结果，直接返回 TopK
	candidates := rrfMerge(vectorChunks, keywordChunks, req.TopK*3)
	if len(candidates) == 0 {
		return []milvus.RetrievedChunk{}, nil
	}

	// 直接返回 RRF 融合后的 TopK 结果，不再进行重排
	if len(candidates) > req.TopK {
		return candidates[:req.TopK], nil
	}
	return candidates, nil
}

// rrfMerge 使用 Reciprocal Rank Fusion 将多个通道的结果融合
// 参考公式：score(d) = Σ 1 / (k + rank_d,channel)
func rrfMerge(
	vectorChunks []milvus.RetrievedChunk,
	keywordChunks []milvus.RetrievedChunk,
	maxCandidates int,
) []milvus.RetrievedChunk {
	const k = 60.0

	type scored struct {
		chunk milvus.RetrievedChunk
		score float64
	}

	index := make(map[string]*scored)

	// 辅助函数：根据列表顺序累加 RRF 分数
	accumulate := func(chunks []milvus.RetrievedChunk) {
		for rank, c := range chunks {
			id := c.ID
			if id == "" {
				// 兜底：如果没有 ID，用 doc_id+chunk_index 作为 key
				id = fmt.Sprintf("%s#%d", c.DocID, c.ChunkIndex)
			}
			s, ok := index[id] //第一次出现
			if !ok {
				copyChunk := c
				s = &scored{chunk: copyChunk}
				index[id] = s
			}
			s.score += 1.0 / (k + float64(rank+1))
		}
	}

	accumulate(vectorChunks)
	accumulate(keywordChunks)

	out := make([]scored, 0, len(index))
	for _, s := range index {
		out = append(out, *s)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].score > out[j].score
	})

	if maxCandidates > 0 && len(out) > maxCandidates {
		out = out[:maxCandidates]
	}

	result := make([]milvus.RetrievedChunk, len(out))
	for i, s := range out {
		// 把融合后的 RRF 分数写回 Score 字段，便于调试
		s.chunk.Score = float32(s.score)
		result[i] = s.chunk
	}
	return result
}

// crossEncoderRerank 使用外部 Cross-Encoder / Rerank 模型（如 bge-reranker-v2-m3）对候选 chunks 做精排
func (s *RetrievalService) crossEncoderRerank(
	ctx context.Context,
	query string,
	candidates []milvus.RetrievedChunk,
	topK int,
) ([]milvus.RetrievedChunk, error) {
	if len(candidates) == 0 {
		return []milvus.RetrievedChunk{}, nil
	}

	if topK <= 0 || topK > len(candidates) {
		topK = len(candidates)
	}

	// 如果未配置外部重排服务，使用轻量级本地重排（关键词覆盖/命中数）兜底
	if s.rerankSvc == nil {
		return simpleKeywordRerank(query, candidates, topK), nil
	}

	// 提取候选片段文本
	passages := make([]string, len(candidates))
	for i, c := range candidates {
		passages[i] = strings.TrimSpace(c.Content)
	}

	// 调用外部 rerank 服务获取排序后的索引
	idxs, err := s.rerankSvc.Rerank(query, passages, topK)
	if err != nil || len(idxs) == 0 {
		// 重排失败时，使用轻量级本地重排兜底（比直接回退 RRF 稳一点）
		return simpleKeywordRerank(query, candidates, topK), nil
	}

	seen := make(map[int]bool)
	result := make([]milvus.RetrievedChunk, 0, topK)
	for _, idx := range idxs {
		if idx < 0 || idx >= len(candidates) {
			continue
		}
		if seen[idx] {
			continue
		}
		seen[idx] = true
		result = append(result, candidates[idx])
		if len(result) >= topK {
			break
		}
	}
	if len(result) == 0 {
		if len(candidates) > topK {
			return candidates[:topK], nil
		}
		return candidates, nil
	}
	return result, nil
}

var nonWordRegexp = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// simpleKeywordRerank 一个非常轻量的本地重排：
// - 将 query 分词为 token（按非字母数字切分，兼容中英文/数字）
// - 对每个候选计算：命中 token 数 / token 总数（覆盖率） + 命中数微弱加权
// - 作为兜底方案：无需外部服务、无额外依赖，效果通常优于纯 RRF 回退
func simpleKeywordRerank(query string, candidates []milvus.RetrievedChunk, topK int) []milvus.RetrievedChunk {
	qTokens := tokenizeForRerank(query)
	if len(qTokens) == 0 {
		// query 没有可用 token，保持原顺序（RRF）
		if topK > 0 && len(candidates) > topK {
			return candidates[:topK]
		}
		return candidates
	}

	type scored struct {
		chunk milvus.RetrievedChunk
		score float64
	}
	items := make([]scored, 0, len(candidates))

	for _, c := range candidates {
		content := strings.ToLower(strings.TrimSpace(c.Content))
		hits := 0
		seen := make(map[string]struct{}, len(qTokens))
		for _, t := range qTokens {
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			if t == "" {
				continue
			}
			if strings.Contains(content, t) {
				hits++
			}
		}

		coverage := float64(hits) / float64(len(seen))
		// 让“覆盖率”主导，同时给命中数一点点加成，避免覆盖率相同全打平
		score := coverage + float64(hits)*0.0001

		cc := c
		cc.Score = float32(score) // 写回便于观察
		items = append(items, scored{chunk: cc, score: score})
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	if topK <= 0 || topK > len(items) {
		topK = len(items)
	}
	out := make([]milvus.RetrievedChunk, 0, topK)
	for i := 0; i < topK; i++ {
		out = append(out, items[i].chunk)
	}
	return out
}

func tokenizeForRerank(s string) []string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return nil
	}
	parts := nonWordRegexp.Split(s, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 过滤掉很短的噪声 token（对中文单字保留价值不大；这里也会过滤掉英文单字）
		// 你如果希望更“召回友好”，可以把阈值调到 1。
		if len([]rune(p)) < 2 {
			continue
		}
		out = append(out, p)
	}
	return out
}
