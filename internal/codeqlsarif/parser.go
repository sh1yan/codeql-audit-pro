// Package codeqlsarif 负责解析 CodeQL 导出的 SARIF v2.1.0 结果文件。
//
// 与 codeqlcsv 相比，SARIF 除了「规则 / 级别 / 消息 / 位置」之外，还带有：
//
//   - tool.driver.rules 中的规则元数据：CWE、precision、security-severity、tags；
//   - codeFlows[].threadFlows[].locations[] 中的完整污点传播路径；
//   - relatedLocations 与消息里的 [标签](id) 引用；
//   - partialFingerprints / fingerprints 指纹；
//   - 标准化后的 artifact 列表与逻辑位置。
//
// 本包把这些字段全部转成 model.Finding，使审计工作台在载入 SARIF 后能展示
// 与 CSV 相同的全部能力，并额外提供污点路径、CWE 与指纹视图。
package codeqlsarif

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"codeql-audit-pro/internal/codeqlcsv"
	"codeql-audit-pro/internal/model"
)

// ParseResult 是一次 SARIF 解析的产出。
type ParseResult struct {
	Findings []*model.Finding
	Warnings []string
	ToolName string
	Version  string
}

// ------------------------- SARIF 数据结构 -------------------------

type logFile struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []run  `json:"runs"`
}

type run struct {
	Tool      tool       `json:"tool"`
	Artifacts []artifact `json:"artifacts"`
	Results   []result   `json:"results"`
}

type tool struct {
	Driver driver `json:"driver"`
}

type driver struct {
	Name            string `json:"name"`
	Organization    string `json:"organization"`
	SemanticVersion string `json:"semanticVersion"`
	Rules           []rule `json:"rules"`
}

type rule struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	ShortDescription     message           `json:"shortDescription"`
	FullDescription      message           `json:"fullDescription"`
	HelpURI              string            `json:"helpUri"`
	DefaultConfiguration ruleDefaultConfig `json:"defaultConfiguration"`
	Properties           ruleProperties    `json:"properties"`
}

type ruleDefaultConfig struct {
	Enabled bool   `json:"enabled"`
	Level   string `json:"level"`
}

type ruleProperties struct {
	Tags             []string `json:"tags"`
	Description      string   `json:"description"`
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Kind             string   `json:"kind"`
	Precision        string   `json:"precision"`
	ProblemSeverity  string   `json:"problem.severity"`
	SecuritySeverity string   `json:"security-severity"`
	SubSeverity      string   `json:"sub-severity"`
}

type artifact struct {
	Location struct {
		URI       string `json:"uri"`
		URIBaseID string `json:"uriBaseId"`
	} `json:"location"`
}

type result struct {
	RuleID              string                 `json:"ruleId"`
	RuleIndex           *int                   `json:"ruleIndex"`
	Rule                *resultRule            `json:"rule"`
	Level               string                 `json:"level"`
	Message             message                `json:"message"`
	Locations           []location             `json:"locations"`
	RelatedLocations    []relatedLocation      `json:"relatedLocations"`
	CodeFlows           []codeFlow             `json:"codeFlows"`
	PartialFingerprints map[string]string      `json:"partialFingerprints"`
	Fingerprints        map[string]string      `json:"fingerprints"`
	Properties          map[string]interface{} `json:"properties"`
}

type resultRule struct {
	ID    string `json:"id"`
	Index *int   `json:"index"`
}

type message struct {
	Text     string `json:"text"`
	Markdown string `json:"markdown"`
}

func (m message) body() string {
	if strings.TrimSpace(m.Text) != "" {
		return m.Text
	}
	return m.Markdown
}

type location struct {
	PhysicalLocation *physicalLocation `json:"physicalLocation"`
	Message          message           `json:"message"`
}

type physicalLocation struct {
	ArtifactLocation artifactLocation `json:"artifactLocation"`
	Region           region           `json:"region"`
}

type artifactLocation struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId"`
	Index     *int   `json:"index"`
}

