// Package processors · change.go
// Change 单条文本修改（processor 输出）。
//
// 字段与 evie/service EnhancementChange 对齐；调用方（biz / proto 层）按需
// 转换为自有 DTO（如 v1.EnhanceChange）。
package processors

// Change 记录一次 processor 产生的文本修改。
type Change struct {
	From       string  // 原文片段
	To         string  // 修改后片段（空表示删除）
	Action     string  // KEEP / REPLACE / DELETE / SUGGEST / RESOLVE
	Type       string  // CLEAN / FILLER / ALIAS / CORRECTION / PINYIN / FUZZY / CONTEXT / PHRASE
	Source     string  // SYSTEM / TENANT_DICTIONARY / ...
	Confidence float64
	Locked     bool
	Reason     string
}