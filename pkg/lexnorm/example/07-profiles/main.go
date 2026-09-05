// Package main 演示如何使用 WithProfiles 注册多个 Profile 并按 key 路由。
//
// 关键 API：
//   - lexnorm.WithProfiles(map[lexnorm.ProfileID]lexnorm.ProfileBundle{...})
//   - lexnorm.WithDefaultProfile(id)  // 可选；不设则第一次调用必须显式指定
//   - engine.Normalize(ctx, text, lexnorm.WithProfile("asr"))
//
// 场景：
//   - "default" Profile：通用 Pipeline（含 disfluency / alias）
//   - "asr"     Profile：ASR 专用（更激进的 disfluency、更高的容错）
//   - "ocr"     Profile：OCR 专用（强 normalize，对全角字符敏感）
//
// 运行：go run ./example/07-profiles
package main

import (
	"context"
	"fmt"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/alias"
	"github.com/stack-haven/lexnorm/processor/deterministic"
	"github.com/stack-haven/lexnorm/processor/disfluency"
	"github.com/stack-haven/lexnorm/processor/normalize"
)

func main() {
	// -------------------------------------------------------------------------
	// 第 1 步：构造三个 Lexicon。
	//
	// 实际生产环境里每个 Profile 通常对应不同的业务场景，
	// 因此会有不同的 Lexicon。这里演示三种典型变体。
	// -------------------------------------------------------------------------
	defaultLex, _ := lexicon.NewBuilderWithVersion("v1-default").
		Add(lexicon.Entry{
			ID:   "name-zhouliqun",
			Text: "周丽群",
			Variants: []lexicon.Variant{
				{Text: "周莉群", Kind: lexicon.VariantAlias, Confidence: 1.0},
			},
		}).
		Build()

	asrLex, _ := lexicon.NewBuilderWithVersion("v1-asr").
		Add(lexicon.Entry{
			ID:   "name-zhouliqun",
			Text: "周丽群",
			Variants: []lexicon.Variant{
				{Text: "周莉群", Kind: lexicon.VariantAlias, Confidence: 1.0},
				{Text: "周丽裙", Kind: lexicon.VariantCorrection, Confidence: 1.0},
				{Text: "周莉裙", Kind: lexicon.VariantCorrection, Confidence: 0.95},
			},
		}).
		Build()

	ocrLex, _ := lexicon.NewBuilderWithVersion("v1-ocr").
		Add(lexicon.Entry{
			ID:   "name-zhouliqun",
			Text: "周丽群",
			Variants: []lexicon.Variant{
				{Text: "周丽群", Kind: lexicon.VariantAlias, Confidence: 1.0},
			},
		}).
		Build()

	// -------------------------------------------------------------------------
	// 第 2 步：为每个 Profile 构造独立的 Pipeline。
	// -------------------------------------------------------------------------
	defaultPipeline := lexnorm.NewPipeline(
		normalize.New(),
		alias.New(defaultLex),
	)

	asrPipeline := lexnorm.NewPipeline(
		normalize.New(),
		disfluency.New(),
		alias.New(asrLex),
		deterministic.New(asrLex),
	)

	// OCR 场景：开启全角→半角转换（针对 OCR 常见的全角字符）。
	ocrNormalize := normalize.New().WithFullWidthToHalf(true)
	ocrPipeline := lexnorm.NewPipeline(
		ocrNormalize,
		alias.New(ocrLex),
	)

	// -------------------------------------------------------------------------
	// 第 3 步：用 WithProfiles 注册所有 Profile，并设置默认。
	//
	// WithDefaultProfile("default") 让无参 Normalize 调用走 default。
	// -------------------------------------------------------------------------
	engine, err := lexnorm.New(
		lexnorm.WithProfiles(map[lexnorm.ProfileID]lexnorm.ProfileBundle{
			"default": {Lexicon: defaultLex, Pipeline: defaultPipeline},
			"asr":     {Lexicon: asrLex, Pipeline: asrPipeline},
			"ocr":     {Lexicon: ocrLex, Pipeline: ocrPipeline},
		}),
		lexnorm.WithDefaultProfile("default"),
	)
	if err != nil {
		panic(err)
	}

	// -------------------------------------------------------------------------
	// 第 4 步：按 Profile 路由调用。
	//
	// 每次 Normalize 调用通过 WithProfile("xxx") 指定走哪个 Profile。
	// -------------------------------------------------------------------------
	cases := []struct {
		profile lexnorm.ProfileID
		text    string
	}{
		{"default", "周莉群 请确认。"},
		{"asr", "  嗯  周莉裙  呃呃  请确认进度。  "},
		{"ocr", "周丽群（全角符号混入：【】（））。"},
	}

	for _, c := range cases {
		result, err := engine.Normalize(context.Background(), c.text,
			lexnorm.WithProfileID(c.profile))
		if err != nil {
			fmt.Printf("[%s] normalize 失败: %v\n", c.profile, err)
			continue
		}
		fmt.Printf("[%s]\n  输入: %q\n  输出: %q\n  改动: %d\n\n",
			c.profile, c.text, result.Text, len(result.Changes))
	}

	// -------------------------------------------------------------------------
	// 第 5 步：不指定 Profile 时走默认。
	// -------------------------------------------------------------------------
	result, _ := engine.Normalize(context.Background(), "周莉群 已确认。")
	fmt.Printf("[default, 无参] 输入=%q → 输出=%q\n",
		"周莉群 已确认。", result.Text)

	// -------------------------------------------------------------------------
	// 第 6 步：调用不存在的 Profile 会失败。
	// -------------------------------------------------------------------------
	_, err = engine.Normalize(context.Background(), "test",
		lexnorm.WithProfileID("nonexistent"))
	if err != nil {
		fmt.Printf("[nonexistent] 期望失败: %v\n", err)
	}

	// 期望输出（具体改动数会随 Lexicon 版本微调）：
	// [default]
	//   输入: "周莉群 请确认。"
	//   输出: "周丽群 请确认。"
	//   改动: 1
	//
	// [asr]
	//   输入: "  嗯  周莉裙  呃呃  请确认进度。  "
	//   输出: " 周丽群 请确认进度。" （去掉 disfluency + alias 修正）
	//   改动: 多条
	//
	// [ocr]
	//   输入: "周丽群（全角符号混入：【】（））。"
	//   输出: "周丽群(全角符号混入:【】())."
	//   改动: 多条
	//
	// [default, 无参] 输入="周莉群 已确认。" → 输出="周丽群 已确认。"
	// [nonexistent] 期望失败: profile "nonexistent" not found: lexnorm: invalid config
}
