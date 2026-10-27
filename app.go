package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"codeql-audit-pro/internal/assistant"
	"codeql-audit-pro/internal/config"
	"codeql-audit-pro/internal/editor"
	"codeql-audit-pro/internal/model"
	"codeql-audit-pro/internal/pathmap"
	"codeql-audit-pro/internal/project"
	"codeql-audit-pro/internal/sourcefile"
	"codeql-audit-pro/internal/translate"
)

// appVersion 是界面与关于页展示的版本号。
const appVersion = "1.2.0"

// App 是 Wails 后端服务。所有暴露给前端的公开方法都必须有明确的 JSON 友好
// 返回值；耗时操作（项目载入、AI 验证）通过事件向前端汇报进度。
type App struct {
	ctx    context.Context
	cfg    *config.Config
	store  *translate.Store
	engine *translate.Engine
	proj   *project.Project

	// mu 保护 cfg / proj / reviewed 与界面状态。
	mu       sync.Mutex
	reviewed map[string]bool

	// opMu 串行化会修改项目模型的操作（换源码目录、路径映射、重译）。
	opMu sync.Mutex
	// loadMu 避免并发载入同一份项目。
	loadMu sync.Mutex

	saveTimer *time.Timer
	closing   bool
	startErr  error

	// 后台 AI 验证用的源码片段缓存（与前端文件缓存分开）。
	snippetMu    sync.Mutex
	snippetCache map[string][]string

	// 源码文件缓存。
	fileMu    sync.Mutex
	fileCache map[string]*FileDTO

	// 批量验证状态。
	batchMu      sync.Mutex
	batchCancel  context.CancelFunc
	batchRunning bool

	initialCSV string
	initialSrc string

	// CodeQL 助手内核（与原 codeql-assistant 功能一一对应）。
	// assistantMu 只保护助手状态，避免与审计项目的长操作互相阻塞。
	assistantMu          sync.Mutex
	assistantRunner      *assistant.Runner
	assistantHistory     *assistant.History
	assistantPump        *assistantOutputPump
	assistantConfig      assistant.Config
	assistantStatus      assistant.Status
	assistantAction      string
	assistantBusy        bool
	assistantStartedAt   time.Time
	assistantLastElapsed time.Duration
	assistantLogPath     string
	assistantQuote       string
}

// NewApp 创建后端服务。配置在进入 Wails 事件循环前读取，窗口尺寸因此可以直接用用户上次的大小。
func NewApp(cfg *config.Config, initialCSV, initialSrc string) *App {
	a := &App{
		cfg:          cfg,
		reviewed:     map[string]bool{},
		snippetCache: map[string][]string{},
		fileCache:    map[string]*FileDTO{},
		initialCSV:   initialCSV,
		initialSrc:   initialSrc,
	}
	a.initAssistant()
	return a
}

// startup 在窗口创建后执行：打开翻译库、建立翻译引擎、注册文件拖放。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.assistantRunner != nil {
		a.assistantRunner.SetStatusCallback(a.assistantStatusHook)
	}
	store, err := translate.OpenStore(a.cfg.TranslateDB)
	if err != nil {
		a.startErr = fmt.Errorf("打开翻译库失败：%w", err)
		return
	}
	engine, err := translate.NewEngine(store, translate.NewLLMClient(toLLMConfig(a.cfg.LLM)))
	if err != nil {
		_ = store.Close()
		a.startErr = fmt.Errorf("初始化翻译引擎失败：%w", err)
		return
	}
	a.store = store
	a.engine = engine

	if strings.TrimSpace(a.initialCSV) != "" {
		a.cfg.LastProject = a.initialCSV
		st := a.cfg.ProjectFor(a.initialCSV, a.initialSrc)
		if strings.TrimSpace(a.initialSrc) != "" {
			st.SrcRoot = a.initialSrc
		}
		_ = a.cfg.Save()
	}

	// Wails 的文件拖放：前端监听 file:dropped 后决定是 CSV 还是源码目录。
	wruntime.OnFileDrop(ctx, func(x, y int, paths []string) {
		if len(paths) == 0 {
			return
		}
		wruntime.EventsEmit(ctx, "file:dropped", map[string]any{
			"paths": paths,
			"x":     x,
			"y":     y,
		})
	})
}

