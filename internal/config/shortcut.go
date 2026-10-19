package config

import (
	"fmt"
	"runtime"
	"strings"
)

// 本文件实现「可自定义的键盘快捷键」的解析与格式化。
//
// 解析放在 config 而不是界面层：它是纯字符串处理，可以单独做单元测试；
// 前端只负责把解析结果映射成键盘事件（KeyboardEvent.key + 修饰键）。
//
// 存储格式始终是「修饰键 + '+' + 主键」，例如 Cmd+F、Ctrl+Shift+G、Alt+F3。
// 解析后再由 String() 统一成规范写法，因此用户写 "shift+cmd+g" 也会被归一成
// "Cmd+Shift+G" 存进配置，界面上显示同一份文本。

// ShortcutSpec 描述一个键盘快捷键：若干修饰键 + 唯一的主键。
//
// Ctrl 与 Cmd 刻意分开：macOS 上 Ctrl 是 Control 键、Cmd 是 Command 键，
// 二者完全不同；其它平台上 Cmd 对应 Super（Windows 键）。
type ShortcutSpec struct {
	// Key 是规范化后的主键：字母 A-Z、数字 0-9、功能键 F1-F12，或符号键的
	// 字面量（如 "/"、"\\"、"="），与前端 KeyboardEvent.key 一致。
	Key string
	// Ctrl 表示 Control 键。
	Ctrl bool
	// Cmd 表示 Command（macOS）/ Super（其它平台）键。
	Cmd bool
	// Alt 表示 Alt（macOS 上也叫 Option）键。
	Alt bool
	// Shift 表示 Shift 键。
	Shift bool
}

// 各平台的默认「在当前代码文件中查找」快捷键：
// macOS 用 Cmd+F，其它平台用 Ctrl+F，这也是绝大多数编辑器/浏览器的习惯键位。
const (
	// DefaultSearchShortcutDarwin 是 macOS 的默认查找快捷键。
	DefaultSearchShortcutDarwin = "Cmd+F"
	// DefaultSearchShortcutOther 是 Windows/Linux 的默认查找快捷键。
	DefaultSearchShortcutOther = "Ctrl+F"
)

// DefaultSearchShortcut 返回当前平台的默认查找快捷键。
func DefaultSearchShortcut() string {
	if runtime.GOOS == "darwin" {
		return DefaultSearchShortcutDarwin
	}
	return DefaultSearchShortcutOther
}

// symbolKeys 把用户可能写的符号键名映射到前端 KeyboardEvent.key 使用的字符。
//
// 只收录有明确字符表示的符号键（/ \ - = . , ; [ ] 空格 +）。
// 箭头、Tab 这类键容易被系统或输入框抢先处理，不予支持。
var symbolKeys = map[string]string{
	"/":  "/",
	"\\": "\\",
	"-":  "-",
	"=":  "=",
	".":  ".",
	",":  ",",
	";":  ";",
	"[":  "[",
	"]":  "]",
}

// symbolAliases 是符号键的英文别名（用户不必记住符号本身怎么写）。
var symbolAliases = map[string]string{
	"SLASH":        "/",
	"BACKSLASH":    "\\",
	"MINUS":        "-",
	"DASH":         "-",
	"HYPHEN":       "-",
	"EQUAL":        "=",
	"EQUALS":       "=",
	"PERIOD":       ".",
	"DOT":          ".",
	"COMMA":        ",",
	"SEMICOLON":    ";",
	"LEFTBRACKET":  "[",
	"RIGHTBRACKET": "]",
	"SPACE":        "Space",
}

