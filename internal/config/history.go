package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动，与翻译库共用
)

// historySchema 是「历史审计项目」表。
//
// 早期版本把每个项目的已读标记、AI 判定、路径映射全部写在 config.json 的
// projects 数组里；结果文件一多，配置文件会膨胀到几 MB，启动时读取与解析
// 都会明显变慢。现在只把真正的设置留在 config.json，项目历史统一存进
// ~/.CodeQLAuditPro/history.db。
const historySchema = `
CREATE TABLE IF NOT EXISTS projects (
  csv_path          TEXT PRIMARY KEY COLLATE NOCASE,
  src_root          TEXT NOT NULL DEFAULT '',
  last_opened       TEXT NOT NULL DEFAULT '',
  total_findings    INTEGER NOT NULL DEFAULT 0,
  resolved_findings INTEGER NOT NULL DEFAULT 0,
  reviewed_count    INTEGER NOT NULL DEFAULT 0,
  verdict_count     INTEGER NOT NULL DEFAULT 0,
  format            TEXT NOT NULL DEFAULT '',
  tool_name         TEXT NOT NULL DEFAULT '',
  state_json        TEXT NOT NULL DEFAULT '{}',
  created_at        TEXT NOT NULL DEFAULT '',
  updated_at        TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_projects_last_opened ON projects (last_opened DESC);
CREATE TABLE IF NOT EXISTS meta (k TEXT PRIMARY KEY, v TEXT NOT NULL);
`

// historyStore 是历史项目 SQLite 库。
type historyStore struct {
	db   *sql.DB
	path string
}

func historyNow() string { return time.Now().UTC().Format(time.RFC3339) }

// openHistoryStore 打开（必要时创建）历史项目库。
func openHistoryStore(path string) (*historyStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("历史项目库路径为空")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("创建历史项目目录失败: %w", err)
		}
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return nil, fmt.Errorf("打开历史项目库失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("设置历史项目库 WAL 模式失败: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("设置历史项目库超时失败: %w", err)
	}
	if _, err := db.Exec(historySchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("初始化历史项目库失败: %w", err)
	}
	return &historyStore{db: db, path: path}, nil
}

func (h *historyStore) Close() error {
	if h == nil || h.db == nil {
		return nil
	}
	return h.db.Close()
}

// loadAll 按最近打开时间倒序读取全部项目状态。
func (h *historyStore) loadAll() ([]ProjectState, error) {
	if h == nil || h.db == nil {
		return nil, nil
	}
	rows, err := h.db.Query(
		`SELECT state_json FROM projects ORDER BY last_opened DESC, csv_path ASC`)
	if err != nil {
		return nil, fmt.Errorf("读取历史项目失败: %w", err)
	}
	defer rows.Close()
	out := make([]ProjectState, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var p ProjectState
		if err := json.Unmarshal([]byte(raw), &p); err != nil || strings.TrimSpace(p.CSVPath) == "" {
			// 单条记录损坏不应该拖垮整个历史列表。
			continue
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// saveAll 在一个事务里同步全部项目：存在则更新，不存在则插入，
// 已从内存中删除的项目会从库里清掉。
func (h *historyStore) saveAll(projects []ProjectState) error {
	if h == nil || h.db == nil {
		return nil
	}
	tx, err := h.db.Begin()
	if err != nil {
		return fmt.Errorf("写入历史项目失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const stmt = `
INSERT INTO projects (
  csv_path, src_root, last_opened, total_findings, resolved_findings,
  reviewed_count, verdict_count, format, tool_name, state_json, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (csv_path) DO UPDATE SET
  src_root          = excluded.src_root,
  last_opened       = excluded.last_opened,
  total_findings    = excluded.total_findings,
  resolved_findings = excluded.resolved_findings,
  reviewed_count    = excluded.reviewed_count,
  verdict_count     = excluded.verdict_count,
  format            = excluded.format,
  tool_name         = excluded.tool_name,
  state_json        = excluded.state_json,
  updated_at        = excluded.updated_at`

	now := historyNow()
	seen := make(map[string]bool, len(projects))
	for i := range projects {
		p := projects[i]
		path := strings.TrimSpace(p.CSVPath)
		if path == "" {
			continue
		}
		raw, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("序列化项目历史失败: %w", err)
		}
		reviewed := 0
		for _, v := range p.Reviewed {
			if v {
				reviewed++
			}
		}
		if _, err := tx.Exec(stmt,
			path,
			strings.TrimSpace(p.SrcRoot),
			p.LastOpened,
			p.TotalFindings,
			p.ResolvedFindings,
			reviewed,
			len(p.Verdicts),
			p.Format,
			p.ToolName,
			string(raw),
			now,
			now,
		); err != nil {
			return fmt.Errorf("写入历史项目失败: %w", err)
		}
		seen[strings.ToLower(path)] = true
	}

	rows, err := tx.Query(`SELECT csv_path FROM projects`)
	if err != nil {
		return err
	}
	var stale []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			_ = rows.Close()
			return err
		}
		if !seen[strings.ToLower(strings.TrimSpace(path))] {
			stale = append(stale, path)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	for _, path := range stale {
		if _, err := tx.Exec(`DELETE FROM projects WHERE csv_path = ?`, path); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// delete 删除单个项目历史。
func (h *historyStore) delete(csvPath string) error {
	if h == nil || h.db == nil {
		return nil
	}
	_, err := h.db.Exec(
		`DELETE FROM projects WHERE csv_path = ? COLLATE NOCASE`, strings.TrimSpace(csvPath))
	if err != nil {
		return fmt.Errorf("删除历史项目失败: %w", err)
	}
	return nil
}

// count 返回历史项目数量（迁移时用于判断库是否为空）。
func (h *historyStore) count() (int, error) {
	if h == nil || h.db == nil {
		return 0, nil
	}
	var n int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
