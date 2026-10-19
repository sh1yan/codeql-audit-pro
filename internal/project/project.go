// Package project 把结果解析（CSV / SARIF）、中文翻译、路径映射编排成一个可用的审计项目。
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codeql-audit-pro/internal/codeqlcsv"
	"codeql-audit-pro/internal/codeqlsarif"
	"codeql-audit-pro/internal/model"
	"codeql-audit-pro/internal/pathmap"
	"codeql-audit-pro/internal/translate"
)

// Options 是载入项目的参数。
type Options struct {
	CSVPath     string
	SrcRoot     string
	PrefixRules []pathmap.PrefixRule
	Overrides   map[string]string
	Engine      *translate.Engine
	// Progress 用于向界面汇报进度，可为 nil。
	Progress func(done, total int, msg string)
}

// Stats 是项目统计信息。
type Stats struct {
	Total        int
	RuleKinds    int
	FileKinds    int
	ByRule       map[string]int
	ByFile       map[string]int
	BySeverity   map[string]int
	Resolved     int
	Missing      int
	Pending      int
	FromLLM      int
	WithCodeFlow int
	CWEKinds     int
	RuleNameZH   map[string]string
}

// Project 是一次审计会话。
type Project struct {
	CSVPath     string
	Format      model.ResultFormat
	ToolName    string
	ToolVersion string
	SrcRoot     string
	Findings    []*model.Finding
	Warnings    []string
	Engine      *translate.Engine
	Resolver    *pathmap.Resolver
	Stats       Stats
}

// Load 载入审计项目。根据扩展名 / 文件内容自动识别 CSV 或 SARIF v2.1.0。
func Load(opt Options) (*Project, error) {
	if strings.TrimSpace(opt.CSVPath) == "" {
		return nil, fmt.Errorf("请先选择 CodeQL 结果文件（CSV 或 SARIF v2.1.0）")
	}
	if opt.Engine == nil {
		return nil, fmt.Errorf("翻译引擎未初始化")
	}
	progress := opt.Progress
	if progress == nil {
		progress = func(int, int, string) {}
	}

	format, err := DetectFormat(opt.CSVPath)
	if err != nil {
		return nil, err
	}

	var (
		findings    []*model.Finding
		warnings    []string
		toolName    string
		toolVersion string
	)
	switch format {
	case model.FormatSARIF:
		progress(0, 1, "正在解析 SARIF v2.1.0 …")
		res, err := codeqlsarif.ParseFile(opt.CSVPath)
		if err != nil {
			return nil, err
		}
		findings = res.Findings
		warnings = res.Warnings
		toolName = res.ToolName
		toolVersion = res.Version
	default:
		progress(0, 1, "正在解析 CSV …")
		res, err := codeqlcsv.ParseFile(opt.CSVPath)
		if err != nil {
			return nil, err
		}
		findings = res.Findings
		warnings = res.Warnings
	}
	for _, f := range findings {
		f.Format = format
	}

	p := &Project{
		CSVPath:     opt.CSVPath,
		Format:      format,
		ToolName:    toolName,
		ToolVersion: toolVersion,
		SrcRoot:     strings.TrimSpace(opt.SrcRoot),
		Findings:    findings,
		Warnings:    warnings,
		Engine:      opt.Engine,
	}
	p.Resolver = pathmap.NewResolver(p.SrcRoot, opt.PrefixRules, opt.Overrides)

	total := len(p.Findings)
	opt.Engine.ResetStats()
	for i, f := range p.Findings {
		p.translateFinding(f)
		p.resolveFinding(f)
		if i%10 == 0 || i == total-1 {
			progress(i+1, total, fmt.Sprintf("正在翻译与定位 %d/%d …", i+1, total))
		}
	}
	p.computeStats()
	return p, nil
}

