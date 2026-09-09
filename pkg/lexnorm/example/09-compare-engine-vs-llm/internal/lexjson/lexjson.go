// Package lexjson 提供本 example 自定义的 JSON 词库 ↔ ark-lexnorm lexicon.Entry 互转。
//
// 我们的 JSON schema 与 lexnorm.Entry 并不完全一致（多 pinyin、kind 用字符串等），
// 这里负责把它们桥接起来。
package lexjson

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/stack-haven/lexnorm/lexicon"
)

// VariantJSON / EntryJSON / LexiconJSON 与 cmd/02-transform 的输出格式一致。
type VariantJSON struct {
	Text       string  `json:"text"`
	Kind       string  `json:"kind"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source,omitempty"`
}

type EntryJSON struct {
	ID       string         `json:"id"`
	Text     string         `json:"text"`
	Pinyin   string         `json:"pinyin,omitempty"`
	Variants []VariantJSON  `json:"variants"`
	Meta     map[string]any `json:"meta,omitempty"`
}

type LexiconJSON struct {
	Version string         `json:"version"`
	Source  string         `json:"source"`
	Entries []EntryJSON    `json:"entries"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// LoadFile 读取 JSON 词库文件并返回 LexiconJSON。
func LoadFile(path string) (*LexiconJSON, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var lex LexiconJSON
	if err := json.Unmarshal(b, &lex); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &lex, nil
}

// ToEntry 把 EntryJSON 转为 lexnorm Entry。
func ToEntry(e EntryJSON) (lexicon.Entry, error) {
	if e.ID == "" {
		return lexicon.Entry{}, fmt.Errorf("entry has empty id: text=%q", e.Text)
	}
	if e.Text == "" {
		return lexicon.Entry{}, fmt.Errorf("entry %q has empty text", e.ID)
	}
	out := lexicon.Entry{
		ID:   lexicon.EntryID(e.ID),
		Text: e.Text,
		Meta: e.Meta,
	}
	for i, v := range e.Variants {
		if v.Text == "" {
			continue
		}
		kind, ok := parseKind(v.Kind)
		if !ok {
			return lexicon.Entry{}, fmt.Errorf("entry %q variant[%d] unknown kind %q", e.ID, i, v.Kind)
		}
		conf := v.Confidence
		if conf == 0 {
			conf = defaultConfidenceFor(kind)
		}
		out.Variants = append(out.Variants, lexicon.Variant{
			Text:       v.Text,
			Kind:       kind,
			Confidence: conf,
			Source:     v.Source,
		})
	}
	return out, nil
}

// ToLexiconEntries 批量转换为 lexnorm Entry。
func ToLexiconEntries(items []EntryJSON) ([]lexicon.Entry, error) {
	out := make([]lexicon.Entry, 0, len(items))
	for i, e := range items {
		le, err := ToEntry(e)
		if err != nil {
			return nil, fmt.Errorf("entries[%d]: %w", i, err)
		}
		out = append(out, le)
	}
	return out, nil
}

// ToSource 把一个 LexiconJSON 转为 lexicon.SliceSource。
func ToSource(lex *LexiconJSON) (lexicon.LexiconSource, error) {
	entries, err := ToLexiconEntries(lex.Entries)
	if err != nil {
		return nil, err
	}
	return lexicon.NewSliceSource(entries, nil, lex.Version), nil
}

func parseKind(s string) (lexicon.VariantKind, bool) {
	switch s {
	case "alias":
		return lexicon.VariantAlias, true
	case "correction":
		return lexicon.VariantCorrection, true
	case "homophone":
		return lexicon.VariantHomophone, true
	case "approximate":
		return lexicon.VariantApproximate, true
	}
	return 0, false
}

// 默认 confidence（与 fuzzy processor 的 silent 行为对应）。
//   - alias / correction: 1.0（高确定性）
//   - homophone / approximate: 0.85（默认高确定性，便于 fuzzy 命中）
func defaultConfidenceFor(k lexicon.VariantKind) float64 {
	switch k {
	case lexicon.VariantAlias, lexicon.VariantCorrection:
		return 1.0
	case lexicon.VariantHomophone, lexicon.VariantApproximate:
		return 0.85
	}
	return 0.85
}