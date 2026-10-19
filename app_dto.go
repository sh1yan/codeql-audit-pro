package main

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codeql-audit-pro/internal/model"
	"codeql-audit-pro/internal/translate"
	"codeql-audit-pro/internal/verify"
)

// 本文件定义暴露给 Wails 前端的 DTO。
//
// 前端只认这些带 json tag 的结构，Go 内部的 model.Finding 不直接外泄：
// 这样既可以按界面需要补齐「展示字段」（中文级别、行范围、已读状态、AI 徽章），
// 也不会因为内部结构演进把前端打挂。

// LocationDTO 是一处源码位置。
type LocationDTO struct {
	File         string `json:"file"`
	ResolvedPath string `json:"resolvedPath"`
	StartLine    int    `json:"startLine"`
	StartCol     int    `json:"startCol"`
	EndLine      int    `json:"endLine"`
	EndCol       int    `json:"endCol"`
	Label        string `json:"label"`
	LabelZh      string `json:"labelZh"`
	Resolved     bool   `json:"resolved"`
	LineRange    string `json:"lineRange"`
	ColumnRange  string `json:"columnRange"`
	LineCol      string `json:"lineCol"`
	DisplayPath  string `json:"displayPath"`
	BaseName     string `json:"baseName"`
}

// LinkDTO 是漏洞详情里内嵌的数据流位置链接。
type LinkDTO struct {
	Label     string `json:"label"`
	LabelZh   string `json:"labelZh"`
	RawURI    string `json:"rawUri"`
	File      string `json:"file"`
	StartLine int    `json:"startLine"`
	StartCol  int    `json:"startCol"`
	EndLine   int    `json:"endLine"`
	EndCol    int    `json:"endCol"`
}

// SegmentDTO 是消息片段：普通文本或可点击的位置链接。
type SegmentDTO struct {
	Kind string   `json:"kind"` // text / link
	Text string   `json:"text"`
	Link *LinkDTO `json:"link,omitempty"`
}

