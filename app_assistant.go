package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"codeql-audit-pro/internal/assistant"
)

// 本文件把 CodeQL Assistant 的后端能力接入 Wails。
//
// 设计要点：
//  1. 运行器、配置、历史全部放在 Go 侧，前端切换视图不会影响进程；
//  2. 输出通过批量事件推送，避免海量日志逐行跨 WebView 边界；
//  3. 配置与历史统一放在 ~/.CodeQLAuditPro/assistant，旧数据首次启动时自动迁移。

// AssistantConfigDTO 是助手配置的前端表示。
type AssistantConfigDTO struct {
	Name         string `json:"name"`
	BinaryPath   string `json:"binaryPath"`
	QueryPath    string `json:"queryPath"`
	DatabasePath string `json:"databasePath"`
	SourceRoot   string `json:"sourceRoot"`
	Language     string `json:"language"`
	OutputFormat string `json:"outputFormat"`
	OutputFile   string `json:"outputFile"`
	RamLimit     int    `json:"ramLimit"`
	Threads      int    `json:"threads"`
	ExtraArgs    string `json:"extraArgs"`
}

func (d AssistantConfigDTO) toConfig() assistant.Config {
	return assistant.Config{
		Name:         d.Name,
		BinaryPath:   strings.TrimSpace(d.BinaryPath),
		QueryPath:    strings.TrimSpace(d.QueryPath),
		DatabasePath: strings.TrimSpace(d.DatabasePath),
		SourceRoot:   strings.TrimSpace(d.SourceRoot),
		Language:     strings.TrimSpace(d.Language),
		OutputFormat: strings.TrimSpace(d.OutputFormat),
		OutputFile:   strings.TrimSpace(d.OutputFile),
		RamLimit:     d.RamLimit,
		Threads:      d.Threads,
		ExtraArgs:    strings.TrimSpace(d.ExtraArgs),
	}
}

func assistantConfigDTO(c assistant.Config) AssistantConfigDTO {
	return AssistantConfigDTO{
		Name:         c.Name,
		BinaryPath:   c.BinaryPath,
		QueryPath:    c.QueryPath,
		DatabasePath: c.DatabasePath,
		SourceRoot:   c.SourceRoot,
		Language:     c.Language,
		OutputFormat: c.OutputFormat,
		OutputFile:   c.OutputFile,
		RamLimit:     c.RamLimit,
		Threads:      c.Threads,
		ExtraArgs:    c.ExtraArgs,
	}
}

// AssistantHistoryDTO 是单条运行历史。
type AssistantHistoryDTO struct {
	Timestamp    string `json:"timestamp"`
	Action       string `json:"action"`
	Status       string `json:"status"`
	OutputFile   string `json:"outputFile"`
	DurationMs   int64  `json:"durationMs"`
	Language     string `json:"language"`
	DatabasePath string `json:"databasePath"`
	QueryPath    string `json:"queryPath"`
}

// AssistantStateDTO 是助手前端的启动/刷新数据。
type AssistantStateDTO struct {
	Config        AssistantConfigDTO    `json:"config"`
	DefaultConfig AssistantConfigDTO    `json:"defaultConfig"`
	Profiles      []string              `json:"profiles"`
	History       []AssistantHistoryDTO `json:"history"`
	Running       bool                  `json:"running"`
	Status        string                `json:"status"`
	StatusLabel   string                `json:"statusLabel"`
	Action        string                `json:"action"`
	StartedAt     string                `json:"startedAt"`
	ElapsedMs     int64                 `json:"elapsedMs"`
	LogPath       string                `json:"logPath"`
	Quote         string                `json:"quote"`
	Languages     []string              `json:"languages"`
	Formats       []string              `json:"formats"`
	DataDir       string                `json:"dataDir"`
	Platform      string                `json:"platform"`
}

// AssistantStatusDTO 是运行状态事件的载荷。
type AssistantStatusDTO struct {
	Running     bool   `json:"running"`
	Status      string `json:"status"`
	StatusLabel string `json:"statusLabel"`
	Action      string `json:"action"`
	StartedAt   string `json:"startedAt"`
	ElapsedMs   int64  `json:"elapsedMs"`
	LogPath     string `json:"logPath"`
}

