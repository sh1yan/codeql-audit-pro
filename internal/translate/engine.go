package translate

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// 翻译来源标记。
const (
	SourceBuiltin = "builtin"
	SourceStore   = "store"
	SourceLLM     = "llm"
	SourceNone    = "none"
)

// Result 是一次翻译的结果。
type Result struct {
	ZH      string // 中文（可能仍含 {{LINK}} 等占位符，由调用方填充）
	Source  string // builtin / store / llm / none
	Pending bool   // 未命中且大模型不可用，暂时保持英文
}

// Stats 记录翻译命中情况。
type Stats struct {
	Total   int
	Builtin int
	Store   int
	LLM     int
	Pending int
}

// Engine 是翻译引擎：内存缓存 -> SQLite 翻译库 -> 模板 -> 大模型。
type Engine struct {
	store *Store
	llm   *LLMClient

	mu    sync.Mutex
	mem   map[string]memEntry
	stats Stats
}

// NewEngine 创建引擎，并用内置词典播种翻译库。
func NewEngine(store *Store, llm *LLMClient) (*Engine, error) {
	e := &Engine{store: store, llm: llm, mem: make(map[string]memEntry)}
	if store != nil {
		if _, err := store.SeedBuiltins(); err != nil {
			return nil, fmt.Errorf("播种内置词典失败: %w", err)
		}
	}
	return e, nil
}

// Store 返回底层翻译库（供界面做校正与浏览）。
func (e *Engine) Store() *Store { return e.store }

// LLM 返回大模型客户端。
func (e *Engine) LLM() *LLMClient { return e.llm }

// SetLLM 替换大模型客户端（设置变更后调用）。
func (e *Engine) SetLLM(c *LLMClient) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.llm = c
}

// Stats 返回统计快照。
func (e *Engine) Stats() Stats {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stats
}

// ResetStats 清零统计。
func (e *Engine) ResetStats() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stats = Stats{}
}

func memKey(kind, en string) string { return kind + "\x00" + en }

// memEntry 记住译文和它最初的来源，保证统计口径真实（内存缓存不应被算成翻译库命中）。
type memEntry struct {
	ZH     string
	Source string
}

func (e *Engine) memGet(kind, en string) (memEntry, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	v, ok := e.mem[memKey(kind, en)]
	return v, ok
}

func (e *Engine) memPut(kind, en, zh, source string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.mem[memKey(kind, en)] = memEntry{ZH: zh, Source: source}
}

func (e *Engine) count(source string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stats.Total++
	switch source {
	case SourceBuiltin:
		e.stats.Builtin++
	case SourceStore:
		e.stats.Store++
	case SourceLLM:
		e.stats.LLM++
	case SourceNone:
		e.stats.Pending++
	}
}

// lookup 在库中查询一条译文，命中即计次。
func (e *Engine) lookup(kind, en string) (Result, bool) {
	if e.store == nil {
		return Result{}, false
	}
	zh, origin, ok := e.store.Lookup(kind, en)
	if !ok {
		return Result{}, false
	}
	e.store.Touch(kind, en)
	src := SourceStore
	if origin == OriginBuiltin {
		src = SourceBuiltin
	} else if origin == OriginLLM {
		src = SourceLLM
	}
	return Result{ZH: zh, Source: src}, true
}

// TranslateStatic 翻译规则名、规则描述、标签等「静态文本」。
func (e *Engine) TranslateStatic(kind, en string) Result {
	en = strings.TrimSpace(en)
	if en == "" {
		return Result{ZH: "", Source: SourceNone}
	}
	if m, ok := e.memGet(kind, en); ok {
		e.count(m.Source)
		return Result{ZH: m.ZH, Source: m.Source}
	}
	if r, ok := e.lookup(kind, en); ok {
		e.memPut(kind, en, r.ZH, r.Source)
		e.count(r.Source)
		return r
	}
	// 规则名/描述可能带动态部分（少见），尝试模板
	if kind == KindRule || kind == KindLabel {
		c := Canonicalize(en)
		if c.Key != en {
			if r, ok := e.lookup(kind, c.Key); ok {
				zh := Fill(r.ZH, c.Vals)
				e.memPut(kind, en, zh, r.Source)
				e.count(r.Source)
				return Result{ZH: zh, Source: r.Source}
			}
		}
	}
	if r, err := e.callLLM(context.Background(), kind, en); err == nil {
		e.memPut(kind, en, r, SourceLLM)
		e.count(SourceLLM)
		return Result{ZH: r, Source: SourceLLM}
	}
	e.count(SourceNone)
	return Result{ZH: en, Source: SourceNone, Pending: true}
}

