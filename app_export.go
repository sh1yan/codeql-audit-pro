package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"codeql-audit-pro/internal/codeqlcsv"
	"codeql-audit-pro/internal/model"
)

// ExportChineseCSV 导出带中文表头的 CodeQL 九列。
func (a *App) ExportChineseCSV(keys []string) (*ExportResultDTO, error) {
	a.mu.Lock()
	p := a.proj
	a.mu.Unlock()
	if p == nil {
		return nil, fmt.Errorf("请先打开 CodeQL 结果文件")
	}
	findings := a.filterFindings(keys)
	if len(findings) == 0 {
		return nil, fmt.Errorf("当前筛选结果为空")
	}
	base := strings.TrimSuffix(filepath.Base(p.CSVPath), filepath.Ext(p.CSVPath))
	path, err := a.saveDialog("导出中文 CSV", base+"_中文.csv", filepath.Dir(p.CSVPath))
	if err != nil || path == "" {
		return nil, err
	}
	if err := p.ExportZH(path, findings); err != nil {
		return nil, err
	}
	return &ExportResultDTO{Path: path, Count: len(findings), Kind: "中文 CSV"}, nil
}

// ExportAuditReport 导出审计宽表（CSV / SARIF 通用，列定义见 codeqlcsv.ReportHeaders）。
func (a *App) ExportAuditReport(keys []string) (*ExportResultDTO, error) {
	a.mu.Lock()
	p := a.proj
	if p == nil {
		a.mu.Unlock()
		return nil, fmt.Errorf("请先打开 CodeQL 结果文件")
	}
	reviewed := make(map[string]bool, len(a.reviewed))
	for k, v := range a.reviewed {
		reviewed[k] = v
	}
	base := strings.TrimSuffix(filepath.Base(p.CSVPath), filepath.Ext(p.CSVPath))
	defaultDir := filepath.Dir(p.CSVPath)
	a.mu.Unlock()

	findings := a.filterFindings(keys)
	if len(findings) == 0 {
		return nil, fmt.Errorf("当前筛选结果为空")
	}
	path, err := a.saveDialog("导出审计报告", base+"_审计报告.csv", defaultDir)
	if err != nil || path == "" {
		return nil, err
	}
	rows := make([]codeqlcsv.ReportRow, 0, len(findings))
	for _, f := range findings {
		rows = append(rows, codeqlcsv.ReportRow{Finding: f, Reviewed: reviewed[f.Key()]})
	}
	if err := codeqlcsv.ExportReportFile(path, rows); err != nil {
		return nil, err
	}
	return &ExportResultDTO{Path: path, Count: len(rows), Kind: "审计报告"}, nil
}

// ExportVerdicts 导出全部 AI 判定结论。
func (a *App) ExportVerdicts() (*ExportResultDTO, error) {
	a.mu.Lock()
	p := a.proj
	if p == nil {
		a.mu.Unlock()
		return nil, fmt.Errorf("请先打开 CodeQL 结果文件")
	}
	findings := append([]*model.Finding(nil), p.Findings...)
	base := strings.TrimSuffix(filepath.Base(p.CSVPath), filepath.Ext(p.CSVPath))
	defaultDir := filepath.Dir(p.CSVPath)
	a.mu.Unlock()

	count := 0
	for _, f := range findings {
		if f.HasAIVerdict() {
			count++
		}
	}
	if count == 0 {
		return nil, fmt.Errorf("还没有任何 AI 判定结果可导出")
	}
	path, err := a.saveDialog("导出 AI 验证结果", base+"_AI验证结果.csv", defaultDir)
	if err != nil || path == "" {
		return nil, err
	}
	if err := writeVerdictCSV(path, findings); err != nil {
		return nil, err
	}
	return &ExportResultDTO{Path: path, Count: count, Kind: "AI 验证结果"}, nil
}

func (a *App) saveDialog(title, defaultName, defaultDir string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("窗口尚未就绪")
	}
	if defaultDir == "" || !pathExists(defaultDir) {
		defaultDir = ""
	}
	return wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:                title,
		DefaultFilename:      defaultName,
		DefaultDirectory:     defaultDir,
		CanCreateDirectories: true,
		Filters: []wruntime.FileFilter{
			{DisplayName: "CSV 文件 (*.csv)", Pattern: "*.csv"},
		},
	})
}

// writeVerdictCSV 写出一份带 UTF-8 BOM 的验证结果表，Excel 双击不乱码。
func writeVerdictCSV(path string, findings []*model.Finding) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"序号", "规则名称", "严重级别", "文件", "起始行", "起始列",
		"AI 判定", "置信度", "判定理由", "模型", "判定时间"})
	for _, fd := range findings {
		if !fd.HasAIVerdict() {
			continue
		}
		conf := ""
		if fd.AIConfidence > 0 {
			conf = fmt.Sprintf("%.0f%%", fd.AIConfidence*100)
		}
		_ = w.Write([]string{
			fmt.Sprintf("%d", fd.Index), fd.RuleDisplayName(), fd.Severity.ZH(),
			fd.Sink.File, fmt.Sprintf("%d", fd.Sink.StartLine), fmt.Sprintf("%d", fd.Sink.StartCol),
			fd.AIVerdictZH(), conf, oneLine(fd.AIReason), fd.AIModel, fd.AIAt,
		})
	}
	w.Flush()
	return w.Error()
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}