// AssistantOutputLineDTO 是一行控制台输出。
type AssistantOutputLineDTO struct {
	Text  string `json:"text"`
	Error bool   `json:"error"`
}

// AssistantOutputDTO 是批量输出事件。
type AssistantOutputDTO struct {
	Lines []AssistantOutputLineDTO `json:"lines"`
	Reset bool                     `json:"reset"`
}

// AssistantDoneDTO 是一次任务结束事件。
type AssistantDoneDTO struct {
	Action      string `json:"action"`
	Status      string `json:"status"`
	StatusLabel string `json:"statusLabel"`
	OutputFile  string `json:"outputFile"`
	DurationMs  int64  `json:"durationMs"`
	Error       string `json:"error"`
}

// AssistantCommandsDTO 是「命令示例」中可复制的两条命令。
type AssistantCommandsDTO struct {
	Create  string `json:"create"`
	Analyze string `json:"analyze"`
}

// assistantOutputPump 把高频输出合并成低频事件，同时负责完整日志落盘。
type assistantOutputPump struct {
	mu      sync.Mutex
	pending []AssistantOutputLineDTO
	timer   *time.Timer
	logger  *assistant.RunLogger
	closed  bool
	emit    func(AssistantOutputDTO)
}

func newAssistantOutputPump(emit func(AssistantOutputDTO)) *assistantOutputPump {
	return &assistantOutputPump{emit: emit}
}

func (p *assistantOutputPump) start() string {
	p.mu.Lock()
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
	if p.logger != nil {
		p.logger.Close()
	}
	p.pending = nil
	p.closed = false
	p.logger = assistant.StartRunLog()
	path := p.logger.Path()
	p.mu.Unlock()

	p.emit(AssistantOutputDTO{Reset: true})
	return path
}

func (p *assistantOutputPump) add(line string, isError bool) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	if p.logger != nil {
		p.logger.WriteLine(line)
	}
	p.pending = append(p.pending, AssistantOutputLineDTO{Text: line, Error: isError})
	needFlush := len(p.pending) >= 400
	if !needFlush && p.timer == nil {
		p.timer = time.AfterFunc(90*time.Millisecond, p.flush)
	}
	p.mu.Unlock()

	if needFlush {
		p.flush()
	}
}

func (p *assistantOutputPump) flush() {
	p.mu.Lock()
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
	if len(p.pending) == 0 {
		p.mu.Unlock()
		return
	}
	lines := p.pending
	p.pending = nil
	p.mu.Unlock()
	p.emit(AssistantOutputDTO{Lines: lines})
}

func (p *assistantOutputPump) finish() {
	p.mu.Lock()
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
	lines := p.pending
	p.pending = nil
	logger := p.logger
	p.logger = nil
	p.closed = true
	p.mu.Unlock()

	if len(lines) > 0 {
		p.emit(AssistantOutputDTO{Lines: lines})
	}
	if logger != nil {
		logger.Close()
	}
}

func (p *assistantOutputPump) logPath() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.logger == nil {
		return ""
	}
	return p.logger.Path()
}

// ------------------------- 启动与状态 -------------------------

// initAssistant 在 App 构造时准备助手内核。它在 Wails 事件循环之前执行。
func (a *App) initAssistant() {
	assistant.EnsureDataDir()
	cfg, err := assistant.LoadConfigFromFile(assistant.LastConfigPath())
	if err != nil {
		cfg = assistant.DefaultConfig()
	}
	a.assistantConfig = normalizeAssistantConfig(cfg)
	a.assistantRunner = assistant.NewRunner()
	a.assistantHistory = assistant.NewHistory()
	a.assistantStatus = assistant.StatusIdle
	a.assistantQuote = assistant.GetRandomQuote()
	a.assistantPump = newAssistantOutputPump(func(payload AssistantOutputDTO) {
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "assistant:output", payload)
		}
	})
}

