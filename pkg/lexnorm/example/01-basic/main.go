// Package main 演示 ark-lexnorm 的最简用法：构建一个 Lexicon，配置一个
// Pipeline，把它装进 Engine，然后调用 Normalize。
//
// 关键 API：
//   - lexicon.NewBuilder().Add(lexicon.Entry{...}).Build()
//   - lexnorm.NewPipeline(processor...)         // 顺序敏感
//   - lexnorm.New(lexnorm.WithLexicon(...), lexnorm.WithPipeline(...))
//   - engine.Normalize(ctx, "原始文本")
//
// 运行：go run ./example/01-basic
package main

import (
	"context"
	"fmt"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/alias"
	"github.com/stack-haven/lexnorm/processor/normalize"
)

func main() {
	// -------------------------------------------------------------------------
	// 第 1 步：用 Builder 构建一个内存 Lexicon。
	//
	// Entry 表示一个"标准词条"：
	//   - ID    ：全局唯一 ID
	//   - Text  ：canonical（标准）写法
	//   - Variants：所有已知变体（错别字、别名等）
	//
	// 这里登记三条常见的语音识别错误。
	// -------------------------------------------------------------------------
	lex, err := lexicon.NewBuilderWithVersion("v1").
		Add(lexicon.Entry{
			ID:   "name-zhouliqun",
			Text: "周丽群",
			Variants: []lexicon.Variant{
				{Text: "周莉裙", Kind: lexicon.VariantCorrection, Confidence: 1.0},
				{Text: "周莉群", Kind: lexicon.VariantAlias, Confidence: 1.0},
			},
		}).
		Add(lexicon.Entry{
			ID:   "name-tianhua",
			Text: "田华",
			Variants: []lexicon.Variant{
				{Text: "小田", Kind: lexicon.VariantAlias, Confidence: 1.0},
			},
		}).
		Build()
	if err != nil {
		panic(fmt.Errorf("build lexicon: %w", err))
	}

	// -------------------------------------------------------------------------
	// 第 2 步：组合一个最小 Pipeline。
	//
	// Normalize：清洗（去空格、控制字符、全角→半角）
	// Alias    ：根据 Lexicon 做变体→标准词替换
	//
	// Pipeline 是有序的：先 Normalize 清理输入，再 Alias 做替换。
	// -------------------------------------------------------------------------
	pipeline := lexnorm.NewPipeline(
		normalize.New(),
		alias.New(lex),
	)

	// -------------------------------------------------------------------------
	// 第 3 步：构造 Engine。
	//
	// WithLexicon + WithPipeline 是最简单的"单 Profile"模式。
	// New() 返回 error：如果配置错误（例如 Pipeline 为空），会立即报错（fail-fast）。
	// -------------------------------------------------------------------------
	engine, err := lexnorm.New(
		lexnorm.WithLexicon(lex),
		lexnorm.WithPipeline(pipeline),
	)
	if err != nil {
		panic(fmt.Errorf("new engine: %w", err))
	}

	// -------------------------------------------------------------------------
	// 第 4 步：调用 Normalize。
	//
	// Engine 是并发安全的；多次 Normalize 可在不同 goroutine 并行调用。
	// 返回值 Result 包含：
	//   - Text        最终文本
	//   - Original    原始输入
	//   - Changes     所有 Change（应用 / 建议 / 锁定）
	//   - Status      StatusOK / StatusPartial / StatusFailed
	// -------------------------------------------------------------------------
	text := "  周莉群  同事  小田  都说项目进展很顺利。  "
	result, err := engine.Normalize(context.Background(), text)
	if err != nil {
		panic(fmt.Errorf("normalize: %w", err))
	}

	fmt.Printf("输入: %q\n", text)
	fmt.Printf("输出: %q\n", result.Text)
	fmt.Printf("状态: %s\n", result.Status)
	fmt.Printf("改动: %d 条\n", len(result.Changes))
	fmt.Println("--- 详情 ---")
	for i, c := range result.Changes {
		fmt.Printf("  [%d] %s at %v: %q → %q\n",
			i, c.Source, c.Span, c.From, c.To)
	}

	// 期望输出：
	// 输入: "  周莉群  同事  小田  都说项目进展很顺利。  "
	// 输出: "周丽群 同事 田华 都说项目进展很顺利。"
	// 状态: success
	// 改动: 12 条
	// --- 详情 ---
	//   [0] normalize at Span[60,61): " " → ""
	//   [1] normalize at Span[59,60): " " → ""
	//   [2] normalize at Span[28,29): " " → ""
	//   [3] normalize at Span[27,28): " " → " "
	//   [4] normalize at Span[20,21): " " → ""
	//   [5] normalize at Span[19,20): " " → " "
	//   [6] normalize at Span[12,13): " " → ""
	//   [7] normalize at Span[11,12): " " → " "
	//   [8] normalize at Span[1,2): " " → ""
	//   [9] normalize at Span[0,1): " " → ""
	//   [10] alias at Span[2,11): "周莉群" → "周丽群"
	//   [11] alias at Span[21,27): "小田" → "田华"
}