// shutdown 在窗口关闭时保存状态并释放资源。
func (a *App) shutdown(ctx context.Context) {
	a.closeAssistant()

	a.batchMu.Lock()
	if a.batchCancel != nil {
		a.batchCancel()
		a.batchCancel = nil
	}
	a.batchMu.Unlock()

	a.mu.Lock()
	a.closing = true
	if a.saveTimer != nil {
		a.saveTimer.Stop()
		a.saveTimer = nil
	}
	a.flushProjectLocked()
	if w, h := wruntime.WindowGetSize(a.ctx); w > 0 && h > 0 {
		a.cfg.WindowWidth = float32(w)
		a.cfg.WindowHeight = float32(h)
	}
	_ = a.cfg.Save()
	a.mu.Unlock()
	if a.store != nil {
		_ = a.store.Close()
	}
	_ = a.cfg.Close()
}

// ------------------------- 生命周期与设置 -------------------------

// Bootstrap 返回启动数据（设置、最近项目、路径、版本）。
func (a *App) Bootstrap() (*BootstrapDTO, error) {
	if a.startErr != nil {
		return nil, a.startErr
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	settings := a.settingsDTO()
	last := strings.TrimSpace(a.cfg.LastProject)
	if last == "" {
		last = strings.TrimSpace(a.initialCSV)
	}
	lastSrc := ""
	if ps := a.cfg.Last(); ps != nil {
		lastSrc = ps.SrcRoot
	} else {
		lastSrc = strings.TrimSpace(a.initialSrc)
	}
	// 只有「命令行显式指定了 -csv」才在启动时自动载入。
	// 普通双击启动始终保持审计工作台的选择界面，历史项目由用户自己点开；
	// 否则一个 15MB 的 SARIF + 上千条翻译会把窗口卡在启动画面。
	autoLoad := strings.TrimSpace(a.initialCSV) != ""
	version := appVersion
	return &BootstrapDTO{
		Settings:              settings,
		LastProject:           last,
		LastSrcRoot:           lastSrc,
		AutoLoad:              autoLoad,
		Platform:              runtime.GOOS,
		Version:               version,
		DefaultSearchShortcut: config.DefaultSearchShortcut(),
		DefaultEditorArgs:     editor.DefaultArgs,
		DataDir:               a.cfg.DataDir(),
		ConfigPath:            a.cfg.Path(),
		Warning:               a.cfg.Warning,
	}, nil
}

// GetSettings 返回当前设置。
func (a *App) GetSettings() (*SettingsDTO, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.settingsDTO()
	return &s, nil
}

// SaveSettings 保存设置并立即重建大模型客户端。
func (a *App) SaveSettings(in SettingsDTO) (*SettingsDTO, error) {
	shortcut := strings.TrimSpace(in.CodeSearchShortcut)
	if shortcut != "" {
		if _, err := config.ParseShortcut(shortcut); err != nil {
			return nil, fmt.Errorf("查找快捷键格式有误：%w", err)
		}
	}
	lang := strings.TrimSpace(in.DisplayLang)
	if lang != "zh" && lang != "en" && lang != "both" {
		lang = "zh"
	}
	to := in.LLM.TimeoutSeconds
	if to <= 0 {
		to = 60
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.LLM = config.LLMSettings{
		Enabled:        in.LLM.Enabled,
		BaseURL:        strings.TrimSpace(in.LLM.BaseURL),
		APIKey:         strings.TrimSpace(in.LLM.APIKey),
		Model:          strings.TrimSpace(in.LLM.Model),
		TimeoutSeconds: to,
		TemplateOnly:   in.LLM.TemplateOnly,
	}
	a.cfg.Editor = config.EditorSettings{
		Cmd:  strings.TrimSpace(in.Editor.Cmd),
		Args: strings.TrimSpace(in.Editor.Args),
	}
	a.cfg.DisplayLang = lang
	a.cfg.SetAIVerify(in.AIVerifyEnabled)
	a.cfg.SetAISendCode(in.AISendCode)
	a.cfg.SetColumnRuler(in.ShowColumnRuler)
	if shortcut == "" {
		a.cfg.SetSearchShortcut("")
	} else {
		a.cfg.SetSearchShortcut(shortcut)
	}
	if a.engine != nil {
		a.engine.SetLLM(translate.NewLLMClient(toLLMConfig(a.cfg.LLM)))
	}
	if err := a.cfg.Save(); err != nil {
		return nil, err
	}
	s := a.settingsDTO()
	return &s, nil
}

// UIPrefsDTO 是频繁变化、单独保存的界面偏好。
type UIPrefsDTO struct {
	DisplayLang      string  `json:"displayLang"`
	ShowColumnRuler  bool    `json:"showColumnRuler"`
	SplitLeft        float64 `json:"splitLeft"`
	SplitDetail      float64 `json:"splitDetail"`
	WindowWidth      int     `json:"windowWidth"`
	WindowHeight     int     `json:"windowHeight"`
	TopInfoCollapsed bool    `json:"topInfoCollapsed"`
}

// SaveUIPreferences 保存语言、列标尺、三栏间距与窗口尺寸（防抖落盘）。
func (a *App) SaveUIPreferences(in UIPrefsDTO) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if in.DisplayLang == "zh" || in.DisplayLang == "en" || in.DisplayLang == "both" {
		a.cfg.DisplayLang = in.DisplayLang
	}
	a.cfg.SetColumnRuler(in.ShowColumnRuler)
	if in.SplitLeft > 0.02 && in.SplitLeft < 0.98 {
		a.cfg.SplitLeft = in.SplitLeft
	}
	if in.SplitDetail > 0.02 && in.SplitDetail < 0.98 {
		a.cfg.SplitDetail = in.SplitDetail
	}
	if in.WindowWidth > 320 {
		a.cfg.WindowWidth = float32(in.WindowWidth)
	}
	if in.WindowHeight > 240 {
		a.cfg.WindowHeight = float32(in.WindowHeight)
	}
	a.cfg.TopInfoCollapsed = in.TopInfoCollapsed
	a.scheduleSaveLocked()
	return nil
}

// SetNativeTheme 让 macOS 标题栏跟随前端主题。
func (a *App) SetNativeTheme(theme string) {
	if a.ctx == nil {
		return
	}
	if theme == "light" {
		wruntime.WindowSetLightTheme(a.ctx)
		return
	}
	wruntime.WindowSetDarkTheme(a.ctx)
}

// ------------------------- 文件与项目 -------------------------

// ChooseResultsFile 打开系统文件选择框，返回 CodeQL 结果文件路径（取消时为空串）。
func (a *App) ChooseResultsFile() (string, error) {
	if a.startErr != nil {
		return "", a.startErr
	}
	a.mu.Lock()
	defaultDir := ""
	if a.cfg.LastProject != "" {
		defaultDir = filepath.Dir(a.cfg.LastProject)
	}
	if !pathExists(defaultDir) {
		defaultDir = ""
	}
	a.mu.Unlock()
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:            "选择 CodeQL 结果文件",
		DefaultDirectory: defaultDir,
		Filters: []wruntime.FileFilter{
			{DisplayName: "CodeQL 结果文件 (*.csv;*.sarif;*.json)", Pattern: "*.csv;*.sarif;*.json"},
			{DisplayName: "SARIF v2.1.0 (*.sarif;*.json)", Pattern: "*.sarif;*.json"},
			{DisplayName: "CSV (*.csv)", Pattern: "*.csv"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*"},
		},
	})
}

