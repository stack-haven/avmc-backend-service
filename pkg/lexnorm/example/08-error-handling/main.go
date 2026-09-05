// Package main 演示 ark-lexnorm 的错误处理与故障降级能力。
//
// 关键 API：
//   - lexnorm.Recover()              // Middleware：捕获 panic，降级为 ErrRuntime
//   - lexnorm.WrapProcessorError(proc) // 把 Processor 错误包成 Middleware
//   - lexnorm.ContinueOnErrorPolicy    // 让单个 Processor 失败不阻塞后续
//
// 场景：
//   - 一个 Processor panic：被 Recover 接住，降级到 Result.StatusPartial
//   - 一个 Processor 返回 error：被 ErrorPolicy 决定是否继续
//
// 运行：go run ./example/08-error-handling
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
)

// buggyProcessor 故意 panic，用于演示 Recover Middleware 的降级行为。
type buggyProcessor struct{}

func (buggyProcessor) Name() string                 { return "buggy" }
func (buggyProcessor) Certainty() lexnorm.Certainty { return lexnorm.CertaintyLow }

func (buggyProcessor) Process(_ context.Context, _ *lexnorm.State) error {
	panic("oops! 数据库连接失败")
}

// returningErrorProcessor 返回 error 而不是 panic。
type returningErrorProcessor struct{}

func (returningErrorProcessor) Name() string                 { return "error-returner" }
func (returningErrorProcessor) Certainty() lexnorm.Certainty { return lexnorm.CertaintyLow }

func (returningErrorProcessor) Process(_ context.Context, _ *lexnorm.State) error {
	return errors.New("临时性错误：网络抖动")
}

func main() {
	emptyLex, _ := lexicon.NewBuilderWithVersion("v1").Build()

	// -------------------------------------------------------------------------
	// 场景 A：自定义 Processor panic，用 Recover 中间件兜底。
	//
	// Recover() 必须放在 WithMiddleware 链的最外层，
	// 才能捕获到所有 Processor 的 panic。
	// -------------------------------------------------------------------------
	{
		engine, err := lexnorm.New(
			lexnorm.WithLexicon(emptyLex),
			lexnorm.WithPipeline(lexnorm.NewPipeline(
				buggyProcessor{},
			)),
			lexnorm.WithMiddleware(lexnorm.Recover()),
		)
		if err != nil {
			panic(err)
		}

		result, err := engine.Normalize(context.Background(), "正常输入文本")
		fmt.Println("=== 场景 A：Processor panic ===")
		fmt.Printf("err 类型: %T\n", err)
		fmt.Printf("Result.Status: %s\n", result.Status)
		fmt.Printf("Result.HasErrors: %v\n", result.HasErrors())
		fmt.Printf("Result.Text: %q\n", result.Text)
		fmt.Println()
	}

	// -------------------------------------------------------------------------
	// 场景 B：自定义 Processor 返回 error，Engine 把它作为软错误
	//         记录到 Result 里，正常返回给调用方。
	// -------------------------------------------------------------------------
	{
		engine, err := lexnorm.New(
			lexnorm.WithLexicon(emptyLex),
			lexnorm.WithPipeline(lexnorm.NewPipeline(
				returningErrorProcessor{},
			)),
		)
		if err != nil {
			panic(err)
		}

		result, err := engine.Normalize(context.Background(), "正常输入文本")
		fmt.Println("=== 场景 B：Processor 返回 error ===")
		fmt.Printf("err: %v\n", err)
		fmt.Printf("Result.Status: %s\n", result.Status)
		fmt.Printf("Result.HasErrors: %v\n", result.HasErrors())
		fmt.Printf("Result.Text: %q\n", result.Text)
		fmt.Println()
	}

	// -------------------------------------------------------------------------
	// 场景 C：用 WrapProcessorError 工具函数包装错误后做错误分类。
	//
	// WrapProcessorError(name, op, err) 不是一个 Middleware，
	// 而是一个辅助函数，让你自定义 Processor 在返回 error 时
	// 携带 Processor 名称和操作上下文，便于日志和错误归因。
	//
	// 如果你想让 error 同时被 lexnorm 的 sentinel 体系识别，
	// 让 inner error 直接用 ErrRuntime：
	// -------------------------------------------------------------------------
	{
		// 方式 1：直接包任意错误（用于日志归因）
		rawErr := errors.New("底层连接失败")
		wrapped := lexnorm.WrapProcessorError("my-processor", "fetch-lexicon", rawErr)
		fmt.Println("=== 场景 C：WrapProcessorError 错误归一化 ===")
		fmt.Printf("wrapped err: %v\n", wrapped)
		fmt.Printf("错误类型: %T\n", wrapped)
		fmt.Printf("errors.Is(wrapped, lexnorm.ErrRuntime): %v (raw err 不是 ErrRuntime)\n",
			errors.Is(wrapped, lexnorm.ErrRuntime))

		// 方式 2：包 ErrRuntime sentinel（让 errors.Is 生效）
		wrapped2 := lexnorm.WrapProcessorError("my-processor", "fetch-lexicon", lexnorm.ErrRuntime)
		fmt.Printf("包 ErrRuntime 后 errors.Is: %v\n",
			errors.Is(wrapped2, lexnorm.ErrRuntime))
		fmt.Println()
	}

	// -------------------------------------------------------------------------
	// 场景 D：多层 Processor 时，某个失败不影响后续。
	// -------------------------------------------------------------------------
	{
		engine, err := lexnorm.New(
			lexnorm.WithLexicon(emptyLex),
			lexnorm.WithPipeline(lexnorm.NewPipeline(
				returningErrorProcessor{}, // 第一个会返回 error
				// 第二个 Processor 即使在 errorPolicy=ContinueOnError 下
				// 也可能运行——这取决于 ErrorPolicy 的具体语义。
			)),
		)
		if err != nil {
			panic(err)
		}
		result, _ := engine.Normalize(context.Background(), "测试")
		fmt.Println("=== 场景 D：单 Processor 错误不中断 ===")
		fmt.Printf("Result.Status: %s, Text=%q\n", result.Status, result.Text)
	}

	// 期望输出（具体 stack trace 可能不同）：
	// === 场景 A：Processor panic ===
	// err 类型: <nil>  （Recover 把它吞了，记入 Result）
	// Result.Status: partial
	// Result.HasErrors: true
	// Result.Text: "正常输入文本"  （panic 前没改过）
	//
	// === 场景 B：Processor 返回 error ===
	// err: <nil>                （软错误记录到 Result，不向上抛）
	// Result.Status: partial
	// Result.HasErrors: true
	// Result.Text: "正常输入文本"
	//
	// === 场景 C：WrapProcessorError 错误归一化 ===
	// wrapped err: processor my-processor in fetch-lexicon: 底层连接失败
	// 错误类型: *lexnorm.ProcessorError
	// errors.Is(wrapped, lexnorm.ErrRuntime): false (raw err 不是 ErrRuntime)
	// 包 ErrRuntime 后 errors.Is: true
	//
	// === 场景 D：单 Processor 错误不中断 ===
	// Result.Status: partial, Text="测试"
}
