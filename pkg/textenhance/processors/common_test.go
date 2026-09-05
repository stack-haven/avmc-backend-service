package processors

import (
	"context"
	"errors"
	"testing"
)

// ============================================================================
// Text Utilities 测试
// ============================================================================

func TestIsPunctOrSpace(t *testing.T) {
	cases := []struct {
		r    rune
		want bool
	}{
		{' ', true}, {'\t', true}, {'\n', true},
		{'，', true}, {'。', true}, {'！', true}, {'？', true}, {'、', true},
		{',', true}, {'.', true}, {'!', true}, {'?', true},
		{'"', true}, {'\'', true}, {'(', true}, {')', true},
		{'a', false}, {'A', false}, {'1', false}, {'中', false},
		{'_', true}, {'-', true}, {'.', true}, // 标点
	}
	for _, c := range cases {
		if got := IsPunctOrSpace(c.r); got != c.want {
			t.Errorf("IsPunctOrSpace(%q) = %v, want %v", c.r, got, c.want)
		}
	}
}

func TestIsCJKChar(t *testing.T) {
	if !IsCJKChar('中') {
		t.Error("中 should be CJK")
	}
	if !IsCJKChar('国') {
		t.Error("国 should be CJK")
	}
	if IsCJKChar('a') {
		t.Error("a should not be CJK")
	}
	if IsCJKChar('1') {
		t.Error("1 should not be CJK")
	}
}

func TestHasCJK(t *testing.T) {
	if !HasCJK("hello中国") {
		t.Error("hello中国 should have CJK")
	}
	if HasCJK("hello world") {
		t.Error("hello world should not have CJK")
	}
}

func TestCountCJK(t *testing.T) {
	if got := CountCJK("hello中国world"); got != 2 {
		t.Errorf("CountCJK = %d, want 2", got)
	}
	if got := CountCJK(""); got != 0 {
		t.Errorf("CountCJK empty = %d, want 0", got)
	}
}

func TestReplaceAll(t *testing.T) {
	cases := []struct {
		s, old, new, want string
	}{
		{"hello world", "world", "go", "hello go"},
		{"aaa", "a", "b", "bbb"},
		{"hello", "x", "y", "hello"},
		{"hello", "", "x", "hello"}, // 空 old 返回原文
		{"aXaXa", "X", "", "aaa"},
		{"", "x", "y", ""},
	}
	for _, c := range cases {
		if got := ReplaceAll(c.s, c.old, c.new); got != c.want {
			t.Errorf("ReplaceAll(%q, %q, %q) = %q, want %q",
				c.s, c.old, c.new, got, c.want)
		}
	}
}

func TestContainsAny(t *testing.T) {
	if !ContainsAny("hello world", []string{"foo", "world"}) {
		t.Error("expected hit on world")
	}
	if ContainsAny("hello", []string{"foo", "bar"}) {
		t.Error("expected no hit")
	}
	if ContainsAny("hello", []string{""}) {
		t.Error("empty keyword should be ignored")
	}
}

func TestContainsAll(t *testing.T) {
	if !ContainsAll("hello world", []string{"hello", "world"}) {
		t.Error("expected all hit")
	}
	if ContainsAll("hello", []string{"hello", "world"}) {
		t.Error("expected miss on world")
	}
}

// ============================================================================
// Common Types 测试
// ============================================================================

func TestStopword(t *testing.T) {
	s := NewStopword("呃", StopwordStrongFiller, "sentence start filler")
	if s.Word != "呃" || s.Type != StopwordStrongFiller {
		t.Errorf("Stopword wrong: %+v", s)
	}
}

func TestPinyinService_Default(t *testing.T) {
	svc := NewDefaultPinyinService()
	if svc == nil {
		t.Fatal("DefaultPinyinService nil")
	}
	r, err := svc.Convert(context.Background(), "你好", true)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if r == nil {
		t.Fatal("result nil")
	}
	if r.Pinyin == "" {
		t.Error("Pinyin should be non-empty")
	}
}

// ============================================================================
// EnhancementContext 测试
// ============================================================================

