// Package filler 实现口水词处理策略（第 2 步）。
//
// 算法与 evie/service/internal/biz/enhancement.go 的 FillerStep 一致；
// 标点/空白判定改用 processors.IsPunctOrSpace 公共函数。
package filler

import (
	"context"
	"strings"

	"backend-service/pkg/textenhance/processors"
)

// Processor 口水词处理策略。
type Processor struct {
	caseSensitive bool
	strongFillers []string // 强口水词（呃/额/啊/哦/哈）
}

// Option 配置 Processor 的函数。
type Option func(*Processor)

// WithCaseSensitive 控制是否区分大小写（默认 false）。
func WithCaseSensitive(enabled bool) Option {
	return func(p *Processor) { p.caseSensitive = enabled }
}

// WithStrongFillers 自定义强口水词列表（默认 ["呃","额","啊","哦","哈"]）。
func WithStrongFillers(words []string) Option {
	return func(p *Processor) {
		if len(words) > 0 {
			p.strongFillers = append([]string(nil), words...)
		}
	}
}

// WithStopwords 用 Stopword 列表（含元数据）注入。
// 与 WithStrongFillers 二选一；同时设置时后者覆盖前者。
func WithStopwords(stopwords []processors.Stopword) Option {
	return func(p *Processor) {
		if len(stopwords) == 0 {
			return
		}
		words := make([]string, 0, len(stopwords))
		for _, sw := range stopwords {
			if sw.Word != "" {
				words = append(words, sw.Word)
			}
		}
		if len(words) > 0 {
			p.strongFillers = words
		}
	}
}

// NewFillerProcessor 构造口水词策略。
func NewFillerProcessor(opts ...Option) *Processor {
	p := &Processor{
		caseSensitive: false,
		strongFillers: []string{"呃", "额", "啊", "哦", "哈"},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name 返回 processor 标识。
func (p *Processor) Name() string { return "filler" }

// Process 实现 textenhance.TextProcessor。
//
// 算法（与 evie/service FillerStep 一致）：
//   1. 遍历字符，识别强口水词
//   2. 仅删除句首或标点/空白后的口水词
//   3. 「嗯」不在删除列表（可能表示肯定）
func (p *Processor) Process(ctx context.Context, ec *processors.EnhancementContext) {
	if ec == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}

	text := ec.Text
	runes := []rune(text)
	var out []rune
	var removed []string
	changed := false

	matchSet := make(map[string]bool, len(p.strongFillers))
	for _, f := range p.strongFillers {
		if p.caseSensitive {
			matchSet[f] = true
		} else {
			matchSet[strings.ToLower(f)] = true
		}
	}

	for i, r := range runes {
		key := string(r)
		if !p.caseSensitive {
			key = strings.ToLower(key)
		}
		if matchSet[key] {
			if i == 0 || processors.IsPunctOrSpace(runes[i-1]) {
				removed = append(removed, string(r))
				changed = true
				continue
			}
		}
		out = append(out, r)
	}

	if !changed {
		return
	}
	ec.Text = string(out)

	for _, f := range removed {
		ec.Changes = append(ec.Changes, processors.Change{
			From:       f,
			To:         "",
			Action:     processors.ActionDelete,
			Type:       processors.TypeFiller,
			Source:     processors.SourceSystem,
			Confidence: 1.0,
			Locked:     true,
			Reason:     "strong filler at sentence start / after punctuation",
		})
	}
}

// 编译期断言
var _ processors.TextProcessor = (*Processor)(nil)