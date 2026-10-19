// Package verify 用大模型对 CodeQL 漏洞做「是否误报」的辅助判定。
//
// 设计要点：
//   - 只做**辅助**判定：结论只有「误报 / 真实漏洞 / 存疑」三档，并给出置信度与理由，
//     最终仍然由人拍板；
//   - 判定需要上下文，因此会把「规则名 + 规则描述 + 漏洞详情 + 沉淀点/来源点代码片段」
//     组织成一段紧凑的提示词；是否发送源码片段由配置控制（config.AISendCode）；
//   - 模型输出要求严格 JSON，但现实中模型经常加代码块或前后缀，所以解析做了多级容错。
package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Verdict 是判定结论。
type Verdict string

const (
	// FalsePositive 表示大概率是误报。
	FalsePositive Verdict = "false_positive"
	// TruePositive 表示大概率是真实漏洞。
	TruePositive Verdict = "true_positive"
	// Uncertain 表示信息不足，需要人工确认。
	Uncertain Verdict = "uncertain"
)

// NormalizeVerdict 把模型给出的各种写法归一到标准取值。
func NormalizeVerdict(s string) Verdict {
	v := strings.ToLower(strings.TrimSpace(s))
	v = strings.NewReplacer("-", "_", " ", "_").Replace(v)
	switch {
	case v == "":
		return Uncertain
	case strings.Contains(v, "false") || strings.Contains(v, "误报") || v == "fp":
		return FalsePositive
	case strings.Contains(v, "true") || strings.Contains(v, "confirm") ||
		strings.Contains(v, "真实") || strings.Contains(v, "确认") || v == "tp":
		return TruePositive
	default:
		return Uncertain
	}
}

// Result 是一次 AI 误报判定的结果。
type Result struct {
	Verdict    Verdict
	Confidence float64 // 0~1；<=0 表示模型未给出
	Reason     string
	Model      string
	Raw        string // 模型原始输出，便于人工核对
}

// Snippet 是一段带行号的源码片段。
type Snippet struct {
	Path string
	Line int // 片段第一行在文件中的行号
	Text string
}

// Input 是发给模型的全部上下文。
type Input struct {
	RuleName    string
	RuleNameZH  string
	Description string
	SeverityZH  string
	MessageZH   string
	MessageEN   string
	Sink        *Snippet
	Sources     []Snippet
	// SendCode 为 false 时只发送位置，不发送源码内容（隐私模式）。
	SendCode bool
}

// Chatter 抽象「一次对话补全」，便于用假实现做测试。
type Chatter interface {
	Chat(ctx context.Context, system, user string) (string, error)
	Configured() bool
}

// Verifier 执行 AI 误报判定。
type Verifier struct {
	client Chatter
	model  string
}

// New 创建判定器。client 为 nil 或未配置时，Verify 会直接报错。
func New(client Chatter, model string) *Verifier {
	return &Verifier{client: client, model: model}
}

// Configured 返回是否具备调用大模型的条件。
func (v *Verifier) Configured() bool {
	return v != nil && v.client != nil && v.client.Configured()
}

// Model 返回当前使用的模型名。
func (v *Verifier) Model() string {
	if v == nil {
		return ""
	}
	return v.model
}

// systemPrompt 是判定用的系统提示词。
const systemPrompt = `你是一名资深应用安全审计工程师，负责复核 CodeQL 静态扫描结果，判断它是不是误报。
请基于给出的规则、漏洞详情与代码片段做判断，重点关注：
1. 数据来源是否真的可被外部攻击者控制（而非常量、配置、已有校验的值）；
2. 沉淀点是否存在有效的过滤 / 参数化 / 转义 / 白名单等净化措施；
3. 该数据流在当前代码里是否真实可达（是否有死代码、被上层拦截）；
4. 规则语义与实际代码是否匹配（例如框架自身会做转义、类型不可控等）。
判定要克制：证据不足时必须给 uncertain，不要为了给出结论而臆测。
只输出一个 JSON 对象，不要 Markdown 代码块，不要额外解释，格式严格为：
{"verdict":"false_positive|true_positive|uncertain","confidence":0.0,"reason":"不超过 150 字的中文理由，说明关键依据"}
confidence 取 0~1 的小数，表示你对该结论的把握。`

