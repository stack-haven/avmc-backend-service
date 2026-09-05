// Package processors · common.go
// 公共工具、类型、依赖：供 9 个默认 processor 共用，避免重复实现。
//
// 本文件内容：
//   1. Text Utilities：IsPunctOrSpace / IsCJKChar / HasCJK / CountCJK / ReplaceAll / ContainsAny
//   2. Common Types：Stopword / StopwordType
//   3. Common Dependencies：PinyinService interface + DefaultPinyinService
//   4. Common Option Patterns：option 写法的统一范式（文档化）
package processors

import (
	"context"
	"sort"
	"strings"

	"backend-service/pkg/pinyin"
)

// ============================================================================
// 1. Text Utilities
// ============================================================================

// SortedEntries 返回 ec.Vocab.Entries 的有序列表（按 ID 升序）。
//
// 背景：VocabularySnapshot.Entries 是 map，map 迭代顺序随机。
// 多个 processor 遍历时会导致：相同输入多次跑结果不一致（多候选场景下选词不稳定）。
//
// 所有 processor 在修改 ec 状态前应使用本函数，确保输出可复现。
func SortedEntries(vocab *VocabularySnapshot) []*VocabularyEntry {
	out := make([]*VocabularyEntry, 0, len(vocab.Entries))
	for _, e := range vocab.Entries {
		if e != nil {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// IsPunctOrSpace 判定字符是否为中英文标点或空白。
// 用于：filler 判定口水词位置、context_correction 判定上下文边界等。
func IsPunctOrSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '　',
		'，', '。', '！', '？', '；', '：', '、', '（', '）', '【', '】',
		',', '.', '!', '?', ';', ':', '(', ')', '[', ']',
		'"', '\'', '<', '>', '/', '\\', '=', '+', '*', '&', '^', '%', '$', '#', '@', '~', '`',
		'_', '-':
		return true
	}
	return false
}

// IsCJKChar 判定字符是否为 CJK 统一表意文字（U+4E00 ~ U+9FFF）。
// 用于：vocab_matching 中文词条优先匹配。
func IsCJKChar(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

// HasCJK 判定字符串是否含 CJK 字符。
func HasCJK(s string) bool {
	for _, r := range s {
		if IsCJKChar(r) {
			return true
		}
	}
	return false
}

// CountCJK 统计 CJK 字符数。
func CountCJK(s string) int {
	n := 0
	for _, r := range s {
		if IsCJKChar(r) {
			n++
		}
	}
	return n
}

// ReplaceAll 全量字符串替换（独立实现，便于 processor 复用，避免重复 import 链）。
func ReplaceAll(s, old, new string) string {
	if old == "" {
		return s
	}
	var b strings.Builder
	i := 0
	for {
		j := strings.Index(s[i:], old)
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(s[i : i+j])
		b.WriteString(new)
		i += j + len(old)
	}
	return b.String()
}

// ContainsAny 判定 s 是否含任一 keywords（任一命中即 true）。
func ContainsAny(s string, keywords []string) bool {
	for _, k := range keywords {
		if k != "" && strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// ContainsAll 判定 s 是否含全部 keywords。
func ContainsAll(s string, keywords []string) bool {
	for _, k := range keywords {
		if k == "" || !strings.Contains(s, k) {
			return false
		}
	}
	return true
}

// ============================================================================
// 2. Common Types
// ============================================================================

// StopwordType 停用词类型。
type StopwordType int

const (
	// StopwordStrongFiller 强口水词（呃/额/啊/哦/哈）→ 通常删除
	StopwordStrongFiller StopwordType = iota + 1
	// StopwordWeakFiller 弱口水词（嗯/呃呃）→ 视上下文决定
	StopwordWeakFiller
	// StopwordCustom 用户自定义停用词
	StopwordCustom
)

// Stopword 停用词（带元数据）。
// 用于 filler 扩展（默认只删 strong filler；带元数据后能精细控制）。
type Stopword struct {
	Word   string       // 停用词原文
	Type   StopwordType // 类别
	Reason string       // 删除原因（日志 / 调试用）
}

// NewStopword 构造（便捷）。
func NewStopword(word string, t StopwordType, reason string) Stopword {
	return Stopword{Word: word, Type: t, Reason: reason}
}

// ============================================================================
// 3. Common Dependencies
// ============================================================================

// PinyinService 抽象接口（便于 mock / 替换 pinyin 工具）。
//
// 用于：
//   - pinyin_correction 注入 pinyin 工具
//   - 未来 pinyin_based processor 共用
//   - 单元测试注入 mock pinyin
type PinyinService interface {
	// Convert 文本 → 拼音结果。
	// includeInitials=true 时附带首字母。
	Convert(ctx context.Context, text string, includeInitials bool) (*pinyin.Result, error)
}

// DefaultPinyinService 默认基于 pkg/pinyin 的实现。
type DefaultPinyinService struct {
	converter pinyin.Converter
}

// NewDefaultPinyinService 构造默认 pinyin 服务。
func NewDefaultPinyinService() *DefaultPinyinService {
	return &DefaultPinyinService{converter: pinyin.NewConverter()}
}

// Convert 实现 PinyinService。
func (s *DefaultPinyinService) Convert(ctx context.Context, text string, includeInitials bool) (*pinyin.Result, error) {
	// 注：pinyin.Converter.Convert 不支持 ctx；这里忽略 ctx 但保留签名以备未来
	_ = ctx
	return s.converter.Convert(text, includeInitials)
}

// 编译期断言
var _ PinyinService = (*DefaultPinyinService)(nil)

// ============================================================================
// 4. Common Option Patterns（文档化范式）
// ============================================================================
//
// 各 processor 子包定义 Option 时的统一范式：
//
//   // Option 是 processor 的配置函数类型。
//   type Option func(*Processor)
//
//   // WithCaseSensitive 设置是否区分大小写（bool 字段）。
//   func WithCaseSensitive(enabled bool) Option {
//       return func(p *Processor) { p.caseSensitive = enabled }
//   }
//
//   // WithMinLength 设置最小匹配长度（int 字段）。
//   func WithMinLength(n int) Option {
//       return func(p *Processor) {
//           if n > 0 { p.minLen = n }
//       }
//   }
//
//   // WithCustomStopwords 设置自定义停用词列表（[]Stopword 字段）。
//   func WithCustomStopwords(words []Stopword) Option {
//       return func(p *Processor) { p.stopwords = words }
//   }
//
//   // WithPinyinService 注入 pinyin 服务（PinyinService 字段）。
//   func WithPinyinService(svc PinyinService) Option {
//       return func(p *Processor) { p.pinyinSvc = svc }
//   }
//
// 命名约定：
//   - With + <字段名 PascalCase>（如 WithCaseSensitive 而非 WithCase）
//   - 字段名直接出现在 Option 名中（无需注释）
//   - 字段类型不在 Option 名中（int / bool / []T 都是 WithXxx）
//
// 校验约定：
//   - 数值 > 0 / 字符串非空 / 切片非空 → 接受；否则忽略
//   - 阈值类（float64 ∈ [0, 1]）→ 越界则 clamp 到 0（Pipeline 兜底）
//
// 注入 vs 默认：
//   - 默认值在 NewXxxProcessor 内设置（如 p.caseSensitive = false）
//   - Options 覆盖默认值
//   - 注入型资源（pinyinSvc / stopwords）→ 必传或 nil=不启用
//
// ============================================================================