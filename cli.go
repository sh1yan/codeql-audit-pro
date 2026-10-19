package main

import (
	"fmt"
	"os"

	"codeql-audit-pro/internal/config"
	"codeql-audit-pro/internal/project"
	"codeql-audit-pro/internal/translate"
)

// runCLI 在命令行下完成解析、中文化与导出，便于批量处理与自动化验证。
func runCLI(cfg *config.Config, csvPath, srcRoot, outPath string) error {
	if csvPath == "" {
		return fmt.Errorf("请用 -csv 指定 CodeQL 结果文件")
	}
	store, err := translate.OpenStore(cfg.TranslateDB)
	if err != nil {
		return err
	}
	defer store.Close()
	engine, err := translate.NewEngine(store, translate.NewLLMClient(translate.LLMConfig{
		Enabled:        cfg.LLM.Enabled,
		BaseURL:        cfg.LLM.BaseURL,
		APIKey:         cfg.LLM.APIKey,
		Model:          cfg.LLM.Model,
		TimeoutSeconds: cfg.LLM.TimeoutSeconds,
		TemplateOnly:   cfg.LLM.TemplateOnly,
	}))
	if err != nil {
		return err
	}

	p, err := project.Load(project.Options{
		CSVPath: csvPath,
		SrcRoot: srcRoot,
		Engine:  engine,
		Progress: func(done, total int, msg string) {
			fmt.Fprintf(os.Stderr, "\r%s", msg)
			if done >= total {
				fmt.Fprintln(os.Stderr)
			}
		},
	})
	if err != nil {
		return err
	}
	s := p.Stats
	fmt.Printf("结果文件　：%s\n", csvPath)
	fmt.Printf("结果格式　：%s\n", p.Format.Label())
	if p.ToolName != "" {
		fmt.Printf("生成工具　：%s %s\n", p.ToolName, p.ToolVersion)
	}
	fmt.Printf("源码目录　：%s\n", orDash(srcRoot))
	fmt.Printf("漏洞总数　：%d\n", s.Total)
	fmt.Printf("规则种类　：%d\n", s.RuleKinds)
	fmt.Printf("涉及文件　：%d\n", s.FileKinds)
	fmt.Printf("已定位源码：%d/%d\n", s.Resolved, s.Total)
	fmt.Printf("待翻译条目：%d\n", s.Pending)
	st := engine.Stats()
	fmt.Printf("翻译命中　：内置词典 %d，翻译库 %d，大模型 %d\n", st.Builtin, st.Store, st.LLM)

	if outPath != "" {
		if err := p.ExportZH(outPath, nil); err != nil {
			return err
		}
		fmt.Printf("已导出中文结果（带中文表头）：%s\n", outPath)
	}
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "（未指定）"
	}
	return s
}