// BuildPrompt 把上下文组织成用户消息。抽成独立函数便于单元测试。
func BuildPrompt(in Input) string {
	var b strings.Builder
	b.WriteString("【规则】\n")
	b.WriteString(strings.TrimSpace(in.RuleName))
	if strings.TrimSpace(in.RuleNameZH) != "" {
		b.WriteString("（" + strings.TrimSpace(in.RuleNameZH) + "）")
	}
	b.WriteString("\n")
	if d := strings.TrimSpace(in.Description); d != "" {
		b.WriteString("规则说明：" + d + "\n")
	}
	if s := strings.TrimSpace(in.SeverityZH); s != "" {
		b.WriteString("严重级别：" + s + "\n")
	}

	b.WriteString("\n【CodeQL 漏洞详情】\n")
	if zh := strings.TrimSpace(in.MessageZH); zh != "" {
		b.WriteString(zh + "\n")
	}
	if en := strings.TrimSpace(in.MessageEN); en != "" && en != strings.TrimSpace(in.MessageZH) {
		b.WriteString("英文原文：" + en + "\n")
	}

	if in.Sink != nil {
		b.WriteString("\n【沉淀点】" + in.Sink.Path + " 第 " + fmt.Sprint(in.Sink.Line) + " 行\n")
		if in.SendCode && strings.TrimSpace(in.Sink.Text) != "" {
			b.WriteString("```\n" + in.Sink.Text + "\n```\n")
		}
	}
	if len(in.Sources) > 0 {
		b.WriteString("\n【数据流来源】\n")
		for i, s := range in.Sources {
			b.WriteString(fmt.Sprintf("来源 %d：%s 第 %d 行\n", i+1, s.Path, s.Line))
			if in.SendCode && strings.TrimSpace(s.Text) != "" {
				b.WriteString("```\n" + s.Text + "\n```\n")
			}
		}
	}
	if !in.SendCode {
		b.WriteString("\n（隐私模式：未提供源码内容，请仅依据位置与规则语义判断，不确定时给 uncertain）\n")
	}
	b.WriteString("\n请给出你的 JSON 判定：")
	return b.String()
}

// Verify 对一条漏洞做判定。
func (v *Verifier) Verify(ctx context.Context, in Input) (Result, error) {
	if v == nil || v.client == nil {
		return Result{}, fmt.Errorf("未配置大模型，无法做 AI 验证")
	}
	if !v.client.Configured() {
		return Result{}, fmt.Errorf("未配置大模型：请在「设置」里填写接口地址与模型后重试")
	}
	out, err := v.client.Chat(ctx, systemPrompt, BuildPrompt(in))
	if err != nil {
		return Result{}, err
	}
	res := ParseResult(out)
	res.Model = v.model
	return res, nil
}

// ParseResult 从模型输出里解析判定结论。
//
// 容错顺序：直接 JSON -> 截取最外层花括号 -> 正则抽 verdict/confidence/reason
// -> 整体作为理由并给 uncertain。
func ParseResult(raw string) Result {
	res := Result{Raw: raw}
	text := strings.TrimSpace(raw)
	if text == "" {
		res.Verdict = Uncertain
		res.Reason = "模型没有返回内容"
		return res
	}

	for _, cand := range jsonCandidates(text) {
		var payload struct {
			Verdict    string          `json:"verdict"`
			Confidence json.RawMessage `json:"confidence"`
			Reason     string          `json:"reason"`
		}
		if err := json.Unmarshal([]byte(cand), &payload); err != nil {
			continue
		}
		if strings.TrimSpace(payload.Verdict) == "" && strings.TrimSpace(payload.Reason) == "" {
			continue
		}
		res.Verdict = NormalizeVerdict(payload.Verdict)
		res.Confidence = parseConfidence(string(payload.Confidence))
		res.Reason = strings.TrimSpace(payload.Reason)
		if res.Reason == "" {
			res.Reason = "模型未给出理由"
		}
		return res
	}

	// 兜底：模型没按 JSON 输出，但往往仍然写了结论词
	res.Verdict = NormalizeVerdict(text)
	res.Reason = collapse(text)
	return res
}

// jsonCandidates 依次给出可能的 JSON 片段。
func jsonCandidates(text string) []string {
	out := []string{text}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		inner := text[start : end+1]
		if inner != text {
			out = append(out, inner)
		}
	}
	return out
}

// parseConfidence 解析置信度：兼容 0~1 的小数与 0~100 的百分数。
func parseConfidence(s string) float64 {
	s = strings.TrimSpace(strings.Trim(s, `"`))
	s = strings.TrimSuffix(s, "%")
	if s == "" || s == "null" {
		return 0
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%g", &f); err != nil {
		return 0
	}
	if f > 1 {
		f /= 100
	}
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return f
}

// collapse 把多行文本压成单行，避免列表里撑高。
func collapse(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	if len([]rune(s)) > 300 {
		s = string([]rune(s)[:300]) + "…"
	}
	return s
}

// BuildSnippet 从文件行里截取目标行附近的片段，并给每行加上行号。
//
// start/end 是 1-based 的闭区间；context 是上下各多取的行数。
func BuildSnippet(path string, lines []string, start, end, context int) Snippet {
	if start <= 0 || len(lines) == 0 {
		return Snippet{Path: path}
	}
	if end < start {
		end = start
	}
	from := start - context
	if from < 1 {
		from = 1
	}
	to := end + context
	if to > len(lines) {
		to = len(lines)
	}
	var b strings.Builder
	for ln := from; ln <= to; ln++ {
		mark := "  "
		if ln == start {
			mark = "> "
		}
		b.WriteString(fmt.Sprintf("%s%5d | %s\n", mark, ln, clipLine(lines[ln-1])))
	}
	return Snippet{Path: path, Line: from, Text: strings.TrimRight(b.String(), "\n")}
}

// maxSnippetRunes 限制单行长度，避免超长行把提示词撑爆。
const maxSnippetRunes = 400

func clipLine(s string) string {
	rs := []rune(strings.TrimRight(s, " \t"))
	if len(rs) > maxSnippetRunes {
		rs = append(rs[:maxSnippetRunes], '…')
	}
	return string(rs)
}
