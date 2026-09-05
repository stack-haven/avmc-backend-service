// Package textenhance · context.go
// 编排层的 EnhancementContext 重新导出。
//
// 真正的定义在 pkg/textenhance/processors/context.go（与 TextProcessor 同层）。
// 这里做 type alias 让 textenhance 根 package 的代码（Pipeline / Registry / tests）能直接
// 用 textenhance.EnhancementContext 这种短名，而 processor 子包用 processors.EnhancementContext。
package textenhance

import (
	"backend-service/pkg/textenhance/processors"
)

// EnhancementContext = processors.EnhancementContext（type alias）。
type EnhancementContext = processors.EnhancementContext

// Change = processors.Change（type alias）。
type Change = processors.Change

// 重新导出常用常量（避免 test 写长链 processors.ActionKeep）。
const (
	ActionKeep    = processors.ActionKeep
	ActionReplace = processors.ActionReplace
	ActionDelete  = processors.ActionDelete
	ActionSuggest = processors.ActionSuggest
	ActionResolve = processors.ActionResolve
)

// VocabularySnapshot = processors.VocabularySnapshot（type alias）。
type VocabularySnapshot = processors.VocabularySnapshot

// VocabularyEntry = processors.VocabularyEntry。
type VocabularyEntry = processors.VocabularyEntry

// VocabularyRelation = processors.VocabularyRelation。
type VocabularyRelation = processors.VocabularyRelation

// NewVocabularySnapshot 转发。
func NewVocabularySnapshot(entries []*processors.VocabularyEntry, relations []*processors.VocabularyRelation) *processors.VocabularySnapshot {
	return processors.NewVocabularySnapshot(entries, relations)
}

// EmptyVocabularySnapshot 转发。
func EmptyVocabularySnapshot() *processors.VocabularySnapshot {
	return processors.EmptyVocabularySnapshot()
}

// NewEnhancementContext 转发（type alias 允许直接用）。
func NewEnhancementContext(rawText string, vocab *VocabularySnapshot, policy PolicyReader) *EnhancementContext {
	return processors.NewEnhancementContext(rawText, vocab, policy)
}

// PolicyReader 接口转发。
type PolicyReader = processors.PolicyReader