// ChooseCSVFile 是 ChooseResultsFile 的兼容别名（旧前端 / 脚本仍可能调用）。
func (a *App) ChooseCSVFile() (string, error) { return a.ChooseResultsFile() }

// ChooseDirectory 打开系统目录选择框。
func (a *App) ChooseDirectory(title string) (string, error) {
	if a.startErr != nil {
		return "", a.startErr
	}
	a.mu.Lock()
	defaultDir := ""
	if a.proj != nil {
		defaultDir = a.proj.SrcRoot
	}
	if !pathExists(defaultDir) {
		defaultDir = ""
	}
	a.mu.Unlock()
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: defaultDir,
	})
}

// LoadProject 解析 CSV / SARIF、翻译、定位源码并返回完整项目载荷。
func (a *App) LoadProject(csvPath, srcRoot string) (*ProjectDTO, error) {
	if a.startErr != nil {
		return nil, a.startErr
	}
	csvPath = strings.TrimSpace(csvPath)
	if csvPath == "" {
		return nil, fmt.Errorf("请先选择 CodeQL 结果文件（CSV 或 SARIF v2.1.0）")
	}
	a.loadMu.Lock()
	defer a.loadMu.Unlock()

	a.mu.Lock()
	st := a.cfg.ProjectFor(csvPath, srcRoot)
	root := st.SrcRoot
	if strings.TrimSpace(srcRoot) != "" {
		root = strings.TrimSpace(srcRoot)
		st.SrcRoot = root
	}
	prefixRules := append([]pathmap.PrefixRule(nil), st.PrefixRules...)
	overrides := make(map[string]string, len(st.Overrides))
	for k, v := range st.Overrides {
		overrides[k] = v
	}
	a.mu.Unlock()

	a.emitProjectProgress("parse", 0, 1, "正在解析结果文件 …")
	p, err := project.Load(project.Options{
		CSVPath:     csvPath,
		SrcRoot:     root,
		PrefixRules: prefixRules,
		Overrides:   overrides,
		Engine:      a.engine,
		Progress: func(done, total int, msg string) {
			a.emitProjectProgress("load", done, total, msg)
		},
	})
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.proj = p
	a.cfg.LastProject = csvPath
	a.reviewed = map[string]bool{}
	st = a.cfg.ProjectFor(csvPath, root)
	st.LastOpened = time.Now().Format(time.RFC3339)
	st.SrcRoot = root
	st.TotalFindings = p.Stats.Total
	st.ResolvedFindings = p.Stats.Resolved
	st.Format = string(p.Format)
	st.ToolName = p.ToolName
	for k, v := range st.Reviewed {
		if v {
			a.reviewed[k] = true
		}
	}
	for _, f := range p.Findings {
		if rec, ok := st.Verdicts[f.Key()]; ok {
			f.AIVerdict = rec.Status
			f.AIReason = rec.Reason
			f.AIConfidence = rec.Confidence
			f.AIModel = rec.Model
			f.AIAt = rec.At
		}
	}
	a.fileMu.Lock()
	a.fileCache = map[string]*FileDTO{}
	a.fileMu.Unlock()
	a.snippetMu.Lock()
	a.snippetCache = map[string][]string{}
	a.snippetMu.Unlock()
	_ = a.cfg.Save()
	a.mu.Unlock()

	a.emitProjectProgress("done", 1, 1, "载入完成")
	dto, err := a.projectDTO()
	if err != nil {
		return nil, err
	}
	return dto, nil
}