// AIDTO 是一条漏洞的 AI 误报判定结果。
type AIDTO struct {
	Verdict    string  `json:"verdict"`
	VerdictZh  string  `json:"verdictZh"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
	Model      string  `json:"model"`
	At         string  `json:"at"`
	Short      string  `json:"short"`
}

// FingerprintDTO 是一条 SARIF 指纹。
type FingerprintDTO struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RuleInfoDTO 是 SARIF 规则元数据。
type RuleInfoDTO struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	ShortDescription string   `json:"shortDescription"`
	FullDescription  string   `json:"fullDescription"`
	HelpURI          string   `json:"helpUri"`
	DefaultLevel     string   `json:"defaultLevel"`
	SecuritySeverity string   `json:"securitySeverity"`
	Precision        string   `json:"precision"`
	Kind             string   `json:"kind"`
	ProblemSeverity  string   `json:"problemSeverity"`
	Tags             []string `json:"tags"`
	CWEs             []string `json:"cwes"`
}

// FlowStepDTO 是污点传播路径上的一个节点。
type FlowStepDTO struct {
	Index        int         `json:"index"`
	Location     LocationDTO `json:"location"`
	Message      string      `json:"message"`
	NestingLevel int         `json:"nestingLevel"`
	Kinds        []string    `json:"kinds"`
}

// CodeFlowDTO 是一条完整的污点传播路径。
type CodeFlowDTO struct {
	Index   int           `json:"index"`
	Message string        `json:"message"`
	Steps   []FlowStepDTO `json:"steps"`
}

// RelatedLocationDTO 是 SARIF 的相关位置。
type RelatedLocationDTO struct {
	ID       int         `json:"id"`
	Location LocationDTO `json:"location"`
	Message  string      `json:"message"`
}

// FindingDTO 是漏洞列表与详情页需要的全部信息。
type FindingDTO struct {
	Key                string               `json:"key"`
	Index              int                  `json:"index"`
	RuleName           string               `json:"ruleName"`
	RuleNameZh         string               `json:"ruleNameZh"`
	RuleDisplay        string               `json:"ruleDisplay"`
	Description        string               `json:"description"`
	DescriptionZh      string               `json:"descriptionZh"`
	DescriptionDisplay string               `json:"descriptionDisplay"`
	Severity           string               `json:"severity"`
	SeverityZh         string               `json:"severityZh"`
	Sink               LocationDTO          `json:"sink"`
	Sources            []LocationDTO        `json:"sources"`
	MessageEn          []SegmentDTO         `json:"messageEn"`
	MessageZh          []SegmentDTO         `json:"messageZh"`
	MessageRaw         string               `json:"messageRaw"`
	MessageKey         string               `json:"messageKey"`
	MessageZhTemplate  string               `json:"messageZhTemplate"`
	TranslateSource    string               `json:"translateSource"`
	TranslateSourceZh  string               `json:"translateSourceZh"`
	TranslatePending   bool                 `json:"translatePending"`
	Reviewed           bool                 `json:"reviewed"`
	AI                 *AIDTO               `json:"ai"`
	Format             string               `json:"format"`
	FormatLabel        string               `json:"formatLabel"`
	ToolName           string               `json:"toolName"`
	ToolVersion        string               `json:"toolVersion"`
	CWEs               []string             `json:"cwes"`
	Tags               []string             `json:"tags"`
	Fingerprints       []FingerprintDTO     `json:"fingerprints"`
	PrimaryFingerprint string               `json:"primaryFingerprint"`
	RuleInfo           *RuleInfoDTO         `json:"ruleInfo"`
	CodeFlows          []CodeFlowDTO        `json:"codeFlows"`
	RelatedLocations   []RelatedLocationDTO `json:"relatedLocations"`
}

// StatsDTO 是底栏与项目卡片的统计快照。
type StatsDTO struct {
	Total               int            `json:"total"`
	RuleKinds           int            `json:"ruleKinds"`
	FileKinds           int            `json:"fileKinds"`
	Resolved            int            `json:"resolved"`
	Missing             int            `json:"missing"`
	Unread              int            `json:"unread"`
	Pending             int            `json:"pending"`
	Verified            int            `json:"verified"`
	FalsePositive       int            `json:"falsePositive"`
	TruePositive        int            `json:"truePositive"`
	Uncertain           int            `json:"uncertain"`
	TranslationBuiltin  int            `json:"translationBuiltin"`
	TranslationStore    int            `json:"translationStore"`
	TranslationLLM      int            `json:"translationLLM"`
	BySeverity          map[string]int `json:"bySeverity"`
	TotalSourceLocation int            `json:"totalSourceLocation"`
	WithCodeFlow        int            `json:"withCodeFlow"`
	CWEKinds            int            `json:"cweKinds"`
}

// OptionDTO 是筛选下拉的一项。
type OptionDTO struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// UnresolvedDTO 是一个尚未定位到本地源码的逻辑路径。
type UnresolvedDTO struct {
	File       string   `json:"file"`
	Count      int      `json:"count"`
	Severities []string `json:"severities"`
}

// PrefixRuleDTO 是一条路径前缀替换规则。
type PrefixRuleDTO struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ProjectDTO 是一次审计项目的完整前端载荷。
type ProjectDTO struct {
	CSVPath       string            `json:"csvPath"`
	CSVName       string            `json:"csvName"`
	Format        string            `json:"format"`
	FormatLabel   string            `json:"formatLabel"`
	ToolName      string            `json:"toolName"`
	ToolVersion   string            `json:"toolVersion"`
	SrcRoot       string            `json:"srcRoot"`
	Findings      []FindingDTO      `json:"findings"`
	Stats         StatsDTO          `json:"stats"`
	Rules         []OptionDTO       `json:"rules"`
	Files         []OptionDTO       `json:"files"`
	Severities    []OptionDTO       `json:"severities"`
	Warnings      []string          `json:"warnings"`
	Unresolved    []UnresolvedDTO   `json:"unresolved"`
	PrefixRules   []PrefixRuleDTO   `json:"prefixRules"`
	Overrides     map[string]string `json:"overrides"`
	LoadedAt      string            `json:"loadedAt"`
	TranslationDB string            `json:"translationDb"`
}

// LLMSettingsDTO 是大模型配置。
type LLMSettingsDTO struct {
	Enabled        bool   `json:"enabled"`
	BaseURL        string `json:"baseUrl"`
	APIKey         string `json:"apiKey"`
	Model          string `json:"model"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	TemplateOnly   bool   `json:"templateOnly"`
}

