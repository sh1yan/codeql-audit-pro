// Package model 定义工具的核心数据结构：漏洞条目、源码位置、消息片段。
package model

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Severity 是 CodeQL 报告的问题严重级别。
type Severity string

const (
	SevError          Severity = "error"
	SevWarning        Severity = "warning"
	SevNote           Severity = "note"
	SevRecommendation Severity = "recommendation"
)

// ZH 返回严重级别的中文名称。
func (s Severity) ZH() string {
	switch Severity(strings.ToLower(strings.TrimSpace(string(s)))) {
	case SevError:
		return "严重"
	case SevWarning:
		return "警告"
	case SevNote:
		return "提示"
	case SevRecommendation:
		return "建议"
	}
	if strings.TrimSpace(string(s)) == "" {
		return "未知"
	}
	return string(s)
}

// Location 表示一处源码位置。File 是结果文件（CSV / SARIF）中给出的原始路径（已去掉 URI 前缀），
// ResolvedPath 是映射到本地源码目录后的真实路径。
type Location struct {
	File      string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	Label     string // 证据链标签（英文原文）
	LabelZH   string // 证据链标签（中文）

	ResolvedPath  string
	ResolvedScore int
}

// DisplayLabel 返回用于展示的证据链标签，优先中文。
func (l Location) DisplayLabel() string {
	if strings.TrimSpace(l.LabelZH) != "" {
		return l.LabelZH
	}
	return l.Label
}

// Resolved 表示该位置是否已经定位到本地真实文件。
func (l Location) Resolved() bool { return l.ResolvedPath != "" }

// Display 返回 file:line:col 形式的简短描述。
func (l Location) Display() string {
	f := l.File
	if f == "" {
		f = l.ResolvedPath
	}
	if l.StartLine <= 0 {
		return f
	}
	if l.StartCol <= 0 {
		return fmt.Sprintf("%s:%d", f, l.StartLine)
	}
	return fmt.Sprintf("%s:%d:%d", f, l.StartLine, l.StartCol)
}

// DisplayPath 返回展示用的文件路径。若已定位到本地文件，优先显示本地路径，
// 便于用户在「证据链」里区分同名文件（例如三处 admin.py 分属不同目录）。
func (l Location) DisplayPath() string {
	if l.File != "" {
		return l.File
	}
	return l.ResolvedPath
}

// LineRange 返回人类可读的行范围：单行 "16"、多行 "16-18"、未知 "?"。
//
// 用它替代「file:line:col」里的行号，避免用户把列号误读成行号。
func (l Location) LineRange() string {
	if l.StartLine <= 0 {
		return "?"
	}
	if l.EndLine > l.StartLine {
		return fmt.Sprintf("%d-%d", l.StartLine, l.EndLine)
	}
	return strconv.Itoa(l.StartLine)
}

// ColumnRange 返回人类可读的列范围：单列 "26"、多列 "26-28"、未知 ""。
func (l Location) ColumnRange() string {
	if l.StartCol <= 0 {
		return ""
	}
	if l.EndCol > l.StartCol {
		return fmt.Sprintf("%d-%d", l.StartCol, l.EndCol)
	}
	return strconv.Itoa(l.StartCol)
}

// LineCol 返回 "行:列"（列未知时只返回行号）。
func (l Location) LineCol() string {
	if l.StartLine <= 0 {
		return "?"
	}
	if l.StartCol <= 0 {
		return strconv.Itoa(l.StartLine)
	}
	return fmt.Sprintf("%d:%d", l.StartLine, l.StartCol)
}