// GetProject 返回当前项目（未载入时返回 null）。
func (a *App) GetProject() (*ProjectDTO, error) {
	return a.projectDTO()
}

// SetSrcRoot 更换源码目录并重新定位所有漏洞。
func (a *App) SetSrcRoot(root string) (*ProjectDTO, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("源码目录不能为空")
	}
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.mu.Lock()
	if a.proj == nil {
		a.mu.Unlock()
		return nil, fmt.Errorf("请先打开 CodeQL 结果文件")
	}
	st := a.cfg.ProjectFor(a.proj.CSVPath, root)
	st.SrcRoot = root
	a.mu.Unlock()

	a.proj.SetSrcRoot(root)

	a.mu.Lock()
	st = a.cfg.ProjectFor(a.proj.CSVPath, root)
	st.SrcRoot = root
	st.ResolvedFindings = a.proj.Stats.Resolved
	st.TotalFindings = a.proj.Stats.Total
	st.LastOpened = time.Now().Format(time.RFC3339)
	_ = a.cfg.Save()
	a.fileMu.Lock()
	a.fileCache = map[string]*FileDTO{}
	a.fileMu.Unlock()
	a.mu.Unlock()
	return a.projectDTO()
}

// GetSourceFile 读取源码文件。resolvedPath 为空时按结果文件中的逻辑路径解析。
func (a *App) GetSourceFile(csvPath, resolvedPath string) (*FileDTO, error) {
	path := strings.TrimSpace(resolvedPath)
	if path == "" {
		a.mu.Lock()
		p := a.proj
		a.mu.Unlock()
		if p == nil {
			return nil, fmt.Errorf("项目尚未载入")
		}
		r := p.Resolver.Resolve(csvPath)
		if !r.Resolved() {
			return nil, fmt.Errorf("未定位：%s\n可在「路径映射」中手工指定本地文件", csvPath)
		}
		path = r.Path
	}
	a.fileMu.Lock()
	if f, ok := a.fileCache[path]; ok {
		a.fileMu.Unlock()
		return f, nil
	}
	a.fileMu.Unlock()

	lines, enc, err := sourcefile.ReadLines(path)
	if err != nil {
		return nil, fmt.Errorf("读取源码失败：%w", err)
	}
	dto := &FileDTO{
		Path:        path,
		DisplayPath: csvPath,
		Encoding:    enc,
		Lines:       lines,
		TotalLines:  len(lines),
	}
	a.fileMu.Lock()
	a.fileCache[path] = dto
	a.fileMu.Unlock()
	return dto, nil
}