// EditorSettingsDTO 是外部编辑器配置。
type EditorSettingsDTO struct {
	Cmd  string `json:"cmd"`
	Args string `json:"args"`
}

// SettingsDTO 是可编辑的持久化设置。
type SettingsDTO struct {
	LLM                LLMSettingsDTO    `json:"llm"`
	AIVerifyEnabled    bool              `json:"aiVerifyEnabled"`
	AISendCode         bool              `json:"aiSendCode"`
	Editor             EditorSettingsDTO `json:"editor"`
	DisplayLang        string            `json:"displayLang"`
	ShowColumnRuler    bool              `json:"showColumnRuler"`
	CodeSearchShortcut string            `json:"codeSearchShortcut"`
	TranslateDB        string            `json:"translateDb"`
	DataDir            string            `json:"dataDir"`
	ConfigPath         string            `json:"configPath"`
	WindowWidth        int               `json:"windowWidth"`
	WindowHeight       int               `json:"windowHeight"`
	SplitLeft          float64           `json:"splitLeft"`
	SplitDetail        float64           `json:"splitDetail"`
	TopInfoCollapsed   bool              `json:"topInfoCollapsed"`
}

// BootstrapDTO 是前端启动时拿到的第一份数据。
type BootstrapDTO struct {
	Settings              SettingsDTO `json:"settings"`
	LastProject           string      `json:"lastProject"`
	LastSrcRoot           string      `json:"lastSrcRoot"`
	AutoLoad              bool        `json:"autoLoad"`
	Platform              string      `json:"platform"`
	Version               string      `json:"version"`
	DefaultSearchShortcut string      `json:"defaultSearchShortcut"`
	DefaultEditorArgs     string      `json:"defaultEditorArgs"`
	DataDir               string      `json:"dataDir"`
	ConfigPath            string      `json:"configPath"`
	Warning               string      `json:"warning,omitempty"`
}

// FileDTO 是源码文件内容。
type FileDTO struct {
	Path        string   `json:"path"`
	DisplayPath string   `json:"displayPath"`
	Encoding    string   `json:"encoding"`
	Lines       []string `json:"lines"`
	TotalLines  int      `json:"totalLines"`
}

// VerifyResponseDTO 是单条 AI 验证的返回。
type VerifyResponseDTO struct {
	FindingKey string   `json:"findingKey"`
	AI         *AIDTO   `json:"ai"`
	Stats      StatsDTO `json:"stats"`
}

// BatchProgressDTO 是批量验证的进度事件载荷。
type BatchProgressDTO struct {
	Done          int    `json:"done"`
	Total         int    `json:"total"`
	FalsePositive int    `json:"falsePositive"`
	TruePositive  int    `json:"truePositive"`
	Uncertain     int    `json:"uncertain"`
	Failed        int    `json:"failed"`
	LastError     string `json:"lastError,omitempty"`
	FindingKey    string `json:"findingKey,omitempty"`
	AI            *AIDTO `json:"ai,omitempty"`
}

// BatchSummaryDTO 是批量验证结束事件载荷。
type BatchSummaryDTO struct {
	Done          int    `json:"done"`
	Total         int    `json:"total"`
	FalsePositive int    `json:"falsePositive"`
	TruePositive  int    `json:"truePositive"`
	Uncertain     int    `json:"uncertain"`
	Failed        int    `json:"failed"`
	Stopped       bool   `json:"stopped"`
	Message       string `json:"message"`
}

// LibraryEntryDTO 是翻译库中的一条记录。
type LibraryEntryDTO struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	KindZh    string `json:"kindZh"`
	SourceEN  string `json:"sourceEn"`
	SourceZH  string `json:"sourceZh"`
	Origin    string `json:"origin"`
	OriginZh  string `json:"originZh"`
	Hits      int    `json:"hits"`
	UpdatedAt string `json:"updatedAt"`
}

