// Package config 负责应用配置与项目状态的持久化。
//
// 所有操作系统的默认数据目录统一为 ~/.CodeQLAuditPro/：
//   - config.json      审计设置与界面偏好
//   - history.db       历史审计项目（已读标记、AI 判定、路径映射）
//   - translations.db  离线翻译库
//
// 旧版本把历史项目写在 config.json 的 projects 数组里，首次启动会自动
// 迁移到 history.db，并从 config.json 中移除，避免配置文件越用越大。
//
// 设置环境变量 CODEQLAUDITPRO_HOME 可以改为绿色/便携模式。
// 旧版目录（macOS 的 Application Support/codeql-audit、CODEQL_AUDIT_HOME 指向的
// 目录，以及 ~/.codeql-audit）会在首次启动时自动迁移到新目录。
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codeql-audit-pro/internal/pathmap"
)

const (
	// EnvHome 是覆盖数据目录的环境变量名。
	EnvHome = "CODEQLAUDITPRO_HOME"
	// LegacyEnvHome 是旧版审计工具使用的便携目录变量，仅用于迁移旧数据。
	LegacyEnvHome = "CODEQL_AUDIT_HOME"
	// AppDirName 是统一的数据目录名，所有平台都放在用户主目录下。
	AppDirName = ".CodeQLAuditPro"
)

