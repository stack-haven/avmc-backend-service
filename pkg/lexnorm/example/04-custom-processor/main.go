// Package main 演示如何实现一个自定义 Processor。
//
// 关键 API：
//   - 实现 lexnorm.Processor 接口（Name + Process 两个方法）
//   - 用 s.Replace(span, replacement, meta) 修改文本
//   - 用 s.Original() 拿原始字节切片
//   - 用 s.Suggest(span, replacement, meta) 提议但不应用
//
// 示例 Processor：把"非常"替换为"特别"——一个完全无业务价值的演示，
// 只为展示接口实现方式。
//
// 运行：go run ./example/04-custom-processor
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
)

// veryEnhancer 是一个示例 Processor：把"非常"替换为"特别"。
type veryEnhancer struct{}

// Name 实现 lexnorm.Processor。
func (veryEnhancer) Name() string { return "very-enhancer" }

// Certainty 实现 lexnorm.CertaintyReporter（可选）。
// 返回 CertaintyHigh 表示这个 Processor 的替换是高置信度的。
func (veryEnhancer) Certainty() lexnorm.Certainty { return lexnorm.CertaintyHigh }

// Process 实现 lexnorm.Processor。
//
// 实际查找：遍历 s.Original() 找所有"非常"出现的位置，依次 Replace。
func (veryEnhancer) Process(_ context.Context, s *lexnorm.State) error {
	const target = "非常"
	const replacement = "特别"

	original := s.Original()
	meta := lexnorm.ChangeMeta{
		Source:     "very-enhancer",
		Confidence: 1.0,
		Reason:     "将'非常'替换为'特别'",
	}

	// 收集所有匹配，最后从右到左应用。
	type match struct{ start, end int }
	var matches []match

	idx := 0
	for {
		i := strings.Index(original[idx:], target)
		if i < 0 {
			break
		}
		start := idx + i
		matches = append(matches, match{start, start + len(target)})
		idx = start + len(target)
	}

	// 倒序应用，保证之前的 Original 偏移不会被后续 Replace 影响。
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		if err := s.Replace(
			lexnorm.Span{Start: m.start, End: m.end},
			replacement,
			meta,
		); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	// 构造一个空的 Lexicon（Processor 不需要查词典，但 Engine 要求有 Lexicon）。
	emptyLex, _ := lexicon.NewBuilderWithVersion("v1").Build()

	engine, err := lexnorm.New(
		lexnorm.WithLexicon(emptyLex),
		lexnorm.WithPipeline(lexnorm.NewPipeline(
			veryEnhancer{},
		)),
	)
	if err != nil {
		panic(err)
	}

	// -------------------------------------------------------------------------
	// 注意：自定义 Processor 可以独立运行（不依赖 Engine）。
	// 这是架构不变性 I1：Processor 不依赖 Engine 的状态。
	// -------------------------------------------------------------------------
	{
		s, _ := lexnorm.NewState(context.Background(), "我非常喜欢这个项目。", nil,
			lexnorm.DefaultConfig())
		p := veryEnhancer{}
		if procErr := p.Process(context.Background(), s); procErr != nil {
			panic(procErr)
		}
		fmt.Printf("独立运行: %q\n", s.Text())
	}

	// -------------------------------------------------------------------------
	// 也可以放进 Engine 的 Pipeline 里跑（最常见用法）。
	// -------------------------------------------------------------------------
	text := "他非常努力，工作非常出色，结果非常理想。"
	result, err := engine.Normalize(context.Background(), text)
	if err != nil {
		panic(err)
	}

	fmt.Printf("输入: %q\n", text)
	fmt.Printf("输出: %q\n", result.Text)
	fmt.Printf("状态: %s\n", result.Status)
	fmt.Println("改动:")
	for i, c := range result.Changes {
		fmt.Printf("  [%d] %s at %v: %q → %q\n",
			i, c.Source, c.Span, c.From, c.To)
	}

	// 期望输出：
	// 独立运行: "我特别喜欢这个项目。"
	// 输入: "他非常努力，工作非常出色，结果非常理想。"
	// 输出: "他特别努力，工作特别出色，结果特别理想。"
	// 状态: success
	// 改动:
	//   [0] very-enhancer at Span[45,51): "非常" → "特别"
	//   [1] very-enhancer at Span[24,30): "非常" → "特别"
	//   [2] very-enhancer at Span[3,9): "非常" → "特别"
}
