// Package pathmap 负责把 CodeQL 结果里的路径映射到本地真实源码路径。
//
// CodeQL 数据库通常是在容器或构建机上创建的，CSV 里的路径形如
// /app/views/user.py，而本机可能位于 ~/code/pyhackme/app/views/user.py。
// 本包通过「直接拼接 -> 前缀规则 -> 后缀匹配 -> 同名文件」四级策略完成定位。
package pathmap

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// PrefixRule 是人工配置的路径前缀替换规则。
type PrefixRule struct {
	From string `json:"from"` // CSV 路径前缀，例如 "app"
	To   string `json:"to"`   // 本地前缀，例如 "/Users/me/code/pyhackme"
}

// Result 是一次路径解析的结果。
type Result struct {
	Path    string   // 本地绝对路径，未命中时为空
	Score   int      // 置信度 0~100
	How     string   // 命中方式，便于界面解释
	Ambient []string // 有歧义时的候选列表
}

// Resolved 判断是否命中。
func (r Result) Resolved() bool { return r.Path != "" }

// skipDirs 是索引时跳过的目录名。
var skipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true,
	"vendor": true, "__pycache__": true, ".venv": true, "venv": true,
	"dist": true, "build": true, "target": true, ".idea": true,
	".vscode": true, ".next": true, ".cache": true, "bower_components": true,
	"Pods": true, "DerivedData": true, ".mypy_cache": true, ".pytest_cache": true,
	".tox": true, "site-packages": true, ".gradle": true, ".terraform": true,
}

const maxIndexFiles = 400000

// Resolver 在本机源码目录中定位 CSV 里的路径。
type Resolver struct {
	mu        sync.Mutex
	srcRoot   string
	prefixes  []PrefixRule
	overrides map[string]string // CSV 路径 -> 本地路径（人工指定，优先级最高）

	byRel      map[string][]string // 相对路径(小写) -> 绝对路径
	byBase     map[string][]string // 文件名(小写) -> 绝对路径
	indexed    bool
	indexedErr error
	cache      map[string]Result
}

// NewResolver 创建解析器。srcRoot 为用户指定的源码目录。
func NewResolver(srcRoot string, prefixes []PrefixRule, overrides map[string]string) *Resolver {
	ov := map[string]string{}
	for k, v := range overrides {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			ov[Normalize(k)] = v
		}
	}
	ps := make([]PrefixRule, 0, len(prefixes))
	for _, p := range prefixes {
		if strings.TrimSpace(p.From) != "" && strings.TrimSpace(p.To) != "" {
			ps = append(ps, PrefixRule{From: Normalize(p.From), To: strings.TrimSpace(p.To)})
		}
	}
	return &Resolver{
		srcRoot:   strings.TrimSpace(srcRoot),
		prefixes:  ps,
		overrides: ov,
		byRel:     map[string][]string{},
		byBase:    map[string][]string{},
		cache:     map[string]Result{},
	}
}

// SrcRoot 返回源码根目录。
func (r *Resolver) SrcRoot() string { return r.srcRoot }

// SetSrcRoot 更换源码根目录并清空缓存与索引。
func (r *Resolver) SetSrcRoot(root string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.srcRoot = strings.TrimSpace(root)
	r.indexed = false
	r.indexedErr = nil
	r.byRel = map[string][]string{}
	r.byBase = map[string][]string{}
	r.cache = map[string]Result{}
}

// SetOverride 记录一条人工指定的映射。
func (r *Resolver) SetOverride(csvPath, localPath string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.overrides[Normalize(csvPath)] = localPath
	delete(r.cache, Normalize(csvPath))
}

// Overrides 返回全部人工映射。
func (r *Resolver) Overrides() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(r.overrides))
	for k, v := range r.overrides {
		out[k] = v
	}
	return out
}

// PrefixRules 返回全部前缀规则。
func (r *Resolver) PrefixRules() []PrefixRule {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]PrefixRule, len(r.prefixes))
	copy(out, r.prefixes)
	return out
}

// AddPrefixRule 追加一条前缀规则并清空缓存。
func (r *Resolver) AddPrefixRule(from, to string) {
	from, to = Normalize(from), strings.TrimSpace(to)
	if from == "" || to == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prefixes = append(r.prefixes, PrefixRule{From: from, To: to})
	r.cache = map[string]Result{}
	r.indexed = false
}

// Normalize 规范化结果文件中的路径：去掉 URI scheme、反斜杠、前导 ./ 与 /。
func Normalize(p string) string {
	s := strings.TrimSpace(p)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	s = strings.ReplaceAll(s, "\\", "/")
	for strings.Contains(s, "//") {
		s = strings.ReplaceAll(s, "//", "/")
	}
	s = strings.TrimPrefix(s, "/")
	s = strings.TrimPrefix(s, "./")
	return path.Clean(s)
}

// suffixes 依次返回路径的各级后缀（由长到短）。
func suffixes(rel string) []string {
	segs := strings.Split(rel, "/")
	out := make([]string, 0, len(segs))
	for i := 0; i < len(segs); i++ {
		out = append(out, strings.Join(segs[i:], "/"))
	}
	return out
}

