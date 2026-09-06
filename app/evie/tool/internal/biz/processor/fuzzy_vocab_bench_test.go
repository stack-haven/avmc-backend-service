package processor

import (
	"fmt"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
)

// BenchmarkFuzzyVocabProcessor_Process 测量 1000 词库下模糊匹配延迟。
//
// 输入文本包含正常短语 + 一个与词库词距离 1 的子串，用于触发 fuzzy 命中。
func BenchmarkFuzzyVocabProcessor_Process(b *testing.B) {
	const n = 1000
	entries := make([]lexicon.Entry, 0, n)
	for i := 0; i < n; i++ {
		text := fmt.Sprintf("name%04d", i)
		entries = append(entries, lexicon.Entry{
			ID:   lexicon.EntryID(fmt.Sprintf("%d", i)),
			Text: text,
			Meta: map[string]any{"category": "PERSON"},
		})
	}
	// 多音字/同音条目用于 fuzzy 匹配触发。
	entries = append(entries, lexicon.Entry{
		ID:   "ref1",
		Text: "佘丽群",
		Meta: map[string]any{"category": "PERSON"},
	})

	lex, err := lexicon.NewBuilderWithVersion("v1").Add(entries...).Build()
	if err != nil {
		b.Fatalf("lex build: %v", err)
	}
	cfg := DefaultFuzzyVocabConfig()
	proc := NewFuzzyVocabProcessor(lex, cfg)
	cfgLex := lexnorm.DefaultConfig()
	input := "今天会议由周丽群负责，请大家配合 name0123 完成材料准备。"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state, err := lexnorm.NewState(b.Context(), input, lex, cfgLex)
		if err != nil {
			b.Fatalf("state: %v", err)
		}
		if err := proc.Process(b.Context(), state); err != nil {
			b.Fatalf("process: %v", err)
		}
	}
}
