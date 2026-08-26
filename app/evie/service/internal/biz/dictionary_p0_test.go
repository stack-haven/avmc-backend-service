package biz

import (
	"context"
	"errors"
	"testing"

	pb "backend-service/api/evie/service/v1"
	kerrors "github.com/go-kratos/kratos/v2/errors"
)

// TestDictionaryUsecase_GetStats_BadRequest 验证 dictionary_id=0 返回 BadRequest。
// biz 层应先于 repo 做参数校验，避免空查询浪费资源。
func TestDictionaryUsecase_GetStats_BadRequest(t *testing.T) {
	uc := NewDictionaryUsecase(&dictionaryRepoStub{}, nil, nil)
	stats, err := uc.GetStats(context.Background(), 0)
	if err == nil {
		t.Fatalf("expected error for zero dictionary_id, got stats=%+v", stats)
	}
	ke := new(kerrors.Error)
	if !errors.As(err, &ke) || ke.Reason != "DICTIONARY_ID_REQUIRED" {
		t.Fatalf("expected DICTIONARY_ID_REQUIRED error, got: %v", err)
	}
}

// TestDictionaryUsecase_GetStats_Passthrough 验证 usecase 透传到 repo。
func TestDictionaryUsecase_GetStats_Passthrough(t *testing.T) {
	called := false
	stub := &dictionaryRepoStub{
		statsFn: func(id uint32) (*pb.DictionaryStats, error) {
			called = true
			if id != 42 {
				t.Errorf("unexpected dictionary_id: got %d, want 42", id)
			}
			return &pb.DictionaryStats{DictionaryId: id, EntryCount: 10, RelationCount: 5}, nil
		},
	}
	uc := NewDictionaryUsecase(stub, nil, nil)
	stats, err := uc.GetStats(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("repo.GetStats not invoked")
	}
	if stats.EntryCount != 10 || stats.RelationCount != 5 {
		t.Errorf("stats not propagated: %+v", stats)
	}
}

// TestDictionaryUsecase_ListRelationsByDictionary_BadRequest 验证 dictionary_id=0 返回 BadRequest。
func TestDictionaryUsecase_ListRelationsByDictionary_BadRequest(t *testing.T) {
	uc := NewDictionaryUsecase(&dictionaryRepoStub{}, nil, nil)
	rels, total, err := uc.ListRelationsByDictionary(context.Background(), &pb.ListRelationsByDictionaryRequest{})
	if err == nil {
		t.Fatalf("expected error for zero dictionary_id, got rels=%d total=%d", len(rels), total)
	}
	ke := new(kerrors.Error)
	if !errors.As(err, &ke) || ke.Reason != "DICTIONARY_ID_REQUIRED" {
		t.Fatalf("expected DICTIONARY_ID_REQUIRED error, got: %v", err)
	}
}

// TestDictionaryUsecase_ListRelationsByDictionary_Passthrough 验证 usecase 透传。
func TestDictionaryUsecase_ListRelationsByDictionary_Passthrough(t *testing.T) {
	called := false
	expected := []*pb.DictionaryRelation{
		{Id: 1, EntryId: 10, RelationType: "ALIAS", RelatedText: "您好", EntryStandardText: "客服您好"},
		{Id: 2, EntryId: 10, RelationType: "HOMOPHONE", RelatedText: "苦伏", EntryStandardText: "客服您好"},
	}
	stub := &dictionaryRepoStub{
		relationsByDictionaryFn: func(req *pb.ListRelationsByDictionaryRequest) ([]*pb.DictionaryRelation, int32, error) {
			called = true
			if req.GetDictionaryId() != 100 {
				t.Errorf("unexpected dictionary_id: got %d, want 100", req.GetDictionaryId())
			}
			return expected, 2, nil
		},
	}
	uc := NewDictionaryUsecase(stub, nil, nil)
	rels, total, err := uc.ListRelationsByDictionary(context.Background(), &pb.ListRelationsByDictionaryRequest{DictionaryId: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("repo.ListRelationsByDictionary not invoked")
	}
	if total != 2 || len(rels) != 2 {
		t.Errorf("unexpected result: total=%d len=%d", total, len(rels))
	}
	// 验证 JOIN 字段被传递
	if rels[0].EntryStandardText != "客服您好" {
		t.Errorf("entry_standard_text not propagated: %+v", rels[0])
	}
}