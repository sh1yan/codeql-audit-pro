package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"codeql-audit-pro/internal/config"
	"codeql-audit-pro/internal/model"
	"codeql-audit-pro/internal/sourcefile"
	"codeql-audit-pro/internal/translate"
	"codeql-audit-pro/internal/verify"
)

// batchConcurrency 是批量 AI 验证的并发数。取 3 是保守值：
// 多数 OpenAI 兼容服务对并发有限制，串行又太慢。
const batchConcurrency = 3

// BatchStartDTO 是批量验证启动后的返回。
type BatchStartDTO struct {
	Total int `json:"total"`
}

// BatchStateDTO 是批量验证的实时状态。
//
// 前端可能在中途关掉批量窗口再打开，甚至刷新界面；
// 只要后端还在跑，就必须能读回「正在进行 / 进度 / 是否已在停止中」，
// 否则界面会显示成「未开始」，用户就只能干等整批跑完。
type BatchStateDTO struct {
	Running       bool   `json:"running"`
	Stopping      bool   `json:"stopping"`
	Done          int    `json:"done"`
	Total         int    `json:"total"`
	FalsePositive int    `json:"falsePositive"`
	TruePositive  int    `json:"truePositive"`
	Uncertain     int    `json:"uncertain"`
	Failed        int    `json:"failed"`
	LastError     string `json:"lastError"`
	StartedAt     string `json:"startedAt"`
	ElapsedSecond int    `json:"elapsedSecond"`
}

// BatchState 返回当前批量验证状态（不改变任何东西，可随时轮询）。
func (a *App) BatchState() *BatchStateDTO {
	a.batchMu.Lock()
	defer a.batchMu.Unlock()
	st := &BatchStateDTO{
		Running:       a.batchRunning,
		Stopping:      a.batchStopping,
		Done:          a.batchDone,
		Total:         a.batchTotal,
		FalsePositive: a.batchFalsePositive,
		TruePositive:  a.batchTruePositive,
		Uncertain:     a.batchUncertain,
		Failed:        a.batchFailed,
		LastError:     a.batchLastErr,
	}
	if !a.batchStarted.IsZero() {
		st.StartedAt = a.batchStarted.Format(time.RFC3339)
		st.ElapsedSecond = int(time.Since(a.batchStarted).Seconds())
	}
	return st
}

// verifier 返回当前 AI 判定器。只看 AI 验证自己的开关，不看翻译开关。
func (a *App) verifier() *verify.Verifier {
	a.mu.Lock()
	llm := a.cfg.LLM
	enabled := a.cfg.AIVerify()
	a.mu.Unlock()
	cfg := toLLMConfig(llm)
	cfg.Enabled = enabled
	client := translate.NewLLMClient(cfg)
	return verify.New(client, cfg.Model)
}

// aiVerifyMissingHint 返回「AI 验证还不能用」时可操作的中文提示。
func (a *App) aiVerifyMissingHint() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var missing []string
	if !a.cfg.AIVerify() {
		missing = append(missing, "勾选「AI 误报验证 → 启用 AI 误报验证」")
	}
	if strings.TrimSpace(a.cfg.LLM.BaseURL) == "" {
		missing = append(missing, "填写接口地址（OpenAI 兼容，如 https://api.deepseek.com/v1）")
	}
	if strings.TrimSpace(a.cfg.LLM.Model) == "" {
		missing = append(missing, "填写模型名（如 deepseek-chat）")
	}
	if len(missing) == 0 {
		return ""
	}
	return "请在「设置」里：" + strings.Join(missing, "；") + "。"
}

// buildVerifyInput 组装一条漏洞的判定上下文（含源码片段）。
func (a *App) buildVerifyInput(f *model.Finding) verify.Input {
	a.mu.Lock()
	sendCode := a.cfg.AISendCode()
	a.mu.Unlock()
	in := verify.Input{
		RuleName:    f.RuleName,
		RuleNameZH:  f.RuleNameZH,
		Description: f.Description,
		SeverityZH:  f.Severity.ZH(),
		MessageZH:   f.MessageZHText(),
		MessageEN:   f.MessageENText(),
		SendCode:    sendCode,
	}
	const ctx = 3
	if sendCode {
		if snip, ok := a.snippetFor(f.Sink, ctx); ok {
			in.Sink = &snip
		}
	} else if f.Sink.StartLine > 0 {
		in.Sink = &verify.Snippet{Path: f.Sink.DisplayPath(), Line: f.Sink.StartLine}
	}
	for _, s := range f.Sources {
		if sendCode {
			if snip, ok := a.snippetFor(s, ctx); ok {
				in.Sources = append(in.Sources, snip)
			}
			continue
		}
		if s.StartLine > 0 {
			in.Sources = append(in.Sources, verify.Snippet{Path: s.DisplayPath(), Line: s.StartLine})
		}
	}
	return in
}

