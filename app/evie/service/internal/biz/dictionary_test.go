package biz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
)

// dictionaryRepoStub is a minimal DictionaryRepo stub for usecase tests.
type dictionaryRepoStub struct {
	words                   []string
	createdDict             *pb.Dictionary
	createdEntry            *pb.DictionaryEntry
	updatedDict             *pb.Dictionary
	updatedEntry            *pb.DictionaryEntry
	deletedDict             uint32
	deletedEntry            uint32
	relationsByDictionaryFn func(*pb.ListRelationsByDictionaryRequest) ([]*pb.DictionaryRelation, int32, error)
	statsFn                 func(uint32) (*pb.DictionaryStats, error)
}

func (s *dictionaryRepoStub) ListDictionaries(context.Context, *pb.ListDictionariesRequest) ([]*pb.Dictionary, int32, error) {
	return nil, 0, nil
}
func (s *dictionaryRepoStub) GetDictionary(context.Context, uint32) (*pb.Dictionary, error) {
	return nil, nil
}
func (s *dictionaryRepoStub) CreateDictionary(_ context.Context, d *pb.Dictionary) (*pb.Dictionary, error) {
	s.createdDict = d
	return d, nil
}
func (s *dictionaryRepoStub) UpdateDictionary(_ context.Context, d *pb.Dictionary) (*pb.Dictionary, error) {
	s.updatedDict = d
	return d, nil
}
func (s *dictionaryRepoStub) DeleteDictionary(_ context.Context, id uint32) error {
	s.deletedDict = id
	return nil
}

func (s *dictionaryRepoStub) ListEntries(context.Context, *pb.ListEntriesRequest) ([]*pb.DictionaryEntry, int32, error) {
	return nil, 0, nil
}
func (s *dictionaryRepoStub) GetEntry(context.Context, uint32) (*pb.DictionaryEntry, error) {
	return nil, nil
}
func (s *dictionaryRepoStub) CreateEntry(_ context.Context, e *pb.DictionaryEntry) (*pb.DictionaryEntry, error) {
	s.createdEntry = e
	return e, nil
}
func (s *dictionaryRepoStub) UpdateEntry(_ context.Context, e *pb.DictionaryEntry) (*pb.DictionaryEntry, error) {
	s.updatedEntry = e
	return e, nil
}
func (s *dictionaryRepoStub) DeleteEntry(_ context.Context, id uint32) error {
	s.deletedEntry = id
	return nil
}
func (s *dictionaryRepoStub) ListActiveEntryTexts(context.Context) ([]string, error) {
	return s.words, nil
}

func (s *dictionaryRepoStub) ListRelations(context.Context, *pb.ListRelationsRequest) ([]*pb.DictionaryRelation, int32, error) {
	return nil, 0, nil
}
func (s *dictionaryRepoStub) ListRelationsByDictionary(_ context.Context, req *pb.ListRelationsByDictionaryRequest) ([]*pb.DictionaryRelation, int32, error) {
	if s.relationsByDictionaryFn != nil {
		return s.relationsByDictionaryFn(req)
	}
	return nil, 0, nil
}
func (s *dictionaryRepoStub) GetRelation(context.Context, uint32) (*pb.DictionaryRelation, error) {
	return nil, nil
}
func (s *dictionaryRepoStub) CreateRelation(context.Context, *pb.DictionaryRelation) (*pb.DictionaryRelation, error) {
	return nil, nil
}
func (s *dictionaryRepoStub) UpdateRelation(context.Context, *pb.DictionaryRelation) (*pb.DictionaryRelation, error) {
	return nil, nil
}
func (s *dictionaryRepoStub) DeleteRelation(context.Context, uint32) error { return nil }

func (s *dictionaryRepoStub) GetStats(_ context.Context, dictionaryID uint32) (*pb.DictionaryStats, error) {
	if s.statsFn != nil {
		return s.statsFn(dictionaryID)
	}
	return &pb.DictionaryStats{DictionaryId: dictionaryID}, nil
}

