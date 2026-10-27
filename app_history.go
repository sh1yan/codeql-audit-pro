package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// GetProjectHistory 返回历史审计项目列表（最近打开优先）。
func (a *App) GetProjectHistory() ([]ProjectHistoryDTO, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.projectHistoryLocked(), nil
}

// DeleteProjectHistory 删除一个历史项目。若删除的是当前打开的项目，
// 同时退出该项目，界面会回到审计工作台的选择界面。
func (a *App) DeleteProjectHistory(csvPath string) ([]ProjectHistoryDTO, error) {
	key := strings.TrimSpace(csvPath)
	if key == "" {
		return nil, fmt.Errorf("项目路径为空")
	}
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	active := a.proj != nil && strings.EqualFold(strings.TrimSpace(a.proj.CSVPath), key)
	if active {
		a.clearProjectLocked()
	}
	if err := a.cfg.DeleteProject(key); err != nil {
		return nil, err
	}
	return a.projectHistoryLocked(), nil
}

// ClearProjectHistory 清空全部历史项目；当前项目也会被关闭。
func (a *App) ClearProjectHistory() ([]ProjectHistoryDTO, error) {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.clearProjectLocked()
	if err := a.cfg.ClearProjects(); err != nil {
		return nil, err
	}
	return a.projectHistoryLocked(), nil
}

// CloseProject 退出当前项目，保留历史记录，界面回到审计工作台选择页。
func (a *App) CloseProject() error {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proj == nil {
		return nil
	}
	a.flushProjectLocked()
	err := a.cfg.Save()
	a.clearProjectLocked()
	return err
}

// clearProjectLocked 清空当前项目相关的内存状态（调用方需持有 a.mu）。
func (a *App) clearProjectLocked() {
	a.proj = nil
	a.reviewed = map[string]bool{}
	a.fileMu.Lock()
	a.fileCache = map[string]*FileDTO{}
	a.fileMu.Unlock()
	a.snippetMu.Lock()
	a.snippetCache = map[string][]string{}
	a.snippetMu.Unlock()
}

func (a *App) projectHistoryLocked() []ProjectHistoryDTO {
	out := make([]ProjectHistoryDTO, 0, len(a.cfg.Projects))
	for i := range a.cfg.Projects {
		st := &a.cfg.Projects[i]
		path := strings.TrimSpace(st.CSVPath)
		if path == "" {
			continue
		}
		info := ProjectHistoryDTO{
			CSVPath:          path,
			CSVName:          baseName(path),
			SrcRoot:          strings.TrimSpace(st.SrcRoot),
			LastOpened:       st.LastOpened,
			TotalFindings:    st.TotalFindings,
			ResolvedFindings: st.ResolvedFindings,
			ReviewedCount:    st.ReviewedCount(),
			VerdictCount:     len(st.Verdicts),
			Format:           st.Format,
			FormatLabel:      formatLabel(st.Format),
			ToolName:         st.ToolName,
			Active: a.proj != nil &&
				strings.EqualFold(strings.TrimSpace(a.proj.CSVPath), path),
		}
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			info.ResultExists = true
			info.ResultSize = fi.Size()
		}
		if info.SrcRoot != "" {
			if fi, err := os.Stat(info.SrcRoot); err == nil && fi.IsDir() {
				info.SourceExists = true
			}
		}
		out = append(out, info)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ti, tj := parseHistoryTime(out[i].LastOpened), parseHistoryTime(out[j].LastOpened)
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return strings.ToLower(out[i].CSVPath) < strings.ToLower(out[j].CSVPath)
	})
	return out
}

func parseHistoryTime(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, strings.TrimSpace(s)); err == nil {
		return t
	}
	return time.Time{}
}

func formatLabel(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "sarif":
		return "SARIF v2.1.0"
	case "csv":
		return "CSV"
	default:
		return ""
	}
}
