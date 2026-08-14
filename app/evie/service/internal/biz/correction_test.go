package biz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
)

// ruleRepoStub is a stub CorrectionRuleRepo.
type ruleRepoStub struct{ rules []CorrectionRule }

func (s *ruleRepoStub) List(context.Context) ([]CorrectionRule, error) { return s.rules, nil }

func TestRuleCorrector(t *testing.T) {
	corrector := NewRuleCorrector(&ruleRepoStub{rules: []CorrectionRule{
		{Source: "功课", Target: "攻克", Type: "dictionary", Priority: 100},
		{Source: "种子", Target: "种籽", Type: "dictionary", Priority: 90},
	}})
	candidates, err := corrector.Correct(context.Background(), "今天功课了一个技术难点，申请200个种子")
	if err != nil {
		t.Fatalf("Correct error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].From != "功课" || candidates[0].To != "攻克" {
		t.Errorf("unexpected candidate: %+v", candidates[0])
	}
	if candidates[0].Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %v", candidates[0].Confidence)
	}
}

func TestCorrectionEngineApply(t *testing.T) {
	engine := newCorrectionEngine(nil, log.NewStdLogger(nil))

	// 高置信度 → 替换
	out, changes, needConfirm := engine.apply("今天功课了一个技术难点", []CorrectionCandidate{
		{From: "功课", To: "攻克", Type: "dictionary", Confidence: 1.0, Source: "rule"},
	})
	if out != "今天攻克了一个技术难点" {
		t.Errorf("expected corrected text, got %q", out)
	}
	if len(changes) != 1 || needConfirm {
		t.Errorf("expected 1 change, no confirm; got %d changes confirm=%v", len(changes), needConfirm)
	}

	// 中置信度 → need_confirm，不替换
	out2, changes2, needConfirm2 := engine.apply("小田", []CorrectionCandidate{
		{From: "小田", To: "田华", Type: "person", Confidence: 0.85, Source: "entity"},
	})
	if out2 != "小田" {
		t.Errorf("expected unchanged text for medium confidence, got %q", out2)
	}
	if len(changes2) != 1 || !needConfirm2 {
		t.Errorf("expected 1 change + confirm; got %d changes confirm=%v", len(changes2), needConfirm2)
	}

	// 低置信度 → 丢弃
	_, changes3, _ := engine.apply("测试", []CorrectionCandidate{
		{From: "测试", To: "測试", Confidence: 0.5, Source: "pinyin"},
	})
	if len(changes3) != 0 {
		t.Errorf("expected 0 changes for low confidence, got %d", len(changes3))
	}
}

func TestCorrectionEngineCorrect(t *testing.T) {
	logStub := &logRepoStub{}
	engine := NewCorrectionEngine(logStub, &ruleRepoStub{rules: []CorrectionRule{
		{Source: "金种子", Target: "金种籽", Type: "product", Priority: 100},
	}}, log.NewStdLogger(nil))
	resp, err := engine.Correct(context.Background(), &pb.CorrectRequest{Text: "申请金种子奖励"})
	if err != nil {
		t.Fatalf("Correct error: %v", err)
	}
	if resp.GetCorrectedText() != "申请金种籽奖励" {
		t.Errorf("expected corrected text, got %q", resp.GetCorrectedText())
	}
	if resp.GetNeedConfirm() {
		t.Error("expected no confirm for rule hit")
	}
	if logStub.saved == nil {
		t.Error("expected correction log to be saved")
	}
}

type logRepoStub struct{ saved *CorrectionLog }

func (s *logRepoStub) Save(_ context.Context, log *CorrectionLog) error {
	s.saved = log
	return nil
}
