package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"ragent-go/internal/pkg/ai"
)

// QueryRewriteResult 问题重写结果
// - RewrittenQuestion：改写后的标准问题（始终有值，失败则退回到原始问题的简单归一化）
// - SubQuestions：如果模型返回的是 JSON 数组子问题列表，则放在这里；否则为空
// - Raw：模型原始输出，便于调试
type QueryRewriteResult struct {
	RewrittenQuestion string   `json:"rewritten_question"`
	SubQuestions      []string `json:"sub_questions,omitempty"`
	Raw               string   `json:"raw,omitempty"`
}

// QueryRewriteService 基于 LLM 的问题重写 / 拆分服务
// 核心思路（按你的要求简化为）：
// - Prompt 中带：最近几轮对话（通过向量检索出来的历史 Q+A 文本片段）+ 当前原始问题
// - 要求模型输出：
//   - 要么是「改写后的单个标准问题字符串」
//   - 要么是 JSON 数组形式的子问题列表：["子问题1", "子问题2", ...]
//
// - 解析失败或调用异常时，兜底返回归一化后的原始问题
type QueryRewriteService struct {
	llm *ai.LLMService
}

// NewQueryRewriteService 创建问题重写服务
func NewQueryRewriteService(llm *ai.LLMService) *QueryRewriteService {
	if llm == nil {
		return nil
	}
	return &QueryRewriteService{llm: llm}
}

// RewriteQuestion 问题重写 / 拆分
//
// recentQAPairs：最近若干轮对话的 Q+A 文本片段（一般来自向量检索的记忆内容，如 "Q: ...\nA: ..."）
// question：当前用户原始问题
//
// 返回：
// - RewrittenQuestion：用于检索 / 意图识别的主问题（必定非空）
// - SubQuestions：如果模型返回 JSON 数组子问题，则放在这里，否则为空
// - Raw：模型原始输出
//
// 注意：
// - 为了保证鲁棒性，LLM 调用异常或解析失败时，不会返回 error，而是使用归一化后的原始问题兜底。
func (s *QueryRewriteService) RewriteQuestion(
	recentQAPairs []string,
	question string,
) (*QueryRewriteResult, error) {
	if s == nil {
		return nil, fmt.Errorf("query rewrite service is nil")
	}
	q := strings.TrimSpace(question)
	if q == "" {
		return nil, fmt.Errorf("question is empty")
	}

	systemPrompt := s.buildSystemPrompt()
	userPrompt := buildUserPrompt(recentQAPairs, q)
	fmt.Printf("systemPrompt: %s", systemPrompt)
	fmt.Printf("userPrompt: %s", userPrompt)
	req := ai.ChatRequest{
		Messages: []ai.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.2,
		MaxTokens:   512,
	}

	raw, err := s.llm.Chat(req)
	fmt.Printf("raw: %s", raw)
	if err != nil {
		// LLM 调用失败：兜底为简单归一化后的原始问题
		return &QueryRewriteResult{
			RewrittenQuestion: normalizeQuestion(q),
			SubQuestions:      nil,
			Raw:               "",
		}, nil
	}

	raw = strings.TrimSpace(raw)

	// 去掉可能的 ```json / ``` 包裹
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```JSON")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	result := &QueryRewriteResult{
		RewrittenQuestion: normalizeQuestion(q), // 先用兜底值，后面再根据解析结果覆盖
		SubQuestions:      nil,
		Raw:               raw,
	}

	// 尝试解析为 JSON 数组子问题列表
	if strings.HasPrefix(raw, "[") {
		var subs []string
		if err := json.Unmarshal([]byte(raw), &subs); err == nil && len(subs) > 0 {
			// 解析成功：认为这是子问题拆分结果
			cleanSubs := make([]string, 0, len(subs))
			for _, s := range subs {
				s = strings.TrimSpace(s)
				if s != "" {
					cleanSubs = append(cleanSubs, s)
				}
			}
			if len(cleanSubs) > 0 {
				result.SubQuestions = cleanSubs
				// 可以约定主问题使用第一个子问题，或者仍然使用原始问题的归一化形式
				result.RewrittenQuestion = cleanSubs[0]
				return result, nil
			}
		}
	}

	// 否则认为是单个改写后问题字符串
	if raw != "" {
		result.RewrittenQuestion = normalizeQuestion(raw)
	}

	return result, nil
}

func (s *QueryRewriteService) buildSystemPrompt() string {
	// 目前直接返回常量字符串，后续如需迁移到外部模板文件，只需在此处替换为文件加载逻辑。
	return queryRewriteSystemPrompt
}

// buildUserPrompt 构建用户侧提示词，将最近几轮 Q+A 片段 / 当前问题拼在一起
func buildUserPrompt(recentQAPairs []string, question string) string {
	var b strings.Builder

	b.WriteString("【最近相关的历史问答片段】:\n")
	if len(recentQAPairs) == 0 {
		b.WriteString("（无）\n\n")
	} else {
		for i, qa := range recentQAPairs {
			b.WriteString(fmt.Sprintf("片段%d:\n", i+1))
			b.WriteString(strings.TrimSpace(qa))
			b.WriteString("\n\n")
		}
	}

	b.WriteString("【当前用户原始问题】:\n")
	b.WriteString(strings.TrimSpace(question))
	b.WriteString("\n\n")

	b.WriteString("请根据上述历史问答片段，对当前问题进行改写或拆分。")
	if len(recentQAPairs) > 0 {
		b.WriteString("特别注意：如果历史问答片段中包含流程、步骤、操作方法等信息，且当前问题是省略问法（如\"那年假呢\"），应优先改写为询问流程的问题。")
	}
	b.WriteString("严格按照系统提示中的\"输出要求\"给出结果。")
	fmt.Printf("buildUserPrompt: %s", b.String())
	return b.String()
}

// normalizeQuestion 做一个非常轻量的归一化兜底（保留更多语义给上层控制）
func normalizeQuestion(q string) string {
	q = strings.TrimSpace(q)
	// 这里可以按需增加更多轻量规则，比如全角空格替换、去掉多余换行等
	q = strings.ReplaceAll(q, "　", " ")
	q = strings.Join(strings.Fields(q), " ")
	return q
}
