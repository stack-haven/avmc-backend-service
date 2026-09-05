// Package biz · vocab_normalizer.go
// 通用词条规范化器：按 YAML 规则把 RawEntity → NormalizedEntry。
//
// 规则结构（与 conf proto 对齐，参考 conf.Bootstrap.VocabRules）：
//
//   sources:
//     qua:
//       entity_mappings:
//         - match: { entity_type: "user" }
//           emit:
//             standard_text: "realName"          # dot-path
//             category: "PERSON"
//             aliases: ["nickname", "alias"]
//             pinyin_hint: "realName"
//             include_when: "status==1"          # 简单表达式
//
// 设计要点：
//   1. Normalizer 是纯函数（输入 RawEntity → 输出 NormalizedEntry），无副作用
//   2. 字段路径采用 dot-notation（realName → data["realName"]；支持任意嵌套）
//   3. IncludeWhen 支持 == / != / 真值 三类简单条件
//   4. 规则不存在 / 字段缺失 → 跳过该实体（warn 不阻断，C 决定）
//   5. 同一 source 多个 entity_type 共存（user/department 各一条规则）
package biz

import (
	"fmt"
	"strconv"
	"strings"
)

// NormalizedEntry 规范化后的通用词条（VocabularyBuilder 直接消费）。
//
// 一旦 Normalizer 完成转换，下游不再区分来源；这保证：
//   - VocabularyBuilder / EnhancementEngine 不知道外部系统的存在
//   - 加新 source = 新 adapter + 新规则 YAML；零核心代码变更
type NormalizedEntry struct {
	StandardText string
	Category     string
	Aliases      []string
	PinyinHint   string
	Priority     int32
	Source       string
	SourceID     string
}

// EntityMapping 一条 entity_type 的字段映射规则。
type EntityMapping struct {
	Match MatchCondition
	Emit  EmitSpec
}

// MatchCondition 匹配条件。
type MatchCondition struct {
	EntityType string // 匹配 RawEntity.EntityType
}

// EmitSpec 输出规范：定义如何从 RawEntity.Data 提取字段。
type EmitSpec struct {
	StandardText string   // 必填：dot-path → standard_text
	Category     string   // 必填：分类
	Aliases      []string // 可选：dot-path 列表
	PinyinHint   string   // 可选：dot-path；空表示不派生拼音
	Priority     string   // 可选：dot-path 或字面量（"100"）
	IncludeWhen  string   // 可选：简单条件表达式；空 = 总是包含
}

// SourceRules 一个来源的全部规则。
type SourceRules struct {
	Source         string
	EntityMappings []EntityMapping
}

// RuleSet 全部来源的规则集合（从 YAML / 配置文件加载）。
type RuleSet struct {
	Sources map[string]*SourceRules // key = source name
}

// Normalizer 按 RuleSet 把 RawEntity 规范化为 NormalizedEntry。
//
// Normalizer 本身**无状态**：所有规则通过构造时注入的 RuleSet 持有。
// Normalizer 实例可安全并发使用（Flyweight 模式）。
type Normalizer struct {
	rules *RuleSet
	log   WarnLogger // 可选；nil = 静默跳过
}

// WarnLogger warn logger 最小接口（避免 Normalizer 依赖具体 logger 类型）。
type WarnLogger interface {
	Warnf(format string, args ...interface{})
}

// NewNormalizer 构造 Normalizer。
func NewNormalizer(rs *RuleSet) *Normalizer {
	return &Normalizer{rules: rs}
}

// NewNormalizerWithLogger 构造带 warn logger 的 Normalizer（warn 不阻断，C 决定）。
func NewNormalizerWithLogger(rs *RuleSet, logger WarnLogger) *Normalizer {
	return &Normalizer{rules: rs, log: logger}
}

// Rules 返回底层 RuleSet（调试 / 运维接口读取）。
func (n *Normalizer) Rules() *RuleSet {
	return n.rules
}

