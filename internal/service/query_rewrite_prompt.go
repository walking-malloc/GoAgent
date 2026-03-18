package service

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// queryRewritePromptConfig 对应 configs/query_rewrite.yaml 的结构
type queryRewritePromptConfig struct {
	QueryRewrite struct {
		System string `yaml:"system"`
	} `yaml:"query_rewrite"`
}

// 默认系统提示词：问题重写与拆分
// 如果存在 configs/query_rewrite.yaml，会在 init 时覆盖为配置中的内容。
var queryRewriteSystemPrompt string

// init 尝试从 YAML 配置覆盖默认的系统提示词
// 配置缺失或解析失败时，回退到内置默认值，并打印调试日志。
func init() {
	fmt.Println("[query_rewrite] init start")

	// 这里假设可执行程序工作目录是 GoAgent/，所以 YAML 放在项目根 configs，需要 ../configs
	paths := []string{
		"configs/query_rewrite.yaml",    // 如果以后把 configs 挪到 GoAgent 下
		"../configs/query_rewrite.yaml", // 当前目录结构：项目根下的 configs
	}

	var data []byte
	var err error
	for _, p := range paths {
		data, err = os.ReadFile(p)
		if err == nil {
			fmt.Println("[query_rewrite] loaded yaml from", p)
			break
		}
	}
	if err != nil {
		fmt.Println("[query_rewrite] no yaml found, use default prompt, last error:", err)
		return
	}

	var cfg queryRewritePromptConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Println("[query_rewrite] yaml unmarshal error, use default prompt:", err)
		return
	}

	if sys := strings.TrimSpace(cfg.QueryRewrite.System); sys != "" {
		queryRewriteSystemPrompt = sys
		fmt.Println("[query_rewrite] system prompt overridden from yaml")
	} else {
		fmt.Println("[query_rewrite] yaml system field empty, keep default prompt")
	}
}
