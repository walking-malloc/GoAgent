package service

import (
	"encoding/json"
	"fmt"
	"os"
	"ragent-go/internal/pkg/ai"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// IntentNode 意图节点定义（从 YAML 配置加载）
type IntentNode struct {
	ID             string   `yaml:"id"`              // 意图唯一ID
	Path           string   `yaml:"path"`            // 意图路径
	Description    string   `yaml:"description"`     // 意图描述
	Examples       []string `yaml:"examples"`        // 示例问句
	KBID           string   `yaml:"kb_id"`           // 关联的知识库ID（可选）
	CollectionName string   `yaml:"collection_name"` // 关联的 Milvus Collection 名称（可选）
}

// IntentScore 意图识别结果
type IntentScore struct {
	ID          string  `json:"id"`
	Path        string  `json:"path"`
	Description string  `json:"description"`
	KBID        string  `json:"kb_id,omitempty"`
	Collection  string  `json:"collection_name,omitempty"`
	Score       float64 `json:"score"`
	Reason      string  `json:"reason,omitempty"`
}

// IntentService LLM 意图识别服务（简化版本）
type IntentService struct {
	llm     *ai.LLMService
	intents []IntentNode
}

// intentConfig 用于反序列化 YAML 配置
type intentConfig struct {
	Intents []IntentNode `yaml:"intents"`
}

// NewIntentService 创建意图识别服务，从 YAML 配置加载意图列表。
// 默认读取路径为 ./configs/intents.yaml
func NewIntentService(llm *ai.LLMService) *IntentService {
	if llm == nil {
		return nil
	}

	data, err := os.ReadFile("configs/intents.yaml")
	if err != nil {
		// 配置缺失时返回空服务，后续调用时给出友好错误
		return &IntentService{llm: llm, intents: []IntentNode{}}
	}

	var cfg intentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return &IntentService{llm: llm, intents: []IntentNode{}}
	}

	return &IntentService{
		llm:     llm,
		intents: cfg.Intents,
	}
}

// Classify 使用 LLM 对用户问题进行意图识别，返回按 score 降序排序的意图列表
func (s *IntentService) Classify(question string, topK int, minScore float64) ([]IntentScore, error) {
	if s == nil {
		return nil, fmt.Errorf("intent service is nil")
	}
	if strings.TrimSpace(question) == "" {
		return nil, fmt.Errorf("question is empty")
	}
	if topK <= 0 {
		topK = 5
	}

	systemPrompt := s.buildSystemPrompt()
	userPrompt := fmt.Sprintf(`请根据上面的意图列表，对下面这个问题进行意图打分：

问题：%s

请严格按照以下 JSON 数组格式输出，多余内容不要返回：
[
  {"id": "意图ID", "score": 0.0, "reason": "简要说明"},
  ...
]

注意：
- 只允许使用给定意图列表中的 id
- score 取值范围为 0.0 ~ 1.0
- 如果问题完全无关，可以不给该意图返回记录。`, question)

	req := ai.ChatRequest{
		Messages: []ai.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.1,
		MaxTokens:   512,
	}

	raw, err := s.llm.Chat(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM for intent classify: %w", err)
	}

	raw = strings.TrimSpace(raw)
	// 去掉可能的 ```json ``` 包裹
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```JSON")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var resp []struct {
		ID     string  `json:"id"`
		Score  float64 `json:"score"`
		Reason string  `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		// 解析失败时，为了健壮性直接返回空结果，而不是报错中断
		return []IntentScore{}, nil
	}

	// 构造结果，并过滤/排序
	intentMap := make(map[string]IntentNode, len(s.intents))
	for _, n := range s.intents {
		intentMap[n.ID] = n
	}

	results := make([]IntentScore, 0, len(resp))
	for _, item := range resp {
		if item.ID == "" {
			continue
		}
		if item.Score < minScore {
			continue
		}
		node, ok := intentMap[item.ID]
		if !ok {
			continue
		}
		results = append(results, IntentScore{
			ID:          node.ID,
			Path:        node.Path,
			Description: node.Description,
			KBID:        node.KBID,
			Collection:  node.CollectionName,
			Score:       item.Score,
			Reason:      item.Reason,
		})
	}

	if len(results) == 0 {
		return results, nil
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// buildSystemPrompt 拼接意图列表，提供给 LLM 作为系统提示
func (s *IntentService) buildSystemPrompt() string {
	var b strings.Builder
	b.WriteString("你是一个企业知识助手的意图分类器，任务是：\n")
	b.WriteString("给定用户的问题，从下面的【意图列表】中选择相关的意图，并为每个相关意图打一个 0.0~1.0 的相似度分数。\n")
	b.WriteString("意图列表如下（id / path / description / examples）：\n\n")

	for _, n := range s.intents {
		b.WriteString("- id=")
		b.WriteString(n.ID)
		b.WriteString("\n  path=")
		b.WriteString(n.Path)
		if n.Description != "" {
			b.WriteString("\n  description=")
			b.WriteString(n.Description)
		}
		if len(n.Examples) > 0 {
			b.WriteString("\n  examples=")
			b.WriteString(strings.Join(n.Examples, " / "))
		}
		b.WriteString("\n\n")
	}

	b.WriteString("请只在上述 id 中选择，按 JSON 数组形式输出结果，不要返回其它自然语言解释。\n")
	return b.String()
}