type region struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
	EndLine     int `json:"endLine"`
	EndColumn   int `json:"endColumn"`
	CharOffset  int `json:"charOffset"`
	CharLength  int `json:"charLength"`
}

type relatedLocation struct {
	ID               *int              `json:"id"`
	PhysicalLocation *physicalLocation `json:"physicalLocation"`
	Message          message           `json:"message"`
}

type codeFlow struct {
	Message     message      `json:"message"`
	ThreadFlows []threadFlow `json:"threadFlows"`
}

type threadFlow struct {
	Message   message              `json:"message"`
	Locations []threadFlowLocation `json:"locations"`
}

type threadFlowLocation struct {
	Location     location `json:"location"`
	NestingLevel int      `json:"nestingLevel"`
	Kinds        []string `json:"kinds"`
	Importance   string   `json:"importance"`
}

// markdownLinkRe 匹配 SARIF 消息中的 Markdown 链接：[标签](1)。
// 只有目标是纯数字（relatedLocations.id）的链接才会被转成可跳转证据链。
var markdownLinkRe = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)\)`)

// cweRe 从标签中提取 CWE 编号，例如 external/cwe/cwe-094。
var cweRe = regexp.MustCompile(`(?i)cwe[-_]?(\d+)`)

// Parse 从 reader 中解析 SARIF v2.1.0。
func Parse(r io.Reader) (*ParseResult, error) {
	var doc logFile
	dec := json.NewDecoder(r)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("SARIF JSON 解析失败：%w", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(doc.Version), "2.1") {
		return nil, fmt.Errorf("不支持的 SARIF 版本 %q（仅支持 v2.1.0）", doc.Version)
	}
	if len(doc.Runs) == 0 {
		return nil, fmt.Errorf("SARIF 文件中没有 runs")
	}

	res := &ParseResult{}
	if len(doc.Runs) > 1 {
		res.Warnings = append(res.Warnings,
			fmt.Sprintf("SARIF 含有 %d 个 run，已按顺序合并", len(doc.Runs)))
	}

	index := 0
	for runIdx, rn := range doc.Runs {
		if runIdx == 0 {
			res.ToolName = rn.Tool.Driver.Name
			res.Version = rn.Tool.Driver.SemanticVersion
		}
		artifacts := artifactURIs(rn.Artifacts)
		rules := ruleIndex(rn.Tool.Driver.Rules)
		for _, raw := range rn.Results {
			index++
			f, err := convertResult(raw, index, rn, rules, artifacts, res.ToolName, res.Version)
			if err != nil {
				res.Warnings = append(res.Warnings,
					fmt.Sprintf("第 %d 条结果已跳过：%v", index, err))
				index--
				continue
			}
			res.Findings = append(res.Findings, f)
		}
	}

	if len(res.Findings) == 0 {
		return nil, fmt.Errorf("SARIF 文件中没有可用的 results")
	}
	return res, nil
}

// ParseFile 解析指定 SARIF 文件。
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

// artifactURIs 把 run.artifacts 展开成 index -> uri 的映射。
func artifactURIs(items []artifact) []string {
	out := make([]string, len(items))
	for i, a := range items {
		out[i] = a.Location.URI
	}
	return out
}

// ruleIndex 同时按数组下标与规则 ID 索引规则。
func ruleIndex(rules []rule) struct {
	byIndex map[int]rule
	byID    map[string]rule
} {
	idx := struct {
		byIndex map[int]rule
		byID    map[string]rule
	}{map[int]rule{}, map[string]rule{}}
	for i, r := range rules {
		idx.byIndex[i] = r
		if r.ID != "" {
			idx.byID[r.ID] = r
		}
	}
	return idx
}

func convertResult(
	raw result,
	seq int,
	rn run,
	rules struct {
		byIndex map[int]rule
		byID    map[string]rule
	},
	artifacts []string,
	toolName, toolVersion string,
) (*model.Finding, error) {
	sink, ok := firstLocation(raw.Locations, artifacts)
	if !ok {
		if len(raw.CodeFlows) > 0 {
			// 少数工具把最终位置只写在 codeFlow 末尾，兜底取最后一步。
			if loc, ok := flowFallback(raw.CodeFlows, artifacts); ok {
				sink = loc
				ok = true
			}
		}
	}
	if !ok {
		return nil, fmt.Errorf("缺少 locations，无法确定沉淀点")
	}

	rule, ruleMeta := resolveRule(raw, rules)
	ruleID := strings.TrimSpace(raw.RuleID)
	if ruleID == "" && ruleMeta != nil {
		ruleID = ruleMeta.ID
	}
	if ruleID == "" {
		ruleID = "unknown-rule"
	}

	severity := resolveSeverity(raw.Level, rule)
	description := ""
	if ruleMeta != nil {
		description = ruleMeta.FullDescription
		if strings.TrimSpace(description) == "" {
			description = ruleMeta.ShortDescription
		}
	}
	if strings.TrimSpace(description) == "" {
		description = strings.TrimSpace(rule.Properties.Description)
	}

	f := &model.Finding{
		Index:       seq,
		RuleName:    ruleID,
		Description: description,
		Severity:    severity,
		Sink:        sink,
		Format:      model.FormatSARIF,
		ToolName:    toolName,
		ToolVersion: toolVersion,
		RuleInfo:    ruleMeta,
	}
	if ruleMeta != nil {
		f.CWEs = append([]string(nil), ruleMeta.CWEs...)
		f.Tags = append([]string(nil), ruleMeta.Tags...)
	}

	// 相关位置先解析，消息里的 [标签](id) 才能变成可跳转链接。
	related := make(map[int]model.Location, len(raw.RelatedLocations))
	for i, rl := range raw.RelatedLocations {
		loc, ok := locationFromPhysical(rl.PhysicalLocation, artifacts)
		if !ok {
			continue
		}
		id := i + 1
		if rl.ID != nil {
			id = *rl.ID
		}
		loc.Label = messageText(rl.Message)
		related[id] = loc
		f.RelatedLocations = append(f.RelatedLocations, model.RelatedLocation{
			ID:       id,
			Location: loc,
			Message:  messageText(rl.Message),
		})
	}

	f.MessageRaw = messageText(raw.Message)
	f.MessageEN = ParseMessage(f.MessageRaw, related)
	for _, seg := range f.MessageEN {
		if seg.Kind == model.SegLink && seg.Link != nil {
			loc := seg.Link.Location()
			if !hasLocation(f.Sources, loc) {
				f.Sources = append(f.Sources, loc)
			}
		}
	}

	f.CodeFlows = parseCodeFlows(raw.CodeFlows, artifacts)
	f.Fingerprints = parseFingerprints(raw.PartialFingerprints, raw.Fingerprints)
	f.PrimaryFingerprint = primaryFingerprint(f.Fingerprints)

	return f, nil
}

func resolveRule(raw result, rules struct {
	byIndex map[int]rule
	byID    map[string]rule
}) (rule, *model.RuleInfo) {
	var (
		r  rule
		ok bool
	)
	if raw.RuleIndex != nil {
		r, ok = rules.byIndex[*raw.RuleIndex]
	}
	if !ok && raw.Rule != nil {
		if raw.Rule.Index != nil {
			r, ok = rules.byIndex[*raw.Rule.Index]
		}
		if !ok && raw.Rule.ID != "" {
			r, ok = rules.byID[raw.Rule.ID]
		}
	}
	if !ok && raw.RuleID != "" {
		r, ok = rules.byID[raw.RuleID]
	}
	if !ok {
		return rule{}, nil
	}

	tags := append([]string(nil), r.Properties.Tags...)
	info := &model.RuleInfo{
		ID:               firstNonEmpty(r.ID, raw.RuleID),
		Name:             r.Name,
		ShortDescription: messageText(r.ShortDescription),
		FullDescription:  messageText(r.FullDescription),
		HelpURI:          r.HelpURI,
		DefaultLevel:     r.DefaultConfiguration.Level,
		SecuritySeverity: r.Properties.SecuritySeverity,
		Precision:        r.Properties.Precision,
		Kind:             r.Properties.Kind,
		ProblemSeverity:  r.Properties.ProblemSeverity,
		Tags:             tags,
		CWEs:             extractCWEs(tags),
	}
	return r, info
}

// resolveSeverity 按 SARIF 的优先级确定严重级别：
// result.level -> rule.defaultConfiguration.level -> properties.problem.severity ->
// security-severity 数值。
func resolveSeverity(resultLevel string, r rule) model.Severity {
	candidates := []string{
		resultLevel,
		r.DefaultConfiguration.Level,
		r.Properties.ProblemSeverity,
	}
	for _, c := range candidates {
		if s := severityFromLevel(c); s != "" {
			return s
		}
	}
	if score, err := strconv.ParseFloat(strings.TrimSpace(r.Properties.SecuritySeverity), 64); err == nil {
		switch {
		case score >= 7:
			return model.SevError
		case score >= 4:
			return model.SevWarning
		default:
			return model.SevNote
		}
	}
	return model.SevNote
}

func severityFromLevel(level string) model.Severity {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "error":
		return model.SevError
	case "warning":
		return model.SevWarning
	case "note", "none":
		return model.SevNote
	case "recommendation":
		return model.SevRecommendation
	}
	return ""
}

// extractCWEs 从 CodeQL 的 external/cwe/cwe-094 标签中提取并去重。
func extractCWEs(tags []string) []string {
	seen := map[int]bool{}
	nums := make([]int, 0, len(tags))
	for _, tag := range tags {
		m := cweRe.FindStringSubmatch(tag)
		if len(m) != 2 {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil || seen[n] {
			continue
		}
		seen[n] = true
		nums = append(nums, n)
	}
	sort.Ints(nums)
	out := make([]string, 0, len(nums))
	for _, n := range nums {
		out = append(out, fmt.Sprintf("CWE-%d", n))
	}
	return out
}

func parseCodeFlows(flows []codeFlow, artifacts []string) []model.CodeFlow {
	out := make([]model.CodeFlow, 0, len(flows))
	for _, cf := range flows {
		flowMessage := messageText(cf.Message)
		for _, tf := range cf.ThreadFlows {
			msg := flowMessage
			if strings.TrimSpace(msg) == "" {
				msg = messageText(tf.Message)
			}
			steps := make([]model.FlowStep, 0, len(tf.Locations))
			for i, tfl := range tf.Locations {
				loc, ok := locationFromPhysical(tfl.Location.PhysicalLocation, artifacts)
				if !ok {
					continue
				}
				loc.Label = messageText(tfl.Location.Message)
				steps = append(steps, model.FlowStep{
					Index:        i + 1,
					Location:     loc,
					Message:      loc.Label,
					NestingLevel: tfl.NestingLevel,
					Kinds:        append([]string(nil), tfl.Kinds...),
				})
			}
			if len(steps) == 0 {
				continue
			}
			out = append(out, model.CodeFlow{
				Index:   len(out) + 1,
				Message: msg,
				Steps:   steps,
			})
		}
	}
	return out
}

func parseFingerprints(partial, full map[string]string) []model.Fingerprint {
	seen := map[string]bool{}
	out := make([]model.Fingerprint, 0, len(partial)+len(full))
	add := func(m map[string]string) {
		for name, value := range m {
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, model.Fingerprint{Name: name, Value: value})
		}
	}
	add(partial)
	add(full)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func primaryFingerprint(fps []model.Fingerprint) string {
	// CodeQL 的 primaryLocationLineHash 对行内容做哈希，
	// primaryLocationStartColumnFingerprint 补足同一行多个告警的区分度。
	want := []string{"primaryLocationLineHash", "primaryLocationStartColumnFingerprint"}
	parts := make([]string, 0, len(want))
	for _, name := range want {
		for _, fp := range fps {
			if fp.Name == name {
				parts = append(parts, name+"="+fp.Value)
			}
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "|")
	}
	if len(fps) > 0 {
		return fps[0].Name + "=" + fps[0].Value
	}
	return ""
}

func firstLocation(locs []location, artifacts []string) (model.Location, bool) {
	for _, l := range locs {
		if loc, ok := locationFromPhysical(l.PhysicalLocation, artifacts); ok {
			loc.Label = messageText(l.Message)
			return loc, true
		}
	}
	return model.Location{}, false
}

func flowFallback(flows []codeFlow, artifacts []string) (model.Location, bool) {
	for i := len(flows) - 1; i >= 0; i-- {
		for j := len(flows[i].ThreadFlows) - 1; j >= 0; j-- {
			locs := flows[i].ThreadFlows[j].Locations
			for k := len(locs) - 1; k >= 0; k-- {
				if loc, ok := locationFromPhysical(locs[k].Location.PhysicalLocation, artifacts); ok {
					return loc, true
				}
			}
		}
	}
	return model.Location{}, false
}

func locationFromPhysical(pl *physicalLocation, artifacts []string) (model.Location, bool) {
	if pl == nil {
		return model.Location{}, false
	}
	uri := pl.ArtifactLocation.URI
	if strings.TrimSpace(uri) == "" && pl.ArtifactLocation.Index != nil {
		idx := *pl.ArtifactLocation.Index
		if idx >= 0 && idx < len(artifacts) {
			uri = artifacts[idx]
		}
	}
	if strings.TrimSpace(uri) == "" {
		return model.Location{}, false
	}
	path := codeqlcsv.NormalizePath(uri)
	if unescaped, err := url.PathUnescape(path); err == nil {
		path = unescaped
	}
	r := pl.Region
	sl, sc := r.StartLine, r.StartColumn
	el, ec := r.EndLine, r.EndColumn
	if sl <= 0 {
		return model.Location{}, false
	}
	if el <= 0 {
		el = sl
	}
	if ec <= 0 {
		ec = sc
	}
	return model.Location{
		File:      path,
		StartLine: sl,
		StartCol:  sc,
		EndLine:   el,
		EndCol:    ec,
	}, true
}

// ParseMessage 把 SARIF 的 Markdown 消息拆成「文本 + 相关位置链接」片段。
//
//	[Stack trace information](1) flows to this location ...
//
// 数字目标对应 relatedLocations[].id；外部 URL 保留为普通文本。
func ParseMessage(raw string, related map[int]model.Location) []model.Segment {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	matches := markdownLinkRe.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		return []model.Segment{{Kind: model.SegText, Text: raw}}
	}
	segs := make([]model.Segment, 0, len(matches)*2+1)
	last := 0
	for _, m := range matches {
		label := strings.TrimSpace(raw[m[2]:m[3]])
		target := strings.TrimSpace(raw[m[4]:m[5]])
		if m[0] > last {
			segs = append(segs, model.Segment{Kind: model.SegText, Text: raw[last:m[0]]})
		}
		id, err := strconv.Atoi(target)
		loc, ok := related[id]
		if err != nil || !ok {
			// 外链或无法解析的引用：原样保留 Markdown，避免丢信息。
			segs = append(segs, model.Segment{Kind: model.SegText, Text: raw[m[0]:m[1]]})
			last = m[1]
			continue
		}
		rawURI := fmt.Sprintf("relative:///%s:%d:%d:%d:%d",
			loc.File, loc.StartLine, loc.StartCol, loc.EndLine, loc.EndCol)
		segs = append(segs, model.Segment{
			Kind: model.SegLink,
			Link: &model.LinkRef{
				Label:     label,
				RawURI:    rawURI,
				File:      loc.File,
				StartLine: loc.StartLine,
				StartCol:  loc.StartCol,
				EndLine:   loc.EndLine,
				EndCol:    loc.EndCol,
			},
		})
		last = m[1]
	}
	if last < len(raw) {
		segs = append(segs, model.Segment{Kind: model.SegText, Text: raw[last:]})
	}
	return segs
}

func messageText(m message) string {
	return strings.TrimSpace(m.body())
}

func hasLocation(list []model.Location, loc model.Location) bool {
	for _, l := range list {
		if l.File == loc.File && l.StartLine == loc.StartLine && l.StartCol == loc.StartCol {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