// Normalize 规范化单个 RawEntity。
//
// 返回值：
//   - (*NormalizedEntry, true, nil)：成功
//   - (nil, false, nil)：规则不存在 / 字段缺失 / 条件不满足（业务跳过，非错误）
//   - (nil, false, error)：配置错误（如 dot-path 语法错、规则格式异常）
func (n *Normalizer) Normalize(raw RawEntity) (*NormalizedEntry, bool, error) {
	if n.rules == nil {
		if n.log != nil {
			n.log.Warnf("normalizer: rule set is nil, skipping raw entity source=%s type=%s id=%s",
				raw.Source, raw.EntityType, raw.SourceID)
		}
		return nil, false, nil
	}
	sr, ok := n.rules.Sources[raw.Source]
	if !ok {
		// 无规则配置视为 warn，不阻断
		if n.log != nil {
			n.log.Warnf("normalizer: no rules for source=%s, skipping raw id=%s",
				raw.Source, raw.SourceID)
		}
		return nil, false, nil
	}

	// 找到匹配的 entity_type 规则
	var mapping *EntityMapping
	for i := range sr.EntityMappings {
		if sr.EntityMappings[i].Match.EntityType == raw.EntityType {
			mapping = &sr.EntityMappings[i]
			break
		}
	}
	if mapping == nil {
		if n.log != nil {
			n.log.Warnf("normalizer: no mapping for source=%s entity_type=%s id=%s",
				raw.Source, raw.EntityType, raw.SourceID)
		}
		return nil, false, nil
	}

	// 1. include_when 过滤
	if mapping.Emit.IncludeWhen != "" {
		pass, err := evalCondition(mapping.Emit.IncludeWhen, raw.Data)
		if err != nil {
			// 配置错误（C：warn 不阻断）
			if n.log != nil {
				n.log.Warnf("normalizer: include_when eval error source=%s type=%s id=%s err=%v",
					raw.Source, raw.EntityType, raw.SourceID, err)
			}
			return nil, false, nil
		}
		if !pass {
			return nil, false, nil
		}
	}

	// 2. standard_text 必填
	stdText, ok := lookupPath(raw.Data, mapping.Emit.StandardText)
	if !ok || stdText == "" {
		if n.log != nil {
			n.log.Warnf("normalizer: standard_text=%q missing source=%s id=%s",
				mapping.Emit.StandardText, raw.Source, raw.SourceID)
		}
		return nil, false, nil
	}

	// 3. 收集别名（去重 + 排除与 standard_text 相同）
	aliases := make([]string, 0, len(mapping.Emit.Aliases))
	seen := map[string]bool{stdText: true}
	for _, path := range mapping.Emit.Aliases {
		if v, ok := lookupPath(raw.Data, path); ok && v != "" && !seen[v] {
			aliases = append(aliases, v)
			seen[v] = true
		}
	}

	// 4. 拼音提示（默认 standard_text；显式覆盖时取 dot-path 值）
	pinyinHint := stdText
	if mapping.Emit.PinyinHint != "" {
		if v, ok := lookupPath(raw.Data, mapping.Emit.PinyinHint); ok && v != "" {
			pinyinHint = v
		}
	}

	// 5. 优先级（dot-path 值优先，回退到字面量）
	var priority int32
	if mapping.Emit.Priority != "" {
		if v, ok := lookupPath(raw.Data, mapping.Emit.Priority); ok {
			priority = toInt32(v)
		} else {
			priority = toInt32(mapping.Emit.Priority) // 字面量
		}
	}

	return &NormalizedEntry{
		StandardText: stdText,
		Category:     mapping.Emit.Category,
		Aliases:      aliases,
		PinyinHint:   pinyinHint,
		Priority:     priority,
		Source:       raw.Source,
		SourceID:     raw.SourceID,
	}, true, nil
}

