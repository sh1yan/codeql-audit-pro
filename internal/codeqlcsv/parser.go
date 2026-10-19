// Package codeqlcsv 负责解析 CodeQL 导出的 CSV 结果文件。
//
// CodeQL 的 `codeql database analyze --format=csv` 输出固定 9 列且没有表头：
//
//	规则名称 | 规则描述 | 严重级别 | 漏洞详情 | 文件路径 | 起始行 | 起始列 | 结束行 | 结束列
//
// 其中「漏洞详情」可能包含换行，并内嵌形如
// [["标签"|"relative:///app/views/user.py:1:37:1:43"]] 的位置引用。
package codeqlcsv

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"codeql-audit-pro/internal/model"
)

// ColumnCount 是固定的列数。
const ColumnCount = 9

// HeadersEN 是原始英文表头。
var HeadersEN = [ColumnCount]string{
	"name", "description", "severity", "message", "file",
	"start_line", "start_column", "end_line", "end_column",
}

// HeadersZH 是工具重建的中文表头。
var HeadersZH = [ColumnCount]string{
	"规则名称", "规则描述", "严重级别", "漏洞详情", "文件路径",
	"起始行", "起始列", "结束行", "结束列",
}

// linkRe 匹配消息内嵌的数据流位置引用。
var linkRe = regexp.MustCompile(`\[\["(.*?)"\|"(.*?)"\]\]`)

// ParseResult 是一次解析的产出。
type ParseResult struct {
	Findings []*model.Finding
	Warnings []string
	// Skipped 记录因列数不足被跳过的行号。
	Skipped []int
}

// NormalizePath 把 CodeQL 的路径/URI 规范化为「不含 scheme 的斜杠路径」。
//
//	relative:///app/views/user.py     -> app/views/user.py
//	file:///Users/me/src/a.py         -> Users/me/src/a.py
//	/app/views/user.py                -> app/views/user.py
//	C:\src\a.py                       -> C:/src/a.py
func NormalizePath(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// 去掉 scheme（relative:// 、file:// 等）
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	s = strings.ReplaceAll(s, "\\", "/")
	// 折叠连续的斜杠，但保留 Windows 盘符后的结构
	for strings.Contains(s, "//") {
		s = strings.ReplaceAll(s, "//", "/")
	}
	s = strings.TrimPrefix(s, "/")
	return s
}

// SplitLocationURI 拆分「路径:起始行:起始列:结束行:结束列」。
// 数字部分允许缺失，缺失时返回 0。
func SplitLocationURI(uri string) (path string, sl, sc, el, ec int) {
	s := strings.TrimSpace(uri)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	s = strings.ReplaceAll(s, "\\", "/")

	parts := strings.Split(s, ":")
	// 从尾部收集最多 4 个纯数字，其余部分视为路径
	nums := make([]int, 0, 4)
	cut := len(parts)
	for cut > 1 && len(nums) < 4 {
		v, err := strconv.Atoi(parts[cut-1])
		if err != nil {
			break
		}
		nums = append([]int{v}, nums...)
		cut--
	}
	path = strings.Join(parts[:cut], ":")
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	path = strings.TrimPrefix(path, "/")

	switch len(nums) {
	case 4:
		sl, sc, el, ec = nums[0], nums[1], nums[2], nums[3]
	case 3:
		sl, sc, el = nums[0], nums[1], nums[2]
		ec = sc
	case 2:
		sl, sc = nums[0], nums[1]
		el, ec = sl, sc
	case 1:
		sl = nums[0]
		el = sl
	}
	return path, sl, sc, el, ec
}

// ParseMessage 把英文原始消息拆分为「文本 + 链接」片段序列。
func ParseMessage(raw string) []model.Segment {
	matches := linkRe.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		if raw == "" {
			return nil
		}
		return []model.Segment{{Kind: model.SegText, Text: raw}}
	}
	segs := make([]model.Segment, 0, len(matches)*2+1)
	last := 0
	for _, m := range matches {
		if m[0] > last {
			segs = append(segs, model.Segment{Kind: model.SegText, Text: raw[last:m[0]]})
		}
		label := raw[m[2]:m[3]]
		uri := raw[m[4]:m[5]]
		file, sl, sc, el, ec := SplitLocationURI(uri)
		segs = append(segs, model.Segment{
			Kind: model.SegText, // 占位，稍后由渲染层决定
			Text: "",
			Link: &model.LinkRef{
				Label: label, RawURI: uri, File: file,
				StartLine: sl, StartCol: sc, EndLine: el, EndCol: ec,
			},
		})
		last = m[1]
	}
	if last < len(raw) {
		segs = append(segs, model.Segment{Kind: model.SegText, Text: raw[last:]})
	}
	// 标记链接片段
	for i := range segs {
		if segs[i].Link != nil {
			segs[i].Kind = model.SegLink
		}
	}
	return segs
}