// buildIndex 惰性构建源码目录索引。
func (r *Resolver) buildIndex() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.indexed {
		return r.indexedErr
	}
	r.indexed = true
	if r.srcRoot == "" {
		r.indexedErr = fs.ErrNotExist
		return r.indexedErr
	}
	count := 0
	err := filepath.WalkDir(r.srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != r.srcRoot && skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		count++
		if count > maxIndexFiles {
			return fs.SkipAll
		}
		rel, relErr := filepath.Rel(r.srcRoot, p)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		key := strings.ToLower(rel)
		r.byRel[key] = append(r.byRel[key], p)
		base := strings.ToLower(path.Base(rel))
		r.byBase[base] = append(r.byBase[base], p)
		return nil
	})
	r.indexedErr = err
	return err
}

// pick 从多个候选中挑一个：优先路径最短（通常层级更接近源码根）。
func pick(list []string) (string, bool) {
	if len(list) == 0 {
		return "", false
	}
	cp := append([]string(nil), list...)
	sort.Slice(cp, func(i, j int) bool {
		if len(cp[i]) != len(cp[j]) {
			return len(cp[i]) < len(cp[j])
		}
		return cp[i] < cp[j]
	})
	return cp[0], true
}

func fileExists(p string) bool {
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// Resolve 解析一个结果文件中的逻辑路径。
func (r *Resolver) Resolve(csvPath string) Result {
	rel := Normalize(csvPath)
	if rel == "" || rel == "." {
		return Result{}
	}

	r.mu.Lock()
	if res, ok := r.cache[rel]; ok {
		r.mu.Unlock()
		return res
	}
	override, hasOverride := r.overrides[rel]
	root := r.srcRoot
	prefixes := append([]PrefixRule(nil), r.prefixes...)
	r.mu.Unlock()

	// ① 人工指定
	if hasOverride {
		res := Result{Path: override, Score: 100, How: "人工指定"}
		r.store(rel, res)
		return res
	}

	// ② 前缀规则替换
	for _, p := range prefixes {
		if rel == p.From || strings.HasPrefix(rel, p.From+"/") {
			tail := strings.TrimPrefix(strings.TrimPrefix(rel, p.From), "/")
			cand := filepath.Join(p.To, filepath.FromSlash(tail))
			if fileExists(cand) {
				res := Result{Path: cand, Score: 100, How: "前缀规则 " + p.From + " → " + p.To}
				r.store(rel, res)
				return res
			}
		}
	}

	if root == "" {
		r.store(rel, Result{})
		return Result{}
	}

	// ③ 直接拼接
	if cand := filepath.Join(root, filepath.FromSlash(rel)); fileExists(cand) {
		res := Result{Path: cand, Score: 100, How: "源码目录直接拼接"}
		r.store(rel, res)
		return res
	}

	// ④ 逐级去掉前导目录后匹配（容器里 /app 往往就是源码根）
	if err := r.buildIndex(); err == nil {
		sufs := suffixes(rel)
		for i := 1; i < len(sufs); i++ {
			key := strings.ToLower(sufs[i])
			r.mu.Lock()
			cands := append([]string(nil), r.byRel[key]...)
			r.mu.Unlock()
			if p, ok := pick(cands); ok {
				score := 95 - i*5
				if score < 70 {
					score = 70
				}
				res := Result{Path: p, Score: score, How: "后缀匹配 " + sufs[i], Ambient: cands}
				r.store(rel, res)
				return res
			}
		}

		// ⑤ 同名文件兜底
		base := strings.ToLower(path.Base(rel))
		r.mu.Lock()
		cands := append([]string(nil), r.byBase[base]...)
		r.mu.Unlock()
		if len(cands) == 1 {
			res := Result{Path: cands[0], Score: 60, How: "同名文件唯一匹配"}
			r.store(rel, res)
			return res
		}
		if len(cands) > 1 {
			p, _ := pick(cands)
			// 多个同名文件：优先取路径尾部与 CSV 路径重合度最高的
			best, bestScore := p, 0
			for _, c := range cands {
				sc := commonSuffixSegments(filepath.ToSlash(c), rel)
				if sc > bestScore {
					best, bestScore = c, sc
				}
			}
			res := Result{Path: best, Score: 40, How: "同名文件多候选（需人工确认）", Ambient: cands}
			r.store(rel, res)
			return res
		}
	}

	r.store(rel, Result{})
	return Result{}
}

// commonSuffixSegments 计算本地路径与 CSV 路径尾部重合的段数。
func commonSuffixSegments(local, rel string) int {
	a := strings.Split(local, "/")
	b := strings.Split(rel, "/")
	n := 0
	for i, j := len(a)-1, len(b)-1; i >= 0 && j >= 0; i, j = i-1, j-1 {
		if !strings.EqualFold(a[i], b[j]) {
			break
		}
		n++
	}
	return n
}

func (r *Resolver) store(rel string, res Result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[rel] = res
}