func (s *dictionaryRepoStub) ListCategories(context.Context, *pb.ListCategoriesRequest) ([]*pb.DictionaryCategory, int32, error) {
	return nil, 0, nil
}
func (s *dictionaryRepoStub) CreateCategory(context.Context, *pb.DictionaryCategory) (*pb.DictionaryCategory, error) {
	return nil, nil
}
func (s *dictionaryRepoStub) UpdateCategory(context.Context, *pb.DictionaryCategory) (*pb.DictionaryCategory, error) {
	return nil, nil
}
func (s *dictionaryRepoStub) DeleteCategory(context.Context, uint32) error { return nil }

func (s *dictionaryRepoStub) ListVersions(context.Context, *pb.ListVersionsRequest) ([]*pb.DictionaryVersion, int32, error) {
	return nil, 0, nil
}
func (s *dictionaryRepoStub) GetVersion(context.Context, uint32) (*pb.DictionaryVersion, error) {
	return nil, nil
}
func (s *dictionaryRepoStub) PublishDictionary(context.Context, *pb.PublishDictionaryRequest) (*pb.DictionaryVersion, error) {
	return nil, nil
}
func (s *dictionaryRepoStub) LoadVocabularyEntries(context.Context, uint32) ([]*pb.DictionaryEntry, []VocabularyRelationData, error) {
	return nil, nil, nil
}

func TestDictionaryUsecaseCreateDictionaryDefaults(t *testing.T) {
	stub := &dictionaryRepoStub{}
	uc := NewDictionaryUsecase(stub, nil, log.NewStdLogger(nil))

	created, err := uc.CreateDictionary(context.Background(), &pb.Dictionary{Name: "企业词库"})
	if err != nil {
		t.Fatalf("CreateDictionary error: %v", err)
	}
	if created.GetScope() != "TENANT" {
		t.Errorf("expected default scope 'TENANT', got %q", created.GetScope())
	}
	if created.GetSource() != "MANUAL" {
		t.Errorf("expected default source 'MANUAL', got %q", created.GetSource())
	}
}

func TestDictionaryUsecaseCreateDictionaryRequiresName(t *testing.T) {
	uc := NewDictionaryUsecase(&dictionaryRepoStub{}, nil, log.NewStdLogger(nil))
	if _, err := uc.CreateDictionary(context.Background(), &pb.Dictionary{}); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestDictionaryUsecaseCreateEntryDefaults(t *testing.T) {
	stub := &dictionaryRepoStub{}
	uc := NewDictionaryUsecase(stub, nil, log.NewStdLogger(nil))

	created, err := uc.CreateEntry(context.Background(), &pb.DictionaryEntry{
		DictionaryId: 1,
		StandardText: "田华",
	})
	if err != nil {
		t.Fatalf("CreateEntry error: %v", err)
	}
	if created.GetEntryType() != "WORD" {
		t.Errorf("expected default entry_type 'WORD', got %q", created.GetEntryType())
	}
	if created.GetCategory() != "OTHER" {
		t.Errorf("expected default category 'OTHER', got %q", created.GetCategory())
	}
	if created.GetSource() != "MANUAL" {
		t.Errorf("expected default source 'MANUAL', got %q", created.GetSource())
	}
}

func TestDictionaryUsecaseCreateEntryRequiresDictionary(t *testing.T) {
	uc := NewDictionaryUsecase(&dictionaryRepoStub{}, nil, log.NewStdLogger(nil))
	if _, err := uc.CreateEntry(context.Background(), &pb.DictionaryEntry{StandardText: "田华"}); err == nil {
		t.Fatal("expected error for empty dictionary_id")
	}
}

func TestDictionaryUsecaseDeleteRequiresID(t *testing.T) {
	uc := NewDictionaryUsecase(&dictionaryRepoStub{}, nil, log.NewStdLogger(nil))
	if err := uc.DeleteDictionary(context.Background(), 0); err == nil {
		t.Fatal("expected error for empty dictionary id")
	}
	if err := uc.DeleteEntry(context.Background(), 0); err == nil {
		t.Fatal("expected error for empty entry id")
	}
}
