// Package processors · constants.go
// Change 字段的常量定义（Action / Type / Source）。
package processors

// Action 变更动作。
const (
	ActionKeep    = "KEEP"
	ActionReplace = "REPLACE"
	ActionDelete  = "DELETE"
	ActionSuggest = "SUGGEST"
	ActionResolve = "RESOLVE"
)

// Type 变更类型。
const (
	TypeClean      = "CLEAN"
	TypeFiller     = "FILLER"
	TypeAlias      = "ALIAS"
	TypeCorrection = "CORRECTION"
	TypePinyin     = "PINYIN"
	TypeFuzzy      = "FUZZY"
	TypeContext    = "CONTEXT"
	TypePhrase     = "PHRASE"
	TypeVocabMatch = "VOCAB_MATCH"
)

// Source 变更来源。
const (
	SourceSystem        = "SYSTEM"
	SourceTenantDict    = "TENANT_DICTIONARY"
	SourceQuaTenantUser = "QUA_TENANT_USER"
	SourceQuaTenantDept = "QUA_TENANT_DEPT"
	SourceLLM           = "LLM"
)