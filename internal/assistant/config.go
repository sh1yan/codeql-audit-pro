// Package assistant 是 CodeQL Assistant 的后端内核。
//
// 它保留了原 Fyne 版本的全部能力（配置快照、运行历史、数据库创建、
// 查询分析、实时输出、进程生命周期管理、日志落盘），但不再依赖任何
// GUI 框架。CodeQLAuditPro 通过 Wails 绑定把这里的能力暴露给 React 界面，
// 因此旧的 Fyne UI 被完全替换，而功能保持不变。
package assistant

import (
	"encoding/json"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Config 保存一次 CodeQL 运行所需的全部参数。
// JSON 字段与旧版 ~/.codeql-assistant 配置保持兼容；首次启动会把旧的
// last_config.json、history.json 和 profiles/ 自动迁移到新目录。
type Config struct {
	Name         string `json:"name,omitempty"`
	BinaryPath   string `json:"binary_path"`
	QueryPath    string `json:"query_path"`
	DatabasePath string `json:"database_path"`
	SourceRoot   string `json:"source_root"`
	Language     string `json:"language"`
	OutputFormat string `json:"output_format"`
	OutputFile   string `json:"output_file"`
	RamLimit     int    `json:"ram_limit"`
	Threads      int    `json:"threads"`
	ExtraArgs    string `json:"extra_args"`
}

// 助手只支持两种分析结果输出格式。
const (
	// FormatSARIF 是 CodeQL 标准 SARIF v2.1.0 输出，供 GitHub Code Scanning、CI 等工具使用。
	FormatSARIF = "sarifv2.1.0"
	// FormatCSV 是审计工作台读取的 CSV 输出。
	FormatCSV = "csv"
)

// DefaultConfig 返回一份包含合理默认值的配置。
//
// RamLimit / Threads 默认为 4096 与 4；把其中任意一项改成 0 表示不向
// CodeQL 传递对应的 --ram / --threads 参数，由 CodeQL 自己决定。
func DefaultConfig() Config {
	return Config{
		Language:     "java",
		OutputFormat: FormatSARIF,
		RamLimit:     4096,
		Threads:      4,
	}
}

// SupportedLanguages 返回 CodeQL 可分析的编程语言。
func SupportedLanguages() []string {
	return []string{
		"java", "python", "javascript", "typescript",
		"cpp", "csharp", "go", "ruby", "swift", "kotlin",
	}
}

// SupportedFormats 返回分析结果支持的输出格式。
//
// 助手只导出 SARIF v2.1.0 与 CSV，不再提供 sarif-latest、json、bqrs、
// graphs、dgml 等其他格式。
func SupportedFormats() []string {
	return []string{FormatSARIF, FormatCSV}
}

// NormalizeOutputFormat 把历史配置里可能残留的输出格式收敛为受支持的格式。
// 空值或不受支持的格式统一回落到 SARIF v2.1.0。
func NormalizeOutputFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case FormatSARIF, FormatCSV:
		return format
	default:
		return FormatSARIF
	}
}

// SaveToFile 将配置持久化到 JSON 文件。
func (c Config) SaveToFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadConfigFromFile 从 JSON 文件读取配置。
func LoadConfigFromFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultConfig(), err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), err
	}
	return cfg, nil
}

const (
	// EnvHome 是覆盖 CodeQLAuditPro 根目录的环境变量名。
	// 必须与 internal/config.EnvHome 保持一致。
	EnvHome = "CODEQLAUDITPRO_HOME"
	// AppDirName 是所有平台统一的根目录名。
	// 必须与 internal/config.AppDirName 保持一致。
	AppDirName = ".CodeQLAuditPro"
	// legacyDirName 是原 CodeQL Assistant 的数据目录名，仅用于迁移。
	legacyDirName = ".codeql-assistant"
)

var migrateMu sync.Mutex

// RootDir 返回 CodeQLAuditPro 的根数据目录。
//
// 默认在所有操作系统上都使用 ~/.CodeQLAuditPro；设置 CODEQLAUDITPRO_HOME
// 可以切换到便携目录。
func RootDir() string {
	if h := strings.TrimSpace(os.Getenv(EnvHome)); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		home = os.Getenv("HOME")
	}
	if strings.TrimSpace(home) == "" {
		return filepath.Join(os.TempDir(), AppDirName)
	}
	return filepath.Join(home, AppDirName)
}

// DataDir 是助手功能的持久化目录：~/.CodeQLAuditPro/assistant。
func DataDir() string {
	return filepath.Join(RootDir(), "assistant")
}

// LegacyDataDir 返回原 CodeQL Assistant 的数据目录（~/.codeql-assistant）。
func LegacyDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		home = os.Getenv("HOME")
	}
	if strings.TrimSpace(home) == "" {
		return legacyDirName
	}
	return filepath.Join(home, legacyDirName)
}

// EnsureDataDir 创建助手数据目录，并在首次使用时迁移旧版数据。
func EnsureDataDir() string {
	dir := DataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return dir
	}
	migrateLegacyData(dir)
	return dir
}

