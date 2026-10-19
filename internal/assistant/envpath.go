package assistant

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	envMarkerBegin = "__CODEQL_AUDIT_PRO_ENV_BEGIN__"
	envMarkerEnd   = "__CODEQL_AUDIT_PRO_ENV_END__"
	envLoadTimeout = 6 * time.Second
)

// relevantEnvKeys 是打包后的 GUI 进程里可能缺失、但 CodeQL 工具链需要的环境变量。
var relevantEnvKeys = []string{
	"CODEQL_HOME",
	"JAVA_HOME",
	"NVM_DIR",
	"PNPM_HOME",
	"GOROOT",
	"GOPATH",
	"GVM_ROOT",
	"SDKMAN_DIR",
	"MAVEN_OPTS",
	"GRADLE_OPTS",
	"LANG",
}

// InitUserEnv 从用户登录 shell 读取环境变量并合并到当前进程。
//
// macOS 上通过 Finder/LaunchServices 启动的 GUI 应用不会继承终端里的 PATH，
// 这会导致 CodeQL 及其子进程（java、mvn、gradle、node 等）找不到命令。
// 该函数在 Wails 启动前调用，行为与原 CodeQL Assistant 完全一致。
func InitUserEnv() {
	if runtime.GOOS == "windows" {
		return
	}

	shellPath := ""
	if shellEnv := loadShellEnv(); shellEnv != nil {
		shellPath = shellEnv["PATH"]
		for _, key := range relevantEnvKeys {
			if value, ok := shellEnv[key]; ok && value != "" {
				_ = os.Setenv(key, value)
			}
		}
	}
	_ = os.Setenv("PATH", buildPath(shellPath, os.Getenv("PATH")))
}

func loadShellEnv() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), envLoadTimeout)
	defer cancel()

	command := fmt.Sprintf("printf '%%s' '%s'; env -0; printf '%%s' '%s'", envMarkerBegin, envMarkerEnd)
	for _, shell := range candidateShells() {
		cmd := exec.CommandContext(ctx, shell, "-lic", command)
		out, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}
		if env := parseEnvSnapshot(out); len(env) > 0 {
			return env
		}
	}
	return nil
}

func candidateShells() []string {
	var shells []string
	seen := make(map[string]bool)
	add := func(shell string) {
		shell = strings.TrimSpace(shell)
		if shell == "" || seen[shell] {
			return
		}
		seen[shell] = true
		shells = append(shells, shell)
	}

	add(os.Getenv("SHELL"))
	add("/bin/zsh")
	add("/usr/bin/zsh")
	add("/bin/bash")
	add("/usr/bin/bash")
	return shells
}

func parseEnvSnapshot(out []byte) map[string]string {
	start := bytes.Index(out, []byte(envMarkerBegin))
	end := bytes.Index(out, []byte(envMarkerEnd))
	if start < 0 || end <= start {
		return nil
	}

	payload := out[start+len(envMarkerBegin) : end]
	env := make(map[string]string)
	entries := bytes.Split(payload, []byte{0})
	if len(entries) == 1 {
		for _, line := range bytes.Split(payload, []byte{'\n'}) {
			addEnvEntry(env, line)
		}
	} else {
		for _, entry := range entries {
			addEnvEntry(env, entry)
		}
	}
	if len(env) == 0 {
		return nil
	}
	return env
}

func addEnvEntry(env map[string]string, entry []byte) {
	eq := bytes.IndexByte(entry, '=')
	if eq <= 0 {
		return
	}
	env[string(entry[:eq])] = string(entry[eq+1:])
}

func buildPath(shellPath, currentPath string) string {
	parts := append(splitPath(shellPath), splitPath(currentPath)...)
	parts = append(parts, knownToolDirs()...)
	return strings.Join(dedupePath(parts), string(os.PathListSeparator))
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, string(os.PathListSeparator))
}

func dedupePath(parts []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	return out
}

func knownToolDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}

	dirs := []string{
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/usr/local/bin",
		"/usr/local/sbin",
		"/usr/bin",
		"/bin",
		"/usr/sbin",
		"/sbin",
		"/usr/local/go/bin",
	}
	if home != "" {
		dirs = append(dirs,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, ".jenv", "bin"),
			filepath.Join(home, ".jenv", "shims"),
			filepath.Join(home, "Library", "pnpm", "bin"),
			filepath.Join(home, ".gvm", "bin"),
			filepath.Join(home, ".volta", "bin"),
			filepath.Join(home, ".fnm"),
			filepath.Join(home, ".asdf", "bin"),
			filepath.Join(home, ".asdf", "shims"),
			filepath.Join(home, ".bun", "bin"),
			filepath.Join(home, ".yarn", "bin"),
			filepath.Join(home, ".config", "yarn", "global", "node_modules", ".bin"),
			filepath.Join(home, "go", "bin"),
			filepath.Join(home, ".cargo", "bin"),
		)
		dirs = append(dirs, nvmNodeBins(home)...)
		dirs = append(dirs, sdkmanCandidateBins(home)...)
	}

	seen := make(map[string]bool)
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = filepath.Clean(dir)
		if dir == "." || seen[dir] {
			continue
		}
		seen[dir] = true
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			out = append(out, dir)
		}
	}
	return out
}

func nvmNodeBins(home string) []string {
	matches, err := filepath.Glob(filepath.Join(home, ".nvm", "versions", "node", "*", "bin"))
	if err != nil {
		return nil
	}
	sort.Slice(matches, func(i, j int) bool {
		return compareNodeVersion(matches[i], matches[j]) > 0
	})
	return matches
}

func compareNodeVersion(a, b string) int {
	av := versionParts(filepath.Base(a))
	bv := versionParts(filepath.Base(b))
	for i := range av {
		if av[i] != bv[i] {
			if av[i] > bv[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func versionParts(name string) [3]int {
	var parts [3]int
	raw := strings.TrimPrefix(name, "v")
	fields := strings.Split(raw, ".")
	for i := 0; i < len(fields) && i < len(parts); i++ {
		if n, err := strconv.Atoi(fields[i]); err == nil {
			parts[i] = n
		}
	}
	return parts
}

func sdkmanCandidateBins(home string) []string {
	matches, err := filepath.Glob(filepath.Join(home, ".sdkman", "candidates", "*", "current", "bin"))
	if err != nil {
		return nil
	}
	return matches
}
