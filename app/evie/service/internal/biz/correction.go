package biz

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
)

// CorrectionRule 纠错规则（业务模型）。
type CorrectionRule struct {
	Source   string
	Target   string
	Type     string
	Priority int32
}

// CorrectionRuleRepo 纠错规则仓库接口。
type CorrectionRuleRepo interface {
	List(ctx context.Context) ([]CorrectionRule, error)
}

// CorrectionLog 纠错记录（业务模型）。
type CorrectionLog struct {
	UserID        uint32
	SessionID     string
	OriginalText  string
	CorrectedText string
	ChangesJSON   string
	Confidence    float64
	NeedConfirm   bool
	DurationMs    int64
	RuleHits      int
	PinyinHits    int
	EntityHits    int
	LLMHits       int
}

// CorrectionLogRepo 纠错记录仓库接口。
type CorrectionLogRepo interface {
	Save(ctx context.Context, log *CorrectionLog) error
}

// CorrectionCandidate 单路纠错候选。
type CorrectionCandidate struct {
	From       string
	To         string
	Type       string
	Confidence float64
	Source     string // rule/pinyin/entity/edit_distance/llm
}

// Corrector 单路纠错器（策略模式）。
type Corrector interface {
	Name() string
	Priority() int
	Correct(ctx context.Context, text string) ([]CorrectionCandidate, error)
}

// 置信度阈值。
const (
	highConfidenceThreshold   = 0.95
	mediumConfidenceThreshold = 0.75
)

// CorrectionEngine 纠错引擎（编排五路纠错器 + 评分 + 决策）。
type CorrectionEngine struct {
	correctors []Corrector
	logRepo    CorrectionLogRepo
	log        *log.Helper
}

// NewCorrectionEngine 创建纠错引擎（wire 友好：注入规则 + 热词 + 编辑距离纠错器）。
func NewCorrectionEngine(logRepo CorrectionLogRepo, ruleRepo CorrectionRuleRepo, dictRepo DictionaryRepo, hotwordRepo HotwordRepo, logger log.Logger) *CorrectionEngine {
	return newCorrectionEngine(logRepo, logger,
		NewRuleCorrector(ruleRepo),
		NewHotwordCorrector(hotwordRepo),
		NewEditDistanceCorrector(dictRepo),
	)
}

// newCorrectionEngine 内部构造函数，支持注入任意纠错器列表（测试用）。
func newCorrectionEngine(logRepo CorrectionLogRepo, logger log.Logger, correctors ...Corrector) *CorrectionEngine {
	sort.Slice(correctors, func(i, j int) bool {
		return correctors[i].Priority() > correctors[j].Priority()
	})
	return &CorrectionEngine{correctors: correctors, logRepo: logRepo, log: log.NewHelper(logger)}
}

// Correct 执行纠错全流程。
func (e *CorrectionEngine) Correct(ctx context.Context, req *pb.CorrectRequest) (*pb.CorrectResponse, error) {
	start := time.Now()
	text := req.GetText()

	var all []CorrectionCandidate
	for _, c := range e.correctors {
		candidates, err := c.Correct(ctx, text)
		if err != nil {
			e.log.Warnf("corrector %s failed: %v", c.Name(), err)
			continue
		}
		all = append(all, candidates...)
	}

	output, changes, needConfirm := e.apply(text, all)

	resp := &pb.CorrectResponse{
		OriginalText:  text,
		CorrectedText: output,
		Changes:       changes,
		Confidence:    float32(overallConfidence(changes)),
		NeedConfirm:   needConfirm,
	}

	if e.logRepo != nil {
		_ = e.logRepo.Save(ctx, &CorrectionLog{
			SessionID:     req.GetSessionId(),
			OriginalText:  text,
			CorrectedText: output,
			Confidence:    float64(resp.Confidence),
			NeedConfirm:   needConfirm,
			DurationMs:    time.Since(start).Milliseconds(),
			RuleHits:      countSource(all, "rule"),
			PinyinHits:    countSource(all, "pinyin"),
			EntityHits:    countSource(all, "entity"),
			LLMHits:       countSource(all, "llm"),
		})
	}

	return resp, nil
}

// apply 将候选按置信度决策：高置信度直接替换，中置信度标记待确认，低置信度丢弃。
func (e *CorrectionEngine) apply(text string, candidates []CorrectionCandidate) (string, []*pb.CorrectionChange, bool) {
	output := text
	needConfirm := false
	changes := make([]*pb.CorrectionChange, 0, len(candidates))
	for _, c := range candidates {
		if c.Confidence >= highConfidenceThreshold {
			if strings.Contains(output, c.From) {
				output = strings.ReplaceAll(output, c.From, c.To)
			}
			changes = append(changes, &pb.CorrectionChange{From: c.From, To: c.To, Type: c.Type, Confidence: float32(c.Confidence)})
		} else if c.Confidence >= mediumConfidenceThreshold {
			needConfirm = true
			changes = append(changes, &pb.CorrectionChange{From: c.From, To: c.To, Type: c.Type, Confidence: float32(c.Confidence)})
		}
		// < 0.75 丢弃
	}
	return output, changes, needConfirm
}

// RuleCorrector 规则纠错器：按 correction_rule 表的 source→target 精确替换。
type RuleCorrector struct {
	repo CorrectionRuleRepo
}

// NewRuleCorrector 创建规则纠错器。
func NewRuleCorrector(repo CorrectionRuleRepo) *RuleCorrector {
	return &RuleCorrector{repo: repo}
}

func (r *RuleCorrector) Name() string  { return "rule" }
func (r *RuleCorrector) Priority() int { return 100 }

// Correct 规则纠错：命中 source 的文本段替换为 target，置信度 1.0。
func (r *RuleCorrector) Correct(ctx context.Context, text string) ([]CorrectionCandidate, error) {
	rules, err := r.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	var candidates []CorrectionCandidate
	for _, rule := range rules {
		if rule.Source != "" && strings.Contains(text, rule.Source) {
			candidates = append(candidates, CorrectionCandidate{
				From:       rule.Source,
				To:         rule.Target,
				Type:       rule.Type,
				Confidence: 1.0,
				Source:     "rule",
			})
		}
	}
	return candidates, nil
}

func overallConfidence(changes []*pb.CorrectionChange) float64 {
	if len(changes) == 0 {
		return 1.0
	}
	var sum float64
	for _, c := range changes {
		sum += float64(c.GetConfidence())
	}
	return sum / float64(len(changes))
}

func countSource(candidates []CorrectionCandidate, source string) int {
	n := 0
	for _, c := range candidates {
		if c.Source == source {
			n++
		}
	}
	return n
}
