// Command CodeQLAuditPro 是 CodeQL 数据库构建、查询分析与结果审计的一体化工具。
//
// 用法：
//
//	CodeQLAuditPro                        # 打开图形界面（首页选择工作流）
//	CodeQLAuditPro -csv pyhackme.sarif    # 打开审计工作台并直接载入结果文件
//	CodeQLAuditPro -cli -csv x.csv -src /path/to/src -out x_中文.csv
//	                                      # 命令行模式：解析 + 中文化 + 导出
//
// -csv 与 -results 等价，均接受 CSV 或 SARIF v2.1.0。
package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"codeql-audit-pro/internal/assistant"
	"codeql-audit-pro/internal/config"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	var (
		csvPath     = flag.String("csv", "", "CodeQL 结果文件路径（CSV 或 SARIF v2.1.0）")
		resultsPath = flag.String("results", "", "同 -csv：CodeQL 结果文件路径（CSV 或 SARIF v2.1.0）")
		srcRoot     = flag.String("src", "", "被审计的源码目录")
		outPath     = flag.String("out", "", "命令行模式下导出的中文 CSV 路径")
		cli         = flag.Bool("cli", false, "使用命令行模式（不启动图形界面）")
		fontsFlag   = flag.Bool("fonts", false, "打印 WebView 字体说明（Wails 版由系统渲染中文字体）")
		showVersion = flag.Bool("version", false, "显示版本信息")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("CodeQLAuditPro v%s (Wails)\n", appVersion)
		return
	}
	if strings.TrimSpace(*resultsPath) != "" {
		*csvPath = *resultsPath
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "读取配置失败：", err)
		os.Exit(1)
	}
	if *csvPath != "" {
		cfg.LastProject = *csvPath
		st := cfg.ProjectFor(*csvPath, *srcRoot)
		if *srcRoot != "" {
			st.SrcRoot = *srcRoot
		}
		_ = cfg.Save()
	}

	if *fontsFlag {
		fmt.Println("Wails 版界面使用系统 WebView 渲染，中文字体由操作系统负责（macOS 的 PingFang SC、Windows 的微软雅黑等）。")
		fmt.Println("如仍有字体问题，可在 frontend/src/index.css 里调整 --font-sans / --font-mono。")
		return
	}

	if *cli {
		if err := runCLI(cfg, *csvPath, *srcRoot, *outPath); err != nil {
			fmt.Fprintln(os.Stderr, "错误：", err)
			os.Exit(1)
		}
		return
	}

	// 打包后通过 Finder/LaunchServices 启动时不会继承终端 PATH，
	// 启动界面之前先补齐 CodeQL 及其构建工具需要的环境。
	assistant.InitUserEnv()

	app := NewApp(cfg, *csvPath, *srcRoot)

	width, height := int(cfg.WindowWidth), int(cfg.WindowHeight)
	if width < 1100 {
		width = 1480
	}
	if height < 720 {
		height = 930
	}

	err = wails.Run(&options.App{
		Title:     "CodeQLAuditPro · CodeQL 数据库、查询与审计",
		Width:     width,
		Height:    height,
		MinWidth:  1120,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 11, G: 15, B: 25, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "CodeQLAuditPro",
				Message: "v" + appVersion + "\nCodeQL 数据库构建、查询分析与结果审计的一体化工作台。\n支持 SARIF v2.1.0 与 CSV。",
				Icon:    appIcon,
			},
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "启动界面失败：", err)
		os.Exit(1)
	}
}