// ResolvePath 把结果文件中的逻辑路径解析成本地绝对路径。
func (a *App) ResolvePath(csvPath string) (string, error) {
	a.mu.Lock()
	p := a.proj
	a.mu.Unlock()
	if p == nil {
		return "", fmt.Errorf("项目尚未载入")
	}
	r := p.Resolver.Resolve(csvPath)
	if !r.Resolved() {
		return "", fmt.Errorf("未定位：%s\n可在「路径映射」中手工指定本地文件", csvPath)
	}
	return r.Path, nil
}

// ------------------------- 已读状态 -------------------------

// SetReviewed 批量设置已读状态。keys 为空时作用于全部记录。
func (a *App) SetReviewed(keys []string, reviewed bool) (*StatsDTO, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proj == nil {
		return nil, fmt.Errorf("项目尚未载入")
	}
	targets := keys
	if len(targets) == 0 {
		targets = make([]string, 0, len(a.proj.Findings))
		for _, f := range a.proj.Findings {
			targets = append(targets, f.Key())
		}
	}
	for _, k := range targets {
		if reviewed {
			a.reviewed[k] = true
		} else {
			delete(a.reviewed, k)
		}
	}
	// 清理已不存在的指纹，避免配置无限膨胀。
	if reviewed {
		alive := map[string]bool{}
		for _, f := range a.proj.Findings {
			alive[f.Key()] = true
		}
		for k := range a.reviewed {
			if !alive[k] {
				delete(a.reviewed, k)
			}
		}
	}
	a.flushProjectLocked()
	a.scheduleSaveLocked()
	s := a.statsLocked()
	return &s, nil
}

// ResetReviewed 清空当前项目的全部已读标记。
func (a *App) ResetReviewed() (*StatsDTO, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proj == nil {
		return nil, fmt.Errorf("项目尚未载入")
	}
	a.reviewed = map[string]bool{}
	st := a.cfg.ProjectFor(a.proj.CSVPath, a.proj.SrcRoot)
	st.Reviewed = nil
	st.LastOpened = time.Now().Format(time.RFC3339)
	_ = a.cfg.Save()
	s := a.statsLocked()
	return &s, nil
}

// ------------------------- 路径映射 -------------------------

