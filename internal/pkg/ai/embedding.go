package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"ragent-go/internal/config"
)

// EmbeddingService Embedding 服务
type EmbeddingService struct {
	cfg *config.Config
}

// NewEmbeddingService 创建 Embedding 服务
func NewEmbeddingService(cfg *config.Config) *EmbeddingService {
	return &EmbeddingService{cfg: cfg}
}

// EmbeddingRequest Embedding 请求
type EmbeddingRequest struct {
	Model string `json:"model"`
	Input struct {
		Texts []string `json:"texts"`
	} `json:"input"`
}

// EmbeddingResponse Embedding 响应
type EmbeddingResponse struct {
	Output struct {
		Embeddings []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"embeddings"`
	} `json:"output"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// EmbedText 对文本进行向量化
func (s *EmbeddingService) EmbedText(text string) ([]float32, error) {
	embeddings, err := s.EmbedTexts([]string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}
	return embeddings[0], nil
}

// EmbedTexts 批量对文本进行向量化
func (s *EmbeddingService) EmbedTexts(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts is empty")
	}

	// 使用 Dashscope API
	apiKey := s.cfg.AI.Dashscope.APIKey
	baseURL := s.cfg.AI.Dashscope.BaseURL
	model := s.cfg.AI.Embedding.Model

	if apiKey == "" {
		return nil, fmt.Errorf("dashscope api key is not configured")
	}

	// 构建请求（DashScope API 格式）
	reqBody := EmbeddingRequest{
		Model: model,
	}
	reqBody.Input.Texts = texts

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送请求
	// DashScope API 路径格式：/api/v1/services/embeddings/text-embedding/text-embedding
	url := fmt.Sprintf("%s/services/embeddings/text-embedding/text-embedding", baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("X-DashScope-SSE", "disable")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// 解析响应
	var embeddingResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 提取向量
	result := make([][]float32, len(embeddingResp.Output.Embeddings))
	for i, emb := range embeddingResp.Output.Embeddings {
		result[i] = emb.Embedding
	}

	// 检测并记录向量维度（用于调试）
	if len(result) > 0 && len(result[0]) > 0 {
		actualDim := len(result[0])
		expectedDim := s.cfg.AI.Embedding.Dimension
		if expectedDim > 0 && actualDim != expectedDim {
			fmt.Printf("⚠️  Warning: Embedding dimension mismatch! Expected: %d, Actual: %d (Model: %s)\n",
				expectedDim, actualDim, model)
			fmt.Printf("💡 Please update config.yaml embedding.dimension to %d\n", actualDim)
		} else if expectedDim == 0 {
			fmt.Printf("ℹ️  Detected embedding dimension: %d (Model: %s)\n", actualDim, model)
		}
	}

	return result, nil
}