// LibraryDTO 是翻译库查询结果。
type LibraryDTO struct {
	Total   int               `json:"total"`
	Shown   int               `json:"shown"`
	Entries []LibraryEntryDTO `json:"entries"`
}

// MappingDTO 是路径映射工作台需要的全部数据。
type MappingDTO struct {
	Unresolved    []UnresolvedDTO   `json:"unresolved"`
	PrefixRules   []PrefixRuleDTO   `json:"prefixRules"`
	Overrides     map[string]string `json:"overrides"`
	TotalFiles    int               `json:"totalFiles"`
	TotalFindings int               `json:"totalFindings"`
	Resolved      int               `json:"resolved"`
	Stats         StatsDTO          `json:"stats"`
}

// ExportResultDTO 是导出操作的返回。
type ExportResultDTO struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
	Kind  string `json:"kind"`
}

// TestConnectionDTO 是大模型连通性测试的返回。
type TestConnectionDTO struct {
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
	Reply    string `json:"reply"`
}

// ------------------------- 转换函数 -------------------------

func locationDTO(loc model.Location) LocationDTO {
	return LocationDTO{
		File:         loc.File,
		ResolvedPath: loc.ResolvedPath,
		StartLine:    loc.StartLine,
		StartCol:     loc.StartCol,
		EndLine:      loc.EndLine,
		EndCol:       loc.EndCol,
		Label:        loc.Label,
		LabelZh:      loc.LabelZH,
		Resolved:     loc.Resolved(),
		LineRange:    loc.LineRange(),
		ColumnRange:  loc.ColumnRange(),
		LineCol:      loc.LineCol(),
		DisplayPath:  loc.DisplayPath(),
		BaseName:     loc.BaseName(),
	}
}

func linkDTO(l *model.LinkRef) *LinkDTO {
	if l == nil {
		return nil
	}
	return &LinkDTO{
		Label:     l.Label,
		LabelZh:   l.LabelZH,
		RawURI:    l.RawURI,
		File:      l.File,
		StartLine: l.StartLine,
		StartCol:  l.StartCol,
		EndLine:   l.EndLine,
		EndCol:    l.EndCol,
	}
}

func segmentDTOs(segs []model.Segment) []SegmentDTO {
	out := make([]SegmentDTO, 0, len(segs))
	for _, s := range segs {
		if s.Kind == model.SegLink && s.Link != nil {
			text := s.Text
			if strings.TrimSpace(text) == "" {
				text = s.Link.Label
				if s.Link.LabelZH != "" {
					text = s.Link.LabelZH
				}
			}
			out = append(out, SegmentDTO{Kind: "link", Text: text, Link: linkDTO(s.Link)})
			continue
		}
		if s.Text == "" {
			continue
		}
		out = append(out, SegmentDTO{Kind: "text", Text: s.Text})
	}
	return out
}

func aiDTO(f *model.Finding) *AIDTO {
	if f == nil || !f.HasAIVerdict() {
		return nil
	}
	return &AIDTO{
		Verdict:    f.AIVerdict,
		VerdictZh:  f.AIVerdictZH(),
		Reason:     f.AIReason,
		Confidence: f.AIConfidence,
		Model:      f.AIModel,
		At:         f.AIAt,
		Short:      model.AIVerdictShort(f.AIVerdict),
	}
}

func fingerprintDTOs(fps []model.Fingerprint) []FingerprintDTO {
	out := make([]FingerprintDTO, 0, len(fps))
	for _, fp := range fps {
		out = append(out, FingerprintDTO{Name: fp.Name, Value: fp.Value})
	}
	return out
}

