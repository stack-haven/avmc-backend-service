package biz

import (
	"context"
	"testing"

	pb "backend-service/api/evie/service/v1"
)

func TestHotwordCorrector(t *testing.T) {
	corrector := NewHotwordCorrector(&hotwordRepoStub{hotwords: []*pb.Hotword{
		{Word: "田工", Target: "田华"},
		{Word: "陈新", Target: "陈新进"},
		{Word: "金种子", Target: ""}, // target 空 → 不做替换（纯热词增强）
	}})
	candidates, err := corrector.Correct(context.Background(), "田工和陈新负责项目")
	if err != nil {
		t.Fatalf("Correct error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %+v", len(candidates), candidates)
	}
	if candidates[0].From != "田工" || candidates[0].To != "田华" || candidates[0].Source != "hotword" {
		t.Errorf("unexpected candidate: %+v", candidates[0])
	}
	if candidates[0].Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %v", candidates[0].Confidence)
	}
}

func TestHotwordCorrectorSkipsEmptyTarget(t *testing.T) {
	corrector := NewHotwordCorrector(&hotwordRepoStub{hotwords: []*pb.Hotword{
		{Word: "金种子", Target: ""},
	}})
	candidates, err := corrector.Correct(context.Background(), "申请金种子")
	if err != nil {
		t.Fatalf("Correct error: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates for empty target, got %+v", candidates)
	}
}
