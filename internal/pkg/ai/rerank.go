package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"ragent-go/internal/config"
)

// RerankService 调用外部 Cross-Encoder / Rerank 模型（例如 bge-reranker-v2-m3）
type RerankService struct {
	cfg *config.Config
}

// NewRerankService 创建 RerankService
// 如果未启用 rerank（config.ai.rerank.enabled=false），可以传入 nil 表示不启用。
func NewRerankService(cfg *config.Config) *RerankService {
	if cfg == nil || !cfg.AI.Rerank.Enabled || strings.TrimSpace(cfg.AI.Rerank.BaseURL) == "" {
		return nil
	}
	return &RerankService{cfg: cfg}
}

// rerankRequest / rerankResponse 定义与外部重排服务的协议
// 你可以根据自己部署的 bge-reranker 服务调整字段。
type rerankRequest struct {
	Model    string   `json:"model"`
	Query    string   `json:"query"`
	Passages []string `json:"passages"`
	TopK     int      `json:"top_k,omitempty"`
}

type rerankResponse struct {
	// Scores 与输入 passages 一一对应的相关性分数
	Scores []float32 `json:"scores"`
}

// Rerank 调用外部 Rerank 服务，根据 query 对 passages 打分并返回按分数从高到低排序后的索引列表。
func (s *RerankService) Rerank(query string, passages []string, topK int) ([]int, error) {
	if s == nil {
		return nil, fmt.Errorf("rerank service is nil")
	}
	if len(passages) == 0 {
		return []int{}, nil
	}

	cfg := s.cfg.AI.Rerank
	model := cfg.Model
	if strings.TrimSpace(model) == "" {
		model = "BAAI/bge-reranker-v2-m3"
	}

	reqBody := rerankRequest{
		Model:    model,
		Query:    query,
		Passages: passages,
	}
	if topK > 0 {
		reqBody.TopK = topK
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rerank request: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/rerank"
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create rerank request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call rerank service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rerank service error: status=%d", resp.StatusCode)
	}

	var rr rerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return nil, fmt.Errorf("failed to decode rerank response: %w", err)
	}
	if len(rr.Scores) == 0 {
		return []int{}, nil
	}

	// 将分数转成索引排序
	type scored struct {
		Idx   int
		Score float32
	}
	items := make([]scored, 0, len(rr.Scores))
	for i, s := range rr.Scores {
		items = append(items, scored{Idx: i, Score: s})
	}

	// 按分数降序排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})

	if topK <= 0 || topK > len(items) {
		topK = len(items)
	}
	if len(items) > topK {
		items = items[:topK]
	}

	result := make([]int, len(items))
	for i, it := range items {
		result[i] = it.Idx
	}
	return result, nil
}