func ruleInfoDTO(info *model.RuleInfo) *RuleInfoDTO {
	if info == nil {
		return nil
	}
	tags := info.Tags
	if tags == nil {
		tags = []string{}
	}
	cwes := info.CWEs
	if cwes == nil {
		cwes = []string{}
	}
	return &RuleInfoDTO{
		ID:               info.ID,
		Name:             info.Name,
		ShortDescription: info.ShortDescription,
		FullDescription:  info.FullDescription,
		HelpURI:          info.HelpURI,
		DefaultLevel:     info.DefaultLevel,
		SecuritySeverity: info.SecuritySeverity,
		Precision:        info.Precision,
		Kind:             info.Kind,
		ProblemSeverity:  info.ProblemSeverity,
		Tags:             tags,
		CWEs:             cwes,
	}
}

func codeFlowDTOs(flows []model.CodeFlow) []CodeFlowDTO {
	out := make([]CodeFlowDTO, 0, len(flows))
	for _, cf := range flows {
		steps := make([]FlowStepDTO, 0, len(cf.Steps))
		for _, step := range cf.Steps {
			kinds := step.Kinds
			if kinds == nil {
				kinds = []string{}
			}
			steps = append(steps, FlowStepDTO{
				Index:        step.Index,
				Location:     locationDTO(step.Location),
				Message:      step.Message,
				NestingLevel: step.NestingLevel,
				Kinds:        kinds,
			})
		}
		out = append(out, CodeFlowDTO{Index: cf.Index, Message: cf.Message, Steps: steps})
	}
	return out
}

func relatedLocationDTOs(items []model.RelatedLocation) []RelatedLocationDTO {
	out := make([]RelatedLocationDTO, 0, len(items))
	for _, item := range items {
		out = append(out, RelatedLocationDTO{
			ID:       item.ID,
			Location: locationDTO(item.Location),
			Message:  item.Message,
		})
	}
	return out
}

func findingDTO(f *model.Finding, reviewed bool) FindingDTO {
	sources := make([]LocationDTO, 0, len(f.Sources))
	for _, s := range f.Sources {
		sources = append(sources, locationDTO(s))
	}
	// CodeQL 的 rule name 是 py/sql-injection 这类规则 ID，直接当标题并不好读。
	// 优先用中文规则名；SARIF 还有可读的 shortDescription；最后才用 ID。
	ruleDisplay := strings.TrimSpace(f.RuleNameZH)
	if ruleDisplay == "" && f.RuleInfo != nil {
		ruleDisplay = strings.TrimSpace(f.RuleInfo.ShortDescription)
	}
	if ruleDisplay == "" {
		ruleDisplay = strings.TrimSpace(f.DescriptionZH)
	}
	if ruleDisplay == "" {
		ruleDisplay = strings.TrimSpace(f.Description)
	}
	if ruleDisplay == "" {
		ruleDisplay = f.RuleName
	}
	cwes := f.CWEs
	if cwes == nil {
		cwes = []string{}
	}
	tags := f.Tags
	if tags == nil {
		tags = []string{}
	}
	return FindingDTO{
		Key:                f.Key(),
		Index:              f.Index,
		RuleName:           f.RuleName,
		RuleNameZh:         f.RuleNameZH,
		RuleDisplay:        ruleDisplay,
		Description:        f.Description,
		DescriptionZh:      f.DescriptionZH,
		DescriptionDisplay: f.DescriptionDisplay(),
		Severity:           string(f.Severity),
		SeverityZh:         f.Severity.ZH(),
		Sink:               locationDTO(f.Sink),
		Sources:            sources,
		MessageEn:          segmentDTOs(f.MessageEN),
		MessageZh:          segmentDTOs(f.MessageZH),
		MessageRaw:         f.MessageRaw,
		MessageKey:         f.MessageKey,
		MessageZhTemplate:  f.MessageZHTemplate,
		TranslateSource:    f.TranslateSource,
		TranslateSourceZh:  translateSourceZh(f.TranslateSource),
		TranslatePending:   f.TranslatePending,
		Reviewed:           reviewed,
		AI:                 aiDTO(f),
		Format:             string(f.Format),
		FormatLabel:        f.Format.Label(),
		ToolName:           f.ToolName,
		ToolVersion:        f.ToolVersion,
		CWEs:               cwes,
		Tags:               tags,
		Fingerprints:       fingerprintDTOs(f.Fingerprints),
		PrimaryFingerprint: f.PrimaryFingerprint,
		RuleInfo:           ruleInfoDTO(f.RuleInfo),
		CodeFlows:          codeFlowDTOs(f.CodeFlows),
		RelatedLocations:   relatedLocationDTOs(f.RelatedLocations),
	}
}

