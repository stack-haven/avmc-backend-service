// Package builtins 提供 textenhance 的 9 个默认 Processor 工厂注册。
//
// 单独成包是为了避免 import cycle：
//   - textenhance 不依赖任何具体 processor
//   - builtins 依赖 textenhance（提供 Registry/Option 接口）和 processors（提供 TextProcessor 类型）
//   - processors/* 依赖 processors（提供 TextProcessor/Change 等类型）
package builtins

import (
	"backend-service/pkg/textenhance"
	"backend-service/pkg/textenhance/processors"
	"backend-service/pkg/textenhance/processors/alias_resolution"
	"backend-service/pkg/textenhance/processors/cleaning"
	"backend-service/pkg/textenhance/processors/context_correction"
	"backend-service/pkg/textenhance/processors/deterministic_replacement"
	"backend-service/pkg/textenhance/processors/filler"
	"backend-service/pkg/textenhance/processors/fuzzy_matching"
	"backend-service/pkg/textenhance/processors/llm_reserved"
	"backend-service/pkg/textenhance/processors/phrase_standardization"
	"backend-service/pkg/textenhance/processors/pinyin_correction"
	"backend-service/pkg/textenhance/processors/vocab_matching"
)

// NewDefaultRegistry 返回含 9 个默认 Processor 工厂的 Registry。
func NewDefaultRegistry() *textenhance.Registry {
	r := textenhance.NewRegistry()

	// 1. cleaning
	r.Register("cleaning", func(opts ...textenhance.Option) (processors.TextProcessor, error) {
		co, err := textenhance.ToOptions[cleaning.Option](opts)
		if err != nil {
			return nil, textenhance.ErrProcessorOptionType("cleaning", nil, err)
		}
		return cleaning.NewTextCleaningProcessor(co...), nil
	})

	// 2. filler
	r.Register("filler", func(opts ...textenhance.Option) (processors.TextProcessor, error) {
		fo, err := textenhance.ToOptions[filler.Option](opts)
		if err != nil {
			return nil, textenhance.ErrProcessorOptionType("filler", nil, err)
		}
		return filler.NewFillerProcessor(fo...), nil
	})

	// 3. vocab_matching
	r.Register("vocab_matching", func(opts ...textenhance.Option) (processors.TextProcessor, error) {
		vo, err := textenhance.ToOptions[vocab_matching.Option](opts)
		if err != nil {
			return nil, textenhance.ErrProcessorOptionType("vocab_matching", nil, err)
		}
		return vocab_matching.NewVocabularyMatchingProcessor(vo...), nil
	})

	// 4. alias_resolution
	r.Register("alias_resolution", func(opts ...textenhance.Option) (processors.TextProcessor, error) {
		ao, err := textenhance.ToOptions[alias_resolution.Option](opts)
		if err != nil {
			return nil, textenhance.ErrProcessorOptionType("alias_resolution", nil, err)
		}
		return alias_resolution.NewAliasResolutionProcessor(ao...), nil
	})

	// 5. deterministic_replacement
	r.Register("deterministic_replacement", func(opts ...textenhance.Option) (processors.TextProcessor, error) {
		do, err := textenhance.ToOptions[deterministic_replacement.Option](opts)
		if err != nil {
			return nil, textenhance.ErrProcessorOptionType("deterministic_replacement", nil, err)
		}
		return deterministic_replacement.NewDeterministicReplacementProcessor(do...), nil
	})

	// 6. phrase_standardization
	r.Register("phrase_standardization", func(opts ...textenhance.Option) (processors.TextProcessor, error) {
		po, err := textenhance.ToOptions[phrase_standardization.Option](opts)
		if err != nil {
			return nil, textenhance.ErrProcessorOptionType("phrase_standardization", nil, err)
		}
		return phrase_standardization.NewPhraseStandardizationProcessor(po...), nil
	})

	// 7. pinyin_correction
	r.Register("pinyin_correction", func(opts ...textenhance.Option) (processors.TextProcessor, error) {
		pc, err := textenhance.ToOptions[pinyin_correction.Option](opts)
		if err != nil {
			return nil, textenhance.ErrProcessorOptionType("pinyin_correction", nil, err)
		}
		return pinyin_correction.NewPinyinCorrectionProcessor(pc...), nil
	})

	// 8. fuzzy_matching
	r.Register("fuzzy_matching", func(opts ...textenhance.Option) (processors.TextProcessor, error) {
		fm, err := textenhance.ToOptions[fuzzy_matching.Option](opts)
		if err != nil {
			return nil, textenhance.ErrProcessorOptionType("fuzzy_matching", nil, err)
		}
		// 生产场景：PERSON 类人名 3 字词 conf=0.667 是常态，启用专用阈值。
		//   - ORGANIZATION / PRODUCT 等固定术语保持默认 0.80
		//   - PERSON 放宽到 0.65，覆盖“佘丽群↔周丽群”这类 ASR 错字
		fm = append(fm,
			fuzzy_matching.WithCategoryAutoThreshold("PERSON", 0.65),
			fuzzy_matching.WithCategorySuggestThreshold("PERSON", 0.55),
		)
		return fuzzy_matching.NewFuzzyMatchingProcessor(fm...), nil
	})

	// 9. context_correction
	r.Register("context_correction", func(opts ...textenhance.Option) (processors.TextProcessor, error) {
		cc, err := textenhance.ToOptions[context_correction.Option](opts)
		if err != nil {
			return nil, textenhance.ErrProcessorOptionType("context_correction", nil, err)
		}
		return context_correction.NewContextCorrectionProcessor(cc...), nil
	})

	// 10. llm_reserved
	r.Register("llm_reserved", func(opts ...textenhance.Option) (processors.TextProcessor, error) {
		lo, err := textenhance.ToOptions[llm_reserved.Option](opts)
		if err != nil {
			return nil, textenhance.ErrProcessorOptionType("llm_reserved", nil, err)
		}
		return llm_reserved.NewLLMReservedProcessor(lo...), nil
	})

	return r
}

// DefaultRegistryNames 返回已注册 processor 名称（按 DefaultProcessorOrder 排序）。
func DefaultRegistryNames() []string {
	out := make([]string, 0, len(textenhance.DefaultProcessorOrder)+1)
	out = append(out, textenhance.DefaultProcessorOrder...)
	out = append(out, "llm_reserved")
	return out
}