// snippetFor 读取一处位置附近的源码片段。
func (a *App) snippetFor(loc model.Location, ctx int) (verify.Snippet, bool) {
	path := loc.ResolvedPath
	if path == "" || loc.StartLine <= 0 {
		return verify.Snippet{}, false
	}
	a.snippetMu.Lock()
	lines, ok := a.snippetCache[path]
	a.snippetMu.Unlock()
	if !ok {
		var err error
		lines, _, err = sourcefile.ReadLines(path)
		if err != nil {
			return verify.Snippet{}, false
		}
		a.snippetMu.Lock()
		a.snippetCache[path] = lines
		a.snippetMu.Unlock()
	}
	el, _ := loc.NormalizedEnd()
	return verify.BuildSnippet(loc.DisplayPath(), lines, loc.StartLine, el, ctx), true
}

// verifyTimeout 返回单条 AI 验证的超时时间（至少 30 秒）。
func (a *App) verifyTimeout() time.Duration {
	a.mu.Lock()
	t := a.cfg.LLM.TimeoutSeconds
	a.mu.Unlock()
	if t <= 0 {
		t = 60
	}
	if t < 30 {
		t = 30
	}
	return time.Duration(t) * time.Second
}

// VerifyFinding 对一条漏洞做 AI 误报判定（同步返回，前端用 Promise 等待）。
func (a *App) VerifyFinding(key string) (*VerifyResponseDTO, error) {
	a.mu.Lock()
	f := a.findFindingLocked(key)
	a.mu.Unlock()
	if f == nil {
		return nil, fmt.Errorf("找不到这条漏洞，可能项目已重新载入")
	}
	v := a.verifier()
	if !v.Configured() {
		return nil, fmt.Errorf("AI 误报验证尚不可用：%s\n\n隐私说明：可在设置里关闭「AI 验证时发送源码片段」。",
			a.aiVerifyMissingHint())
	}
	input := a.buildVerifyInput(f)
	ctx, cancel := context.WithTimeout(context.Background(), a.verifyTimeout())
	defer cancel()
	res, err := v.Verify(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("AI 验证失败：%w", err)
	}
	return a.applyAIVerdict(f, res)
}

// applyAIVerdict 把判定写回漏洞、项目状态与配置，并返回最新统计。
func (a *App) applyAIVerdict(f *model.Finding, res verify.Result) (*VerifyResponseDTO, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if f == nil {
		return nil, fmt.Errorf("漏洞为空")
	}
	f.AIVerdict = string(res.Verdict)
	f.AIReason = res.Reason
	f.AIConfidence = res.Confidence
	f.AIModel = res.Model
	f.AIAt = time.Now().Format(time.RFC3339)
	if a.proj != nil {
		st := a.cfg.ProjectFor(a.proj.CSVPath, a.proj.SrcRoot)
		if st.Verdicts == nil {
			st.Verdicts = map[string]config.VerdictRecord{}
		}
		st.Verdicts[f.Key()] = config.VerdictRecord{
			Status:     f.AIVerdict,
			Reason:     f.AIReason,
			Confidence: f.AIConfidence,
			Model:      f.AIModel,
			At:         f.AIAt,
		}
	}
	a.scheduleSaveLocked()
	s := a.statsLocked()
	return &VerifyResponseDTO{FindingKey: f.Key(), AI: aiDTO(f), Stats: s}, nil
}

// ClearVerdict 清除一条漏洞的 AI 判定结果。
func (a *App) ClearVerdict(key string) (*StatsDTO, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	f := a.findFindingLocked(key)
	if f == nil {
		return nil, fmt.Errorf("找不到这条漏洞")
	}
	f.AIVerdict, f.AIReason, f.AIConfidence, f.AIModel, f.AIAt = "", "", 0, "", ""
	if a.proj != nil {
		st := a.cfg.ProjectFor(a.proj.CSVPath, a.proj.SrcRoot)
		if st.Verdicts != nil {
			delete(st.Verdicts, key)
		}
	}
	a.scheduleSaveLocked()
	s := a.statsLocked()
	return &s, nil
}