// TranslateMessage 翻译漏洞详情。key 必须是 Canonicalize 产出的规范化模板键。
// 返回的中文仍含 {{LINK}} 等占位符，需由调用方用 Fill 回填。
func (e *Engine) TranslateMessage(key string) Result {
	key = strings.TrimSpace(key)
	if key == "" {
		return Result{ZH: "", Source: SourceNone}
	}
	if m, ok := e.memGet(KindMessage, key); ok {
		e.count(m.Source)
		return Result{ZH: m.ZH, Source: m.Source}
	}
	if r, ok := e.lookup(KindMessage, key); ok {
		e.memPut(KindMessage, key, r.ZH, r.Source)
		e.count(r.Source)
		return r
	}
	// 多行消息（CodeQL 会把同一规则的多条证据拼在一个消息里）：逐行翻译后重新拼接
	if strings.Contains(key, "\n") {
		lines := strings.Split(key, "\n")
		outs := make([]string, 0, len(lines))
		allOK := true
		src := SourceStore
		for _, ln := range lines {
			if strings.TrimSpace(ln) == "" {
				outs = append(outs, ln)
				continue
			}
			r, ok := e.lookup(KindMessage, ln)
			if !ok {
				if m, ok2 := e.memGet(KindMessage, ln); ok2 {
					outs = append(outs, m.ZH)
					continue
				}
				allOK = false
				break
			}
			if r.Source == SourceBuiltin {
				src = SourceBuiltin
			}
			outs = append(outs, r.ZH)
		}
		if allOK {
			zh := strings.Join(outs, "\n")
			e.memPut(KindMessage, key, zh, src)
			e.count(src)
			return Result{ZH: zh, Source: src}
		}
	}
	if zh, err := e.callLLM(context.Background(), KindMessage, key); err == nil {
		e.memPut(KindMessage, key, zh, SourceLLM)
		e.count(SourceLLM)
		return Result{ZH: zh, Source: SourceLLM}
	}
	e.count(SourceNone)
	return Result{ZH: key, Source: SourceNone, Pending: true}
}

// callLLM 调用大模型并把结果写入翻译库。
func (e *Engine) callLLM(ctx context.Context, kind, text string) (string, error) {
	e.mu.Lock()
	client := e.llm
	e.mu.Unlock()
	if client == nil || !client.Configured() {
		return "", fmt.Errorf("未配置大模型")
	}
	zh, err := client.Translate(ctx, kind, text)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(zh) == "" {
		return "", fmt.Errorf("大模型返回空译文")
	}
	if e.store != nil {
		_ = e.store.Save(kind, text, zh, OriginLLM)
	}
	return zh, nil
}

// TranslateLabel 翻译证据链标签，返回可直接使用的中文标签。
func (e *Engine) TranslateLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	if zh, ok := builtinLabels[label]; ok {
		return zh
	}
	c := CanonicalizeLabel(label)
	// 优先精确匹配
	if r, ok := e.lookup(KindLabel, label); ok {
		return r.ZH
	}
	// 再匹配标签模板，例如 "ControlFlowNode for ImportMember"
	if c.Key != label {
		if r, ok := e.lookup(KindLabel, c.Key); ok {
			return Fill(r.ZH, c.Vals)
		}
	}
	// 兜底：查英文子串词典（例如 "user-provided value" 出现在长标签里）
	for en, zh := range builtinLabels {
		if strings.Contains(label, en) {
			return strings.ReplaceAll(label, en, zh)
		}
	}
	return label
}

// Correct 保存人工校正的译文（界面「译得不对」入口）。
func (e *Engine) Correct(kind, en, zh string) error {
	if e.store == nil {
		return fmt.Errorf("翻译库未打开")
	}
	if kind == KindMessage {
		// 与 TranslateMessage 的查询键保持一致：消息模板两端空白不参与匹配。
		en = strings.TrimSpace(en)
	}
	if err := e.store.Save(kind, en, zh, OriginUser); err != nil {
		return err
	}
	e.memPut(kind, en, zh, SourceStore)
	return nil
}

// Invalidate 丢弃内存缓存中的一条译文（译文被修改后调用）。
func (e *Engine) Invalidate(kind, en string) {
	if strings.TrimSpace(en) == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.mem, memKey(kind, en))
}
