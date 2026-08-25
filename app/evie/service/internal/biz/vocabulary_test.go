package biz

import (
	"context"
	"testing"
)

// conflictRecorderStub 记录冲突的桩。
type conflictRecorderStub struct{ conflicts []*DictionaryConflict }

func (s *conflictRecorderStub) RecordConflict(_ context.Context, c *DictionaryConflict) error {
	s.conflicts = append(s.conflicts, c)
	return nil
}
func (s *conflictRecorderStub) ListConflicts(context.Context) ([]*DictionaryConflict, error) {
	return s.conflicts, nil
}

func TestDetectConflicts(t *testing.T) {
	builder := &VocabularyBuilder{}
	relations := []VocabularyRelationData{
		{EntryID: 1, RelationType: "ALIAS", RelatedText: "小田", TargetEntryID: 1, Scope: "PLATFORM"},
		{EntryID: 2, RelationType: "ALIAS", RelatedText: "小田", TargetEntryID: 2, Scope: "TENANT"},
		{EntryID: 3, RelationType: "ALIAS", RelatedText: "技术部", TargetEntryID: 3, Scope: "SYSTEM"},
	}
	conflicts := builder.detectConflicts(relations)
	// 小田 在两个 scope 且 target 不同 → 1 个冲突（PLATFORM 低优先级被解析掉）
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d: %+v", len(conflicts), conflicts)
	}
	c := conflicts[0]
	if c.Input != "小田" || c.SourceScope != "PLATFORM" {
		t.Errorf("unexpected conflict: %+v", c)
	}
	if c.ResolvedCandidate == "" {
		t.Error("expected resolved candidate")
	}
}

func TestDetectConflictsNoConflictSameTarget(t *testing.T) {
	builder := &VocabularyBuilder{}
	relations := []VocabularyRelationData{
		{EntryID: 1, RelationType: "ALIAS", RelatedText: "小田", TargetEntryID: 1, Scope: "PLATFORM"},
		{EntryID: 1, RelationType: "ALIAS", RelatedText: "小田", TargetEntryID: 1, Scope: "TENANT"},
	}
	conflicts := builder.detectConflicts(relations)
	if len(conflicts) != 0 {
		t.Fatalf("expected 0 conflicts for same target, got %d", len(conflicts))
	}
}

func TestScopePriority(t *testing.T) {
	if scopePriority("TENANT") <= scopePriority("SYSTEM") {
		t.Error("TENANT 应高于 SYSTEM")
	}
	if scopePriority("SYSTEM") <= scopePriority("PLATFORM") {
		t.Error("SYSTEM 应高于 PLATFORM")
	}
}
