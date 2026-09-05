// Package textenhance · policy.go
// Policy：决定 processor 启停顺序 + 推断阈值。
//
// 设计要点：
//   1. 不可变（构造后只读）
//   2. nil policy = 全部启用（默认 STANDARD 行为）
//   3. 构造时校验 + clamp（HA：配置错误不阻断）
//   4. 9 个 processor 的固定顺序（与 evie/service 一致）
package textenhance

import "fmt"

// processorNames 全部 processor 的固定顺序（pipeline 必须按此顺序执行）。
//
// 顺序依据（与 evie/service 一致）：
//   1. cleaning           → 基础清洗
//   2. filler             → 口水词
//   3. vocab_matching     → 词库精确匹配（锁定）
//   4. alias_resolution   → 别名（确定性）
//   5. deterministic_replacement → 确定性替换
//   6. phrase_standardization → 短语标准化（确定性）
//   7. pinyin_correction  → 拼音纠错（推断）
//   8. fuzzy_matching     → 模糊匹配（推断）
//   9. context_correction → 上下文纠错（推断）
var DefaultProcessorOrder = []string{
	"cleaning",
	"filler",
	"vocab_matching",
	"alias_resolution",
	"deterministic_replacement",
	"phrase_standardization",
	"pinyin_correction",
	"fuzzy_matching",
	"context_correction",
	// "llm_reserved"  // 单独控制
}

// Policy 决定 processor 启用列表 + 推断阈值。
//
// 构造后不可变；多个 Pipeline 可共享同一 Policy 实例。
type Policy struct {
	// EnabledProcessors 按顺序的 processor 名列表。
	// 空切片 = 不启用任何 processor（= noop Pipeline）。
	// 顺序敏感：Pipeline 按此顺序执行。
	EnabledProcessors []string

	// LLMEnabled 是否启用 LLM 步骤（默认 false，保留位）。
	LLMEnabled bool

	// PinyinThreshold 拼音自动替换阈值（>= 阈值 REPLACE）。
	PinyinThreshold float64

	// FuzzyAutoThreshold 模糊匹配自动替换阈值。
	FuzzyAutoThreshold float64

	// FuzzySuggestThreshold 模糊匹配建议阈值（< 阈值 KEEP）。
	FuzzySuggestThreshold float64
}

// DefaultPolicy 返回标准策略（8 层流水线全启用，LLM 关闭）。
//
// 与 evie/service 的 STANDARD 模式一致。
func DefaultPolicy() *Policy {
	return &Policy{
		EnabledProcessors:     append([]string(nil), DefaultProcessorOrder...),
		LLMEnabled:            false,
		PinyinThreshold:        0.85,
		FuzzyAutoThreshold:     0.80,
		FuzzySuggestThreshold:  0.60,
	}
}

// IsEnabled 检查 processor 是否启用。
//
// nil policy = 全部启用（HA 默认行为：配置缺失时降级到全启用）。
func (p *Policy) IsEnabled(name string) bool {
	if p == nil {
		return true
	}
	for _, n := range p.EnabledProcessors {
		if n == name {
			return true
		}
	}
	return false
}

// EnabledOrder 返回按序的启用 processor 列表。
// 用于 BuildPipeline 按顺序构建实例。
func (p *Policy) EnabledOrder() []string {
	if p == nil {
		return append([]string(nil), DefaultProcessorOrder...)
	}
	out := make([]string, 0, len(p.EnabledProcessors))
	for _, n := range p.EnabledProcessors {
		// 仅返回 DefaultProcessorOrder 中存在的 processor（防御性）
		if !isKnownProcessor(n) {
			continue
		}
		out = append(out, n)
	}
	return out
}

func isKnownProcessor(name string) bool {
	for _, n := range DefaultProcessorOrder {
		if n == name {
			return true
		}
	}
	if name == "llm_reserved" {
		return true
	}
	return false
}

// Validate 构造时校验 + clamp 阈值。
//
// HA 行为：Policy 有问题不阻断；返回 clamp 后的 policy + 错误详情。
func (p *Policy) Validate() (*Policy, []error) {
	if p == nil {
		return DefaultPolicy(), nil
	}
	out := &Policy{
		EnabledProcessors:     append([]string(nil), p.EnabledProcessors...),
		LLMEnabled:            p.LLMEnabled,
		PinyinThreshold:       p.PinyinThreshold,
		FuzzyAutoThreshold:    p.FuzzyAutoThreshold,
		FuzzySuggestThreshold: p.FuzzySuggestThreshold,
	}
	var errs []error
	out.PinyinThreshold, errs = clampThreshold("pinyin_threshold", p.PinyinThreshold, 0.85, errs)
	out.FuzzyAutoThreshold, errs = clampThreshold("fuzzy_auto_threshold", p.FuzzyAutoThreshold, 0.80, errs)
	out.FuzzySuggestThreshold, errs = clampThreshold("fuzzy_suggest_threshold", p.FuzzySuggestThreshold, 0.60, errs)
	// 顺序校验：去重 + 保留 DefaultProcessorOrder 的次序
	out.EnabledProcessors = normalizeOrder(out.EnabledProcessors)
	return out, errs
}

// clampThreshold 把阈值限制在 [0, 1]；非法值回落 0 + warn。
func clampThreshold(name string, v, def float64, errs []error) (float64, []error) {
	if v < 0 || v > 1 {
		errs = append(errs, fmt.Errorf("policy: %s=%v out of [0,1], fallback to 0", name, v))
		return 0, errs
	}
	if v == 0 {
		return def, errs
	}
	return v, errs
}

// normalizeOrder 去重 + 按 DefaultProcessorOrder 排序。
// 未知 processor 名（不在白名单内）会被丢弃。
func normalizeOrder(in []string) []string {
	if len(in) == 0 {
		return append([]string(nil), DefaultProcessorOrder...)
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	// 优先按 DefaultProcessorOrder 排
	for _, n := range DefaultProcessorOrder {
		for _, x := range in {
			if x == n && !seen[n] {
				seen[n] = true
				out = append(out, n)
				break
			}
		}
	}
	// 处理 llm_reserved（特殊：不在 DefaultProcessorOrder 中）
	for _, x := range in {
		if !seen[x] && isKnownProcessor(x) {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}