// StartBatchVerify 启动批量 AI 验证。keys 为空时验证全部记录。
//
// 验证在后台 goroutine 中以 3 路并发执行，通过 batch:progress / batch:done
// 事件向前端汇报；同一时间只允许一个批次。
func (a *App) StartBatchVerify(keys []string) (*BatchStartDTO, error) {
	v := a.verifier()
	if !v.Configured() {
		return nil, fmt.Errorf("批量 AI 验证尚不可用：%s", a.aiVerifyMissingHint())
	}
	targets := a.filterFindings(keys)
	if len(targets) == 0 {
		return nil, fmt.Errorf("当前范围内没有需要验证的漏洞")
	}

	a.batchMu.Lock()
	if a.batchRunning {
		a.batchMu.Unlock()
		return nil, fmt.Errorf("已有批量验证在进行中")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.batchCancel = cancel
	a.batchRunning = true
	a.batchStopping = false
	a.batchTotal = len(targets)
	a.batchDone = 0
	a.batchFalsePositive = 0
	a.batchTruePositive = 0
	a.batchUncertain = 0
	a.batchFailed = 0
	a.batchLastErr = ""
	a.batchStarted = time.Now()
	a.batchMu.Unlock()

	total := len(targets)
	go a.runBatch(ctx, v, targets)
	return &BatchStartDTO{Total: total}, nil
}

// CancelBatchVerify 请求强制停止批量验证。
//
// 取消 context 会立刻中断排队中的任务与在途 HTTP 请求；
// 已经拿到的结论照常写回并保存，所以强制停止不会丢掉已完成的部分。
func (a *App) CancelBatchVerify() {
	a.batchMu.Lock()
	cancel := a.batchCancel
	if a.batchRunning {
		a.batchStopping = true
	}
	running := a.batchRunning
	total := a.batchTotal
	done := a.batchDone
	a.batchMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if running && a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "batch:stopping", map[string]any{
			"done":  done,
			"total": total,
		})
	}
}

func (a *App) runBatch(ctx context.Context, v *verify.Verifier, targets []*model.Finding) {
	var (
		done    atomic.Int64
		failed  atomic.Int64
		fp      atomic.Int64
		tp      atomic.Int64
		un      atomic.Int64
		lastErr atomic.Value
	)
	emitProgress := func(f *model.Finding, ai *AIDTO) {
		errMsg := ""
		if v := lastErr.Load(); v != nil {
			errMsg, _ = v.(string)
		}
		doneN := int(done.Load())
		// 先把进度写进 App 状态：这样前端关掉窗口再打开（或重新载入界面）
		// 也能通过 BatchState 读回真实进度并强制停止，而不是看到「未开始」。
		a.batchMu.Lock()
		a.batchDone = doneN
		a.batchFalsePositive = int(fp.Load())
		a.batchTruePositive = int(tp.Load())
		a.batchUncertain = int(un.Load())
		a.batchFailed = int(failed.Load())
		a.batchLastErr = errMsg
		a.batchTotal = len(targets)
		stopping := a.batchStopping
		a.batchMu.Unlock()
		if a.ctx == nil {
			return
		}
		wruntime.EventsEmit(a.ctx, "batch:progress", BatchProgressDTO{
			Done:          doneN,
			Total:         len(targets),
			FalsePositive: int(fp.Load()),
			TruePositive:  int(tp.Load()),
			Uncertain:     int(un.Load()),
			Failed:        int(failed.Load()),
			LastError:     errMsg,
			FindingKey:    keyOf(f),
			AI:            ai,
			Stopping:      stopping,
		})
	}
	emitProgress(nil, nil)

	// 预先在主线程组装上下文（含源码片段），后台只做大模型调用。
	jobs := make([]verify.Input, len(targets))
	a.mu.Lock()
	sendCode := a.cfg.AISendCode()
	a.mu.Unlock()
	for i, f := range targets {
		_ = sendCode
		jobs[i] = a.buildVerifyInput(f)
	}

	sem := make(chan struct{}, batchConcurrency)
	var wg sync.WaitGroup
	for i, f := range targets {
		wg.Add(1)
		go func(f *model.Finding, input verify.Input) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			reqCtx, cancel := context.WithTimeout(ctx, a.verifyTimeout())
			res, err := v.Verify(reqCtx, input)
			cancel()
			if ctx.Err() != nil {
				return
			}
			var ai *AIDTO
			if err != nil {
				failed.Add(1)
				lastErr.Store(err.Error())
			} else {
				resp, applyErr := a.applyAIVerdict(f, res)
				if applyErr != nil {
					failed.Add(1)
					lastErr.Store(applyErr.Error())
				} else {
					ai = resp.AI
					switch res.Verdict {
					case verify.FalsePositive:
						fp.Add(1)
					case verify.TruePositive:
						tp.Add(1)
					default:
						un.Add(1)
					}
				}
			}
			done.Add(1)
			emitProgress(f, ai)
		}(f, jobs[i])
	}

	wg.Wait()
	a.mu.Lock()
	a.flushProjectLocked()
	_ = a.cfg.Save()
	a.mu.Unlock()

	stopped := ctx.Err() != nil
	summary := BatchSummaryDTO{
		Done:          int(done.Load()),
		Total:         len(targets),
		FalsePositive: int(fp.Load()),
		TruePositive:  int(tp.Load()),
		Uncertain:     int(un.Load()),
		Failed:        int(failed.Load()),
		Stopped:       stopped,
	}
	if stopped {
		summary.Message = fmt.Sprintf("已停止：共处理 %d / %d 条。结论已保存，可稍后继续。", summary.Done, summary.Total)
	} else {
		summary.Message = fmt.Sprintf("完成：共 %d 条，误报 %d，确认 %d，存疑 %d，失败 %d。结论已保存。",
			summary.Done, summary.FalsePositive, summary.TruePositive, summary.Uncertain, summary.Failed)
	}
	a.batchMu.Lock()
	a.batchRunning = false
	a.batchStopping = false
	a.batchCancel = nil
	a.batchDone = int(done.Load())
	a.batchMu.Unlock()
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "batch:done", summary)
	}
}