func translateSourceZh(s string) string {
	switch strings.TrimSpace(s) {
	case translate.SourceBuiltin:
		return "内置词典"
	case translate.SourceStore:
		return "翻译库"
	case translate.SourceLLM:
		return "大模型"
	case translate.SourceNone:
		return "未翻译（保持英文）"
	case "":
		return ""
	}
	return s
}

func (a *App) statsDTO() StatsDTO {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.statsLocked()
}

func (a *App) statsLocked() StatsDTO {
	s := StatsDTO{BySeverity: map[string]int{}}
	if a.proj == nil {
		return s
	}
	ps := a.proj.Stats
	s.Total = ps.Total
	s.RuleKinds = ps.RuleKinds
	s.FileKinds = ps.FileKinds
	s.Resolved = ps.Resolved
	s.Missing = ps.Missing
	s.Pending = ps.Pending
	s.WithCodeFlow = ps.WithCodeFlow
	s.CWEKinds = ps.CWEKinds
	s.TotalSourceLocation = 0
	for _, f := range a.proj.Findings {
		s.BySeverity[string(f.Severity)]++
		s.TotalSourceLocation += len(f.Sources)
		if !a.reviewed[f.Key()] {
			s.Unread++
		}
		if f.HasAIVerdict() {
			s.Verified++
			switch f.AIVerdict {
			case string(verify.FalsePositive):
				s.FalsePositive++
			case string(verify.TruePositive):
				s.TruePositive++
			default:
				s.Uncertain++
			}
		}
	}
	if a.engine != nil {
		tr := a.engine.Stats()
		s.TranslationBuiltin = tr.Builtin
		s.TranslationStore = tr.Store
		s.TranslationLLM = tr.LLM
	}
	return s
}

func (a *App) projectDTO() (*ProjectDTO, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proj == nil {
		return nil, nil
	}
	p := a.proj
	findings := make([]FindingDTO, 0, len(p.Findings))
	for _, f := range p.Findings {
		findings = append(findings, findingDTO(f, a.reviewed[f.Key()]))
	}
	rules := make([]OptionDTO, 0, len(p.Stats.ByRule))
	for _, r := range p.Rules() {
		label := r
		if zh := p.Stats.RuleNameZH[r]; zh != "" {
			label = zh
		}
		rules = append(rules, OptionDTO{Value: r, Label: label, Count: p.Stats.ByRule[r]})
	}
	files := make([]OptionDTO, 0, len(p.Stats.ByFile))
	for _, f := range p.Files() {
		files = append(files, OptionDTO{Value: f, Label: f, Count: p.Stats.ByFile[f]})
	}
	sevOrder := []string{"error", "warning", "note", "recommendation"}
	severities := make([]OptionDTO, 0, 4)
	for _, s := range sevOrder {
		if n := p.Stats.BySeverity[s]; n > 0 {
			severities = append(severities, OptionDTO{Value: s, Label: model.Severity(s).ZH(), Count: n})
		}
	}
	unresolved := make([]UnresolvedDTO, 0)
	{
		counts := map[string]int{}
		sevSet := map[string]map[string]bool{}
		for _, f := range p.Findings {
			if f.Sink.Resolved() {
				continue
			}
			counts[f.Sink.File]++
			if sevSet[f.Sink.File] == nil {
				sevSet[f.Sink.File] = map[string]bool{}
			}
			sevSet[f.Sink.File][string(f.Severity)] = true
		}
		for file, n := range counts {
			sevs := make([]string, 0, len(sevSet[file]))
			for _, s := range sevOrder {
				if sevSet[file][s] {
					sevs = append(sevs, s)
				}
			}
			unresolved = append(unresolved, UnresolvedDTO{File: file, Count: n, Severities: sevs})
		}
		sort.Slice(unresolved, func(i, j int) bool {
			if unresolved[i].Count != unresolved[j].Count {
				return unresolved[i].Count > unresolved[j].Count
			}
			return unresolved[i].File < unresolved[j].File
		})
	}
	prefixes := make([]PrefixRuleDTO, 0, len(p.Resolver.PrefixRules()))
	for _, r := range p.Resolver.PrefixRules() {
		prefixes = append(prefixes, PrefixRuleDTO{From: r.From, To: r.To})
	}
	// 没有解析提示时 p.Warnings 是 nil，Go 会把它序列化成 JSON null。
	// 前端「项目信息」条会读取 warnings.length，null 会抛异常导致整页白屏，
	// 所以这里始终返回空数组。
	warnings := p.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return &ProjectDTO{
		CSVPath:       p.CSVPath,
		CSVName:       filepath.Base(p.CSVPath),
		Format:        string(p.Format),
		FormatLabel:   p.Format.Label(),
		ToolName:      p.ToolName,
		ToolVersion:   p.ToolVersion,
		SrcRoot:       p.SrcRoot,
		Findings:      findings,
		Stats:         a.statsLocked(),
		Rules:         rules,
		Files:         files,
		Severities:    severities,
		Warnings:      warnings,
		Unresolved:    unresolved,
		PrefixRules:   prefixes,
		Overrides:     p.Resolver.Overrides(),
		LoadedAt:      time.Now().Format(time.RFC3339),
		TranslationDB: a.cfg.TranslateDB,
	}, nil
}

