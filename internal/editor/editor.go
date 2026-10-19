// Package editor 负责「用外部编辑器打开源码」。
//
// 与界面层解耦：既服务于 Wails 前端，也可以被命令行/测试直接调用。
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"codeql-audit-pro/internal/config"
)

// DefaultArgs 是 VS Code 家族的默认参数模板。
const DefaultArgs = "-g {file}:{line}:{col}"

// Candidates 返回各平台自动探测编辑器的候选路径/命令（按优先级）。
func Candidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
			"/Applications/Visual Studio Code - Insiders.app/Contents/Resources/app/bin/code",
			"/Applications/Cursor.app/Contents/Resources/app/bin/cursor",
			"/Applications/Windsurf.app/Contents/Resources/app/bin/windsurf",
			"/opt/homebrew/bin/code", "/usr/local/bin/code",
			"/Applications/Sublime Text.app/Contents/SharedSupport/bin/subl",
			"/Applications/Zed.app/Contents/MacOS/cli",
			"code", "cursor", "subl", "zed",
		}
	case "windows":
		return []string{"code.cmd", "code", "cursor.cmd", "cursor", "subl.exe"}
	default:
		return []string{"/usr/bin/code", "/usr/local/bin/code", "/snap/bin/code", "/usr/bin/cursor", "code", "cursor", "subl"}
	}
}

// Detect 自动探测一个可用的编辑器；找不到时返回空串。
func Detect() string {
	for _, c := range Candidates() {
		if strings.ContainsAny(c, "/\\") {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c
			}
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// RenderArgs 把参数模板里的 {file} {line} {col} 换成实际值。
func RenderArgs(tmpl, file string, line, col int) []string {
	if strings.TrimSpace(tmpl) == "" {
		tmpl = DefaultArgs
	}
	if line <= 0 {
		line = 1
	}
	if col <= 0 {
		col = 1
	}
	repl := strings.NewReplacer(
		"{file}", file,
		"{path}", file,
		"{line}", fmt.Sprintf("%d", line),
		"{col}", fmt.Sprintf("%d", col),
	)
	rendered := repl.Replace(tmpl)
	return strings.Fields(rendered)
}

// Command 组装「用编辑器打开」要执行的命令。
//
// 支持三种配置：
//  1. 可执行文件路径/命令名（例如 /usr/local/bin/code）→ 直接执行 + 参数模板；
//  2. macOS 的 .app 包路径 → 走 open -a，用 --args 传参数；
//  3. 留空 → 自动探测常见编辑器。
func Command(cfg config.EditorSettings, file string, line, col int) (*exec.Cmd, string, error) {
	cmdPath := strings.TrimSpace(cfg.Cmd)
	detected := false
	if cmdPath == "" {
		cmdPath = Detect()
		detected = true
	}
	if cmdPath == "" {
		return nil, "", fmt.Errorf("没有找到可用的编辑器。请在「设置 → 用编辑器打开」里填写编辑器的可执行文件路径")
	}
	isApp := strings.HasSuffix(strings.ToLower(cmdPath), ".app")
	if strings.ContainsAny(cmdPath, "/\\") {
		st, err := os.Stat(cmdPath)
		if err != nil || (!isApp && st.IsDir()) {
			return nil, "", fmt.Errorf("配置的编辑器不存在或不是可执行文件：%s", cmdPath)
		}
	}
	args := RenderArgs(cfg.Args, file, line, col)

	if isApp {
		openArgs := []string{"-a", cmdPath}
		if len(args) > 0 {
			openArgs = append(openArgs, "--args")
			openArgs = append(openArgs, args...)
		}
		return exec.Command("open", openArgs...), "open -a " + cmdPath, nil
	}
	if runtime.GOOS == "windows" && !strings.ContainsAny(cmdPath, "/\\") {
		return exec.Command("cmd", append([]string{"/c", cmdPath}, args...)...), cmdPath, nil
	}
	desc := cmdPath
	if detected {
		desc = cmdPath + "（自动探测）"
	}
	return exec.Command(cmdPath, args...), desc + " " + strings.Join(args, " "), nil
}

// Open 在外部编辑器中打开文件并定位到行列。
func Open(cfg config.EditorSettings, file string, line, col int) (string, error) {
	cmd, desc, err := Command(cfg, file, line, col)
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return desc, fmt.Errorf("未能启动编辑器：%w（命令：%s）", err, desc)
	}
	return desc, nil
}

// Reveal 在系统文件管理器中显示文件。
func Reveal(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("路径为空")
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	case "windows":
		return exec.Command("explorer", "/select,", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
