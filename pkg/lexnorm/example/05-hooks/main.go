// Package main 演示如何用 Hook 监听 Engine 的事件，实现日志 / Trace /
// 指标等观测能力。
//
// 关键 API：
//   - lexnorm.WithHooks(hookFunc, ...)   // 注册多个 Hook
//   - hookFunc(Event)                   // 每次事件被调用
//   - Event.Type / Event.Processor / Event.Result / Event.State
//
// Hook 在以下时机被触发：
//   - pipeline-start   整个 Pipeline 开始
//   - processor-start  每个 Processor 开始前
//   - processor-end    每个 Processor 结束后（含 error）
//   - pipeline-end     整个 Pipeline 结束（Result 已构造）
//
// 运行：go run ./example/05-hooks
package main

import (
	"context"
	"fmt"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/alias"
	"github.com/stack-haven/lexnorm/processor/disfluency"
	"github.com/stack-haven/lexnorm/processor/normalize"
)

func main() {
	lex, _ := lexicon.NewBuilderWithVersion("v1").
		Add(lexicon.Entry{
			ID:   "name-zhouliqun",
			Text: "周丽群",
			Variants: []lexicon.Variant{
				{Text: "周莉群", Kind: lexicon.VariantAlias, Confidence: 1.0},
			},
		}).
		Build()

	engine, err := lexnorm.New(
		lexnorm.WithLexicon(lex),
		lexnorm.WithPipeline(lexnorm.NewPipeline(
			normalize.New(),
			disfluency.New(),
			alias.New(lex),
		)),
		lexnorm.WithHooks(
			// Hook 1：打印每个事件的精简日志（生产环境风格）。
			func(e lexnorm.Event) {
				switch e.Type {
				case lexnorm.EventPipelineStart:
					fmt.Printf("[trace] ▶ pipeline-start   text=%q\n",
						e.State.Original())
				case lexnorm.EventProcessorStart:
					fmt.Printf("[trace]   ▶ processor-start %s\n", e.Processor)
				case lexnorm.EventProcessorEnd:
					fmt.Printf("[trace]   ◀ processor-end   %s\n", e.Processor)
				case lexnorm.EventPipelineEnd:
					fmt.Printf("[trace] ◀ pipeline-end     result=%q, changes=%d\n",
						e.Result.Text, len(e.Result.Changes))
				}
			},

			// Hook 2：按 Processor 累计耗时和 Change 数（指标风格）。
			newMetricsHook(),
		),
	)
	if err != nil {
		panic(err)
	}

	text := "嗯 周莉群 说呃 项目进展很顺利。  "
	result, err := engine.Normalize(context.Background(), text)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Printf("最终输出: %q\n", result.Text)

	// 期望输出（事件顺序）：
	// [trace] ▶ pipeline-start   text="嗯 周莉群 说呃 项目进展很顺利。  "
	// [trace]   ▶ processor-start normalize
	// [trace]   ◀ processor-end   normalize
	// [metric] normalize: 5 changes
	// [trace]   ▶ processor-start disfluency
	// [trace]   ◀ processor-end   disfluency
	// [metric] disfluency: 7 changes
	// [trace]   ▶ processor-start alias
	// [trace]   ◀ processor-end   alias
	// [metric] alias: 8 changes
	// [trace] ◀ pipeline-end     result=" 周丽群 说 项目进展很顺利。", changes=8
	//
	// 最终输出: " 周丽群 说 项目进展很顺利。"
}

// metricsHook 演示如何用闭包累计每个 Processor 的 Change 数。
//
// 生产环境中更复杂的做法：在 EventProcessorStart 时记录 startTime，
// 在 EventProcessorEnd 时计算 elapsed，存入 Prometheus 指标。
func newMetricsHook() lexnorm.Hook {
	m := map[string]int{}
	return func(e lexnorm.Event) {
		if e.Type != lexnorm.EventProcessorEnd {
			return
		}
		m[e.Processor] = len(e.State.Changes())
		fmt.Printf("[metric] %s: %d changes\n",
			e.Processor, m[e.Processor])
	}
}
