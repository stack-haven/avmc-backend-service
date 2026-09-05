// Package biz · policy.go
// 文本增强策略层：conf.Enhancement → textenhance.Policy 转换 + EnhancementUsecase。
package biz

import (
	"backend-service/pkg/textenhance"

	"backend-service/app/evie/tool/internal/conf"
)

// NewPolicyFromConf 从 conf.Enhancement 构造 textenhance.Policy。
//
// 转换规则：
//   - conf.Pipeline 列表（空 = 使用默认 STANDARD）
//   - conf.PinyinThreshold / FuzzyAutoThreshold / FuzzySuggestThreshold 范围 [0, 1]
//   - conf.LLMEnabled 是否启用 LLM 步骤
//
// 任何字段非法都走 textenhance.Policy.Validate() 的 clamp / 降级。
func NewPolicyFromConf(c *conf.Enhancement) *textenhance.Policy {
	if c == nil {
		return textenhance.DefaultPolicy()
	}
	p := &textenhance.Policy{
		EnabledProcessors:     append([]string(nil), c.Pipeline...),
		LLMEnabled:            c.LlmEnabled,
		PinyinThreshold:        c.PinyinThreshold,
		FuzzyAutoThreshold:     c.FuzzyAutoThreshold,
		FuzzySuggestThreshold:  c.FuzzySuggestThreshold,
	}
	validated, _ := p.Validate()
	return validated
}