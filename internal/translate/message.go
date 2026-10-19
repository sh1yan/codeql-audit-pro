package translate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"codeql-audit-pro/internal/model"
)

// linkSentinelFmt 是渲染阶段的链接哨兵。先用哨兵占位，最后再切成片段，
// 这样即使译文里同时存在多个链接也能准确定位。
const linkSentinelFmt = "\x01%d\x01"

var reSentinel = regexp.MustCompile("\x01(\\d+)\x01")

// BuildCanonical 把消息片段拼成规范化英文模板键。
//
// CodeQL 会把同一规则的多条证据用换行拼在一条消息里，每一行都有自己的
// API 名与位置，因此必须**逐行**规范化，否则第二行之后的内容无法归约。
// 链接被替换为 {{LINK}}，同时抽出 API/数字/关键字等动态值。
// 返回的 links 按出现顺序记录，用于回填中文译文。
func BuildCanonical(segs []model.Segment) (key string, c Canonical, links []*model.LinkRef) {
	var b strings.Builder
	for _, s := range segs {
		if s.Kind == model.SegLink && s.Link != nil {
			links = append(links, s.Link)
			b.WriteString(PhLink)
			continue
		}
		b.WriteString(s.Text)
	}

	lines := strings.Split(b.String(), "\n")
	keys := make([]string, len(lines))
	c = Canonical{Key: "", Vals: map[string][]string{}}
	for i, ln := range lines {
		cc := Canonicalize(ln)
		keys[i] = cc.Key
		for k, v := range cc.Vals {
			c.Vals[k] = append(c.Vals[k], v...)
		}
	}
	// TranslateMessage 查询前会 TrimSpace；这里提前归一化，
	// 保证 MessageKey、翻译库入库键与查询键三者完全一致。
	c.Key = strings.TrimSpace(strings.Join(keys, "\n"))
	return c.Key, c, links
}

// RenderSegments 把中文译文模板渲染成「文本 + 链接」片段序列。
//
//	zh    已填充 API/N/W 等占位符、但仍含链接哨兵的中文
//	links 按顺序排列的链接
//	label 取链接的显示标签（通常是中文标签）
func RenderSegments(zh string, links []*model.LinkRef, label func(*model.LinkRef) string) []model.Segment {
	idx := reSentinel.FindAllStringSubmatchIndex(zh, -1)
	if len(idx) == 0 {
		if zh == "" {
			return nil
		}
		return []model.Segment{{Kind: model.SegText, Text: zh}}
	}
	segs := make([]model.Segment, 0, len(idx)*2+1)
	used := map[int]bool{}
	last := 0
	for _, m := range idx {
		if m[0] > last {
			segs = append(segs, model.Segment{Kind: model.SegText, Text: zh[last:m[0]]})
		}
		n, _ := strconv.Atoi(zh[m[2]:m[3]])
		if n >= 0 && n < len(links) {
			l := links[n]
			used[n] = true
			segs = append(segs, model.Segment{
				Kind: model.SegLink, Text: label(l), Link: l,
			})
		}
		last = m[1]
	}
	if last < len(zh) {
		segs = append(segs, model.Segment{Kind: model.SegText, Text: zh[last:]})
	}
	// 译文丢失了部分链接占位符时，把缺失的链接补到末尾，保证证据链不丢
	var missing []string
	for i, l := range links {
		if !used[i] {
			missing = append(missing, fmt.Sprintf("%s（%s）", label(l), l.Display()))
		}
	}
	if len(missing) > 0 {
		segs = append(segs, model.Segment{
			Kind: model.SegText,
			Text: "［相关位置：" + strings.Join(missing, "；") + "］",
		})
	}
	return segs
}

// Sentinels 生成用于回填的链接哨兵列表。
func Sentinels(n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = fmt.Sprintf(linkSentinelFmt, i)
	}
	return out
}