func normalizeAssistantConfig(c assistant.Config) assistant.Config {
	def := assistant.DefaultConfig()
	if c.Language == "" {
		c.Language = def.Language
	}
	// 助手只支持 sarifv2.1.0 与 csv；历史配置里的其它格式统一收敛。
	c.OutputFormat = assistant.NormalizeOutputFormat(c.OutputFormat)
	// 0 表示不向 CodeQL 传 --ram / --threads，由 CodeQL 自己决定。
	// 这里绝不能把 0 回填成默认值，否则用户设置的 0 会被悄悄改成 4096 / 4。
	if c.RamLimit < 0 {
		c.RamLimit = 0
	}
	if c.Threads < 0 {
		c.Threads = 0
	}
	return c
}

func (a *App) assistantStatusHook(status assistant.Status) {
	a.assistantMu.Lock()
	a.assistantStatus = status
	if status == assistant.StatusRunning && a.assistantStartedAt.IsZero() {
		a.assistantStartedAt = time.Now()
	}
	a.assistantMu.Unlock()
	a.emitAssistantStatus()
}

func (a *App) assistantStatusLocked() AssistantStatusDTO {
	elapsed := a.assistantLastElapsed
	if a.assistantStatus == assistant.StatusRunning && !a.assistantStartedAt.IsZero() {
		elapsed = time.Since(a.assistantStartedAt)
	}
	started := ""
	if !a.assistantStartedAt.IsZero() {
		started = a.assistantStartedAt.Format(time.RFC3339)
	}
	return AssistantStatusDTO{
		Running:     a.assistantStatus == assistant.StatusRunning,
		Status:      string(a.assistantStatus),
		StatusLabel: a.assistantStatus.Label(),
		Action:      a.assistantAction,
		StartedAt:   started,
		ElapsedMs:   elapsed.Milliseconds(),
		LogPath:     a.assistantLogPath,
	}
}

func (a *App) emitAssistantStatus() {
	if a.ctx == nil {
		return
	}
	a.assistantMu.Lock()
	state := a.assistantStatusLocked()
	a.assistantMu.Unlock()
	wruntime.EventsEmit(a.ctx, "assistant:status", state)
}

func (a *App) assistantHistoryDTOs() []AssistantHistoryDTO {
	entries := a.assistantHistory.Snapshot()
	out := make([]AssistantHistoryDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, AssistantHistoryDTO{
			Timestamp:    e.Timestamp.Format(time.RFC3339),
			Action:       e.Action,
			Status:       e.Status,
			OutputFile:   e.OutputFile,
			DurationMs:   e.DurationMs,
			Language:     e.Config.Language,
			DatabasePath: e.Config.DatabasePath,
			QueryPath:    e.Config.QueryPath,
		})
	}
	return out
}

func (a *App) assistantStateLocked() AssistantStateDTO {
	status := a.assistantStatusLocked()
	return AssistantStateDTO{
		Config:        assistantConfigDTO(a.assistantConfig),
		DefaultConfig: assistantConfigDTO(assistant.DefaultConfig()),
		Profiles:      assistant.ListProfiles(),
		History:       a.assistantHistoryDTOs(),
		Running:       status.Running,
		Status:        status.Status,
		StatusLabel:   status.StatusLabel,
		Action:        status.Action,
		StartedAt:     status.StartedAt,
		ElapsedMs:     status.ElapsedMs,
		LogPath:       status.LogPath,
		Quote:         a.assistantQuote,
		Languages:     assistant.SupportedLanguages(),
		Formats:       assistant.SupportedFormats(),
		DataDir:       assistant.DataDir(),
		Platform:      runtime.GOOS,
	}
}

// AssistantBootstrap 返回助手的启动数据。
func (a *App) AssistantBootstrap() (*AssistantStateDTO, error) {
	a.assistantMu.Lock()
	defer a.assistantMu.Unlock()
	state := a.assistantStateLocked()
	return &state, nil
}

// AssistantGetState 返回当前状态、配置、历史与配置快照列表。
func (a *App) AssistantGetState() (*AssistantStateDTO, error) {
	return a.AssistantBootstrap()
}