func keyOf(f *model.Finding) string {
	if f == nil {
		return ""
	}
	return f.Key()
}

// TestConnection 用当前填写的参数真实调用一次大模型，验证配置是否可用。
func (a *App) TestConnection(settings LLMSettingsDTO) (*TestConnectionDTO, error) {
	base := strings.TrimSpace(settings.BaseURL)
	modelName := strings.TrimSpace(settings.Model)
	if base == "" || modelName == "" {
		return nil, fmt.Errorf("请先填写接口地址与模型名，再测试连接")
	}
	cfg := translate.LLMConfig{
		Enabled:        true,
		BaseURL:        base,
		APIKey:         strings.TrimSpace(settings.APIKey),
		Model:          modelName,
		TimeoutSeconds: settings.TimeoutSeconds,
	}
	if cfg.TimeoutSeconds < 15 {
		cfg.TimeoutSeconds = 15
	}
	client := translate.NewLLMClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()
	out, err := client.Chat(ctx, "你是连通性测试助手，只回复 OK。", "请只回复 OK")
	if err != nil {
		return nil, fmt.Errorf("连接失败：%w", err)
	}
	reply := strings.TrimSpace(out)
	if rs := []rune(reply); len(rs) > 160 {
		reply = string(rs[:160]) + "…"
	}
	if reply == "" {
		reply = "（空响应）"
	}
	return &TestConnectionDTO{Endpoint: cfg.Endpoint(), Model: cfg.Model, Reply: reply}, nil
}

