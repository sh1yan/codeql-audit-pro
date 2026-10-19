package codeqlcsv

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"codeql-audit-pro/internal/model"
)

// 本文件实现「审计报告 CSV」导出：把工具界面上能看到、用到的全部信息
// （原始 CodeQL 九列 + 中文翻译 + 路径定位 + 证据链 + 已读状态 + AI 误报判定）
// 汇总成一张宽表，方便离线用 Excel / pandas 做二次分析。
//
// 与既有导出的区别：
//   - ExportZH：只导出「中文重排版」的 CodeQL 九列，用于回填 CodeQL 或交付；
//   - UI 的「导出验证结果 CSV」：只导出有 AI 判定的条目，列很少；
//   - 本文件：一行 = 一条漏洞的全部审计信息，含未做 AI 判定的条目。

// ReportRow 是审计报告中的一行：漏洞本体 + 界面侧的审计状态。
type ReportRow struct {
	Finding  *model.Finding
	Reviewed bool
}

// ReportHeaders 是审计报告的固定表头（32 列）。
var ReportHeaders = []string{
	"序号",
	"规则名称（中文）",
	"规则名称（英文）",
	"严重级别（中文）",
	"严重级别（英文）",
	"规则描述（中文）",
	"规则描述（英文）",
	"文件（结果路径）",
	"本地文件路径",
	"起始行",
	"起始列",
	"结束行",
	"结束列",
	"行范围",
	"列范围",
	"定位状态",
	"已读状态",
	"漏洞详情（中文）",
	"漏洞详情（英文）",
	"证据链（沉淀点 + 来源点 + codeFlow）",
	"AI 判定",
	"AI 置信度",
	"AI 判定理由",
	"AI 模型",
	"AI 判定时间",
	"译文来源",
	"翻译状态",
	"漏洞指纹",
	"结果格式",
	"CWE",
	"SARIF 指纹",
	"污点路径步数",
}

// ReportValues 把一条记录摊平成与 ReportHeaders 一一对应的字符串切片。
func ReportValues(row ReportRow) []string {
	f := row.Finding
	if f == nil {
		return nil
	}
	ruleZH := f.RuleDisplayName()
	descZH := f.DescriptionDisplay()

	msgZH := model.PlainSegments(f.MessageZH)
	if strings.TrimSpace(msgZH) == "" {
		msgZH = model.PlainSegments(f.MessageEN)
	}

	local := f.Sink.ResolvedPath
	resolved := "已定位"
	if strings.TrimSpace(local) == "" {
		local = ""
		resolved = "未定位"
	}

	read := "未读"
	if row.Reviewed {
		read = "已读"
	}

	conf := ""
	if f.AIConfidence > 0 {
		conf = fmt.Sprintf("%.0f%%", f.AIConfidence*100)
	}
	verdict := model.AIVerdictZH(f.AIVerdict)
	if verdict == "" {
		verdict = "未验证"
	}

	pending := "正常"
	if f.TranslatePending {
		pending = "待翻译"
	}

	return []string{
		strconv.Itoa(f.Index),
		ruleZH,
		f.RuleName,
		f.Severity.ZH(),
		string(f.Severity),
		flatten(descZH),
		flatten(f.Description),
		f.Sink.File,
		local,
		strconv.Itoa(f.Sink.StartLine),
		strconv.Itoa(f.Sink.StartCol),
		strconv.Itoa(f.Sink.EndLine),
		strconv.Itoa(f.Sink.EndCol),
		f.Sink.LineRange(),
		f.Sink.ColumnRange(),
		resolved,
		read,
		flatten(msgZH),
		flatten(model.PlainSegments(f.MessageEN)),
		flatten(EvidenceChain(f)),
		verdict,
		conf,
		flatten(f.AIReason),
		f.AIModel,
		f.AIAt,
		translateSourceZH(f.TranslateSource),
		pending,
		f.Key(),
		f.Format.Label(),
		strings.Join(f.CWEs, ", "),
		fingerprintText(f),
		strconv.Itoa(f.FlowStepCount()),
	}
}

// fingerprintText 把 SARIF 指纹压成一行 name=value，便于在 Excel 中筛选。
func fingerprintText(f *model.Finding) string {
	if f == nil || len(f.Fingerprints) == 0 {
		return ""
	}
	parts := make([]string, 0, len(f.Fingerprints))
	for _, fp := range f.Fingerprints {
		parts = append(parts, fp.Name+"="+fp.Value)
	}
	return flatten(strings.Join(parts, "; "))
}

// EvidenceChain 把沉淀点与全部数据流来源点写成一行文本。
//
// 格式与右栏证据链一致：标签 + 文件 + 第 N 行 + 列 A-B（+ 中文标签 / 未定位标记）。
// 已经映射到本地文件的位置会额外带上「（本地 …）」，方便离线分析时直接定位。
func EvidenceChain(f *model.Finding) string {
	if f == nil {
		return ""
	}
	parts := make([]string, 0, len(f.Sources)+1)
	add := func(tag string, loc model.Location) {
		path := loc.File
		if strings.TrimSpace(path) == "" {
			path = loc.ResolvedPath
		}
		s := tag + " " + path
		if loc.ResolvedPath != "" && loc.ResolvedPath != path {
			s += "（本地 " + loc.ResolvedPath + "）"
		}
		s += " 第 " + loc.LineRange() + " 行"
		if c := loc.ColumnRange(); c != "" {
			s += " 列 " + c
		}
		if l := loc.DisplayLabel(); l != "" {
			s += " · " + l
		}
		if !loc.Resolved() {
			s += " [未定位]"
		}
		parts = append(parts, s)
	}
	add("【沉淀点】", f.Sink)
	if f.HasCodeFlow() {
		for _, cf := range f.CodeFlows {
			flow := fmt.Sprintf("【污点路径 %d", cf.Index)
			if cf.Message != "" {
				flow += " · " + flatten(cf.Message)
			}
			flow += "】"
			parts = append(parts, flow)
			for _, step := range cf.Steps {
				add(fmt.Sprintf("  步骤 %d", step.Index), step.Location)
			}
		}
	} else {
		for i, s := range f.Sources {
			add(fmt.Sprintf("【来源 %d】", i+1), s)
		}
	}
	return strings.Join(parts, "；")
}

// translateSourceZH 把译文来源标记翻成中文。
//
// 取值与 internal/translate 的 Source* 常量一致（builtin/store/llm/none）。
// 这里刻意不 import translate：codeqlcsv 是纯解析/导出层，不该拖进翻译引擎
// （它依赖 SQLite 与大模型客户端）。
func translateSourceZH(s string) string {
	switch strings.TrimSpace(s) {
	case "builtin":
		return "内置词典"
	case "store":
		return "翻译库"
	case "template":
		return "模板"
	case "llm":
		return "大模型"
	case "none":
		return "未翻译（保持英文）"
	case "":
		return ""
	}
	return s
}

// flatten 把多行文本压成一行，避免在 Excel 里把行撑得过高。
func flatten(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

// ExportReport 写出审计报告 CSV（带 UTF-8 BOM，Excel 双击不乱码）。
func ExportReport(w io.Writer, rows []ReportRow) error {
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(ReportHeaders); err != nil {
		return err
	}
	for _, row := range rows {
		values := ReportValues(row)
		if len(values) == 0 {
			continue
		}
		if err := cw.Write(values); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// ExportReportFile 把审计报告写到指定文件。
func ExportReportFile(path string, rows []ReportRow) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return ExportReport(f, rows)
}