func (a *App) settingsDTO() SettingsDTO {
	c := a.cfg
	return SettingsDTO{
		LLM: LLMSettingsDTO{
			Enabled:        c.LLM.Enabled,
			BaseURL:        c.LLM.BaseURL,
			APIKey:         c.LLM.APIKey,
			Model:          c.LLM.Model,
			TimeoutSeconds: c.LLM.TimeoutSeconds,
			TemplateOnly:   c.LLM.TemplateOnly,
		},
		AIVerifyEnabled:    c.AIVerify(),
		AISendCode:         c.AISendCode(),
		Editor:             EditorSettingsDTO{Cmd: c.Editor.Cmd, Args: c.Editor.Args},
		DisplayLang:        c.DisplayLang,
		ShowColumnRuler:    c.ColumnRuler(),
		CodeSearchShortcut: c.SearchShortcut(),
		TranslateDB:        c.TranslateDB,
		DataDir:            c.DataDir(),
		ConfigPath:         c.Path(),
		WindowWidth:        int(c.WindowWidth),
		WindowHeight:       int(c.WindowHeight),
		SplitLeft:          c.SplitLeft,
		SplitDetail:        c.SplitDetail,
		TopInfoCollapsed:   c.TopInfoCollapsed,
	}
}

func (a *App) libraryKindZh(kind string) string {
	switch kind {
	case translate.KindRule:
		return "规则名称"
	case translate.KindDescription:
		return "规则描述"
	case translate.KindMessage:
		return "漏洞详情"
	case translate.KindLabel:
		return "证据链标签"
	case translate.KindSeverity:
		return "严重级别"
	}
	return "全部"
}

func (a *App) findingByKey(key string) *model.Finding {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proj == nil {
		return nil
	}
	for _, f := range a.proj.Findings {
		if f.Key() == key {
			return f
		}
	}
	return nil
}

func (a *App) filterFindings(keys []string) []*model.Finding {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proj == nil {
		return nil
	}
	if len(keys) == 0 {
		return append([]*model.Finding(nil), a.proj.Findings...)
	}
	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[k] = true
	}
	out := make([]*model.Finding, 0, len(keys))
	for _, f := range a.proj.Findings {
		if want[f.Key()] {
			out = append(out, f)
		}
	}
	return out
}

func (a *App) reviewedCountLocked() int {
	n := 0
	if a.proj == nil {
		return 0
	}
	for _, f := range a.proj.Findings {
		if a.reviewed[f.Key()] {
			n++
		}
	}
	return n
}