// GetMapping 返回路径映射工作台数据。
func (a *App) GetMapping() (*MappingDTO, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proj == nil {
		return nil, fmt.Errorf("项目尚未载入")
	}
	unresolved := make([]UnresolvedDTO, 0)
	{
		counts := map[string]int{}
		for _, f := range a.proj.Findings {
			if !f.Sink.Resolved() {
				counts[f.Sink.File]++
			}
		}
		for file, n := range counts {
			unresolved = append(unresolved, UnresolvedDTO{File: file, Count: n})
		}
		sortUnresolved(unresolved)
	}
	prefixes := make([]PrefixRuleDTO, 0)
	for _, r := range a.proj.Resolver.PrefixRules() {
		prefixes = append(prefixes, PrefixRuleDTO{From: r.From, To: r.To})
	}
	return &MappingDTO{
		Unresolved:    unresolved,
		PrefixRules:   prefixes,
		Overrides:     a.proj.Resolver.Overrides(),
		TotalFiles:    a.proj.Stats.FileKinds,
		TotalFindings: a.proj.Stats.Total,
		Resolved:      a.proj.Stats.Resolved,
		Stats:         a.statsLocked(),
	}, nil
}

// AddPrefixRule 添加一条「结果文件路径前缀 → 本地目录」规则。
func (a *App) AddPrefixRule(from, to string) (*MappingDTO, error) {
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if from == "" || to == "" {
		return nil, fmt.Errorf("请同时填写结果文件路径前缀与本地目录")
	}
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.mu.Lock()
	p := a.proj
	a.mu.Unlock()
	if p == nil {
		return nil, fmt.Errorf("项目尚未载入")
	}
	p.AddPrefixRule(from, to)
	a.mu.Lock()
	st := a.cfg.ProjectFor(p.CSVPath, p.SrcRoot)
	st.PrefixRules = p.Resolver.PrefixRules()
	_ = a.cfg.Save()
	a.mu.Unlock()
	a.fileMu.Lock()
	a.fileCache = map[string]*FileDTO{}
	a.fileMu.Unlock()
	return a.GetMapping()
}

// RemovePrefixRule 删除指定下标的路径前缀规则。
func (a *App) RemovePrefixRule(index int) (*MappingDTO, error) {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.mu.Lock()
	p := a.proj
	a.mu.Unlock()
	if p == nil {
		return nil, fmt.Errorf("项目尚未载入")
	}
	rules := p.Resolver.PrefixRules()
	if index < 0 || index >= len(rules) {
		return nil, fmt.Errorf("规则不存在")
	}
	rules = append(rules[:index], rules[index+1:]...)
	ov := p.Resolver.Overrides()
	p.Resolver = pathmap.NewResolver(p.SrcRoot, rules, ov)
	p.SetSrcRoot(p.SrcRoot)
	a.mu.Lock()
	st := a.cfg.ProjectFor(p.CSVPath, p.SrcRoot)
	st.PrefixRules = rules
	_ = a.cfg.Save()
	a.mu.Unlock()
	return a.GetMapping()
}

// ChooseOverrideFile 为某个未定位的逻辑路径手工选择本地文件。
func (a *App) ChooseOverrideFile(csvPath string) (*MappingDTO, error) {
	if strings.TrimSpace(csvPath) == "" {
		return nil, fmt.Errorf("结果文件路径为空")
	}
	a.mu.Lock()
	defaultDir := ""
	if a.proj != nil {
		defaultDir = a.proj.SrcRoot
	}
	if !pathExists(defaultDir) {
		defaultDir = ""
	}
	a.mu.Unlock()
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:            "为 " + csvPath + " 选择本地文件",
		DefaultDirectory: defaultDir,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return a.GetMapping()
	}
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.mu.Lock()
	p := a.proj
	a.mu.Unlock()
	if p == nil {
		return nil, fmt.Errorf("项目尚未载入")
	}
	p.SetOverride(csvPath, path)
	a.mu.Lock()
	st := a.cfg.ProjectFor(p.CSVPath, p.SrcRoot)
	st.Overrides = p.Resolver.Overrides()
	_ = a.cfg.Save()
	a.mu.Unlock()
	a.fileMu.Lock()
	a.fileCache = map[string]*FileDTO{}
	a.fileMu.Unlock()
	return a.GetMapping()
}

