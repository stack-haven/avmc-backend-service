package biz

import (
	"context"
	"testing"

	pb "backend-service/api/evie/service/v1"
)

// dictRepoStub 是测试用的 DictionaryRepo 桩，仅实现 ListActiveWords。
type dictRepoStub struct{ words []string }

func (s *dictRepoStub) ListActiveWords(context.Context) ([]string, error) { return s.words, nil }
func (s *dictRepoStub) ListWords(context.Context, *pb.ListWordsRequest) ([]*pb.DictionaryWord, int32, error) {
	return nil, 0, nil
}
func (s *dictRepoStub) GetWord(context.Context, uint32) (*pb.DictionaryWord, error) {
	return nil, nil
}
func (s *dictRepoStub) CreateWord(context.Context, *pb.DictionaryWord) (*pb.DictionaryWord, error) {
	return nil, nil
}
func (s *dictRepoStub) UpdateWord(context.Context, *pb.DictionaryWord) (*pb.DictionaryWord, error) {
	return nil, nil
}
func (s *dictRepoStub) DeleteWord(context.Context, uint32) error { return nil }

func TestLevenshteinDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"金种子", "金种籽", 1},
		{"金种子", "金种子", 0},
		{"金融资", "金种籽", 2},
		{"", "abc", 3},
		{"abc", "", 3},
		{"", "", 0},
	}
	for _, c := range cases {
		if got := levenshteinDistance(c.a, c.b); got != c.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestEditDistanceCorrector(t *testing.T) {
	corrector := NewEditDistanceCorrector(&dictRepoStub{words: []string{"金种籽", "攻克"}})
	text := "昨天的金种子情况，我们要攻克难关"
	candidates, err := corrector.Correct(context.Background(), text)
	if err != nil {
		t.Fatalf("Correct error: %v", err)
	}

	// 期望：金种子 → 金种籽（dist=1, conf=0.667）；攻克已正确不纠；"难关"不在字典不纠
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %+v", len(candidates), candidates)
	}
	c := candidates[0]
	if c.From != "金种子" || c.To != "金种籽" || c.Source != "edit_distance" {
		t.Errorf("unexpected candidate: %+v", c)
	}
	if c.Confidence < 0.6 || c.Confidence > 0.7 {
		t.Errorf("expected confidence ~0.667, got %v", c.Confidence)
	}
}

func TestEditDistanceCorrectorRejectsFarWords(t *testing.T) {
	corrector := NewEditDistanceCorrector(&dictRepoStub{words: []string{"金种籽"}})
	// 金融资 与 金种籽 距离 2，长度 3，conf=0.333 < 0.6，不应纠
	candidates, err := corrector.Correct(context.Background(), "金融资情况")
	if err != nil {
		t.Fatalf("Correct error: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates, got %+v", candidates)
	}
}

func TestEditDistanceCorrectorRejectsShortWord(t *testing.T) {
	corrector := NewEditDistanceCorrector(&dictRepoStub{words: []string{"种子"}})
	// "种子" vs "种籽" 距离 1，长度 2，conf=0.5 < 0.6，不应纠
	candidates, err := corrector.Correct(context.Background(), "申请种子")
	if err != nil {
		t.Fatalf("Correct error: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates for 2-char word, got %+v", candidates)
	}
}

func TestEditDistanceCorrectorEmptyDict(t *testing.T) {
	corrector := NewEditDistanceCorrector(&dictRepoStub{})
	candidates, err := corrector.Correct(context.Background(), "任意文本")
	if err != nil {
		t.Fatalf("Correct error: %v", err)
	}
	if candidates != nil {
		t.Errorf("expected nil candidates for empty dict, got %+v", candidates)
	}
}
