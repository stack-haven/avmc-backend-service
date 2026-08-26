package biz

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
)

// EnhancementAction 增强动作类型（开发说明第三十六节）。
const (
	ActionKeep    = "KEEP"
	ActionReplace = "REPLACE"
	ActionDelete  = "DELETE"
	ActionSuggest = "SUGGEST"
	ActionResolve = "RESOLVE"
)

// EnhancementChange 单条文本增强变更（开发说明第三十七节）。
type EnhancementChange struct {
	OriginalText string  // 原片段
	ResultText   string  // 结果片段（DELETE 时为空）
	Action       string  // KEEP/REPLACE/DELETE/SUGGEST/RESOLVE
	Type         string  // CLEAN/FILLER/ALIAS/CORRECTION/EXACT/...
	Source       string  // PLATFORM/SYSTEM/TENANT 词库 或 SYSTEM
	Confidence   float64 // 确定性=1.0；推断性=score
	Reason       string
	Locked       bool // 确定性结果锁定，后续算法不得覆盖（开发说明第二十五节）
}

// StepSnapshot 单步增强快照（用于步骤图/分词明细展示）。
type StepSnapshot struct {
	Step       string                // 步骤标识：text_cleaning / filler_removal / ...
	Before     string                // 该步骤处理前文本
	After      string                // 该步骤处理后文本
	DurationMs int64                 // 该步骤耗时
	Skipped    bool                  // 策略跳过该步骤时 true
	Changes    []*EnhancementChange  // 该步骤产生的变更明细
}

// EnhancementResult 文本增强结果。
type EnhancementResult struct {
	RawText       string
	EnhancedText  string
	Changes       []*EnhancementChange
	StepTimings   map[string]time.Duration
	TotalTime     time.Duration
	StepSnapshots []*StepSnapshot
}

// EnhancementContext 流水线上下文：贯穿各步骤，维护当前文本与变更。
type EnhancementContext struct {
	RawText string
	Text    string
	Vocab   *VocabularyContext
	Changes []*EnhancementChange
	// lockedSpans 记录确定性处理后的片段（用于后续推断步骤跳过，M7 使用）
	lockedSpans []string
}

// Result 组装最终结果。
func (c *EnhancementContext) Result() *EnhancementResult {
	return &EnhancementResult{RawText: c.RawText, EnhancedText: c.Text, Changes: c.Changes}
}

// lock 标记片段为已锁定（确定性结果）。
func (c *EnhancementContext) lock(span string) {
	c.lockedSpans = append(c.lockedSpans, span)
}

// isLocked 判断片段是否已被确定性处理锁定。
func (c *EnhancementContext) isLocked(span string) bool {
	for _, s := range c.lockedSpans {
		if s == span {
			return true
		}
	}
	return false
}

// EnhancementStep 文本增强流水线步骤（策略模式）。
type EnhancementStep interface {
	Name() string
	Process(ctx *EnhancementContext) error
}

// EnhancementEngine 文本增强引擎：按固定顺序执行 8 层流水线（开发说明第十九节）。
// 第一期为确定性部分（①清洗 ②口水词 ③词库匹配 ④别名解析 ⑤确定性替换），
// 推断部分（⑥拼音 ⑦模糊 ⑧上下文）在 M7 接入。
type EnhancementEngine struct {
	vocab *VocabularyBuilder
	steps []EnhancementStep
	log   *log.Helper
}

// NewEnhancementEngine 创建文本增强引擎。
func NewEnhancementEngine(vocab *VocabularyBuilder, logger log.Logger) *EnhancementEngine {
	e := &EnhancementEngine{vocab: vocab, log: log.NewHelper(logger)}
	e.steps = []EnhancementStep{
		&TextCleaningStep{},             // ① 清洗
		&FillerStep{},                    // ② 口水词
		&VocabularyMatchingStep{},        // ③ 词库匹配
		&AliasResolutionStep{},           // ④ 别名解析
		&DeterministicReplacementStep{},  // ⑤ 确定性替换
		&PhraseStandardizationStep{},     // 短语标准化（确定性规则）
		&PinyinCorrectionStep{},          // ⑥ 拼音纠错（推断）
		&FuzzyMatchingStep{},             // ⑦ 模糊匹配（推断）
		&ContextCorrectionStep{},         // ⑧ 上下文纠错（推断）
	}
	return e
}