// isHeaderRow 判断首行是否为表头（兼容已经带表头的输入）。
func isHeaderRow(rec []string) bool {
	if len(rec) < ColumnCount {
		return false
	}
	if _, err := strconv.Atoi(strings.TrimSpace(rec[5])); err == nil {
		return false // 起始行是数字 -> 数据行
	}
	sev := strings.ToLower(strings.TrimSpace(rec[2]))
	switch sev {
	case "error", "warning", "note", "recommendation":
		return false
	}
	return true
}

func atoiSafe(s string) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return v
}

// Parse 从 reader 中解析 CodeQL CSV。
func Parse(r io.Reader) (*ParseResult, error) {
	br := bufio.NewReader(r)
	// 去掉 UTF-8 BOM
	if b, err := br.Peek(3); err == nil && len(b) == 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		_, _ = br.Discard(3)
	}

	cr := csv.NewReader(br)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.ReuseRecord = false

	res := &ParseResult{}
	lineNo := 0
	idx := 0
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("第 %d 行解析失败: %w", lineNo+1, err)
		}
		lineNo++
		if lineNo == 1 && isHeaderRow(rec) {
			continue // 已经带表头
		}
		if len(rec) < ColumnCount {
			res.Skipped = append(res.Skipped, lineNo)
			res.Warnings = append(res.Warnings,
				fmt.Sprintf("第 %d 行只有 %d 列（应为 %d 列），已跳过", lineNo, len(rec), ColumnCount))
			continue
		}
		if len(rec) > ColumnCount {
			res.Warnings = append(res.Warnings,
				fmt.Sprintf("第 %d 行有 %d 列，多出的列已被忽略", lineNo, len(rec)))
		}

		idx++
		f := &model.Finding{
			Index:       idx,
			RuleName:    strings.TrimSpace(rec[0]),
			Description: strings.TrimSpace(rec[1]),
			Severity:    model.Severity(strings.ToLower(strings.TrimSpace(rec[2]))),
			Sink: model.Location{
				File:      NormalizePath(rec[4]),
				StartLine: atoiSafe(rec[5]),
				StartCol:  atoiSafe(rec[6]),
				EndLine:   atoiSafe(rec[7]),
				EndCol:    atoiSafe(rec[8]),
			},
		}
		f.MessageRaw = rec[3]
		f.MessageEN = ParseMessage(rec[3])
		for _, s := range f.MessageEN {
			if s.Kind == model.SegLink && s.Link != nil {
				f.Sources = append(f.Sources, s.Link.Location())
			}
		}
		res.Findings = append(res.Findings, f)
	}
	if len(res.Findings) == 0 && len(res.Skipped) == 0 {
		return nil, fmt.Errorf("文件中没有解析到任何记录")
	}
	return res, nil
}

// ParseFile 解析指定文件。
func ParseFile(path string) (*ParseResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	res, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return res, nil
}

// ExportZH 导出带中文表头的结果文件。内容为中文（原文缺失时回退英文）。
// 输出带 UTF-8 BOM，保证 Excel 双击打开不乱码。
func ExportZH(w io.Writer, findings []*model.Finding) error {
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(HeadersZH[:]); err != nil {
		return err
	}
	for _, f := range findings {
		msg := model.PlainSegments(f.MessageZH)
		if strings.TrimSpace(msg) == "" {
			msg = model.PlainSegments(f.MessageEN)
		}
		desc := f.DescriptionZH
		if desc == "" {
			desc = f.Description
		}
		rule := f.RuleNameZH
		if rule == "" {
			rule = f.RuleName
		}
		row := []string{
			rule, desc, f.Severity.ZH(), msg,
			f.Sink.File,
			strconv.Itoa(f.Sink.StartLine), strconv.Itoa(f.Sink.StartCol),
			strconv.Itoa(f.Sink.EndLine), strconv.Itoa(f.Sink.EndCol),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// ExportZHFile 导出到文件。
func ExportZHFile(path string, findings []*model.Finding) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return ExportZH(f, findings)
}