// AssistantSaveConfig 保存当前配置（用于自动保存上次参数）。
func (a *App) AssistantSaveConfig(in AssistantConfigDTO) error {
	cfg := normalizeAssistantConfig(in.toConfig())
	a.assistantMu.Lock()
	a.assistantConfig = cfg
	a.assistantMu.Unlock()
	return cfg.SaveToFile(assistant.LastConfigPath())
}

// AssistantSaveProfile 保存命名配置快照。
func (a *App) AssistantSaveProfile(name string, in AssistantConfigDTO) error {
	name, err := assistantProfileName(name)
	if err != nil {
		return err
	}
	cfg := normalizeAssistantConfig(in.toConfig())
	cfg.Name = name
	if err := cfg.SaveToFile(assistant.ProfilePath(name)); err != nil {
		return err
	}
	a.assistantMu.Lock()
	a.assistantConfig = cfg
	a.assistantMu.Unlock()
	_ = cfg.SaveToFile(assistant.LastConfigPath())
	return nil
}

// AssistantLoadProfile 载入命名配置快照。
func (a *App) AssistantLoadProfile(name string) (*AssistantConfigDTO, error) {
	name, err := assistantProfileName(name)
	if err != nil {
		return nil, err
	}
	cfg, err := assistant.LoadConfigFromFile(assistant.ProfilePath(name))
	if err != nil {
		return nil, fmt.Errorf("载入配置失败：%w", err)
	}
	cfg = normalizeAssistantConfig(cfg)
	a.assistantMu.Lock()
	a.assistantConfig = cfg
	a.assistantMu.Unlock()
	_ = cfg.SaveToFile(assistant.LastConfigPath())
	dto := assistantConfigDTO(cfg)
	return &dto, nil
}

// AssistantDeleteProfile 删除命名配置快照。
func (a *App) AssistantDeleteProfile(name string) error {
	name, err := assistantProfileName(name)
	if err != nil {
		return err
	}
	if err := assistant.DeleteProfile(name); err != nil {
		return fmt.Errorf("删除配置失败：%w", err)
	}
	return nil
}

// AssistantListProfiles 返回全部配置快照名称。
func (a *App) AssistantListProfiles() []string {
	return assistant.ListProfiles()
}

func assistantProfileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".json")
	if name == "" {
		return "", fmt.Errorf("请输入配置名称")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("配置名称不能包含路径分隔符")
	}
	return name, nil
}

// AssistantGetHistory 返回运行历史。
func (a *App) AssistantGetHistory() []AssistantHistoryDTO {
	return a.assistantHistoryDTOs()
}

// AssistantClearHistory 清空运行历史。
func (a *App) AssistantClearHistory() error {
	a.assistantHistory.Clear()
	return nil
}

// AssistantRandomQuote 换一条语录，并在本次会话内保持稳定。
func (a *App) AssistantRandomQuote() string {
	quote := assistant.GetRandomQuote()
	a.assistantMu.Lock()
	a.assistantQuote = quote
	a.assistantMu.Unlock()
	return quote
}

// ------------------------- 运行控制 -------------------------

