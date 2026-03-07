package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"ragent-go/internal/config"
	"strings"
)

// LLMService LLM服务
type LLMService struct {
	cfg *config.Config
}

// NewLLMService 创建LLM服务
func NewLLMService(cfg *config.Config) *LLMService {
	return &LLMService{cfg: cfg}
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"`    // system, user, assistant
	Content string `json:"content"` // 消息内容
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// Chat 调用LLM生成回答
func (s *LLMService) Chat(req ChatRequest) (string, error) {
	// 使用DeepSeek API
	apiKey := s.cfg.AI.DeepSeek.APIKey
	baseURL := s.cfg.AI.DeepSeek.BaseURL
	model := s.cfg.AI.DeepSeek.Model

	if apiKey == "" {
		return "", fmt.Errorf("deepseek api key is not configured")
	}

	// 如果没有指定模型，使用配置中的默认模型
	if req.Model == "" {
		req.Model = model
	}

	// 设置默认参数
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 2000
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送请求
	url := fmt.Sprintf("%s/chat/completions", baseURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// 解析响应
	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// BuildRAGPrompt 构建RAG Prompt
func BuildRAGPrompt(question string, contexts []string) []ChatMessage {
	// 组装上下文
	contextText := strings.Join(contexts, "\n\n")

	// 构建系统提示词
	systemPrompt := `你是一个智能助手，能够基于提供的文档内容回答问题。
请仔细阅读以下文档片段，然后回答用户的问题。
如果文档中没有相关信息，请诚实地说"根据提供的文档，我无法找到相关信息"。
回答要准确、简洁，尽量使用文档中的原话。`

	// 构建用户消息
	userMessage := fmt.Sprintf(`文档内容：
%s

问题：%s

请基于以上文档内容回答问题。`, contextText, question)

	return []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}
}
