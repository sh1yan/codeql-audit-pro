package translate

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动，免 CGO 交叉编译问题
)

// 翻译条目的类别。
const (
	KindRule        = "rule"        // 规则名称
	KindDescription = "description" // 规则描述
	KindLabel       = "label"       // 证据链标签
	KindSeverity    = "severity"    // 严重级别
	KindMessage     = "message"     // 漏洞详情（键为规范化后的模板）
)

// 条目来源。
const (
	OriginBuiltin = "builtin" // 内置词典
	OriginLLM     = "llm"     // 大模型翻译
	OriginUser    = "user"    // 人工校正
)

// Entry 是翻译库中的一条「中英对照」记录。
type Entry struct {
	ID        int64
	Kind      string
	SourceEN  string
	SourceZH  string
	Origin    string
	Hits      int
	CreatedAt string
	UpdatedAt string
}

const storeSchema = `
CREATE TABLE IF NOT EXISTS translations (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  kind       TEXT NOT NULL,
  source_en  TEXT NOT NULL,
  source_zh  TEXT NOT NULL,
  origin     TEXT NOT NULL DEFAULT 'user',
  hits       INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE (kind, source_en)
);
CREATE INDEX IF NOT EXISTS idx_translations_kind ON translations (kind);
CREATE TABLE IF NOT EXISTS meta (k TEXT PRIMARY KEY, v TEXT NOT NULL);
`

// Store 是 SQLite 翻译库。
type Store struct {
	db *sql.DB
}

// OpenStore 打开（必要时创建）翻译库。
func OpenStore(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开翻译库失败: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite 写并发控制
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("设置 WAL 模式失败: %w", err)
	}
	if _, err := db.Exec(storeSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("初始化翻译库失败: %w", err)
	}
	return &Store{db: db}, nil
}

// Close 关闭数据库。
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// Lookup 查询中文译文。
func (s *Store) Lookup(kind, en string) (string, string, bool) {
	if s == nil || s.db == nil || strings.TrimSpace(en) == "" {
		return "", "", false
	}
	var zh, origin string
	err := s.db.QueryRow(
		`SELECT source_zh, origin FROM translations WHERE kind = ? AND source_en = ?`,
		kind, en).Scan(&zh, &origin)
	if err != nil || strings.TrimSpace(zh) == "" {
		return "", "", false
	}
	return zh, origin, true
}

// Touch 累加命中次数（用于统计「常用译文」）。
func (s *Store) Touch(kind, en string) {
	if s == nil || s.db == nil {
		return
	}
	_, _ = s.db.Exec(
		`UPDATE translations SET hits = hits + 1 WHERE kind = ? AND source_en = ?`, kind, en)
}

// Save 写入或更新一条译文。
func (s *Store) Save(kind, en, zh, origin string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("翻译库未打开")
	}
	if strings.TrimSpace(en) == "" || strings.TrimSpace(zh) == "" {
		return fmt.Errorf("原文或译文为空")
	}
	ts := now()
	_, err := s.db.Exec(`
		INSERT INTO translations (kind, source_en, source_zh, origin, hits, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, ?)
		ON CONFLICT (kind, source_en) DO UPDATE SET
			source_zh = excluded.source_zh,
			origin    = excluded.origin,
			updated_at = excluded.updated_at`,
		kind, en, zh, origin, ts, ts)
	if err != nil {
		return fmt.Errorf("写入翻译库失败: %w", err)
	}
	return nil
}

// SaveIfAbsent 仅当条目不存在时写入（用于内置词典播种，不覆盖用户校正）。
func (s *Store) SaveIfAbsent(kind, en, zh, origin string) error {
	if s == nil || s.db == nil {
		return nil
	}
	if strings.TrimSpace(en) == "" || strings.TrimSpace(zh) == "" {
		return nil
	}
	ts := now()
	_, err := s.db.Exec(`
		INSERT INTO translations (kind, source_en, source_zh, origin, hits, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, ?)
		ON CONFLICT (kind, source_en) DO NOTHING`,
		kind, en, zh, origin, ts, ts)
	return err
}