func TestEnhancementContext_LockIsLocked(t *testing.T) {
	ec := NewEnhancementContext("test", nil, nil)
	ec.Lock("片段A")
	ec.Lock("片段B")
	if !ec.IsLocked("片段A") {
		t.Error("片段A should be locked")
	}
	if !ec.IsLocked("片段B") {
		t.Error("片段B should be locked")
	}
	if ec.IsLocked("片段C") {
		t.Error("片段C should not be locked")
	}
}

func TestEnhancementContext_AppendChange(t *testing.T) {
	ec := NewEnhancementContext("test", nil, nil)
	ec.AppendChange(Change{From: "a", To: "b", Action: ActionReplace})
	ec.AppendChange(Change{From: "c", To: "", Action: ActionDelete})

	changes := ec.GetChanges()
	if len(changes) != 2 {
		t.Fatalf("GetChanges len = %d, want 2", len(changes))
	}
	if changes[0].Action != ActionReplace {
		t.Errorf("changes[0].Action = %q, want %q", changes[0].Action, ActionReplace)
	}
}

func TestEnhancementContext_SetText(t *testing.T) {
	ec := NewEnhancementContext("hello", nil, nil)
	ec.SetText("world")
	if ec.GetText() != "world" {
		t.Errorf("GetText = %q, want world", ec.GetText())
	}
	if ec.GetRawText() != "hello" {
		t.Errorf("GetRawText = %q, want hello (unchanged)", ec.GetRawText())
	}
}

func TestEnhancementContext_AppendError(t *testing.T) {
	ec := NewEnhancementContext("test", nil, nil)
	ec.AppendError(nil) // nil 跳过
	ec.AppendError(errors.New("test error"))

	if len(ec.Errors) != 1 {
		t.Errorf("Errors len = %d, want 1", len(ec.Errors))
	}
}

func TestEnhancementContext_RecordTiming(t *testing.T) {
	ec := NewEnhancementContext("test", nil, nil)
	ec.RecordTiming("cleaning", 100*1e6) // 100ms in ns
	timings := ec.GetTimings()
	if timings["cleaning"].Nanoseconds() != 100*1e6 {
		t.Errorf("timing[cleaning] = %v, want 100ms", timings["cleaning"])
	}
}

func TestEnhancementContext_JoinErrors(t *testing.T) {
	ec := NewEnhancementContext("test", nil, nil)
	if got := ec.JoinErrors(); got != "" {
		t.Errorf("empty JoinErrors = %q", got)
	}
	ec.AppendError(errors.New("a"))
	ec.AppendError(errors.New("b"))
	got := ec.JoinErrors()
	if got == "" {
		t.Error("JoinErrors should be non-empty")
	}
}

// ============================================================================
// VocabularySnapshot 测试
// ============================================================================

func TestVocabularySnapshot_LookupEntry(t *testing.T) {
	entries := []*VocabularyEntry{
		{ID: 1, StandardText: "金种籽", Category: "PRODUCT"},
	}
	snap := NewVocabularySnapshot(entries, nil)

	if e, ok := snap.LookupEntry("金种籽"); !ok || e.ID != 1 {
		t.Errorf("LookupEntry failed: %+v", e)
	}
	if _, ok := snap.LookupEntry("不存在的词"); ok {
		t.Error("expected not found")
	}
}

func TestVocabularySnapshot_LookupRelations(t *testing.T) {
	relations := []*VocabularyRelation{
		{EntryID: 1, RelationType: "ALIAS", RelatedText: "金种子", TargetEntryID: 1},
	}
	snap := NewVocabularySnapshot(nil, relations)

	rs := snap.LookupRelations("金种子")
	if len(rs) != 1 {
		t.Fatalf("LookupRelations len = %d, want 1", len(rs))
	}
	if rs[0].RelationType != "ALIAS" {
		t.Errorf("RelationType = %q, want ALIAS", rs[0].RelationType)
	}
}

func TestVocabularySnapshot_Empty(t *testing.T) {
	snap := EmptyVocabularySnapshot()
	if snap.EntryCount() != 0 || snap.RelationCount() != 0 {
		t.Errorf("Empty snapshot counts wrong: %d/%d", snap.EntryCount(), snap.RelationCount())
	}
}