// DetectFormat 根据扩展名与文件头判断结果格式。
// 识别不出来时按 CSV 处理，由解析器给出具体错误。
func DetectFormat(path string) (model.ResultFormat, error) {
	name := strings.ToLower(filepath.Base(strings.TrimSpace(path)))
	switch {
	case strings.HasSuffix(name, ".sarif"),
		strings.HasSuffix(name, ".sarif.json"),
		strings.HasSuffix(name, ".json"):
		return model.FormatSARIF, nil
	case strings.HasSuffix(name, ".csv"), strings.HasSuffix(name, ".txt"):
		return model.FormatCSV, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	head := strings.TrimSpace(string(buf[:n]))
	if strings.HasPrefix(head, "{") {
		return model.FormatSARIF, nil
	}
	return model.FormatCSV, nil
}

// translateFinding 生成一条漏洞的中文内容。
func (p *Project) translateFinding(f *model.Finding) {
	// SARIF 的 RuleName 是 py/sql-injection 这类 ID，真正可读的规则名在
	// rule.shortDescription 里；用后者查词典才能命中内置中文名。
	ruleLabel := f.RuleName
	if f.RuleInfo != nil && strings.TrimSpace(f.RuleInfo.ShortDescription) != "" {
		ruleLabel = f.RuleInfo.ShortDescription
	}
	if zh := p.Engine.TranslateStatic(translate.KindRule, ruleLabel).ZH; zh != "" {
		f.RuleNameZH = zh
	}
	if strings.TrimSpace(f.Description) != "" {
		if zh := p.Engine.TranslateStatic(translate.KindDescription, f.Description).ZH; zh != "" {
			f.DescriptionZH = zh
		}
	}

	key, canon, links := translate.BuildCanonical(f.MessageEN)
	f.MessageKey = key
	r := p.Engine.TranslateMessage(key)
	f.MessageZHTemplate = r.ZH
	vals := make(map[string][]string, len(canon.Vals)+1)
	for k, v := range canon.Vals {
		vals[k] = v
	}
	vals[translate.PhLink] = translate.Sentinels(len(links))
	zh := translate.Fill(r.ZH, vals)
	f.MessageZH = translate.RenderSegments(zh, links, func(l *model.LinkRef) string {
		label := p.Engine.TranslateLabel(l.Label)
		l.LabelZH = label
		return label
	})
	// 用（已带中文标签的）链接重建证据链，保证右栏展示的是中文标签
	if len(links) > 0 {
		srcs := make([]model.Location, 0, len(links))
		for _, l := range links {
			loc := l.Location()
			loc.LabelZH = l.LabelZH
			srcs = append(srcs, loc)
		}
		f.Sources = srcs
	}
	// SARIF 的 codeFlow 节点与 relatedLocations 也带标签，一并中文化。
	for ci := range f.CodeFlows {
		for si := range f.CodeFlows[ci].Steps {
			step := &f.CodeFlows[ci].Steps[si]
			if step.Message != "" {
				step.Location.LabelZH = p.Engine.TranslateLabel(step.Message)
			}
		}
	}
	for ri := range f.RelatedLocations {
		rel := &f.RelatedLocations[ri]
		if rel.Message != "" {
			rel.Location.LabelZH = p.Engine.TranslateLabel(rel.Message)
		}
	}
	f.TranslateSource = r.Source
	f.TranslatePending = r.Pending
}

// resolveLocation 把一处逻辑位置解析成本地文件。
func (p *Project) resolveLocation(loc *model.Location) {
	if loc == nil || strings.TrimSpace(loc.File) == "" {
		return
	}
	if r := p.Resolver.Resolve(loc.File); r.Resolved() {
		loc.ResolvedPath = r.Path
		loc.ResolvedScore = r.Score
	}
}

// clearResolved 清空一处位置的定位结果（换源码目录 / 改映射后重算）。
func clearResolved(loc *model.Location) {
	loc.ResolvedPath, loc.ResolvedScore = "", 0
}

// resolveFinding 定位一条漏洞涉及的源码文件，包括消息来源、完整 codeFlow
// 的每一步与 relatedLocations。
func (p *Project) resolveFinding(f *model.Finding) {
	p.resolveLocation(&f.Sink)
	for i := range f.Sources {
		p.resolveLocation(&f.Sources[i])
	}
	for ci := range f.CodeFlows {
		for si := range f.CodeFlows[ci].Steps {
			p.resolveLocation(&f.CodeFlows[ci].Steps[si].Location)
		}
	}
	for ri := range f.RelatedLocations {
		p.resolveLocation(&f.RelatedLocations[ri].Location)
	}
}

// clearFindingResolved 清空一条漏洞所有位置的定位结果。
func clearFindingResolved(f *model.Finding) {
	clearResolved(&f.Sink)
	for i := range f.Sources {
		clearResolved(&f.Sources[i])
	}
	for ci := range f.CodeFlows {
		for si := range f.CodeFlows[ci].Steps {
			clearResolved(&f.CodeFlows[ci].Steps[si].Location)
		}
	}
	for ri := range f.RelatedLocations {
		clearResolved(&f.RelatedLocations[ri].Location)
	}
}

// ResolveLink 按需解析消息内嵌链接的目标位置。
func (p *Project) ResolveLink(l *model.LinkRef) (string, int, bool) {
	if l == nil {
		return "", 0, false
	}
	r := p.Resolver.Resolve(l.File)
	if !r.Resolved() {
		return "", 0, false
	}
	return r.Path, l.StartLine, true
}

// ResolveFinding 重新解析一条漏洞（用户修改映射后调用）。
func (p *Project) ResolveFinding(f *model.Finding) {
	p.Resolver = pathmap.NewResolver(p.SrcRoot, p.Resolver.PrefixRules(), p.Resolver.Overrides())
	p.resolveFinding(f)
}

// SetSrcRoot 更换源码目录并重新定位全部漏洞。
func (p *Project) SetSrcRoot(root string) {
	p.SrcRoot = strings.TrimSpace(root)
	p.Resolver = pathmap.NewResolver(p.SrcRoot, p.Resolver.PrefixRules(), p.Resolver.Overrides())
	for _, f := range p.Findings {
		clearFindingResolved(f)
		p.resolveFinding(f)
	}
	p.computeStats()
}

// SetOverride 记录一条人工映射。
func (p *Project) SetOverride(csvPath, localPath string) {
	p.Resolver.SetOverride(csvPath, localPath)
	// 覆盖可能指向 sinks / sources / codeFlow 中的任意一处，统一重算。
	for _, f := range p.Findings {
		clearFindingResolved(f)
		p.resolveFinding(f)
	}
	p.computeStats()
}

func (p *Project) computeStats() {
	s := Stats{
		Total:      len(p.Findings),
		ByRule:     map[string]int{},
		ByFile:     map[string]int{},
		BySeverity: map[string]int{},
		RuleNameZH: map[string]string{},
	}
	for _, f := range p.Findings {
		s.ByRule[f.RuleName]++
		s.ByFile[f.Sink.File]++
		s.BySeverity[string(f.Severity)]++
		if f.RuleNameZH != "" {
			s.RuleNameZH[f.RuleName] = f.RuleNameZH
		}
		if f.Sink.Resolved() {
			s.Resolved++
		} else {
			s.Missing++
		}
		if f.TranslatePending {
			s.Pending++
		}
		if f.TranslateSource == translate.SourceLLM {
			s.FromLLM++
		}
		if f.HasCodeFlow() {
			s.WithCodeFlow++
		}
	}
	cwes := map[string]bool{}
	for _, f := range p.Findings {
		for _, cwe := range f.CWEs {
			cwes[cwe] = true
		}
	}
	s.CWEKinds = len(cwes)
	s.RuleKinds = len(s.ByRule)
	s.FileKinds = len(s.ByFile)
	p.Stats = s
}

// Rules 返回按出现次数排序的规则列表。
func (p *Project) Rules() []string { return sortedKeys(p.Stats.ByRule) }

// SortedSeverities 返回出现过的严重级别，按严重程度排序。
func (p *Project) SortedSeverities() []string {
	order := []string{"error", "warning", "recommendation", "note"}
	out := make([]string, 0, len(p.Stats.BySeverity))
	for _, s := range order {
		if p.Stats.BySeverity[s] > 0 {
			out = append(out, s)
		}
	}
	for s, n := range p.Stats.BySeverity {
		if n > 0 && !containsStr(order, s) {
			out = append(out, s)
		}
	}
	return out
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// AddPrefixRule 添加一条路径前缀规则并重新定位。
func (p *Project) AddPrefixRule(from, to string) {
	p.Resolver.AddPrefixRule(from, to)
	for _, f := range p.Findings {
		clearFindingResolved(f)
		p.resolveFinding(f)
	}
	p.computeStats()
}

// Retranslate 在译文被修改后重新生成一条漏洞的中文内容。
func (p *Project) Retranslate(f *model.Finding) {
	if f == nil || p.Engine == nil {
		return
	}
	key, _, _ := translate.BuildCanonical(f.MessageEN)
	p.Engine.Invalidate(translate.KindMessage, key)
	for _, ln := range strings.Split(key, "\n") {
		p.Engine.Invalidate(translate.KindMessage, ln)
	}
	ruleLabel := f.RuleName
	if f.RuleInfo != nil && strings.TrimSpace(f.RuleInfo.ShortDescription) != "" {
		ruleLabel = f.RuleInfo.ShortDescription
	}
	p.Engine.Invalidate(translate.KindRule, ruleLabel)
	p.Engine.Invalidate(translate.KindDescription, f.Description)
	p.translateFinding(f)
	p.resolveFinding(f)
	p.computeStats()
}

// Files 返回按出现次数排序的文件列表。
func (p *Project) Files() []string { return sortedKeys(p.Stats.ByFile) }

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if m[out[i]] != m[out[j]] {
			return m[out[i]] > m[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

// Filter 是漏洞列表的过滤条件。
type Filter struct {
	Keyword        string
	Rule           string
	File           string
	Severity       string
	OnlyUnresolved bool
}

// Apply 按条件过滤漏洞。
func (p *Project) Apply(f Filter) []*model.Finding {
	kw := strings.ToLower(strings.TrimSpace(f.Keyword))
	out := make([]*model.Finding, 0, len(p.Findings))
	for _, fd := range p.Findings {
		if f.Rule != "" && fd.RuleName != f.Rule {
			continue
		}
		if f.File != "" && fd.Sink.File != f.File {
			continue
		}
		if f.Severity != "" && string(fd.Severity) != f.Severity {
			continue
		}
		if f.OnlyUnresolved && fd.Sink.Resolved() {
			continue
		}
		if kw != "" {
			hay := strings.ToLower(strings.Join([]string{
				fd.RuleName, fd.RuleNameZH, fd.DescriptionZH,
				fd.Sink.File, fd.MessageENText(), fd.MessageZHText(),
			}, "\n"))
			if !strings.Contains(hay, kw) {
				continue
			}
		}
		out = append(out, fd)
	}
	return out
}

// ExportZH 导出带中文表头的结果文件。
func (p *Project) ExportZH(path string, findings []*model.Finding) error {
	if len(findings) == 0 {
		findings = p.Findings
	}
	return codeqlcsv.ExportZHFile(path, findings)
}

// UnresolvedFiles 返回所有未定位成功的逻辑路径（去重）。
func (p *Project) UnresolvedFiles() []string {
	set := map[string]bool{}
	for _, f := range p.Findings {
		if !f.Sink.Resolved() {
			set[f.Sink.File] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
