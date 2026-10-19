package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMConfig 是大模型翻译配置（兼容 OpenAI Chat Completions 协议）。
type LLMConfig struct {
	Enabled        bool   `json:"enabled"`
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	Model          string `json:"model"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	// 仅发送模板与占位符，不发送源码（默认 true，界面上可关）
	TemplateOnly bool `json:"template_only"`
}

// Valid 判断配置是否可用。
func (c LLMConfig) Valid() bool {
	return c.Enabled &&
		strings.TrimSpace(c.BaseURL) != "" &&
		strings.TrimSpace(c.Model) != ""
}

// Endpoint 返回 chat/completions 的完整地址。
func (c LLMConfig) Endpoint() string {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

// LLMClient 是大模型翻译客户端。
type LLMClient struct {
	cfg LLMConfig
	hc  *http.Client
}

// NewLLMClient 创建客户端。
func NewLLMClient(cfg LLMConfig) *LLMClient {
	to := cfg.TimeoutSeconds
	if to <= 0 {
		to = 60
	}
	return &LLMClient{
		cfg: cfg,
		hc:  &http.Client{Timeout: time.Duration(to) * time.Second},
	}
}

// Configured 返回客户端是否可用。
func (c *LLMClient) Configured() bool { return c != nil && c.cfg.Valid() }

// Config 返回当前配置。
func (c *LLMClient) Config() LLMConfig {
	if c == nil {
		return LLMConfig{}
	}
	return c.cfg
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Stream      bool          `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func systemPrompt(kind string) string {
	switch kind {
	case KindRule:
		return "你是网络安全领域的专业译者。把下面的 CodeQL 漏洞规则名称翻译成简体中文。" +
			"要求：术语准确、简洁，只输出译文本身，不要任何解释、引号或 Markdown 标记。"
	case KindDescription:
		return "你是网络安全领域的专业译者。把下面的 CodeQL 漏洞规则描述翻译成简体中文。" +
			"要求：准确、通顺、符合安全行业习惯用语，只输出译文本身，不要任何解释或 Markdown 标记。"
	case KindLabel:
		return "你是网络安全领域的专业译者。把下面的 CodeQL 数据流标签翻译成简体中文。" +
			"要求：保留其中的标识符/变量名原文不译，只输出译文本身，不要任何解释或标点包装。"
	default:
		return "你是网络安全领域的专业译者。把下面的 CodeQL 漏洞提示翻译成简体中文。\n" +
			"严格遵守：\n" +
			"1. 必须原样保留 {{LINK}}、{{N}}、{{W}}、{{API}} 这些占位符，不得翻译、删除、改写或添加空格；\n" +
			"2. API 名称、函数名、类名、变量名、文件名等代码标识符保持原文不译；\n" +
			"3. 只输出译文本身，不要解释、不要 Markdown 代码块、不要加引号。"
	}
}

// Translate 调用大模型翻译一段文本。
func (c *LLMClient) Translate(ctx context.Context, kind, text string) (string, error) {
	if !c.Configured() {
		return "", fmt.Errorf("未配置大模型翻译")
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("待翻译内容为空")
	}
	out, err := c.Chat(ctx, systemPrompt(kind), text)
	if err != nil {
		return "", err
	}
	return cleanLLMOutput(out), nil
}

// Chat 执行一次通用的对话补全（用于翻译之外的场景，例如 AI 误报验证）。
// 返回的文本未经 cleanLLMOutput 处理。
func (c *LLMClient) Chat(ctx context.Context, system, user string) (string, error) {
	if !c.Configured() {
		return "", fmt.Errorf("未配置大模型")
	}
	reqBody := chatRequest{
		Model:       c.cfg.Model,
		Temperature: 0.1,
		Stream:      false,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Endpoint(), bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.cfg.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.cfg.APIKey))
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求大模型失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("读取大模型响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("大模型返回 HTTP %d: %s", resp.StatusCode, snippet(string(body)))
	}
	var out chatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("解析大模型响应失败: %w (%s)", err, snippet(string(body)))
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", fmt.Errorf("大模型返回错误: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("大模型没有返回内容")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func snippet(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

// isLangTag 判断一行是否为代码块语言标记（如 json、python）。
func isLangTag(s string) bool {
	if s == "" || len(s) > 20 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '+', r == '-', r == '_', r == '#':
		default:
			return false
		}
	}
	return true
}

// cleanLLMOutput 去掉模型偶尔加上的代码块围栏与包裹引号。
func cleanLLMOutput(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = s[3:]
		// 围栏后可能紧跟语言标记（如 ```json）。注意：只有「整行且是纯标记」
		// 才能当作语言标记丢掉，否则会把译文首行误删。
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			head := strings.TrimSpace(s[:i])
			switch {
			case head == "" || isLangTag(head):
				s = s[i+1:]
			default:
				s = head + s[i:] // 译文与围栏同行，保留它
			}
		} else if isLangTag(strings.TrimSpace(s)) {
			s = ""
		}
		s = strings.TrimSpace(s)
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	for _, q := range [][2]string{{`"`, `"`}, {"“", "”"}, {"「", "」"}} {
		if strings.HasPrefix(s, q[0]) && strings.HasSuffix(s, q[1]) && len(s) > len(q[0])+len(q[1]) {
			s = strings.TrimSpace(s[len(q[0]) : len(s)-len(q[1])])
			break
		}
	}
	return s
}