func migrateLegacyData(newDir string) {
	migrateMu.Lock()
	defer migrateMu.Unlock()

	// 新目录已有任何助手数据就不再迁移，避免覆盖用户后来的修改。
	for _, name := range []string{"last_config.json", "history.json", "profiles"} {
		if pathExists(filepath.Join(newDir, name)) {
			return
		}
	}

	oldDir := LegacyDataDir()
	if filepath.Clean(oldDir) == filepath.Clean(newDir) || !dirExists(oldDir) {
		return
	}

	for _, name := range []string{"last_config.json", "history.json"} {
		_ = copyFileIfMissing(filepath.Join(oldDir, name), filepath.Join(newDir, name))
	}

	oldProfiles := filepath.Join(oldDir, "profiles")
	entries, err := os.ReadDir(oldProfiles)
	if err != nil {
		return
	}
	newProfiles := filepath.Join(newDir, "profiles")
	if err := os.MkdirAll(newProfiles, 0o755); err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		_ = copyFileIfMissing(
			filepath.Join(oldProfiles, entry.Name()),
			filepath.Join(newProfiles, entry.Name()),
		)
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func copyFileIfMissing(src, dst string) error {
	if pathExists(dst) {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}

// ConfigDir 返回配置快照目录。
func ConfigDir() string {
	return filepath.Join(DataDir(), "profiles")
}

// LastConfigPath 返回上次使用配置的路径。
func LastConfigPath() string {
	return filepath.Join(DataDir(), "last_config.json")
}

// LogDir 返回完整运行日志目录。
func LogDir() string {
	return filepath.Join(DataDir(), "logs")
}

// ProfilePath 返回指定名称配置文件的完整路径。
func ProfilePath(name string) string {
	return filepath.Join(ConfigDir(), name+".json")
}

// ListProfiles 返回可用的配置快照名称（按字母排序）。
func ListProfiles() []string {
	entries, err := os.ReadDir(ConfigDir())
	if err != nil {
		return []string{}
	}
	profiles := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			profiles = append(profiles, e.Name()[:len(e.Name())-5])
		}
	}
	sort.Strings(profiles)
	return profiles
}

// DeleteProfile 删除指定名称的配置快照。
func DeleteProfile(name string) error {
	return os.Remove(ProfilePath(name))
}

// GetRandomQuote 返回一条安全研究者的随机语录。
func GetRandomQuote() string {
	quotes := []string{
		"每一行代码背后，都藏着值得被发现的真相。",
		"漏洞不会自己说话，但查询可以替它开口。",
		"开源的代码是透明的，但安全需要有人去看见。",
		"你查的每一行，都是社区信任的一分。",
		"别人看代码找功能，你看代码找谎言。",
		"那个你没报出来的漏洞，其实你也救了很多人。",
		"False positive 第一百次，但第一百零一次你还是会点进去。",
		"别人眼里的能跑就行，是你眼里的必须查清楚。",
		"没有你的名字，但有你拦下来的漏洞。",
		"看到用户输入，手指已经比脑子先动了。",
		"第十个误报之后，第十一个你还是点了进去。",
		"三点十七分，咖啡凉了，但调用链终于通了。",
		"每次交报告前，你都比开发多问自己一遍：还有吗？",
		"最漂亮的一行代码，是你让它没机会被利用。",
		"整个仓库，只有你读过这一段的每一行。",
		"你和代码之间，有一种开发不懂的相处方式。",
		"你查的不是代码，是很多人睡不着的理由。",
		"鼠标悬停在 eval 上的时候，你的心跳漏了一拍。",
		"明明已经扫过三遍，睡前还是忍不住再翻一遍。",
		"开发说没问题，你说我再看看。",
		"一个漏洞从发现到修复，中间隔了三个月和十七封邮件。",
		"逛 GitHub 像逛菜市场，顺手就审计起来了。",
		"那个你随手修掉的边界条件，后来真的有人触发了。",
		"家人问你天天对着代码找什么，你说找麻烦。",
		"明明可以报低危，但你就是想知道能不能再深一层。",
		"每次提交报告前，都要把 PoC 再跑一遍，哪怕已经跑了二十遍。",
		"你不认识写代码的人，但你可能比他自己更了解他的程序。",
		"如果上次没多看那一眼，现在上新闻的可能就是这个项目。",
		"你的工作没有发布会，但阻止了很多发布会。",
		"你拦下来的每一个漏洞，都是某个普通人平静的一天。",
		"变量名从 temp 改成 tmp，你都要停下来多看两眼。",
		"还没走到 sink，你已经知道哪里会漏。",
		"开发说这不可能被触发，你花了两小时写了个 PoC。",
		"看到登录框就手痒，看到文件上传就精神。",
		"你的简历上写着发现漏洞，没写的是那些白跑的三百个小时。",
		"别人问这个严重吗，你说让我再确认一下，其实你已经确认了五遍。",
		"开发群里有人说哪个傻子写的这代码，你默默关掉窗口，深藏功与名。",
		"眼睛花了，脑子慢了，但手指还在翻下一页。",
		"你可以不报，但你知道如果你不报，可能没人会报。",
		"朋友让你帮忙看个网站，你顺手就按了 F12。",
		"你和漏洞之间，隔着一万行代码和一杯凉透的咖啡。",
		"最好的结果不是发现漏洞，是发现这里真的没有漏洞。",
		"你读过很多人的代码，但很少有人读过你的。",
		"每一行你认真看过的代码，都在某个地方保护着某个你不认识的人。",
	}

	seen := make(map[string]bool, len(quotes))
	unique := make([]string, 0, len(quotes))
	for _, q := range quotes {
		if !seen[q] {
			seen[q] = true
			unique = append(unique, q)
		}
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return unique[r.Intn(len(unique))]
}
