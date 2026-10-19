package assistant

import (
	"regexp"
	"runtime"
	"strings"
)

var safeShellArg = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// CommandPreview 是「命令示例」里展示的两条可直接复制命令。
type CommandPreview struct {
	Create  string `json:"create"`
	Analyze string `json:"analyze"`
}

// PreviewCommands 根据当前配置生成可复制的 CodeQL 命令。
// 命令与运行器实际使用的参数完全一致，因此可以脱离 CodeQLAuditPro
// 直接粘贴到终端执行。
func PreviewCommands(config Config) CommandPreview {
	binary := strings.TrimSpace(config.BinaryPath)
	if binary == "" {
		binary = "codeql"
	}

	db := config.DatabasePath
	if strings.TrimSpace(db) == "" {
		db = "<数据库目录>"
	}
	src := config.SourceRoot
	if strings.TrimSpace(src) == "" {
		src = "<源码根目录>"
	}
	query := strings.TrimSpace(config.QueryPath)
	lang := CLILanguage(config.Language)
	if lang == "" {
		lang = "java"
	}

	// 线程数 / 内存限额为 0 时不拼接对应参数，交给 CodeQL 自己决定。
	createArgs := []string{"database", "create", db, "--language=" + lang}
	createArgs = append(createArgs, "--source-root="+src)
	if config.Threads > 0 {
		createArgs = append(createArgs, "--threads="+itoa(config.Threads))
	}
	if config.RamLimit > 0 {
		createArgs = append(createArgs, "--ram="+itoa(config.RamLimit))
	}
	createArgs = append(createArgs, "--overwrite")
	createArgs = append(createArgs, SplitArgs(config.ExtraArgs)...)

	analyzeArgs := []string{"database", "analyze", db}
	if query != "" {
		analyzeArgs = append(analyzeArgs, query)
	}
	if config.OutputFormat != "" {
		analyzeArgs = append(analyzeArgs, "--format="+config.OutputFormat)
	}
	output := config.OutputFile
	if strings.TrimSpace(output) == "" {
		output = "<输出文件>"
	}
	analyzeArgs = append(analyzeArgs, "--output="+output)
	if config.Threads > 0 {
		analyzeArgs = append(analyzeArgs, "--threads="+itoa(config.Threads))
	}
	if config.RamLimit > 0 {
		analyzeArgs = append(analyzeArgs, "--ram="+itoa(config.RamLimit))
	}
	analyzeArgs = append(analyzeArgs, SplitArgs(config.ExtraArgs)...)

	return CommandPreview{
		Create:  QuoteCommand(binary, createArgs),
		Analyze: QuoteCommand(binary, analyzeArgs),
	}
}

// QuoteCommand 把可执行文件和参数拼成一行可以粘贴到终端的命令。
func QuoteCommand(binary string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteArg(binary))
	for _, arg := range args {
		parts = append(parts, quoteArg(arg))
	}
	return strings.Join(parts, " ")
}

func quoteArg(arg string) string {
	if arg == "" {
		if runtime.GOOS == "windows" {
			return `""`
		}
		return "''"
	}
	if runtime.GOOS == "windows" {
		if safeShellArg.MatchString(arg) {
			return arg
		}
		return `"` + strings.ReplaceAll(arg, `"`, `\"`) + `"`
	}
	if safeShellArg.MatchString(arg) {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
