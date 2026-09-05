// Package main 演示 ark-lexnorm 的内置 Preset：Standard / HighAccuracy /
// Fast / ASR / OCR。这些是开箱即用的 Pipeline 模板，覆盖绝大多数场景。
//
// 关键 API：
//   - presets.Standard(lex, conv)   // 全功能 Pipeline
//   - presets.HighAccuracy(...)     // 加 LLM 占位，但不开 LLM 服务（按规则匹配）
//   - presets.Fast(...)             // 轻量 Pipeline
//   - presets.ASR(...)              // 语音转写场景
//   - presets.OCR(...)              // 图片识别场景
//
// 运行：go run ./example/03-presets
package main

import (
	"context"
	"fmt"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/presets"
)

// simpleConverter 是一个最小的 PinyinConverter，把每个字符当作单字读音。
//
// 实际应用里你会接外部拼音库（例如 pinyin-pro），
// 这里仅满足 Preset 的接口约束，让示例可独立运行。
type simpleConverter struct{}

func (simpleConverter) ToPinyin(text string) []string {
	out := make([]string, 0, len(text))
	for _, r := range text {
		out = append(out, string(r))
	}
	return out
}

func main() {
	// -------------------------------------------------------------------------
	// 第 1 步：构建一个共享的 Lexicon。
	//
	// 所有 Preset 都接收同一个 Lexicon：Preset 只决定 Pipeline 顺序和
	// Config 阈值，不决定 Lexicon 内容。
	// -------------------------------------------------------------------------
	lex, err := lexicon.NewBuilderWithVersion("v1").
		Add(lexicon.Entry{
			ID:   "name-zhouliqun",
			Text: "周丽群",
			Variants: []lexicon.Variant{
				{Text: "周莉群", Kind: lexicon.VariantAlias, Confidence: 1.0},
				{Text: "周丽裙", Kind: lexicon.VariantCorrection, Confidence: 1.0},
			},
		}).
		Add(lexicon.Entry{
			ID:   "tech-gateway",
			Text: "API 网关",
			Variants: []lexicon.Variant{
				{Text: "api网关", Kind: lexicon.VariantAlias, Confidence: 1.0},
			},
		}).
		Build()
	if err != nil {
		panic(err)
	}

	conv := simpleConverter{}

	// -------------------------------------------------------------------------
	// 第 2 步：选择 Preset。
	//
	// 5 个 Preset 的定位：
	//   - Fast         ：最快（无 fuzzy / pinyin），适合规则明确的批处理
	//   - Standard     ：平衡（默认推荐）
	//   - HighAccuracy ：在 Standard 基础上降低 Suggest 阈值，倾向自动应用
	//   - ASR          ：为语音转写优化（前置 Disfluency + 强 Normalize）
	//   - OCR          ：为图片识别优化（前置 Normalize，针对全角字符）
	// -------------------------------------------------------------------------
	chosenPresets := []*lexnorm.Preset{
		presets.Fast(lex),
		presets.Standard(lex, conv),
		presets.ASR(lex, conv),
	}

	// -------------------------------------------------------------------------
	// 第 3 步：同一个输入，跑三个 Preset 对比。
	// -------------------------------------------------------------------------
	input := "嗯 周莉群 说呃 API网关 已经呃上线了 嗯。"

	for _, p := range chosenPresets {
		engine, err := lexnorm.New(lexnorm.WithPreset(*p))
		if err != nil {
			fmt.Printf("[%s] 构造失败: %v\n", p.Name(), err)
			continue
		}

		result, err := engine.Normalize(context.Background(), input)
		if err != nil {
			fmt.Printf("[%s] normalize 失败: %v\n", p.Name(), err)
			continue
		}

		fmt.Printf("[%s]\n", p.Name())
		fmt.Printf("  输入: %q\n", input)
		fmt.Printf("  输出: %q\n", result.Text)
		fmt.Printf("  改动: %d 条\n", len(result.Changes))
		fmt.Println()
	}

	// -------------------------------------------------------------------------
	// 第 4 步：自定义 Preset 的 Config 阈值。
	//
	// 比如把 HighAccuracy 的 Suggest 阈值降低（更激进的自动应用）：
	// -------------------------------------------------------------------------
	highAcc := presets.HighAccuracy(lex, conv)
	cfg := highAcc.Config()
	fmt.Printf("[%s] 默认 AutoApplyThreshold = %.2f\n",
		highAcc.Name(), cfg.AutoApplyThreshold)
	fmt.Printf("[%s] 默认 SuggestThreshold   = %.2f\n",
		highAcc.Name(), cfg.SuggestThreshold)

	// 修改后再用 NewPreset 重新包装（直接改 cfg 不影响 Preset 副本）。
	cfg.AutoApplyThreshold = 0.7
	tuned := lexnorm.NewPreset(highAcc.Name(), highAcc.Description(),
		highAcc.Pipeline(), cfg)
	engine, _ := lexnorm.New(lexnorm.WithPreset(*tuned))
	result, _ := engine.Normalize(context.Background(), input)
	fmt.Printf("[%s] 调阈后: AutoApply=%.2f, 输出=%q\n",
		tuned.Name(), tuned.Config().AutoApplyThreshold, result.Text)

	// 期望输出（3 个 Preset 行为不同，具体改动条数因 Pipeline 而异）：
	// [fast] 不含 Disfluency，因此保留 "嗯"/"呃" 等语气词；
	//        只做 Normalize + Alias，因此 "API网关" 不会被拆开。
	// [standard] 完整 Pipeline，会删 disfluency 和 normalize 全角符号，
	//             但 "API网关" 因大小写与 Lexicon variant 不同未匹配。
	// [asr] 与 Standard 类似，针对 ASR 场景的 Config 阈值微调。
	//
	// 注意 HighAccuracy 含 Pinyin/Fuzzy，建议用一个明确的输入来观察；
	// 这里仅展示 Config 阈值调优的写法。
}