// NormalizeBatch 批量规范化（输入可能很大，但 Normalizer 是纯函数）。
//
// 跳过被规则过滤的实体（warn），不返回 error。
// 仅当配置错误时返回 error。
func (n *Normalizer) NormalizeBatch(raws []RawEntity) ([]*NormalizedEntry, error) {
	out := make([]*NormalizedEntry, 0, len(raws))
	for _, raw := range raws {
		entry, ok, err := n.Normalize(raw)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, entry)
		}
	}
	return out, nil
}

// ============================================================================
// 内部辅助：dot-path 访问 + 简单条件
// ============================================================================

// lookupPath 按 dot-path 在嵌套 map 中查找。
//
// 示例：
//   "realName"             → data["realName"]
//   "user.realName"        → data["user"]["realName"]
//   "userInfo.nickname"    → data["userInfo"]["nickname"]
//
// 返回值：找到的值（统一转 string）+ 是否存在。
// 非 string 值（数字、bool、嵌套 map/数组）会被 fmt.Sprintf 处理。
func lookupPath(data map[string]any, path string) (string, bool) {
	if data == nil || path == "" {
		return "", false
	}
	parts := strings.Split(path, ".")
	var cur any = data
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		v, exists := m[p]
		if !exists {
			return "", false
		}
		cur = v
	}
	if cur == nil {
		return "", false
	}
	s, ok := cur.(string)
	if !ok {
		return fmt.Sprintf("%v", cur), true
	}
	return s, true
}

// evalCondition 评估简单条件表达式。
//
// 支持的语法（v1，不支持完整 CEL）：
//   "field.path==1"          数字相等（值已被 lookupPath 转 string）
//   "field.path=='literal'" 字符串相等（自动去 ' / " 包裹）
//   "field.path==true"       布尔相等（lookupPath 转为 "true" / "false"）
//   "field.path!=value"      不等
//   "field.path"             真值（非空 / 非 0 / 非 false）
//
// 设计取舍：保留简单语义，避免引入 CEL 库；M9 阶段评估是否升级。
func evalCondition(expr string, data map[string]any) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true, nil
	}

	// 解析操作符
	var op, left, right string
	for _, cand := range []string{"==", "!="} {
		if idx := strings.Index(expr, cand); idx >= 0 {
			op = cand
			left = strings.TrimSpace(expr[:idx])
			right = strings.TrimSpace(expr[idx+len(cand):])
			break
		}
	}

	if op == "" {
		// 单 token：真值判断
		v, ok := lookupPath(data, expr)
		return ok && isTruthy(v), nil
	}

	lv, lok := lookupPath(data, left)
	if !lok {
		// 字段缺失 → 视为 false（更安全的默认）
		return false, nil
	}

	// 右侧字面量去引号（'foo' / "foo" → foo）
	right = unquoteLiteral(right)

	switch op {
	case "==":
		return lv == right, nil
	case "!=":
		return lv != right, nil
	}
	return false, fmt.Errorf("unsupported operator %q in %q", op, expr)
}

// unquoteLiteral 去掉字面量首尾的单/双引号（用于 include_when 中 'foo' / "foo" 形式）。
func unquoteLiteral(s string) string {
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '\'' && last == '\'') || (first == '"' && last == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// toInt32 把字符串/数字/布尔统一转为 int32（用于 Priority 等字段）。
func toInt32(v any) int32 {
	switch x := v.(type) {
	case int32:
		return x
	case int:
		return int32(x)
	case int64:
		return int32(x)
	case float64:
		return int32(x)
	case string:
		if n, err := strconv.ParseInt(x, 10, 32); err == nil {
			return int32(n)
		}
	case bool:
		if x {
			return 1
		}
	}
	return 0
}

// isTruthy 判断字符串是否代表 truthy（用于 include_when 单 token 形式）。
func isTruthy(s string) bool {
	switch strings.ToLower(s) {
	case "", "0", "false", "null", "nil":
		return false
	}
	return true
}