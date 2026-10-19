package translate

import (
	"regexp"
	"strings"
)

// 占位符常量。规范化后的英文模板与中文模板共用同一套占位符。
const (
	PhAPI  = "{{API}}"  // API / 函数名，保持原样
	PhNum  = "{{N}}"    // 数字参数
	PhWord = "{{W}}"    // 标识符
	PhLink = "{{LINK}}" // 证据链位置引用
)

var (
	rePosition = regexp.MustCompile(`\[\s*position\s+\d+\s*\]`)
	reKeyword  = regexp.MustCompile(`\[\s*keyword\s+[A-Za-z_][A-Za-z0-9_.]*\s*\]`)
	reCallTo   = regexp.MustCompile(`^Call to (.+?)(\s+\[|\s+with\s)`)
	reLabelFor = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9]*)\s+for\s+(.+)$`)
	reNumber   = regexp.MustCompile(`\b\d+\b`)
)

// Canonical 保存规范化结果：模板键 + 被抽出的动态值。
type Canonical struct {
	Key  string
	Vals map[string][]string
}

// Canonicalize 把英文消息规范化为模板键，并收集动态值。
//
//	"Call to redis.Redis().setex [position 1] with untrusted data from {{LINK}}."
//	-> key:  "Call to {{API}} [position {{N}}] with untrusted data from {{LINK}}."
//	   vals: {{API}}: ["redis.Redis().setex"], {{N}}: ["1"]
func Canonicalize(en string) Canonical {
	key := strings.TrimSpace(en)
	vals := map[string][]string{}

	// 1) Call to <API> [ / with  ->  Call to {{API}} [
	if m := reCallTo.FindStringSubmatch(key); m != nil {
		vals[PhAPI] = append(vals[PhAPI], m[1])
		key = "Call to " + PhAPI + m[2] + key[len(m[0]):]
	}

	// 2) [position N]
	key = replaceCapture(rePosition, key, func(m string) string {
		nums := reNumber.FindAllString(m, -1)
		if len(nums) > 0 {
			vals[PhNum] = append(vals[PhNum], nums[0])
		}
		return "[position " + PhNum + "]"
	})

	// 3) [keyword word]
	key = replaceCapture(reKeyword, key, func(m string) string {
		inner := strings.TrimSpace(m[1 : len(m)-1])
		parts := strings.Fields(inner)
		if len(parts) >= 2 {
			vals[PhWord] = append(vals[PhWord], parts[1])
		}
		return "[keyword " + PhWord + "]"
	})

	return Canonical{Key: key, Vals: vals}
}

// CanonicalizeLabel 规范化证据链标签。
//
//	"ControlFlowNode for ImportMember" -> "ControlFlowNode for {{W}}"
func CanonicalizeLabel(label string) Canonical {
	s := strings.TrimSpace(label)
	vals := map[string][]string{}
	if m := reLabelFor.FindStringSubmatch(s); m != nil {
		vals[PhWord] = append(vals[PhWord], m[2])
		return Canonical{Key: m[1] + " for " + PhWord, Vals: vals}
	}
	return Canonical{Key: s, Vals: vals}
}

func replaceCapture(re *regexp.Regexp, s string, fn func(match string) string) string {
	idx := re.FindAllStringIndex(s, -1)
	if len(idx) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, loc := range idx {
		b.WriteString(s[last:loc[0]])
		b.WriteString(fn(s[loc[0]:loc[1]]))
		last = loc[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

// Fill 用动态值按顺序填充模板中的占位符。
// 值不足时，多余的占位符会被移除；值多余时忽略多余的值。
func Fill(tmpl string, vals map[string][]string) string {
	if len(vals) == 0 {
		return tmpl
	}
	for ph, list := range vals {
		if !strings.Contains(tmpl, ph) {
			continue
		}
		var b strings.Builder
		rest := tmpl
		i := 0
		for {
			idx := strings.Index(rest, ph)
			if idx < 0 {
				b.WriteString(rest)
				break
			}
			b.WriteString(rest[:idx])
			if i < len(list) {
				b.WriteString(list[i])
			}
			i++
			rest = rest[idx+len(ph):]
		}
		tmpl = b.String()
	}
	return tmpl
}

// CountPlaceholder 统计模板中某个占位符出现的次数。
func CountPlaceholder(tmpl, ph string) int { return strings.Count(tmpl, ph) }

// LooksTemplatable 判断一段文本是否含可模板化的动态部分（用于决定入库类型）。
func LooksTemplatable(s string) bool {
	return strings.Contains(s, PhLink) || strings.Contains(s, PhNum) ||
		strings.Contains(s, PhWord) || strings.Contains(s, PhAPI)
}
