// Package main 演示如何自由组合 Processor 来构造自定义 Pipeline。
//
// 关键 API：
//   - lexnorm.NewPipeline(proc1, proc2, ...)
//   - disfluency.New().WithTokens("嗯", "呃")     // 自定义语气词
//   - Pipeline 是有序的：前面的 Processor 改写后的结果交给后面的 Processor
//
// 运行：go run ./example/02-pipeline
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
	// 第 1 步：构造 Lexicon。
	//
	// 这一步和 01-basic 一致。注意 deterministic 和 alias 都消费同一个 Lexicon，
	// 但用途不同：
	//   - alias：处理 VariantAlias / VariantCorrection
	//   - deterministic：处理跨词条硬规则（如"打开"→"打开"大小写统一）
	// -------------------------------------------------------------------------
	lex, _ := lexicon.NewBuilderWithVersion("v1").
		Add(lexicon.Entry{
			ID:   "name-zhouliqun",
			Text: "周丽群",
			Variants: []lexicon.Variant{
				{Text: "周莉群", Kind: lexicon.VariantAlias, Confidence: 1.0},
			},
		}).
		Add(lexicon.Entry{
			ID:   "tech-gateway",
			Text: "API 网关",
			Variants: []lexicon.Variant{
				{Text: "api网关", Kind: lexicon.VariantAlias, Confidence: 1.0},
				{Text: "网关服务", Kind: lexicon.VariantAlias, Confidence: 1.0},
			},
		}).
		Build()

	// -------------------------------------------------------------------------
	// 第 2 步：自定义语气词列表（默认是"嗯""呃""啊"等）。
	//
	// disfluency.New() 的零值就是默认列表（确定性，删即可），
	// 这里演示如何覆盖：把"呃呃呃""那个那个"也作为填充词删掉。
	// -------------------------------------------------------------------------
	disfluencyProc := disfluency.New().
		WithTokens("嗯", "呃", "啊", "那个", "这个", "呃呃呃", "那个那个")

	// -------------------------------------------------------------------------
	// 第 3 步：组合 Pipeline。
	//
	// 注意顺序：Normalize 必须先做（清洗输入），
	// 然后是 Disfluency（去语气词），
	// 然后才是 Alias / Deterministic（替换）。
	// -------------------------------------------------------------------------
	pipeline := lexnorm.NewPipeline(
		normalize.New(),
		disfluencyProc,
		alias.New(lex),
		deterministic.New(lex),
	)

	engine, err := lexnorm.New(
		lexnorm.WithLexicon(lex),
		lexnorm.WithPipeline(pipeline),
	)
	if err != nil {
		panic(err)
	}

	// -------------------------------------------------------------------------
	// 第 4 步：模拟一段口语化、有语气词、有 ASR 错词的输入。
	// -------------------------------------------------------------------------
	text := "  那个那个呃呃呃 周莉群 说呃 api网关 嗯已经呃呃上线了。  "
	result, err := engine.Normalize(context.Background(), text)
	if err != nil {
		panic(err)
	}

	fmt.Printf("输入: %q\n", text)
	fmt.Printf("输出: %q\n", result.Text)
	fmt.Printf("状态: %s\n", result.Status)
	fmt.Println("--- 改动（按 Processor 分组）---")
	bySource := map[string]int{}
	for _, c := range result.Changes {
		bySource[c.Source]++
	}
	for src, n := range bySource {
		fmt.Printf("  %s: %d 条\n", src, n)
	}

	// 期望输出：
	// 输入: "  那个那个呃呃呃 周莉群 说呃 api网关 嗯已经呃呃上线了。  "
	// 输出: " 周丽群 说 API 网关 已经上线了。"
	// 状态: success
	// --- 改动（按 Processor 分组）---
	//   normalize: 8 条
	//   disfluency: 6 条
	//   alias: 2 条
}