// TranslateProject 立即补齐当前项目中所有缺失的中文翻译。
//
// 早期版本只有在重新打开项目时才会再次调用大模型；这里把同一个过程做成
// 可重复触发的接口，返回更新后的完整项目载荷，前端拿到后立刻刷新列表与详情。
func (a *App) TranslateProject() (*TranslateProjectDTO, error) {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	a.mu.Lock()
	p := a.proj
	engine := a.engine
	a.mu.Unlock()
	if p == nil {
		return nil, fmt.Errorf("项目尚未载入")
	}
	if engine == nil {
		return nil, fmt.Errorf("翻译引擎未初始化")
	}

	missing := p.MissingTranslationCount()
	if missing == 0 {
		dto, err := a.projectDTO()
		if err != nil {
			return nil, err
		}
		return &TranslateProjectDTO{
			Project:   dto,
			Total:     0,
			Remaining: 0,
			Message:   "当前项目没有待翻译的内容。",
		}, nil
	}
	if !engine.LLM().Configured() {
		return nil, fmt.Errorf(
			"大模型翻译不可用：请在「设置 → 模型与 AI」中开启「启用大模型翻译兜底」，并填写接口地址、模型与 API Key。")
	}

	engine.ClearLastError()
	total, translated, remaining := p.TranslateMissing(func(done, total int, msg string) {
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "translate:progress", map[string]any{
				"done":    done,
				"total":   total,
				"message": msg,
			})
		}
	})

	a.mu.Lock()
	a.fileMu.Lock()
	a.fileCache = map[string]*FileDTO{}
	a.fileMu.Unlock()
	lastErr := engine.LastError()
	a.mu.Unlock()

	dto, err := a.projectDTO()
	if err != nil {
		return nil, err
	}
	message := fmt.Sprintf("翻译完成：本次处理 %d 条，补全 %d 条，仍有 %d 条待翻译。", total, translated, remaining)
	if total == 0 {
		message = "当前项目没有待翻译的内容。"
	} else if remaining == 0 {
		message = fmt.Sprintf("翻译完成：本次补全 %d 条，当前项目已全部中文化。", translated)
	}
	return &TranslateProjectDTO{
		Project:    dto,
		Total:      total,
		Translated: translated,
		Remaining:  remaining,
		Message:    message,
		LastError:  lastErr,
	}, nil
}

// ------------------------- 翻译库与纠错 -------------------------

// GetLibrary 查询翻译库，kind 为空表示全部。
func (a *App) GetLibrary(kind, query string) (*LibraryDTO, error) {
	a.mu.Lock()
	engine := a.engine
	a.mu.Unlock()
	if engine == nil || engine.Store() == nil {
		return nil, fmt.Errorf("翻译库未初始化")
	}
	entries, err := engine.Store().List(kind, query, 800)
	if err != nil {
		return nil, err
	}
	total, _ := engine.Store().Count(kind)
	out := make([]LibraryEntryDTO, 0, len(entries))
	for _, e := range entries {
		origin := "人工校正"
		switch e.Origin {
		case translate.OriginBuiltin:
			origin = "内置词典"
		case translate.OriginLLM:
			origin = "大模型"
		}
		out = append(out, LibraryEntryDTO{
			ID:        e.ID,
			Kind:      e.Kind,
			KindZh:    a.libraryKindZh(e.Kind),
			SourceEN:  e.SourceEN,
			SourceZH:  e.SourceZH,
			Origin:    e.Origin,
			OriginZh:  origin,
			Hits:      e.Hits,
			UpdatedAt: e.UpdatedAt,
		})
	}
	return &LibraryDTO{Total: total, Shown: len(out), Entries: out}, nil
}

// SaveLibraryEntry 保存一条人工校正的译文。
func (a *App) SaveLibraryEntry(kind, en, zh string) error {
	a.mu.Lock()
	engine := a.engine
	a.mu.Unlock()
	if engine == nil {
		return fmt.Errorf("翻译引擎未初始化")
	}
	if strings.TrimSpace(zh) == "" {
		return fmt.Errorf("译文不能为空")
	}
	return engine.Correct(kind, en, zh)
}

// CorrectionResponse 是译文纠错的返回：更新后的漏洞 + 最新统计。
type CorrectionResponse struct {
	Finding *FindingDTO `json:"finding"`
	Stats   StatsDTO    `json:"stats"`
}

// CorrectMessage 纠正某条漏洞详情的中文模板，并立即重新翻译该条漏洞。
func (a *App) CorrectMessage(key, zhTemplate string) (*CorrectionResponse, error) {
	zhTemplate = strings.TrimSpace(zhTemplate)
	if zhTemplate == "" {
		return nil, fmt.Errorf("中文模板不能为空")
	}
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.mu.Lock()
	f := a.findFindingLocked(key)
	engine := a.engine
	p := a.proj
	a.mu.Unlock()
	if f == nil || p == nil {
		return nil, fmt.Errorf("找不到这条漏洞")
	}
	if err := engine.Correct(translate.KindMessage, f.MessageKey, zhTemplate); err != nil {
		return nil, err
	}
	p.Retranslate(f)
	a.mu.Lock()
	a.fileMu.Lock()
	a.fileCache = map[string]*FileDTO{}
	a.fileMu.Unlock()
	dto := findingDTO(f, a.reviewed[f.Key()])
	s := a.statsLocked()
	a.mu.Unlock()
	return &CorrectionResponse{Finding: &dto, Stats: s}, nil
}
