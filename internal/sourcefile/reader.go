// Package sourcefile 负责读取源码文件并做编码探测，避免中文注释乱码。
package sourcefile

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// MaxFileSize 是允许读取的单个文件大小上限（64MB）。
const MaxFileSize = 64 << 20

// Detect 探测字节流编码并解码为 UTF-8，同时返回探测到的编码名称。
func Detect(raw []byte) (string, string) {
	if len(raw) == 0 {
		return "", "UTF-8"
	}
	// BOM 优先
	switch {
	case bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}):
		return string(raw[3:]), "UTF-8 (BOM)"
	case bytes.HasPrefix(raw, []byte{0xFF, 0xFE}):
		if s, err := decode(unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM), raw[2:]); err == nil {
			return s, "UTF-16 LE"
		}
	case bytes.HasPrefix(raw, []byte{0xFE, 0xFF}):
		if s, err := decode(unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM), raw[2:]); err == nil {
			return s, "UTF-16 BE"
		}
	}
	if utf8.Valid(raw) {
		return string(raw), "UTF-8"
	}
	// 非合法 UTF-8：优先按 GBK 解码，若解出汉字则认定为国标编码
	if s, err := decode(simplifiedchinese.GBK, raw); err == nil && hasHan(s) {
		return s, "GBK/GB18030"
	}
	if s, err := decode(traditionalchinese.Big5, raw); err == nil && hasHan(s) {
		return s, "Big5"
	}
	// 最后的兜底：按字节直转，保证不丢内容
	return string([]rune(string(toRunes(raw)))), "未知(Latin-1 兜底)"
}

func decode(enc encoding.Encoding, raw []byte) (string, error) {
	out, _, err := transform.Bytes(enc.NewDecoder(), raw)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// toRunes 把无法解码的字节按 Latin-1 映射为 rune。
func toRunes(raw []byte) []rune {
	rs := make([]rune, len(raw))
	for i, b := range raw {
		rs[i] = rune(b)
	}
	return rs
}

// hasHan 判断文本中是否含有汉字。
func hasHan(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

// ReadFile 读取文件并返回 UTF-8 文本与编码名。
func ReadFile(path string) (string, string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", "", err
	}
	if st.IsDir() {
		return "", "", fmt.Errorf("%s 是目录，不是文件", path)
	}
	if st.Size() > MaxFileSize {
		return "", "", fmt.Errorf("文件过大（%.1f MB），已跳过", float64(st.Size())/(1<<20))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	text, encName := Detect(raw)
	return text, encName, nil
}

// ReadLines 读取文件并按行返回（去掉行尾 \r）。
func ReadLines(path string) ([]string, string, error) {
	text, encName, err := ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return SplitLines(text), encName, nil
}

// SplitLines 按行切分文本，兼容 \r\n 与 \r。
func SplitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	return lines
}