func (a *App) validateAssistantRun(cfg assistant.Config, action string) error {
	if cfg.BinaryPath == "" {
		return fmt.Errorf("请先选择 CodeQL 二进制文件路径")
	}
	if strings.ContainsAny(cfg.BinaryPath, `/\`) {
		if st, err := os.Stat(cfg.BinaryPath); err != nil || st.IsDir() {
			return fmt.Errorf("指定的路径处未找到 CodeQL 二进制文件：%s", cfg.BinaryPath)
		}
	} else if _, err := exec.LookPath(cfg.BinaryPath); err != nil {
		return fmt.Errorf("在 PATH 中找不到 CodeQL 命令：%s", cfg.BinaryPath)
	}
	if cfg.DatabasePath == "" {
		return fmt.Errorf("请指定 CodeQL 数据库目录")
	}
	if action == "create" && cfg.SourceRoot == "" {
		return fmt.Errorf("请指定源码根目录")
	}
	return nil
}

// AssistantRunCreate 在后台创建 CodeQL 数据库，立即返回。
func (a *App) AssistantRunCreate(in AssistantConfigDTO) error {
	cfg := normalizeAssistantConfig(in.toConfig())
	if err := a.validateAssistantRun(cfg, "create"); err != nil {
		return err
	}
	a.persistAssistantConfig(cfg)
	return a.startAssistantRun("create", cfg, cfg.DatabasePath, func() error {
		return a.assistantRunner.RunDatabaseCreate(cfg, a.assistantAppendOutput)
	})
}

// AssistantRunAnalyze 在后台运行 codeql database analyze，立即返回。
func (a *App) AssistantRunAnalyze(in AssistantConfigDTO) error {
	cfg := normalizeAssistantConfig(in.toConfig())
	if err := a.validateAssistantRun(cfg, "analyze"); err != nil {
		return err
	}
	a.persistAssistantConfig(cfg)
	output := cfg.OutputFile
	if output == "" {
		output = cfg.DatabasePath
	}
	return a.startAssistantRun("analyze", cfg, output, func() error {
		return a.assistantRunner.RunAnalyze(cfg, a.assistantAppendOutput)
	})
}

func (a *App) persistAssistantConfig(cfg assistant.Config) {
	a.assistantMu.Lock()
	a.assistantConfig = cfg
	a.assistantMu.Unlock()
	_ = cfg.SaveToFile(assistant.LastConfigPath())
}

func (a *App) startAssistantRun(action string, cfg assistant.Config, outputFile string, run func() error) error {
	a.assistantMu.Lock()
	if a.assistantBusy {
		a.assistantMu.Unlock()
		return fmt.Errorf("已有任务正在运行，请先停止当前任务")
	}
	a.assistantBusy = true
	a.assistantAction = action
	a.assistantStartedAt = time.Now()
	a.assistantLastElapsed = 0
	a.assistantStatus = assistant.StatusRunning
	a.assistantLogPath = ""
	a.assistantMu.Unlock()

	logPath := a.assistantPump.start()
	a.assistantMu.Lock()
	a.assistantLogPath = logPath
	a.assistantMu.Unlock()
	a.emitAssistantStatus()

	go func() {
		start := time.Now()
		err := run()
		duration := time.Since(start)
		a.assistantPump.finish()

		status := a.assistantRunner.Status()
		if err != nil && status == assistant.StatusRunning {
			status = assistant.StatusFailed
		}
		statusLabel := status.Label()

		a.assistantHistory.AddEntry(cfg, action, statusLabel, duration, outputFile)

		a.assistantMu.Lock()
		a.assistantBusy = false
		a.assistantStatus = status
		a.assistantStartedAt = time.Time{}
		a.assistantLastElapsed = duration
		a.assistantMu.Unlock()
		a.emitAssistantStatus()

		errText := ""
		if err != nil {
			errText = err.Error()
		}
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "assistant:done", AssistantDoneDTO{
				Action:      action,
				Status:      string(status),
				StatusLabel: statusLabel,
				OutputFile:  outputFile,
				DurationMs:  duration.Milliseconds(),
				Error:       errText,
			})
		}
	}()
	return nil
}

func (a *App) assistantAppendOutput(line string, isError bool) {
	a.assistantPump.add(line, isError)
}

// AssistantStop 强制停止当前任务。任务运行器会先优雅终止，再强制结束进程组。
func (a *App) AssistantStop() {
	a.assistantMu.Lock()
	busy := a.assistantBusy
	a.assistantMu.Unlock()
	if !busy {
		return
	}
	a.assistantPump.add("--- 正在停止任务… ---", false)
	// 覆盖「任务刚提交、Runner 还没 Start」的窗口。
	a.assistantRunner.CancelPending()
	a.assistantRunner.Stop()
}

// closeAssistant 在应用退出时释放进程与日志资源。
func (a *App) closeAssistant() {
	if a.assistantRunner != nil {
		a.assistantRunner.Stop()
	}
	if a.assistantPump != nil {
		a.assistantPump.finish()
	}
}

// ------------------------- 命令预览 -------------------------

// AssistantPreviewCommands 生成当前配置对应的可复制命令。
func (a *App) AssistantPreviewCommands(in AssistantConfigDTO) *AssistantCommandsDTO {
	preview := assistant.PreviewCommands(normalizeAssistantConfig(in.toConfig()))
	return &AssistantCommandsDTO{Create: preview.Create, Analyze: preview.Analyze}
}

// ------------------------- 文件与目录选择 -------------------------

func (a *App) defaultDirFor(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		info, err := os.Stat(value)
		if err == nil {
			if info.IsDir() {
				return value
			}
			return filepath.Dir(value)
		}
	}
	if fallback != "" {
		if _, err := os.Stat(fallback); err == nil {
			return fallback
		}
	}
	return ""
}

// AssistantChooseBinary 选择 CodeQL 可执行文件。
func (a *App) AssistantChooseBinary() (string, error) {
	a.assistantMu.Lock()
	current := a.assistantConfig.BinaryPath
	a.assistantMu.Unlock()
	name := "codeql"
	if runtime.GOOS == "windows" {
		name = "codeql.exe"
	}
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:            "选择 CodeQL 可执行文件",
		DefaultDirectory: a.defaultDirFor(current, ""),
		DefaultFilename:  name,
		Filters: []wruntime.FileFilter{
			{DisplayName: "所有文件", Pattern: "*"},
		},
	})
}

// AssistantChooseQueryFile 选择单个查询或查询套件文件。
func (a *App) AssistantChooseQueryFile() (string, error) {
	a.assistantMu.Lock()
	current := a.assistantConfig.QueryPath
	a.assistantMu.Unlock()
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:            "选择查询文件或查询套件",
		DefaultDirectory: a.defaultDirFor(current, ""),
		Filters: []wruntime.FileFilter{
			{DisplayName: "CodeQL 查询 (*.ql)", Pattern: "*.ql"},
			{DisplayName: "查询套件 (*.qls)", Pattern: "*.qls"},
			{DisplayName: "所有文件", Pattern: "*"},
		},
	})
}

// AssistantChooseQueryDir 选择查询包目录。
func (a *App) AssistantChooseQueryDir() (string, error) {
	a.assistantMu.Lock()
	current := a.assistantConfig.QueryPath
	a.assistantMu.Unlock()
	return a.AssistantChooseDirectory("选择查询包目录", current)
}

// AssistantChooseDirectory 打开目录选择框。
func (a *App) AssistantChooseDirectory(title, startDir string) (string, error) {
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: a.defaultDirFor(startDir, assistant.DataDir()),
	})
}

// AssistantChooseOutputFile 选择分析结果输出文件。
func (a *App) AssistantChooseOutputFile(defaultName string) (string, error) {
	a.assistantMu.Lock()
	current := a.assistantConfig.OutputFile
	a.assistantMu.Unlock()
	if strings.TrimSpace(defaultName) == "" {
		defaultName = "codeql-results.sarif"
	}
	return wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:                "选择分析结果输出文件",
		DefaultFilename:      defaultName,
		DefaultDirectory:     a.defaultDirFor(current, ""),
		CanCreateDirectories: true,
		Filters: []wruntime.FileFilter{
			{DisplayName: "所有文件", Pattern: "*"},
		},
	})
}

// AssistantOpenLog 用系统默认程序打开当前运行日志。
func (a *App) AssistantOpenLog() error {
	a.assistantMu.Lock()
	path := a.assistantLogPath
	a.assistantMu.Unlock()
	if path == "" {
		path = a.assistantPump.logPath()
	}
	if path == "" {
		return fmt.Errorf("当前还没有运行日志")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("日志文件不存在：%s", path)
	}
	return openLocalPath(path)
}

// AssistantOpenPath 用系统默认程序打开一个本地文件或目录。
func (a *App) AssistantOpenPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("路径为空")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("路径不存在：%s", path)
	}
	return openLocalPath(path)
}

// AssistantOpenDataDir 打开助手数据目录。
func (a *App) AssistantOpenDataDir() error {
	dir := assistant.DataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return openLocalPath(dir)
}

func openLocalPath(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("打开失败：%w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