// Enhance 执行完整文本增强流水线（默认 STANDARD 策略：全部确定性 + 推断）。
func (e *EnhancementEngine) Enhance(ctx context.Context, tenantID uint32, rawText string) (*EnhancementResult, error) {
	return e.enhance(ctx, tenantID, rawText, nil)
}

// EnhanceWithPolicy 按策略执行流水线（仅启用策略开启的步骤）。
func (e *EnhancementEngine) EnhanceWithPolicy(ctx context.Context, tenantID uint32, rawText string, policy *pb.EnhancementPolicy) (*EnhancementResult, error) {
	return e.enhance(ctx, tenantID, rawText, policy)
}

func (e *EnhancementEngine) enhance(ctx context.Context, tenantID uint32, rawText string, policy *pb.EnhancementPolicy) (*EnhancementResult, error) {
	totalStart := time.Now()
	vc, err := e.vocab.Build(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	c := &EnhancementContext{RawText: rawText, Text: rawText, Vocab: vc}
	timings := make(map[string]time.Duration, len(e.steps))
	snapshots := make([]*StepSnapshot, 0, len(e.steps))
	for _, step := range e.steps {
		if !stepEnabled(policy, step.Name()) {
			snapshots = append(snapshots, &StepSnapshot{Step: step.Name(), Skipped: true})
			continue
		}
		beforeText := c.Text
		beforeChanges := len(c.Changes)
		t0 := time.Now()
		if err := step.Process(c); err != nil {
			elapsed := time.Since(t0)
			timings[step.Name()] = elapsed
			snapshots = append(snapshots, &StepSnapshot{
				Step: step.Name(), Before: beforeText, After: c.Text,
				DurationMs: elapsed.Milliseconds(),
				Changes:    append([]*EnhancementChange(nil), c.Changes[beforeChanges:]...),
			})
			// 单阶段失败不阻断整个流水线（开发说明第四十八节：失败降级）
			e.log.Warnf("enhancement step %s failed: %v", step.Name(), err)
			continue
		}
		elapsed := time.Since(t0)
		timings[step.Name()] = elapsed
		snapshots = append(snapshots, &StepSnapshot{
			Step: step.Name(), Before: beforeText, After: c.Text,
			DurationMs: elapsed.Milliseconds(),
			Changes:    append([]*EnhancementChange(nil), c.Changes[beforeChanges:]...),
		})
	}
	result := c.Result()
	result.StepTimings = timings
	result.StepSnapshots = snapshots
	result.TotalTime = time.Since(totalStart)
	return result, nil
}

// stepEnabled 判断策略是否启用指定步骤（nil 策略=全部启用）。
func stepEnabled(policy *pb.EnhancementPolicy, stepName string) bool {
	if policy == nil {
		return true
	}
	switch stepName {
	case "cleaning":
		return policy.GetTextCleaning()
	case "filler":
		return policy.GetFillerRemoval()
	case "alias_resolution":
		return policy.GetAliasResolution()
	case "deterministic_replacement", "phrase_standardization":
		return policy.GetDeterministicReplacement()
	case "pinyin_correction":
		return policy.GetPinyinCorrection()
	case "fuzzy_matching":
		return policy.GetFuzzyMatching()
	case "context_correction":
		return policy.GetContextCorrection()
	default:
		return true // 词库匹配等底层能力始终启用
	}
}

// TextCleaningStep 第一层：文本清洗（异常空格、重复标点、特殊字符）。
type TextCleaningStep struct{}

func (TextCleaningStep) Name() string { return "cleaning" }

var (
	multiSpace = regexp.MustCompile(`\s+`)
	multiPunct = regexp.MustCompile(`[。，！？；：、,!.?;:]{2,}`)
	ctrlChars  = regexp.MustCompile(`[\x00-\x1f\x7f]`)
)

func (TextCleaningStep) Process(c *EnhancementContext) error {
	text := ctrlChars.ReplaceAllString(c.Text, "")
	text = multiSpace.ReplaceAllString(text, " ")
	text = multiPunct.ReplaceAllStringFunc(text, func(s string) string {
		return string([]rune(s)[0])
	})
	if text != c.Text {
		c.Changes = append(c.Changes, &EnhancementChange{
			OriginalText: c.Text, ResultText: text, Action: ActionReplace,
			Type: "CLEAN", Source: "SYSTEM", Confidence: 1.0, Locked: true,
		})
		c.Text = text
	}
	return nil
}

// FillerStep 第二层：口水词处理。
// 仅处理 STRONG_FILLER（呃/额/啊/哦/哈），且位于句首或标点后；「嗯」可能表示肯定，不删。
type FillerStep struct{}

func (FillerStep) Name() string { return "filler" }

var strongFillers = map[string]bool{"呃": true, "额": true, "啊": true, "哦": true, "哈": true}

func (FillerStep) Process(c *EnhancementContext) error {
	runes := []rune(c.Text)
	var out []rune
	changed := false
	for i, r := range runes {
		if strongFillers[string(r)] {
			// 仅删除句首或标点/空白后的口水词
			if i == 0 || isPunctOrSpace(runes[i-1]) {
				changed = true
				continue
			}
		}
		out = append(out, r)
	}
	if changed {
		c.Changes = append(c.Changes, &EnhancementChange{
			OriginalText: c.Text, ResultText: string(out), Action: ActionDelete,
			Type: "FILLER", Source: "SYSTEM", Confidence: 1.0, Locked: true,
		})
		c.Text = string(out)
	}
	return nil
}

func isPunctOrSpace(r rune) bool {
	switch r {
	case ' ', '，', '。', '！', '？', '；', '：', '、', ',', '.', '!', '?', ';', ':':
		return true
	}
	return false
}

// VocabularyMatchingStep 第三层：词库精确匹配。
// 精确命中标准词 → 标记为已知表达（锁定，后续推断不处理），不改变文本。
type VocabularyMatchingStep struct{}

func (VocabularyMatchingStep) Name() string { return "vocabulary_matching" }

func (VocabularyMatchingStep) Process(c *EnhancementContext) error {
	if c.Vocab == nil {
		return nil
	}
	for text := range c.Vocab.entries {
		if strings.Contains(c.Text, text) {
			c.lock(text)
		}
	}
	return nil
}

// AliasResolutionStep 第四层：别名解析（ALIAS 关系 → 标准词）。
// 确定性知识，action=RESOLVE，locked=true。
type AliasResolutionStep struct{}

func (AliasResolutionStep) Name() string { return "alias_resolution" }

func (AliasResolutionStep) Process(c *EnhancementContext) error {
	if c.Vocab == nil {
		return nil
	}
	// 遍历所有 related_text 的 ALIAS 关系
	for text, rels := range c.Vocab.relations {
		for _, rel := range rels {
			if rel.RelationType != "ALIAS" {
				continue
			}
			target := c.resolveTarget(rel)
			if target == "" || !strings.Contains(c.Text, text) {
				continue
			}
			c.Text = strings.ReplaceAll(c.Text, text, target)
			c.lock(target)
			c.Changes = append(c.Changes, &EnhancementChange{
				OriginalText: text, ResultText: target, Action: ActionResolve,
				Type: "ALIAS", Source: "TENANT_DICTIONARY", Confidence: 1.0, Locked: true,
			})
		}
	}
	return nil
}

// DeterministicReplacementStep 第五层：确定性替换（CORRECTION 关系 → 标准词）。
// 确定性知识，action=REPLACE，locked=true。
type DeterministicReplacementStep struct{}

func (DeterministicReplacementStep) Name() string { return "deterministic_replacement" }

func (DeterministicReplacementStep) Process(c *EnhancementContext) error {
	if c.Vocab == nil {
		return nil
	}
	for text, rels := range c.Vocab.relations {
		for _, rel := range rels {
			if rel.RelationType != "CORRECTION" {
				continue
			}
			target := c.resolveTarget(rel)
			if target == "" || !strings.Contains(c.Text, text) {
				continue
			}
			c.Text = strings.ReplaceAll(c.Text, text, target)
			c.lock(target)
			c.Changes = append(c.Changes, &EnhancementChange{
				OriginalText: text, ResultText: target, Action: ActionReplace,
				Type: "CORRECTION", Source: "TENANT_DICTIONARY", Confidence: 1.0, Locked: true,
			})
		}
	}
	return nil
}

// resolveTarget 解析关系的目标标准词：按 target_entry_id 反查词条索引。
func (c *EnhancementContext) resolveTarget(rel *VocabularyRelation) string {
	if rel.TargetEntryID == 0 {
		return ""
	}
	for _, e := range c.Vocab.entries {
		if e.ID == rel.TargetEntryID {
			return e.StandardText
		}
	}
	return ""
}