// BaseName 返回文件名部分。
func (l Location) BaseName() string {
	p := l.File
	if p == "" {
		p = l.ResolvedPath
	}
	p = strings.ReplaceAll(p, "\\", "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// ValidRange 判断位置是否带有可用的行范围。
func (l Location) ValidRange() bool { return l.StartLine > 0 }

// NormalizedEnd 返回补齐后的结束位置：CodeQL 有时只给出单点位置。
func (l Location) NormalizedEnd() (int, int) {
	el, ec := l.EndLine, l.EndCol
	if el <= 0 {
		el = l.StartLine
	}
	if ec <= 0 {
		ec = l.StartCol
	}
	if el < l.StartLine {
		el = l.StartLine
	}
	return el, ec
}

// SegmentKind 区分消息片段是普通文本还是可跳转的证据链接。
type SegmentKind int

const (
	SegText SegmentKind = iota
	SegLink
)

// LinkRef 是 CSV 消息中内嵌的数据流位置引用，语法为：
//
//	[["显示标签"|"relative:///app/views/user.py:1:37:1:43"]]
type LinkRef struct {
	Label     string // 英文标签
	LabelZH   string // 标签中文（翻译库命中时填充）
	RawURI    string // 原始 URI
	File      string // 规范化后的路径
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

// Location 把链接转换为通用位置结构。
func (r LinkRef) Location() Location {
	return Location{
		File:      r.File,
		StartLine: r.StartLine,
		StartCol:  r.StartCol,
		EndLine:   r.EndLine,
		EndCol:    r.EndCol,
		Label:     r.Label,
	}
}

// Display 返回 file:line:col。
func (r LinkRef) Display() string { return r.Location().Display() }

// Segment 是消息的一个片段。
type Segment struct {
	Kind SegmentKind
	Text string
	Link *LinkRef
}

// ResultFormat 是审计结果文件的格式。
type ResultFormat string

const (
	// FormatCSV 是 CodeQL CLI 的 --format=csv 输出。
	FormatCSV ResultFormat = "csv"
	// FormatSARIF 是 CodeQL CLI 的 --format=sarifv2.1.0 输出。
	FormatSARIF ResultFormat = "sarif"
)

// Label 返回界面展示用的格式名称。
func (f ResultFormat) Label() string {
	switch ResultFormat(strings.ToLower(string(f))) {
	case FormatSARIF:
		return "SARIF v2.1.0"
	case FormatCSV:
		return "CSV"
	}
	return string(f)
}

// Fingerprint 是 SARIF 中的一条指纹（partialFingerprints / fingerprints）。
// CodeQL 用 primaryLocationLineHash 等指纹在代码小幅变动后仍能识别同一条告警。
type Fingerprint struct {
	Name  string
	Value string
}

// RuleInfo 是 SARIF tool.driver.rules 中的规则元数据。
// CSV 结果没有这些结构化字段，因此为 nil。
type RuleInfo struct {
	ID               string
	Name             string
	ShortDescription string
	FullDescription  string
	HelpURI          string
	DefaultLevel     string
	SecuritySeverity string
	Precision        string
	Kind             string
	ProblemSeverity  string
	Tags             []string
	CWEs             []string
}

// FlowStep 是污点传播路径（SARIF codeFlow）上的一个节点。
type FlowStep struct {
	Index        int
	Location     Location
	Message      string
	NestingLevel int
	Kinds        []string
}

// CodeFlow 是一条完整的污点传播路径。
//
// SARIF 中 codeFlows[].threadFlows[].locations[] 记录 source → … → sink 的
// 每一步。CSV 只会把其中的来源点压缩进消息文本，SARIF 才能还原完整路径。
type CodeFlow struct {
	Index   int
	Message string
	Steps   []FlowStep
}

// RelatedLocation 是 SARIF result.relatedLocations 中的一条相关位置。
// 消息里的 [标签](数字) 链接就是指向这里的 id。
type RelatedLocation struct {
	ID       int
	Location Location
	Message  string
}

// Finding 是一条漏洞记录。
type Finding struct {
	Index int // 从 1 开始的序号

	RuleName      string
	RuleNameZH    string
	Description   string
	DescriptionZH string
	Severity      Severity

	// Sink 是结果给出的位置，即漏洞沉淀点。
	Sink Location
	// Sources 是从消息内嵌链接中解析出的数据流来源点。
	Sources []Location

	// Format 标识结果来自 CSV 还是 SARIF。
	Format ResultFormat
	// ToolName / ToolVersion 是生成结果的工具（SARIF tool.driver）。
	ToolName    string
	ToolVersion string

	// RuleInfo 是 SARIF 的规则结构化元数据（CWE、precision、security-severity 等）。
	RuleInfo *RuleInfo
	// CWEs / Tags 是 RuleInfo 中供界面直接展示的派生字段。
	CWEs []string
	Tags []string
	// Fingerprints 是 SARIF 指纹；PrimaryFingerprint 用于生成稳定的已读/判定键。
	Fingerprints       []Fingerprint
	PrimaryFingerprint string
	// CodeFlows 是完整的污点传播路径（仅 SARIF 有）。
	CodeFlows []CodeFlow
	// RelatedLocations 是结果附带的相关位置（仅 SARIF 有）。
	RelatedLocations []RelatedLocation

	// MessageRaw 是原始消息文本（SARIF 为 message.text / markdown），逐字保留。
	MessageRaw string
	// MessageEN / MessageZH 是同一条消息的英文原文与中文译文片段序列。
	MessageEN []Segment
	MessageZH []Segment

	// MessageKey 是规范化后的英文模板键（含 {{LINK}} 等占位符），
	// MessageZHTemplate 是对应的中文模板。两者一起构成翻译库中的「中英对照」。
	MessageKey        string
	MessageZHTemplate string

	// TranslateSource 说明中文消息的来源：glossary / library / template / llm / none
	TranslateSource string
	// TranslatePending 表示中文尚未生成（需要 LLM 或人工处理）。
	TranslatePending bool

	// AI 误报判定结果（由「AI 验证」填充，并持久化到配置里）。
	AIVerdict    string  // true_positive / false_positive / uncertain
	AIReason     string  // 判定理由
	AIConfidence float64 // 置信度 0~1，<=0 表示模型未给出
	AIModel      string  // 参与判定的模型名
	AIAt         string  // 判定时间（RFC3339）
}

// Key 返回一条漏洞的稳定标识。
//
// 已读状态与 AI 判定都按这个键持久化：
//   - SARIF 优先使用 CodeQL 写入的 primaryLocationLineHash 等指纹，代码行号小幅
//     变化时仍能识别同一条告警；
//   - CSV 没有指纹，退回「规则名 + 沉淀点文件/行列 + 原始消息」的稳定组合。
//
// Index 会随记录顺序变化，不能用来做键。
func (f *Finding) Key() string {
	if f == nil {
		return ""
	}
	var key string
	if strings.TrimSpace(f.PrimaryFingerprint) != "" {
		key = strings.Join([]string{
			"sarif",
			f.RuleName,
			f.PrimaryFingerprint,
		}, "\x00")
	} else {
		key = strings.Join([]string{
			f.RuleName,
			f.Sink.File,
			strconv.Itoa(f.Sink.StartLine),
			strconv.Itoa(f.Sink.StartCol),
			f.MessageRaw,
		}, "\x00")
	}
	h := sha1.Sum([]byte(key))
	return hex.EncodeToString(h[:8])
}

// HasCodeFlow 判断这条漏洞是否带有完整的污点传播路径。
func (f *Finding) HasCodeFlow() bool { return f != nil && len(f.CodeFlows) > 0 }

// FlowStepCount 返回全部 codeFlow 的步骤总数。
func (f *Finding) FlowStepCount() int {
	if f == nil {
		return 0
	}
	n := 0
	for _, cf := range f.CodeFlows {
		n += len(cf.Steps)
	}
	return n
}

// AIVerdictZH 返回 AI 判定结论的中文名。
func (f *Finding) AIVerdictZH() string { return AIVerdictZH(f.AIVerdict) }

// HasAIVerdict 判断这条漏洞是否已有 AI 判定。
func (f *Finding) HasAIVerdict() bool { return f != nil && f.AIVerdict != "" }

// AIVerdictZH 把判定代码翻译成中文。
func AIVerdictZH(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "false_positive":
		return "误报"
	case "true_positive":
		return "真实漏洞"
	case "uncertain":
		return "存疑"
	}
	return ""
}

// AIVerdictShort 返回列表里使用的短标记。
func AIVerdictShort(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "false_positive":
		return "AI 误报"
	case "true_positive":
		return "AI 确认"
	case "uncertain":
		return "AI 存疑"
	}
	return ""
}

// RuleDisplayName 返回用于界面展示的规则名。
func (f *Finding) RuleDisplayName() string {
	if f.RuleNameZH != "" {
		return f.RuleNameZH
	}
	return f.RuleName
}

// DescriptionDisplay 返回用于界面展示的规则描述。
func (f *Finding) DescriptionDisplay() string {
	if f.DescriptionZH != "" {
		return f.DescriptionZH
	}
	return f.Description
}

// MessageZHText 返回中文消息的纯文本形式（链接渲染为「标签 @ 位置」）。
func (f *Finding) MessageZHText() string { return segmentsText(f.MessageZH) }

// MessageENText 返回英文消息的纯文本形式。
func (f *Finding) MessageENText() string { return segmentsText(f.MessageEN) }

func segmentsText(segs []Segment) string {
	var b strings.Builder
	for _, s := range segs {
		if s.Text != "" {
			b.WriteString(s.Text)
			continue
		}
		// 链接片段没有独立文本时，用它的标签回退，保证读起来仍是完整句子
		if s.Kind == SegLink && s.Link != nil {
			b.WriteString(s.Link.Label)
		}
	}
	return b.String()
}

// PlainSegments 把带链接的消息片段转换为纯文本，链接渲染为「标签（位置）」。
// 用于日志、导出与不支持内联超链接的场景。
func PlainSegments(segs []Segment) string {
	var b strings.Builder
	for _, s := range segs {
		if s.Kind == SegLink && s.Link != nil {
			label := s.Link.LabelZH
			if label == "" {
				label = s.Link.Label
			}
			fmt.Fprintf(&b, "%s（%s）", label, s.Link.Display())
			continue
		}
		b.WriteString(s.Text)
	}
	return b.String()
}
