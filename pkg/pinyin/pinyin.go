// Package pinyin 提供中文文本拼音生成能力。
//
// 设计目标：
//   - 服务于文本增强引擎前端兜底与批量离线场景；
//   - 接口简洁（只暴露 Convert 一种入口），不依赖特定库实现；
//   - 集成 mozillazg/go-pinyin（纯 Go、CGO-free），结果含全拼、拼音首字母、规范化文本；
//   - 多租户语境下：本包不存任何租户态，结果函数式确定（输入相同时输出相同）。
//
// 用法：
//
//	resp, err := pinyin.Convert("客服您好", true)
//	// resp.Pinyin = "ke fu nin hao"
//	// resp.PinyinInitial = "kfnh"
//	// resp.NormalizedText = "客服您好"
package pinyin

import (
	"strings"

	"github.com/mozillazg/go-pinyin"
)

// Result 拼音生成结果。
type Result struct {
	// Pinyin 全拼（用空格分隔每个汉字），如 "ke fu nin hao"。
	Pinyin string
	// PinyinInitial 拼音首字母字符串（连续无分隔），如 "kfnh"。
	PinyinInitial string
	// NormalizedText 规范化文本：去除空白与 ASCII 标点，便于匹配。
	NormalizedText string
}

// Converter 拼音生成器接口（解耦具体实现，便于在测试中替换）。
type Converter interface {
	Convert(text string, includeInitials bool) (*Result, error)
}

// NewConverter 返回默认实现（基于 mozillazg/go-pinyin）。
func NewConverter() Converter {
	return &defaultConverter{
		args: pinyin.NewArgs(),
	}
}

// defaultConverter 是 Converter 接口的默认实现，使用 go-pinyin 提供的内置词典。
type defaultConverter struct {
	args pinyin.Args
}

// Convert 将 text 转为拼音结果。
//
// 算法：
//  1. 规范化文本（去除 ASCII 标点与多余空白）；
//  2. 用 go-pinyin 逐字转换（含非汉字字符会原样保留）；
//  3. 拼接全拼（空格分隔）与首字母字符串；
//  4. 当 text 为空或仅含 ASCII 标点时返回空 Result。
func (c *defaultConverter) Convert(text string, includeInitials bool) (*Result, error) {
	normalized := normalize(text)
	if normalized == "" {
		return &Result{}, nil
	}
	pinyins := pinyin.Pinyin(normalized, c.args)
	if len(pinyins) == 0 {
		return &Result{NormalizedText: normalized}, nil
	}

	full := make([]string, 0, len(pinyins))
	initials := make([]string, 0, len(pinyins))
	for _, items := range pinyins {
		if len(items) == 0 {
			continue
		}
		// 多音字：取第一个音（go-pinyin Args 默认返回所有候选音）。
		// 若需手动校正，应在文本增强引擎「拼音纠错」步骤中处理（基于上下文 + 词库约束）。
		full = append(full, items[0])
		if includeInitials {
			if first := []rune(items[0]); len(first) > 0 {
				initials = append(initials, string(first[0]))
			}
		}
	}

	return &Result{
		Pinyin:         strings.Join(full, " "),
		PinyinInitial:  strings.Join(initials, ""),
		NormalizedText: normalized,
	}, nil
}

// Convert 是 NewConverter().Convert 的便捷封装，适合无依赖注入场景。
func Convert(text string, includeInitials bool) (*Result, error) {
	return NewConverter().Convert(text, includeInitials)
}

// normalize 去除 ASCII 与全角标点、合并多余空白，保留中文字符与英文字母/数字。
// 适用于文本增强引擎中的词库匹配预处理——让「客服，您好！」与「客服您好」能匹配同一词条。
func normalize(text string) string {
	if text == "" {
		return ""
	}
	var b strings.Builder
	lastSpace := false
	for _, r := range text {
		if isPunctuationOrSpace(r) {
			if isSpaceLike(r) {
				if !lastSpace && b.Len() > 0 {
					b.WriteByte(' ')
					lastSpace = true
				}
			}
			continue
		}
		lastSpace = false
		b.WriteRune(r)
	}
	out := b.String()
	return strings.TrimSpace(out)
}

// isPunctuationOrSpace 判断 r 是否为标点或空白（含 ASCII 与全角）。
// 剥离以下 Unicode 范围：
//   - ASCII 控制字符与空白（< 0x21）
//   - ASCII 标点（0x21-0x2F、0x3A-0x40、0x5B-0x60、0x7B-0x7E）
//   - CJK 标点（U+3000-U+303F）
//   - 全角 ASCII 标点（U+FF01-U+FF0E、U+FF1A-U+FF20、U+FF3B-U+FF40、U+FF5B-U+FF5E）
// 保留：中文字符（CJK 统一表意文字 U+4E00-U+9FFF）、英文字母、数字。
func isPunctuationOrSpace(r rune) bool {
	if r < 0x21 || r == 0x7F {
		return true // 控制字符 + ASCII 空白 + DEL
	}
	if r <= 0x7E {
		return isASCIIPunctuation(r)
	}
	switch {
	case r >= 0x3000 && r <= 0x303F: // CJK 符号与标点（、。「」【】等）
		return true
	case r >= 0xFF01 && r <= 0xFF0E: // 全角 ! " # $ % & ' ( ) * + , - .
		return true
	case r >= 0xFF1A && r <= 0xFF20: // 全角 : ; < = > ? @
		return true
	case r >= 0xFF3B && r <= 0xFF40: // 全角 [ \ ] ^ _ `
		return true
	case r >= 0xFF5B && r <= 0xFF5E: // 全角 { | } ~
		return true
	}
	return false
}

// isSpaceLike 判断 r 是否为空白类字符（归一化为单空格而非直接丢弃）。
func isSpaceLike(r rune) bool {
	if r < 0x21 {
		return true // ASCII 空白 + 换行 + 制表
	}
	if r == 0x3000 {
		return true // 全角空格 IDEOGRAPHIC SPACE
	}
	return false
}

// isASCIIPunctuation 判断 r 是否为可打印 ASCII 标点。
func isASCIIPunctuation(r rune) bool {
	switch {
	case r >= 0x21 && r <= 0x2F: // ! " # $ % & ' ( ) * + , - .
	case r >= 0x3A && r <= 0x40: // : ; < = > ? @
	case r >= 0x5B && r <= 0x60: // [ \ ] ^ _ `
	case r >= 0x7B && r <= 0x7E: // { | } ~
		return true
	}
	return false
}