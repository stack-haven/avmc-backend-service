package biz

import (
	"testing"
)

// testRule 构造一个简单的 qua 规则集用于单测。
func testRule() *RuleSet {
	return &RuleSet{
		Sources: map[string]*SourceRules{
			"qua": {
				Source: "qua",
				EntityMappings: []EntityMapping{
					{
						Match: MatchCondition{EntityType: "user"},
						Emit: EmitSpec{
							StandardText: "realName",
							Category:     "PERSON",
							Aliases:      []string{"nickname", "alias"},
							PinyinHint:   "realName",
							Priority:     "50",
							IncludeWhen:  "status==1",
						},
					},
					{
						Match: MatchCondition{EntityType: "department"},
						Emit: EmitSpec{
							StandardText: "name",
							Category:     "ORGANIZATION",
							PinyinHint:   "name",
							Priority:     "30",
						},
					},
				},
			},
		},
	}
}

// ============================================================================
// Happy path
// ============================================================================

func TestNormalizer_QuoUser_Success(t *testing.T) {
	n := NewNormalizer(testRule())
	raw := RawEntity{
		SourceID:   "u-1",
		EntityType: "user",
		Source:     "qua",
		Data: map[string]any{
			"realName": "田华",
			"nickname": "小田",
			"alias":    "华仔",
			"status":   1,
		},
	}
	entry, ok, err := n.Normalize(raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if entry.StandardText != "田华" {
		t.Errorf("StandardText = %q, want 田华", entry.StandardText)
	}
	if entry.Category != "PERSON" {
		t.Errorf("Category = %q, want PERSON", entry.Category)
	}
	if len(entry.Aliases) != 2 {
		t.Errorf("Aliases = %v, want 2 items", entry.Aliases)
	}
	if entry.Aliases[0] != "小田" || entry.Aliases[1] != "华仔" {
		t.Errorf("Aliases order/content: %v", entry.Aliases)
	}
	if entry.PinyinHint != "田华" {
		t.Errorf("PinyinHint = %q, want 田华", entry.PinyinHint)
	}
	if entry.Priority != 50 {
		t.Errorf("Priority = %d, want 50", entry.Priority)
	}
}

func TestNormalizer_QuoDepartment_Success(t *testing.T) {
	n := NewNormalizer(testRule())
	raw := RawEntity{
		SourceID:   "d-1",
		EntityType: "department",
		Source:     "qua",
		Data: map[string]any{
			"name": "技术研发部",
		},
	}
	entry, ok, err := n.Normalize(raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if entry.StandardText != "技术研发部" {
		t.Errorf("StandardText = %q", entry.StandardText)
	}
	if entry.Category != "ORGANIZATION" {
		t.Errorf("Category = %q", entry.Category)
	}
	if len(entry.Aliases) != 0 {
		t.Errorf("Aliases = %v, want empty", entry.Aliases)
	}
}

// ============================================================================
// include_when 过滤
// ============================================================================

func TestNormalizer_IncludeWhen_Fail(t *testing.T) {
	n := NewNormalizer(testRule())
	raw := RawEntity{
		SourceID:   "u-1",
		EntityType: "user",
		Source:     "qua",
		Data: map[string]any{
			"realName": "田华",
			"status":   0, // status==1 过滤
		},
	}
	_, ok, err := n.Normalize(raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if ok {
		t.Error("expected ok=false (status filter)")
	}
}

func TestNormalizer_IncludeWhen_NotEqual(t *testing.T) {
	rs := testRule()
	rs.Sources["qua"].EntityMappings[0].Emit.IncludeWhen = "status!=99"
	n := NewNormalizer(rs)
	raw := RawEntity{
		SourceID: "u-1", EntityType: "user", Source: "qua",
		Data: map[string]any{"realName": "X", "status": 1},
	}
	_, ok, _ := n.Normalize(raw)
	if !ok {
		t.Error("expected ok=true (1 != 99)")
	}
}

// ============================================================================
// dot-path
// ============================================================================

func TestNormalizer_DotPath_Nested(t *testing.T) {
	rs := testRule()
	rs.Sources["qua"].EntityMappings[0].Emit.StandardText = "user.realName"
	n := NewNormalizer(rs)
	raw := RawEntity{
		SourceID: "u-1", EntityType: "user", Source: "qua",
		Data: map[string]any{
			"user": map[string]any{
				"realName": "嵌套名字",
			},
			"status": 1,
		},
	}
	entry, ok, _ := n.Normalize(raw)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if entry.StandardText != "嵌套名字" {
		t.Errorf("StandardText = %q, want 嵌套名字", entry.StandardText)
	}
}

func TestNormalizer_Alias_NestedPath(t *testing.T) {
	rs := testRule()
	rs.Sources["qua"].EntityMappings[0].Emit.Aliases = []string{"userInfo.nickname"}
	n := NewNormalizer(rs)
	raw := RawEntity{
		SourceID: "u-1", EntityType: "user", Source: "qua",
		Data: map[string]any{
			"realName": "主名",
			"userInfo": map[string]any{"nickname": "嵌套昵称"},
			"status":   1,
		},
	}
	entry, _, _ := n.Normalize(raw)
	if len(entry.Aliases) != 1 || entry.Aliases[0] != "嵌套昵称" {
		t.Errorf("Aliases = %v", entry.Aliases)
	}
}

// ============================================================================
// 字段缺失 / 规则不存在 → 跳过（warn 不阻断）
// ============================================================================

func TestNormalizer_StandardText_Missing_Skips(t *testing.T) {
	n := NewNormalizer(testRule())
	raw := RawEntity{
		SourceID: "u-1", EntityType: "user", Source: "qua",
		Data: map[string]any{
			// realName 缺失
			"nickname": "昵称",
			"status":   1,
		},
	}
	_, ok, _ := n.Normalize(raw)
	if ok {
		t.Error("expected ok=false (standard_text missing)")
	}
}

func TestNormalizer_UnknownEntityType_Skips(t *testing.T) {
	n := NewNormalizer(testRule())
	raw := RawEntity{
		SourceID: "x-1", EntityType: "product", Source: "qua",
		Data: map[string]any{"name": "X"},
	}
	_, ok, _ := n.Normalize(raw)
	if ok {
		t.Error("expected ok=false (unknown entity_type)")
	}
}

func TestNormalizer_UnknownSource_Skips(t *testing.T) {
	n := NewNormalizer(testRule())
	raw := RawEntity{
		SourceID: "u-1", EntityType: "user", Source: "feishu",
		Data: map[string]any{"realName": "X"},
	}
	_, ok, _ := n.Normalize(raw)
	if ok {
		t.Error("expected ok=false (unknown source)")
	}
}

// ============================================================================
// 优先级：dot-path 值 vs 字面量
// ============================================================================

func TestNormalizer_Priority_DotPath(t *testing.T) {
	rs := testRule()
	rs.Sources["qua"].EntityMappings[0].Emit.Priority = "score" // dot-path
	n := NewNormalizer(rs)
	raw := RawEntity{
		SourceID: "u-1", EntityType: "user", Source: "qua",
		Data: map[string]any{
			"realName": "X", "status": 1,
			"score": float64(99), // JSON 数字默认 float64
		},
	}
	entry, _, _ := n.Normalize(raw)
	if entry.Priority != 99 {
		t.Errorf("Priority = %d, want 99", entry.Priority)
	}
}

func TestNormalizer_Priority_Literal(t *testing.T) {
	n := NewNormalizer(testRule()) // Priority="50" 字面量
	raw := RawEntity{
		SourceID: "u-1", EntityType: "user", Source: "qua",
		Data: map[string]any{"realName": "X", "status": 1},
	}
	entry, _, _ := n.Normalize(raw)
	if entry.Priority != 50 {
		t.Errorf("Priority = %d, want 50 (literal)", entry.Priority)
	}
}

// ============================================================================
// 别名去重 + 排除与 standard_text 相同
// ============================================================================

func TestNormalizer_Alias_Dedup(t *testing.T) {
	n := NewNormalizer(testRule())
	raw := RawEntity{
		SourceID: "u-1", EntityType: "user", Source: "qua",
		Data: map[string]any{
			"realName": "田华",
			"nickname": "田华", // 与 standard_text 相同，应被排除
			"alias":    "华仔",
			"status":   1,
		},
	}
	entry, _, _ := n.Normalize(raw)
	if len(entry.Aliases) != 1 || entry.Aliases[0] != "华仔" {
		t.Errorf("Aliases = %v, want [华仔]", entry.Aliases)
	}
}

// ============================================================================
// 批量
// ============================================================================

func TestNormalizer_Batch_MixedSkip(t *testing.T) {
	n := NewNormalizer(testRule())
	raws := []RawEntity{
		{SourceID: "u-1", EntityType: "user", Source: "qua",
			Data: map[string]any{"realName": "A", "status": 1}},
		{SourceID: "u-2", EntityType: "user", Source: "qua",
			Data: map[string]any{"realName": "B", "status": 0}}, // 过滤
		{SourceID: "d-1", EntityType: "department", Source: "qua",
			Data: map[string]any{"name": "X"}},
	}
	entries, err := n.NormalizeBatch(raws)
	if err != nil {
		t.Fatalf("NormalizeBatch: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("len(entries) = %d, want 2 (1 user + 1 dept)", len(entries))
	}
}

// ============================================================================
// LoadRuleSet 从 conf.VocabRules 加载
// ============================================================================

func TestLoadRuleSet_FromEmpty(t *testing.T) {
	rs := LoadRuleSet(nil)
	if rs == nil || len(rs.Sources) != 0 {
		t.Errorf("expected empty rule set, got %+v", rs)
	}
}

// ============================================================================
// evalCondition 单元测试
// ============================================================================

func TestEvalCondition(t *testing.T) {
	data := map[string]any{
		"status":  1,
		"name":    "田华",
		"enabled": true,
		"score":   float64(95),
	}

	tests := []struct {
		expr string
		want bool
	}{
		{"", true},                     // empty = 总是通过
		{"status==1", true},            // 数字字符串比较
		{"status==2", false},
		{"name=='田华'", true},
		{"name=='其他'", false},
		{"status!=2", true},
		{"enabled", true},              // 真值单 token
		{"status", true},               // "1" 是 truthy
		{"missing_field", false},       // 字段缺失
	}

	for _, tt := range tests {
		got, err := evalCondition(tt.expr, data)
		if err != nil {
			t.Errorf("evalCondition(%q) err=%v", tt.expr, err)
			continue
		}
		if got != tt.want {
			t.Errorf("evalCondition(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}
}