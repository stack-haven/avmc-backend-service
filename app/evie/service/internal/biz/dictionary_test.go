package biz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
)

// dictionaryRepoStub is a minimal DictionaryRepo stub for usecase tests.
type dictionaryRepoStub struct {
	created *pb.DictionaryWord
	updated *pb.DictionaryWord
	deleted uint32
}

func (s *dictionaryRepoStub) ListWords(context.Context, *pb.ListWordsRequest) ([]*pb.DictionaryWord, int32, error) {
	return nil, 0, nil
}
func (s *dictionaryRepoStub) GetWord(context.Context, uint32) (*pb.DictionaryWord, error) {
	return nil, nil
}
func (s *dictionaryRepoStub) CreateWord(_ context.Context, word *pb.DictionaryWord) (*pb.DictionaryWord, error) {
	s.created = word
	return word, nil
}
func (s *dictionaryRepoStub) UpdateWord(_ context.Context, word *pb.DictionaryWord) (*pb.DictionaryWord, error) {
	s.updated = word
	return word, nil
}
func (s *dictionaryRepoStub) DeleteWord(_ context.Context, id uint32) error {
	s.deleted = id
	return nil
}

func TestDictionaryUsecaseCreateWordDefaults(t *testing.T) {
	stub := &dictionaryRepoStub{}
	uc := NewDictionaryUsecase(stub, log.NewStdLogger(nil))

	created, err := uc.CreateWord(context.Background(), &pb.DictionaryWord{
		Word: "田华",
		Aliases: []*pb.DictionaryAlias{
			{Alias: "小田"},
		},
	})
	if err != nil {
		t.Fatalf("CreateWord error: %v", err)
	}
	if created.GetCategory() != "term" {
		t.Errorf("expected default category 'term', got %q", created.GetCategory())
	}
	if created.GetLevel() != "tenant" {
		t.Errorf("expected default level 'tenant', got %q", created.GetLevel())
	}
	if created.GetSource() != "manual" {
		t.Errorf("expected default source 'manual', got %q", created.GetSource())
	}
	if len(created.GetAliases()) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(created.GetAliases()))
	}
	alias := created.GetAliases()[0]
	if alias.GetSource() != "manual" {
		t.Errorf("expected alias default source 'manual', got %q", alias.GetSource())
	}
	if alias.GetWeight() != 1.0 {
		t.Errorf("expected alias default weight 1.0, got %v", alias.GetWeight())
	}
}

func TestDictionaryUsecaseCreateWordRequiresWord(t *testing.T) {
	uc := NewDictionaryUsecase(&dictionaryRepoStub{}, log.NewStdLogger(nil))
	if _, err := uc.CreateWord(context.Background(), &pb.DictionaryWord{}); err == nil {
		t.Fatal("expected error for empty word")
	}
}

func TestDictionaryUsecaseUpdateWordRequiresID(t *testing.T) {
	uc := NewDictionaryUsecase(&dictionaryRepoStub{}, log.NewStdLogger(nil))
	if _, err := uc.UpdateWord(context.Background(), &pb.DictionaryWord{Word: "x"}); err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestDictionaryUsecaseDeleteWordRequiresID(t *testing.T) {
	uc := NewDictionaryUsecase(&dictionaryRepoStub{}, log.NewStdLogger(nil))
	if err := uc.DeleteWord(context.Background(), 0); err == nil {
		t.Fatal("expected error for empty id")
	}
}