// ParseShortcut 解析快捷键描述。
//
// 接受的形式（大小写不敏感、修饰键顺序任意）：
//
//	Cmd+F  Command+F  Super+F  Meta+F  Win+F
//	Ctrl+F  Control+F
//	Alt+F3  Option+F3
//	Ctrl+Shift+G
func ParseShortcut(s string) (ShortcutSpec, error) {
	var spec ShortcutSpec
	raw := strings.TrimSpace(s)
	if raw == "" {
		return spec, fmt.Errorf("快捷键不能为空")
	}
	key := ""
	for _, part := range strings.Split(raw, "+") {
		token := strings.TrimSpace(part)
		if token == "" {
			// 允许 "Ctrl + F" 这种写法：切分后会有空片段。
			continue
		}
		switch strings.ToUpper(token) {
		case "CTRL", "CONTROL":
			if spec.Ctrl {
				return spec, fmt.Errorf("修饰键 Ctrl 重复：%s", raw)
			}
			spec.Ctrl = true
			continue
		case "CMD", "COMMAND", "SUPER", "META", "WIN", "WINDOWS":
			if spec.Cmd {
				return spec, fmt.Errorf("修饰键 Cmd 重复：%s", raw)
			}
			spec.Cmd = true
			continue
		case "ALT", "OPTION", "OPT":
			if spec.Alt {
				return spec, fmt.Errorf("修饰键 Alt 重复：%s", raw)
			}
			spec.Alt = true
			continue
		case "SHIFT":
			if spec.Shift {
				return spec, fmt.Errorf("修饰键 Shift 重复：%s", raw)
			}
			spec.Shift = true
			continue
		}
		normalized, ok := normalizeShortcutKey(token)
		if !ok {
			return spec, fmt.Errorf("无法识别的键 %q（支持字母 A-Z、数字 0-9、F1-F12 与常见符号键）", token)
		}
		if key != "" {
			return spec, fmt.Errorf("只能指定一个主键，收到 %q 与 %q", key, token)
		}
		key = normalized
	}
	if key == "" {
		return spec, fmt.Errorf("快捷键缺少主键，例如 Cmd+F")
	}
	if !spec.Ctrl && !spec.Cmd && !spec.Alt && !spec.Shift {
		return spec, fmt.Errorf("快捷键至少需要一个修饰键（Cmd / Ctrl / Alt / Shift），例如 Cmd+F")
	}
	spec.Key = key
	return spec, nil
}

// normalizeShortcutKey 把一个主键写法规范化。
func normalizeShortcutKey(token string) (string, bool) {
	t := strings.TrimSpace(token)
	upper := strings.ToUpper(t)
	if v, ok := symbolAliases[upper]; ok {
		return v, true
	}
	if v, ok := symbolKeys[t]; ok {
		return v, true
	}
	if len([]rune(upper)) == 1 {
		r := []rune(upper)[0]
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return string(r), true
		}
	}
	if isFunctionKey(upper) {
		return upper, true
	}
	return "", false
}

// isFunctionKey 判断是否是 F1-F12。
func isFunctionKey(upper string) bool {
	if len(upper) < 2 || upper[0] != 'F' {
		return false
	}
	n := 0
	for _, r := range upper[1:] {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
	}
	return n >= 1 && n <= 12
}

// String 返回规范的快捷键写法（修饰键顺序固定：Ctrl、Cmd、Alt、Shift）。
func (s ShortcutSpec) String() string {
	parts := make([]string, 0, 4)
	if s.Ctrl {
		parts = append(parts, "Ctrl")
	}
	if s.Cmd {
		parts = append(parts, "Cmd")
	}
	if s.Alt {
		parts = append(parts, "Alt")
	}
	if s.Shift {
		parts = append(parts, "Shift")
	}
	parts = append(parts, s.Key)
	return strings.Join(parts, "+")
}

// SearchShortcut 返回「在当前代码文件中查找」的快捷键（缺省按平台取 Cmd+F / Ctrl+F）。
//
// 配置里存的可能是用户手写的非规范写法（例如 "shift+cmd+g"），这里统一规范化后返回，
// 保证界面显示与真实注册的键位一致。
func (c *Config) SearchShortcut() string {
	if c == nil {
		return DefaultSearchShortcut()
	}
	raw := strings.TrimSpace(c.CodeSearchShortcut)
	if raw == "" {
		return DefaultSearchShortcut()
	}
	spec, err := ParseShortcut(raw)
	if err != nil {
		// 配置被手改坏了：不阻断启动，退回默认键位。
		return DefaultSearchShortcut()
	}
	return spec.String()
}

// SetSearchShortcut 写入查找快捷键（规范写法，空串表示恢复默认）。
func (c *Config) SetSearchShortcut(spec string) {
	raw := strings.TrimSpace(spec)
	if raw == "" {
		c.CodeSearchShortcut = ""
		return
	}
	if parsed, err := ParseShortcut(raw); err == nil {
		raw = parsed.String()
	}
	c.CodeSearchShortcut = raw
}