// Delete 删除一条译文。
func (s *Store) Delete(kind, en string) error {
	if s == nil || s.db == nil {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM translations WHERE kind = ? AND source_en = ?`, kind, en)
	return err
}

// Count 返回条目总数，可按 kind 过滤（kind 为空表示全部）。
func (s *Store) Count(kind string) (int, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	var n int
	var err error
	if kind == "" {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM translations`).Scan(&n)
	} else {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM translations WHERE kind = ?`, kind).Scan(&n)
	}
	return n, err
}

// List 列出条目，keyword 非空时按原文/译文模糊过滤。
func (s *Store) List(kind, keyword string, limit int) ([]Entry, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	q := `SELECT id, kind, source_en, source_zh, origin, hits, created_at, updated_at FROM translations`
	var args []any
	var where []string
	if kind != "" {
		where = append(where, "kind = ?")
		args = append(args, kind)
	}
	if strings.TrimSpace(keyword) != "" {
		where = append(where, "(source_en LIKE ? OR source_zh LIKE ?)")
		kw := "%" + keyword + "%"
		args = append(args, kw, kw)
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY hits DESC, id DESC"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Kind, &e.SourceEN, &e.SourceZH, &e.Origin,
			&e.Hits, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SeedBuiltins 用内置词典播种翻译库。已存在的条目不会被覆盖，
// 因此用户在界面上做过的校正会一直保留。
func (s *Store) SeedBuiltins() (int, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	ts := now()
	stmt, err := tx.Prepare(`
		INSERT INTO translations (kind, source_en, source_zh, origin, hits, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, ?)
		ON CONFLICT (kind, source_en) DO NOTHING`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	n := 0
	seed := func(kind, en, zh string) {
		if strings.TrimSpace(en) == "" || strings.TrimSpace(zh) == "" {
			return
		}
		if _, err := stmt.Exec(kind, en, zh, OriginBuiltin, ts, ts); err == nil {
			n++
		}
	}
	for en, e := range builtinRules {
		seed(KindRule, en, e.ZH)
	}
	// 规则描述以「描述原文」本身为键
	for _, d := range builtinDescriptions {
		seed(KindDescription, d.EN, d.ZH)
	}
	for en, zh := range builtinLabels {
		seed(KindLabel, en, zh)
	}
	for key, zh := range builtinLabelTemplates {
		seed(KindLabel, key, zh)
	}
	for key, zh := range builtinTemplates {
		seed(KindMessage, key, zh)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

// builtinDescription 是「规则描述原文 -> 中文」的补充词典。
type builtinDescription struct{ EN, ZH string }

var builtinDescriptions = []builtinDescription{
	{"Leaking information about an exception, such as messages and stack traces, to an external user can expose implementation details that are useful to an attacker for developing a subsequent exploit.",
		"将异常信息（如错误消息、堆栈跟踪）泄露给外部用户，会暴露有助于攻击者策划后续利用的实现细节。"},
	{"Making a network request to a URL that is fully user-controlled allows for request forgery attacks.",
		"向完全由用户控制的 URL 发起网络请求，会被用于请求伪造攻击。"},
	{"Building a SQL query from user-controlled sources is vulnerable to insertion of malicious SQL code by the user.",
		"使用用户可控的数据源构建 SQL 查询，容易被注入恶意 SQL 代码。"},
	{"Running a Flask app in debug mode may allow an attacker to run arbitrary code through the Werkzeug debugger.",
		"Flask 应用以调试模式运行，攻击者可能通过 Werkzeug 调试器执行任意代码。"},
	{"Credentials are hard coded in the source code of the application.",
		"凭据被直接硬编码在应用程序的源码中。"},
	{"Data provided remotely is used in this external API without sanitization, which could be a security risk.",
		"来自远端的不可信数据未经净化就被传入此外部 API，可能存在安全风险。"},
	{"Writing user input directly to a web page allows for a cross-site scripting vulnerability.",
		"将用户输入直接写入网页，会造成跨站脚本（XSS）漏洞。"},
}
