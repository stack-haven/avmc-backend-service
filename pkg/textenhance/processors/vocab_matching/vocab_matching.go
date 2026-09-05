// Package vocab_matching 实现词库精确匹配策略（第 3 步）。
//
// 算法与 evie/service/internal/biz/enhancement.go 的 VocabularyMatchingStep 一致。
package vocab_matching

import (
	"context"
	"sort"
	"strings"

	"backend-service/pkg/textenhance/processors"
)

// sortedEntryKeys 按字典序返回 Entries 的 key（确定性遍历）。
func sortedEntryKeys(vocab *processors.VocabularySnapshot) []string {
	keys := make([]string, 0, len(vocab.Entries))
	for k := range vocab.Entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Processor 词库精确匹配策略。
type Processor struct {
	minEntryLen int      // 短于此长度的词条不匹配（防御性）
	lockMatched  bool    // 匹配后是否锁定片段（默认 true）
}

// Option 配置 Processor 的函数。
type Option func(*Processor)

// WithMinEntryLen 设置最小词条长度（默认 1）。
func WithMinEntryLen(n int) Option {
	return func(p *Processor) {
		if n > 0 { p.minEntryLen = n }
	}
}

// WithLockMatched 设置是否锁定匹配片段（默认 true）。
func WithLockMatched(enabled bool) Option {
	return func(p *Processor) { p.lockMatched = enabled }
}

// NewVocabularyMatchingProcessor 构造词库匹配策略。
func NewVocabularyMatchingProcessor(opts ...Option) *Processor {
	p := &Processor{
		minEntryLen: 1,
		lockMatched:  true,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name 返回 processor 标识。
func (p *Processor) Name() string { return "vocab_matching" }

// Process 实现 processors.TextProcessor。
//
// 算法（与 evie/service VocabularyMatchingStep 一致）：
//   1. 遍历 ec.Vocab.Entries
//   2. 命中标准词 → lock(text)
//   注意：本步不修改 text，只锁定片段；下游确定性步骤会基于 lock 做替换
func (p *Processor) Process(ctx context.Context, ec *processors.EnhancementContext) {
	if ec == nil || ec.Vocab == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}

	text := ec.Text
	for _, stdText := range sortedEntryKeys(ec.Vocab) {
		// 防御：过短词条不匹配（避免误命中单字符标点等）
		if len([]rune(stdText)) < p.minEntryLen {
			continue
		}
		if strings.Contains(text, stdText) {
			if p.lockMatched {
				ec.Lock(stdText) // 锁定：下游推断步骤不会修改此片段
			}
		}
	}
}

// 编译期断言
var _ processors.TextProcessor = (*Processor)(nil)