// LLMSettings 是大模型翻译配置。
type LLMSettings struct {
	Enabled        bool   `json:"enabled"`
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	Model          string `json:"model"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	TemplateOnly   bool   `json:"template_only"`
}

// DefaultLLM 返回默认的大模型配置（OpenAI 兼容协议）。
func DefaultLLM() LLMSettings {
	return LLMSettings{
		Enabled:        false,
		BaseURL:        "https://api.deepseek.com/v1",
		Model:          "deepseek-chat",
		TimeoutSeconds: 60,
		TemplateOnly:   true,
	}
}

// EditorSettings 决定「用编辑器打开」时调用哪个程序、传什么参数。
type EditorSettings struct {
	// Cmd 是编辑器的可执行文件（macOS 也可以直接填 .app 包路径）。
	// 留空表示自动探测（VS Code / Cursor / Sublime 等常见安装位置）。
	Cmd string `json:"cmd"`
	// Args 是参数模板，支持 {file} {line} {col}；留空使用 VS Code 的 -g 模板。
	Args string `json:"args"`
}

// VerdictRecord 是一条漏洞的 AI 误报判定结果（持久化）。
type VerdictRecord struct {
	Status     string  `json:"status"` // true_positive / false_positive / uncertain
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
	Model      string  `json:"model"`
	At         string  `json:"at"`
}

// ProjectState 是单个审计项目的持久化状态。
type ProjectState struct {
	CSVPath     string               `json:"csv_path"`
	SrcRoot     string               `json:"src_root"`
	PrefixRules []pathmap.PrefixRule `json:"prefix_rules,omitempty"`
	Overrides   map[string]string    `json:"overrides,omitempty"`
	LastOpened  string               `json:"last_opened"`
	// Reviewed 记录已查看过的漏洞（键为 Finding.Key()），用于「已读/未读」标记。
	Reviewed map[string]bool `json:"reviewed,omitempty"`
	// Verdicts 记录 AI 误报判定结果（键为 Finding.Key()）。
	Verdicts map[string]VerdictRecord `json:"verdicts,omitempty"`
	// 下面几项只用于「历史项目」列表展示，不参与业务判断。
	TotalFindings    int    `json:"total_findings,omitempty"`
	ResolvedFindings int    `json:"resolved_findings,omitempty"`
	Format           string `json:"format,omitempty"`
	ToolName         string `json:"tool_name,omitempty"`
}

// IsReviewed 判断一条漏洞是否已被查看过。
func (p *ProjectState) IsReviewed(key string) bool {
	if p == nil || key == "" {
		return false
	}
	return p.Reviewed[key]
}

// MarkReviewed 设置一条漏洞的已读状态。
func (p *ProjectState) MarkReviewed(key string, read bool) {
	if p == nil || key == "" {
		return
	}
	if p.Reviewed == nil {
		p.Reviewed = map[string]bool{}
	}
	if read {
		p.Reviewed[key] = true
		return
	}
	delete(p.Reviewed, key)
}

// ReviewedCount 返回已读条目数。
func (p *ProjectState) ReviewedCount() int {
	if p == nil {
		return 0
	}
	n := 0
	for _, read := range p.Reviewed {
		if read {
			n++
		}
	}
	return n
}

// ClearReviewed 清空全部已读标记。
func (p *ProjectState) ClearReviewed() {
	if p != nil {
		p.Reviewed = nil
	}
}

// Verdict 读取一条 AI 判定。
func (p *ProjectState) Verdict(key string) (VerdictRecord, bool) {
	if p == nil || key == "" || p.Verdicts == nil {
		return VerdictRecord{}, false
	}
	v, ok := p.Verdicts[key]
	return v, ok
}

// SetVerdict 写入一条 AI 判定。
func (p *ProjectState) SetVerdict(key string, rec VerdictRecord) {
	if p == nil || key == "" {
		return
	}
	if p.Verdicts == nil {
		p.Verdicts = map[string]VerdictRecord{}
	}
	p.Verdicts[key] = rec
}

// ClearVerdict 删除一条 AI 判定。
func (p *ProjectState) ClearVerdict(key string) {
	if p == nil || p.Verdicts == nil {
		return
	}
	delete(p.Verdicts, key)
}

// ClearVerdicts 清空全部 AI 判定。
func (p *ProjectState) ClearVerdicts() {
	if p != nil {
		p.Verdicts = nil
	}
}

// Config 是应用总配置。
type Config struct {
	LLM         LLMSettings `json:"llm"`
	TranslateDB string      `json:"translate_db"`
	// Projects 在内存里保存全部历史项目，落盘到 history.db，不再写进 config.json。
	Projects    []ProjectState `json:"-"`
	LastProject string         `json:"last_project"`
	// 窗口尺寸
	WindowWidth  float32 `json:"window_width"`
	WindowHeight float32 `json:"window_height"`
	// 三栏分隔条位置（左栏占比 / 中栏占比，0 表示用默认值）。
	// 用户把某一栏拖到自己顺手的宽度后，下次启动保持原样。
	SplitLeft   float64 `json:"split_left,omitempty"`
	SplitDetail float64 `json:"split_detail,omitempty"`
	// 默认显示语言：zh / en / both
	DisplayLang string `json:"display_lang"`
	// Warning 记录降级信息（不持久化），例如配置目录不可写
	Warning string `json:"-"`
	// Editor 是「用编辑器打开」的配置
	Editor EditorSettings `json:"editor"`
	// AIVerifySendCode 决定 AI 误报验证时是否把源码片段发给大模型。
	// 用指针是为了区分「老配置里没有这个键（默认 true）」与「用户显式关闭」。
	AIVerifySendCode *bool `json:"ai_verify_send_code,omitempty"`
	// AIVerifyEnabled 是「AI 误报验证」自己的总开关，与翻译用的 LLM.Enabled 完全独立。
	//
	// 以前 AI 验证复用的是翻译开关，用户填对了接口地址/模型/Key，只要没勾那个
	// 标着「未命中翻译库时调用大模型」的翻译开关，AI 验证就永远报「未配置大模型」。
	// 用指针是为了区分「老配置里没有这个键」与「用户显式关闭」，缺省值见 AIVerify。
	AIVerifyEnabled *bool `json:"ai_verify_enabled,omitempty"`

	// TopInfoCollapsed 表示顶栏「项目信息」（结果文件/源码目录/定位状态）是否被用户折叠。
	// 折叠后顶栏只剩工具栏一行，中栏能多出两行代码的高度。
	TopInfoCollapsed bool `json:"top_info_collapsed,omitempty"`
	// ShowColumnRuler 决定代码视图顶部是否显示列标尺（定位列标识）。
	// 用指针是为了区分「老配置里没有这个键（默认显示）」与「用户显式关闭」。
	ShowColumnRuler *bool `json:"show_column_ruler,omitempty"`
	// CodeSearchShortcut 是「在当前代码文件中查找」的快捷键，可在设置里自定义。
	// 存储的是规范写法（Cmd+F / Ctrl+Shift+G / Alt+F3），空串表示按平台取默认值，
	// 解析与校验见 shortcut.go 的 ParseShortcut / SearchShortcut。
	CodeSearchShortcut string `json:"code_search_shortcut,omitempty"`

	path string
	// history 是 history.db；为 nil 时（例如命令行迁移工具）回退到把
	// 项目状态写进 config.json，保证极端情况下数据也不会丢。
	history *historyStore
	// legacyProjects 是旧版 config.json 里的 projects 数组，仅在迁移时读取。
	legacyProjects []ProjectState
}

// UnmarshalJSON 兼容旧版 config.json：旧文件里的 projects 数组只作为
// 迁移来源读入内存，后续统一落到 history.db。
func (c *Config) UnmarshalJSON(data []byte) error {
	type plain Config
	aux := struct {
		*plain
		Projects []ProjectState `json:"projects"`
	}{plain: (*plain)(c)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	c.legacyProjects = aux.Projects
	c.Projects = aux.Projects
	return nil
}

// MarshalJSON 在 history.db 可用时不再输出 projects 数组；只有历史库
// 打不开（例如目录只读）时才退回旧格式，避免历史项目直接丢失。
func (c Config) MarshalJSON() ([]byte, error) {
	type plain Config
	aux := struct {
		*plain
		Projects []ProjectState `json:"projects,omitempty"`
	}{plain: (*plain)(&c)}
	if c.history == nil && len(c.Projects) > 0 {
		aux.Projects = c.Projects
	}
	return json.Marshal(aux)
}

// ColumnRuler 返回是否在代码视图顶部显示列标尺（默认显示）。
func (c *Config) ColumnRuler() bool {
	if c == nil || c.ShowColumnRuler == nil {
		return true
	}
	return *c.ShowColumnRuler
}

// SetColumnRuler 设置是否显示列标尺。
func (c *Config) SetColumnRuler(v bool) {
	c.ShowColumnRuler = &v
}

// AIVerify 返回是否启用 AI 误报验证。
//
// 老配置里没有 ai_verify_enabled 键时按「用户其实已经配好大模型了」推断：
//   - 翻译开关已打开，或
//   - 接口地址/模型被改过、或填过 API Key（即用户确实在设置里配了模型）
//
// 这样升级后已有的正确配置可以立即使用，而未配置过的用户仍然会看到
// 「未配置大模型」的引导，而不是拿着默认地址去请求然后收到 401。
func (c *Config) AIVerify() bool {
	if c == nil {
		return false
	}
	if c.AIVerifyEnabled != nil {
		return *c.AIVerifyEnabled
	}
	return c.LLM.Enabled || c.llmLooksConfigured()
}

// SetAIVerify 显式设置 AI 误报验证的总开关。
func (c *Config) SetAIVerify(v bool) {
	c.AIVerifyEnabled = &v
}

// llmLooksConfigured 判断用户是否真的在设置里填过大模型信息。
//
// 只看「接口地址与模型都非空」，并且与默认值做过对比：全新安装的配置里
// 预填了 DeepSeek 的默认地址和模型，不能据此认为用户已经配置好了。
func (c *Config) llmLooksConfigured() bool {
	if c == nil {
		return false
	}
	base := strings.TrimSpace(c.LLM.BaseURL)
	model := strings.TrimSpace(c.LLM.Model)
	if base == "" || model == "" {
		return false
	}
	if strings.TrimSpace(c.LLM.APIKey) != "" {
		return true
	}
	def := DefaultLLM()
	return base != def.BaseURL || model != def.Model
}

// AISendCode 返回 AI 验证时是否允许发送源码片段（默认允许）。
func (c *Config) AISendCode() bool {
	if c == nil || c.AIVerifySendCode == nil {
		return true
	}
	return *c.AIVerifySendCode
}

// SetAISendCode 设置 AI 验证是否发送源码片段。
func (c *Config) SetAISendCode(v bool) {
	c.AIVerifySendCode = &v
}

// SetPath 覆盖配置文件路径（测试与便携场景使用）。
func (c *Config) SetPath(p string) { c.path = p }

// Path 返回配置文件路径（可能为空，表示尚未落盘）。
func (c *Config) Path() string { return c.path }

// Home 返回统一数据目录。默认是所有平台一致的 ~/.CodeQLAuditPro/；
// 设置 CODEQLAUDITPRO_HOME 可切换到便携目录。用户主目录不可写时才退回临时目录。
func Home() (string, error) {
	if h := strings.TrimSpace(os.Getenv(EnvHome)); h != "" {
		if err := os.MkdirAll(h, 0o755); err != nil {
			return "", err
		}
		if !writable(h) {
			return "", fmt.Errorf("目录不可写: %s", h)
		}
		migrateLegacyConfig(h)
		return h, nil
	}

	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		dir := filepath.Join(home, AppDirName)
		if err := os.MkdirAll(dir, 0o755); err == nil && writable(dir) {
			migrateLegacyConfig(dir)
			return dir, nil
		}
	}

	fallback := fallbackHome()
	if err := os.MkdirAll(fallback, 0o755); err != nil {
		return "", fmt.Errorf("无法创建数据目录: %w", err)
	}
	if !writable(fallback) {
		return "", fmt.Errorf("目录不可写: %s", fallback)
	}
	return fallback, nil
}

func fallbackHome() string {
	return filepath.Join(os.TempDir(), AppDirName)
}

func isFallbackDir(dir string) bool {
	a, errA := filepath.Abs(dir)
	b, errB := filepath.Abs(fallbackHome())
	if errA != nil || errB != nil {
		return filepath.Clean(dir) == filepath.Clean(fallbackHome())
	}
	return a == b
}

// legacyConfigDirs 返回可能存放旧版审计配置的目录，按优先级排序。
//
// CODEQL_AUDIT_HOME 只作为迁移来源读取；新版本默认始终使用 ~/.CodeQLAuditPro，
// 除非显式设置新的 CODEQLAUDITPRO_HOME。
func legacyConfigDirs() []string {
	var dirs []string
	add := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		for _, existing := range dirs {
			if filepath.Clean(existing) == filepath.Clean(dir) {
				return
			}
		}
		dirs = append(dirs, dir)
	}

	add(os.Getenv(LegacyEnvHome))
	if base, err := os.UserConfigDir(); err == nil {
		add(filepath.Join(base, "codeql-audit"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".codeql-audit"))
	}
	add(filepath.Join(os.TempDir(), "codeql-audit"))
	return dirs
}

// migrateLegacyConfig 在首次使用新目录时把旧配置和翻译库复制过来。
// 只做一次，且不会覆盖新目录里已经存在的数据。
func migrateLegacyConfig(newDir string) {
	if fileExists(filepath.Join(newDir, "config.json")) ||
		fileExists(filepath.Join(newDir, "translations.db")) ||
		fileExists(filepath.Join(newDir, "history.db")) {
		return
	}
	oldDir := ""
	for _, dir := range legacyConfigDirs() {
		if filepath.Clean(dir) == filepath.Clean(newDir) {
			continue
		}
		if fileExists(filepath.Join(dir, "config.json")) || fileExists(filepath.Join(dir, "translations.db")) {
			oldDir = dir
			break
		}
	}
	if oldDir == "" {
		return
	}

	oldConfig := filepath.Join(oldDir, "config.json")
	newConfig := filepath.Join(newDir, "config.json")
	newDB := filepath.Join(newDir, "translations.db")

	if fileExists(oldConfig) {
		// 优先按结构解析再落盘，顺便把 translate_db 指到新目录。
		// 解析失败的旧配置不复制，让新版本生成默认配置，避免启动直接报错。
		if cfg, err := LoadFrom(oldConfig); err == nil {
			cfg.TranslateDB = newDB
			cfg.SetPath(newConfig)
			_ = cfg.Save()
		}
	}
	// SQLite 可能启用了 WAL：只复制主库会丢掉尚未 checkpoint 的翻译。
	for _, suffix := range []string{"", "-wal", "-shm"} {
		oldDB := filepath.Join(oldDir, "translations.db"+suffix)
		if fileExists(oldDB) {
			_ = copyFileIfMissing(oldDB, newDB+suffix)
		}
	}
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func copyFileIfMissing(src, dst string) error {
	if fileExists(dst) {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}

// writable 判断目录是否可写。
func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// Load 读取配置，不存在时返回默认配置。
// 只有在完全无法确定数据目录时才会返回错误。
func Load() (*Config, error) {
	dir, err := Home()
	if err != nil {
		return nil, err
	}
	cfg, err := loadFrom(filepath.Join(dir, "config.json"), dir, isFallbackDir(dir))
	if err != nil {
		return nil, err
	}
	if err := cfg.openHistory(filepath.Join(dir, "history.db")); err != nil {
		cfg.Warning = appendWarning(cfg.Warning,
			"历史项目库不可用，本次只读取旧配置："+err.Error())
		return cfg, nil
	}
	// last_project 指向已经被删除的历史项目时顺手清理掉。
	if cfg.LastProject != "" && cfg.FindProject(cfg.LastProject) == nil {
		cfg.LastProject = ""
	}
	// 发生过迁移：先备份旧 config.json，再重写并移除 projects 数组。
	if len(cfg.legacyProjects) > 0 {
		if err := copyFileIfMissing(cfg.Path(), cfg.Path()+".bak"); err != nil {
			cfg.Warning = appendWarning(cfg.Warning, "备份旧配置失败："+err.Error())
		}
		if err := cfg.Save(); err != nil {
			cfg.Warning = appendWarning(cfg.Warning, "迁移历史项目失败："+err.Error())
		}
	}
	return cfg, nil
}

func appendWarning(old, extra string) string {
	old = strings.TrimSpace(old)
	extra = strings.TrimSpace(extra)
	switch {
	case old == "":
		return extra
	case extra == "":
		return old
	default:
		return old + "；" + extra
	}
}

// openHistory 打开历史项目库，并在首次升级时把旧 config.json 的 projects 导进去。
func (c *Config) openHistory(path string) error {
	store, err := openHistoryStore(path)
	if err != nil {
		return err
	}
	loaded, err := store.loadAll()
	if err != nil {
		_ = store.Close()
		return err
	}
	if len(loaded) == 0 && len(c.legacyProjects) > 0 {
		// 首次迁移：旧数据是唯一来源，写入后即可从 JSON 里移除。
		loaded = append([]ProjectState(nil), c.legacyProjects...)
		if err := store.saveAll(loaded); err != nil {
			_ = store.Close()
			return err
		}
	}
	c.history = store
	c.Projects = loaded
	return nil
}

// HistoryPath 返回历史项目库路径（始终在数据目录下）。
func (c *Config) HistoryPath() string {
	if c == nil {
		return ""
	}
	if c.history != nil && strings.TrimSpace(c.history.path) != "" {
		return c.history.path
	}
	if dir := c.DataDir(); dir != "" {
		return filepath.Join(dir, "history.db")
	}
	return "history.db"
}

// Close 关闭历史项目库。进程退出前调用。
func (c *Config) Close() error {
	if c == nil || c.history == nil {
		return nil
	}
	err := c.history.Close()
	c.history = nil
	return err
}

// LoadFrom 从指定路径读取配置（测试、便携模式与多套配置场景使用）。
func LoadFrom(path string) (*Config, error) {
	dir := filepath.Dir(path)
	return loadFrom(path, dir, isFallbackDir(dir))
}

func loadFrom(path, dir string, usingFallback bool) (*Config, error) {
	cfg := &Config{
		LLM:          DefaultLLM(),
		DisplayLang:  "zh",
		WindowWidth:  1480,
		WindowHeight: 900,
		path:         path,
	}
	raw, err := os.ReadFile(cfg.path)
	if err != nil {
		if os.IsNotExist(err) {
			if cfg.TranslateDB == "" {
				cfg.TranslateDB = filepath.Join(dir, "translations.db")
			}
			if usingFallback {
				cfg.Warning = "用户主目录不可写，已改用临时目录存放配置与翻译库：" + dir
			}
			return cfg, nil
		}
		return nil, err
	}
	if usingFallback {
		cfg.Warning = "用户主目录不可写，已改用临时目录存放配置与翻译库：" + dir
	}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("配置文件损坏: %w", err)
	}
	cfg.Warning = ""
	cfg.path = path
	if cfg.TranslateDB == "" {
		cfg.TranslateDB = filepath.Join(dir, "translations.db")
	}
	if cfg.DisplayLang == "" {
		cfg.DisplayLang = "zh"
	}
	if cfg.LLM.TimeoutSeconds <= 0 {
		cfg.LLM.TimeoutSeconds = 60
	}
	if cfg.WindowWidth <= 0 {
		cfg.WindowWidth = 1480
	}
	if cfg.WindowHeight <= 0 {
		cfg.WindowHeight = 900
	}
	return cfg, nil
}

// Save 写回配置：设置写 config.json，历史项目写 history.db。
func (c *Config) Save() error {
	if c.path == "" {
		dir, err := Home()
		if err != nil {
			return err
		}
		c.path = filepath.Join(dir, "config.json")
	}
	var historyErr error
	if c.history != nil {
		if err := c.history.saveAll(c.Projects); err != nil {
			historyErr = err
		} else {
			// 已经成功落进 SQLite，不需要再在 JSON 里兜底。
			c.legacyProjects = nil
		}
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, c.path); err != nil {
		return err
	}
	return historyErr
}

// FindProject 查找某个审计项目的状态（不创建）。
func (c *Config) FindProject(csvPath string) *ProjectState {
	key := strings.TrimSpace(csvPath)
	if key == "" {
		return nil
	}
	for i := range c.Projects {
		if strings.EqualFold(strings.TrimSpace(c.Projects[i].CSVPath), key) {
			return &c.Projects[i]
		}
	}
	return nil
}

// ProjectFor 查找或创建某个审计项目的状态。
func (c *Config) ProjectFor(csvPath, srcRoot string) *ProjectState {
	key := strings.TrimSpace(csvPath)
	if st := c.FindProject(key); st != nil {
		if srcRoot != "" {
			st.SrcRoot = srcRoot
		}
		return st
	}
	c.Projects = append(c.Projects, ProjectState{
		CSVPath:     key,
		SrcRoot:     srcRoot,
		LastOpened:  time.Now().Format(time.RFC3339),
		PrefixRules: nil,
		Overrides:   map[string]string{},
	})
	return &c.Projects[len(c.Projects)-1]
}

// DeleteProject 从历史项目库中删除一个项目，并同步清理 last_project。
// 不会删除结果文件、源码目录或翻译库。
func (c *Config) DeleteProject(csvPath string) error {
	key := strings.TrimSpace(csvPath)
	if key == "" {
		return nil
	}
	next := make([]ProjectState, 0, len(c.Projects))
	for _, st := range c.Projects {
		if strings.EqualFold(strings.TrimSpace(st.CSVPath), key) {
			continue
		}
		next = append(next, st)
	}
	c.Projects = next
	if strings.EqualFold(strings.TrimSpace(c.LastProject), key) {
		c.LastProject = ""
	}
	if c.history != nil {
		if err := c.history.delete(key); err != nil {
			return err
		}
	}
	return c.Save()
}

// ClearProjects 清空全部历史项目。
func (c *Config) ClearProjects() error {
	c.Projects = nil
	c.LastProject = ""
	if c.history != nil {
		if err := c.history.saveAll(nil); err != nil {
			return err
		}
	}
	return c.Save()
}

// Last 返回最近一次打开的项目状态。
func (c *Config) Last() *ProjectState {
	if c.LastProject == "" {
		return nil
	}
	return c.FindProject(c.LastProject)
}

// DataDir 返回配置文件所在目录。
func (c *Config) DataDir() string { return filepath.Dir(c.path) }
