// PinyinConverter 把 pinyinlite 表适配成 lexicon.PinyinConverter 接口，
// 供 ark-lexnorm builder.WithPinyin 使用。

package pinyinlite

import "strings"

// Converter 是 lexicon.PinyinConverter 接口的最小实现。
//
// ToPinyin 返回输入文本的拼音形式（用空格分隔每个汉字的拼音）。
//
// 本实现不做多音字消歧（363 字表覆盖范围内，每字 1 个拼音）；
// 若将来需要扩展多音字支持，可改为返回多元素 slice，
// 调用方（lexicon.PinyinIndex）会自动把所有形式都索引。
//
// 返回非空字符串，除非输入不含本表覆盖的字。
type Converter struct{}

// ToPinyin 实现 lexicon.PinyinConverter。
func (Converter) ToPinyin(text string) []string {
	py := Convert(text)
	if py == "" {
		return nil
	}
	return []string{strings.TrimSpace(py)}
}