// Package pinyin · signature.go
//
// 拼音 signature：从中文文本提取可用于"音近归一"的轻量签名。
//
// 设计动机：ASR 把"陈欣静"识别成"陈新进"（声调 + 前后鼻音混淆）是高频错误，
// 但两者的拼音首字母完全相同（"c-x-j"）。FuzzyVocabProcessor 用 signature 做
// 二次匹配：当 Levenshtein/Hamming 距离超阈值时，若 sub 与某 PERSON 条目的
// signature 完全相同，强制归一。
//
// # 接口
//
//   - Signature(text) → "cxj" 形式的首字母串（去重去噪）
//   - FuzzyEqual(sig1, sig2) → 宽容比较（默认 ≡ ExactEqual；预留宽松规则）
//   - Index(text) → 同 Signature（小写化、去重连续辅音），便于 case-insensitive 比较
//
// # 当前限制
//
//   - 仅取首字母，无法区分"陈兴静" vs "陈新近"（同 cxj，但 j 同）
//   - 多音字：取第一个候选音（与 Convert 一致）
//   - 非汉字字符（含 ASCII）：原样保留（不计入 signature 序列）
//
// 后续可扩展：取韵母首字母（如 "cxjing" vs "cxjin" 仍能区分），或拼写完全量化。
package pinyin

import (
	"strings"
	"sync"
)

// sigConv 共享的拼音转换器（无状态，可全局复用）。
var (
	sigConvOnce sync.Once
	sigConv     Converter
)

func conv() Converter {
	sigConvOnce.Do(func() { sigConv = NewConverter() })
	return sigConv
}

// Signature 返回 text 的"首字母 signature"。
//
// 例：
//   Signature("陈欣静")   = "cxj"
//   Signature("佘丽群")   = "slq"
//   Signature("Hello")    = "hello"   // ASCII 字符原样保留并小写
//   Signature("")         = ""
//
// 设计选择：
//   - 空字符串 / 纯标点 → ""
//   - 汉字 → 拼音首字母
//   - ASCII 字母 → 原样小写
//   - 其他（标点 / 数字）→ 跳过
func Signature(text string) string {
	r, err := conv().Convert(text, true)
	if err != nil || r == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(r.PinyinInitial))
}

// FuzzyEqual 比较两个 signature 是否"音近相等"。
//
// 当前实现 = ExactEqual（精确相等）。预留 hook 用于后续扩展
// 宽松规则（如 n/l 不分、z/zh 不分、in/ing 不分等，按声母韵母 fuzzy 集合）。
func FuzzyEqual(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	return ExactEqual(a, b)
}

// ExactEqual 精确比较（区分所有声母韵母）。
//
// 与 FuzzyEqual 当前等价；导出以便外部测试与未来扩展。
func ExactEqual(a, b string) bool {
	return a == b
}
