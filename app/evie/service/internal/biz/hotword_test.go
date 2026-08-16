package biz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
)

// hotwordRepoStub is a minimal HotwordRepo stub for usecase tests.
type hotwordRepoStub struct {
	hotwords []*pb.Hotword
	upserted *pb.Hotword
	deleted  uint32
}

func (s *hotwordRepoStub) List(context.Context, string) ([]*pb.Hotword, error) {
	return s.hotwords, nil
}
func (s *hotwordRepoStub) Upsert(_ context.Context, h *pb.Hotword) (*pb.Hotword, error) {
	s.upserted = h
	return h, nil
}
func (s *hotwordRepoStub) Delete(_ context.Context, id uint32) error {
	s.deleted = id
	return nil
}

func TestHotwordUsecaseUpsertDefaults(t *testing.T) {
	stub := &hotwordRepoStub{}
	uc := NewHotwordUsecase(stub, log.NewStdLogger(nil))

	upserted, err := uc.Upsert(context.Background(), &pb.Hotword{Word: "金种籽"})
	if err != nil {
		t.Fatalf("Upsert error: %v", err)
	}
	if upserted.GetCategory() != "term" {
		t.Errorf("expected default category 'term', got %q", upserted.GetCategory())
	}
	if upserted.GetWeight() != 5.0 {
		t.Errorf("expected default weight 5.0, got %v", upserted.GetWeight())
	}
}

func TestHotwordUsecaseUpsertRequiresWord(t *testing.T) {
	uc := NewHotwordUsecase(&hotwordRepoStub{}, log.NewStdLogger(nil))
	if _, err := uc.Upsert(context.Background(), &pb.Hotword{}); err == nil {
		t.Fatal("expected error for empty word")
	}
}

func TestHotwordUsecaseDeleteRequiresID(t *testing.T) {
	uc := NewHotwordUsecase(&hotwordRepoStub{}, log.NewStdLogger(nil))
	if err := uc.Delete(context.Background(), 0); err == nil {
		t.Fatal("expected error for empty id")
	}
}