// RemoveOverride 删除手工映射。
func (a *App) RemoveOverride(csvPath string) (*MappingDTO, error) {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.mu.Lock()
	p := a.proj
	a.mu.Unlock()
	if p == nil {
		return nil, fmt.Errorf("项目尚未载入")
	}
	ov := p.Resolver.Overrides()
	delete(ov, csvPath)
	p.Resolver = pathmap.NewResolver(p.SrcRoot, p.Resolver.PrefixRules(), ov)
	p.SetSrcRoot(p.SrcRoot)
	a.mu.Lock()
	st := a.cfg.ProjectFor(p.CSVPath, p.SrcRoot)
	st.Overrides = ov
	_ = a.cfg.Save()
	a.mu.Unlock()
	return a.GetMapping()
}

func sortUnresolved(in []UnresolvedDTO) {
	for i := 0; i < len(in); i++ {
		for j := i + 1; j < len(in); j++ {
			if in[j].Count > in[i].Count || (in[j].Count == in[i].Count && in[j].File < in[i].File) {
				in[i], in[j] = in[j], in[i]
			}
		}
	}
}

// ------------------------- 编辑器与剪贴板 -------------------------

// OpenInEditor 用外部编辑器打开源码并定位到行列。
func (a *App) OpenInEditor(path string, line, col int) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("当前没有打开任何文件")
	}
	a.mu.Lock()
	cfg := a.cfg.Editor
	a.mu.Unlock()
	return editor.Open(cfg, path, line, col)
}

// RevealInFinder 在系统文件管理器中显示文件。
func (a *App) RevealInFinder(path string) error {
	return editor.Reveal(path)
}

// CopyText 把文本写入系统剪贴板。
func (a *App) CopyText(text string) error {
	if a.ctx == nil {
		return fmt.Errorf("剪贴板不可用")
	}
	return wruntime.ClipboardSetText(a.ctx, text)
}

// OpenExternalURL 用系统浏览器打开链接。
func (a *App) OpenExternalURL(url string) {
	if a.ctx != nil && strings.HasPrefix(url, "http") {
		wruntime.BrowserOpenURL(a.ctx, url)
	}
}

// ------------------------- 内部工具 -------------------------

func toLLMConfig(s config.LLMSettings) translate.LLMConfig {
	return translate.LLMConfig{
		Enabled:        s.Enabled,
		BaseURL:        s.BaseURL,
		APIKey:         s.APIKey,
		Model:          s.Model,
		TimeoutSeconds: s.TimeoutSeconds,
		TemplateOnly:   s.TemplateOnly,
	}
}

// emitProjectProgress 向前端发送项目载入进度。
func (a *App) emitProjectProgress(phase string, done, total int, message string) {
	if a.ctx == nil {
		return
	}
	wruntime.EventsEmit(a.ctx, "project:progress", map[string]any{
		"phase":   phase,
		"done":    done,
		"total":   total,
		"message": message,
	})
}

// flushProjectLocked 把内存中的已读状态写回项目状态（调用方需持有 a.mu）。
func (a *App) flushProjectLocked() {
	if a.proj == nil {
		return
	}
	st := a.cfg.ProjectFor(a.proj.CSVPath, a.proj.SrcRoot)
	if len(a.reviewed) == 0 {
		st.Reviewed = nil
	} else {
		st.Reviewed = make(map[string]bool, len(a.reviewed))
		for k, v := range a.reviewed {
			if v {
				st.Reviewed[k] = true
			}
		}
	}
	st.SrcRoot = a.proj.SrcRoot
}

// scheduleSaveLocked 安排一次防抖落盘（调用方需持有 a.mu）。
func (a *App) scheduleSaveLocked() {
	if a.closing {
		return
	}
	if a.saveTimer != nil {
		a.saveTimer.Stop()
	}
	a.saveTimer = time.AfterFunc(1200*time.Millisecond, func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.closing {
			return
		}
		a.saveTimer = nil
		a.flushProjectLocked()
		_ = a.cfg.Save()
	})
}

// findFinding 返回指定 key 的漏洞（调用方持有 a.mu 时使用 findFindingLocked）。
func (a *App) findFindingLocked(key string) *model.Finding {
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

// pathExists 判断文件是否存在。
func pathExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// baseName 返回路径的文件名。
func baseName(p string) string {
	if strings.TrimSpace(p) == "" {
		return ""
	}
	return filepath.Base(p)
}
