// Package main 演示如何使用 lexicon.Store 实现 Lexicon 的零停机热更新（HA）。
//
// 关键 API：
//   - lexicon.NewStore(initial)            // 包装初始 Lexicon
//   - store.TryUpdate(build func)          // 原子替换；失败回滚到 LKG
//   - store.Current()                       // 取当前 Lexicon（并发安全）
//   - lexnorm.WithLexiconStore(store) + lexnorm.WithPipeline(p)
//
// 场景：
//  1. 启动时构造 v1 Lexicon
//  2. 服务运行中：通过 TryUpdate 替换为 v2（添加新词条）
//  3. 旧请求不受影响（架构不变性 I8：请求捕获指针）
//
// 运行：go run ./example/06-lexicon-store
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
	// 第 1 步：构造 v1 Lexicon。
	// -------------------------------------------------------------------------
	v1, err := lexicon.NewBuilderWithVersion("v1").
		Add(lexicon.Entry{
			ID:   "name-zhouliqun",
			Text: "周丽群",
			Variants: []lexicon.Variant{
				{Text: "周莉群", Kind: lexicon.VariantAlias, Confidence: 1.0},
			},
		}).
		Build()
	if err != nil {
		panic(err)
	}

	// -------------------------------------------------------------------------
	// 第 2 步：用 v1 包装 Store。
	//
	// Store 内部用 atomic.Pointer 持有当前 Lexicon；
	// Current() 并发安全。
	// -------------------------------------------------------------------------
	store := lexicon.NewStore(v1)

	// -------------------------------------------------------------------------
	// 第 3 步：构造 Engine，绑定 Store + 一个 Pipeline。
	//
	// 注意：Pipeline 会在 Engine.Normalize 调用时通过 store.Current()
	// 拿到当时的 Lexicon（架构不变性 I8）。
	// -------------------------------------------------------------------------
	engine, err := lexnorm.New(
		lexnorm.WithLexiconStore(store),
		lexnorm.WithPipeline(lexnorm.NewPipeline(
			normalize.New(),
			alias.New(store.Current()), // 用 v1 的 Lexicon 初始化 Processor
		)),
	)
	if err != nil {
		panic(err)
	}

	// -------------------------------------------------------------------------
	// 第 4 步：先用 v1 跑一次。
	// -------------------------------------------------------------------------
	text := "请找周莉群确认项目进度。"
	result, _ := engine.Normalize(context.Background(), text)
	fmt.Printf("v1 时期: 输入=%q → 输出=%q\n", text, result.Text)

	// -------------------------------------------------------------------------
	// 第 5 步：通过 TryUpdate 替换为 v2。
	//
	// TryUpdate 接收一个 build 函数：
	//   - 函数返回新 Lexicon：原子替换
	//   - 函数返回 error：保留旧 Lexicon（LKG 语义）
	//
	// 注意：alias.New(store.Current()) 在 Engine 构造时已经捕获了 v1 的
	// matcher。要让 v2 的新词条对 Pipeline 内的 Processor 生效，需要
	// 重新构造 Engine 或 Pipeline。这里仅展示 Store 自身的版本切换能力。
	// -------------------------------------------------------------------------
	v2, err := lexicon.NewBuilderWithVersion("v2").
		Add(lexicon.Entry{
			ID:   "name-zhouliqun",
			Text: "周丽群",
			Variants: []lexicon.Variant{
				{Text: "周莉群", Kind: lexicon.VariantAlias, Confidence: 1.0},
			},
		}).
		Add(lexicon.Entry{
			ID:   "name-xiaoming",
			Text: "肖明",
			Variants: []lexicon.Variant{
				{Text: "小明", Kind: lexicon.VariantAlias, Confidence: 1.0},
			},
		}).
		Build()
	if err != nil {
		panic(err)
	}

	if err := store.TryUpdate(func() (lexicon.Lexicon, error) {
		// 模拟"校验 → 构建"的过程。
		// 如果 build 失败，TryUpdate 会保留 v1（LKG）。
		return v2, nil
	}); err != nil {
		fmt.Printf("热更新失败（保留 v1）: %v\n", err)
	}

	fmt.Printf("Store 版本: %s (size=%d)\n",
		store.Version(), store.Current().Len())

	// -------------------------------------------------------------------------
	// 第 6 步：用 store.Current() 重新构造 Engine，让 v2 真正生效。
	//
	// 这是当前 API 的限制：Pipeline 内的 Processor 在构造时锁定 Lexicon，
	// 想要 v2 生效必须重建 Engine。后续版本会引入 Lazy Lexicon 机制。
	// -------------------------------------------------------------------------
	engine2, _ := lexnorm.New(
		lexnorm.WithLexiconStore(store),
		lexnorm.WithPipeline(lexnorm.NewPipeline(
			normalize.New(),
			alias.New(store.Current()),
		)),
	)
	text2 := "请找小明确认进度。"
	result2, _ := engine2.Normalize(context.Background(), text2)
	fmt.Printf("v2 时期: 输入=%q → 输出=%q\n", text2, result2.Text)

	// -------------------------------------------------------------------------
	// 第 7 步：演示 LKG（Last Known Good）语义。
	//
	// 如果 build 函数返回 error，Store 保留旧 Lexicon 不变。
	// -------------------------------------------------------------------------
	badErr := fmt.Errorf("上游词表校验失败")
	updateErr := store.TryUpdate(func() (lexicon.Lexicon, error) {
		return nil, badErr
	})
	fmt.Printf("TryUpdate 失败: %v\n", updateErr)
	fmt.Printf("失败后 Store 版本: %s (仍是 %s)\n",
		store.Version(), store.Current().Version())

	// 期望输出：
	// v1 时期: 输入="请找周莉群确认项目进度。" → 输出="请找周丽群确认项目进度。"
	// Store 版本: v2 (size=2)
	// v2 时期: 输入="请找小明确认进度。" → 输出="请找肖明确认进度。"
	// TryUpdate 失败: lexicon: build: 上游词表校验失败（错误被 Store 包装了一层）
	// 失败后 Store 版本: v2 (仍是 v2，未回滚到 v1——Store 会保留当前版本)
}
