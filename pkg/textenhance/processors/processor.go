// Package processors 是 textenhance 的策略层（Strategy Layer）。
//
// 包含：
//   - TextProcessor 统一接口（策略模式核心）
//   - Change / constants（策略输出与字段约定）
//   - common（公共工具、类型、依赖）
//   - 9 个默认 Processor 子包（cleaning / filler / ...）
//
// Pipeline / Registry / Policy / Snapshot / EnhancementContext / Status
// 等编排层结构保留在 pkg/textenhance/ 根 package（不依赖具体 processor）。
package processors

import "context"

// TextProcessor 是所有文本处理策略的统一接口。
//
// 实现规则（强约束）：
//   1. 构造后必须不可变（无 setter / 无导出字段可写）
//   2. 资源通过 NewXxxProcessor(opts ...Option) 注入；禁止构造后再加载
//   3. Process 不允许 panic（Pipeline 会 recover，但污染 ec.Errors）
//   4. Process 不允许做 I/O（vocab 来自 ec.Vocab snapshot；正则/词典在构造时打包）
//   5. Process 必须尊重 ctx 取消（重活前 select ctx.Done()）
//   6. 实例可被多个 goroutine 共享（构造后只读）
type TextProcessor interface {
	// Name 返回 processor 标识（与 Policy.IsEnabled 的 key 对齐）。
	// 命名规范：小写 + 下划线（如 "cleaning" / "vocab_matching"）。
	Name() string

	// Process 对 ec 原地修改（Text / Changes / LockedSpans / Timings / Errors）。
	// 不返回 error：内部 warn + 累积到 ec.Errors（HA 决定）。
	Process(ctx context.Context, ec *EnhancementContext)
}

// 编译期断言：nil interface 不允许实例化
var _ TextProcessor = (TextProcessor)(nil)

// EnhancementContext 是 per-request 不可变状态（来自 pkg/textenhance）。
//
// 此处仅为 type alias / 转发引用，保持 processors 包的零外部业务依赖；
// 真正的 EnhancementContext 定义在 pkg/textenhance/ 根 package。
// 处理器代码直接使用 textenhance.EnhancementContext 即可，无需在此 alias。