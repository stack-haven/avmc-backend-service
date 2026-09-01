package biz

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/pkg/utils/id"

	"backend-service/pkg/auth/authn"
)

// EnhanceUsecase 纯文本增强编排（独立于 ASRUsecase）。
// session_id 在入口处由 backend 统一生成（id.NewSessionID），前端不再传。
type EnhancementUsecase struct {
	enhancer *EnhancementEngine
	epuc     *EnhancementPolicyUsecase
	eluc     *EnhancementLogUsecase
	log      *log.Helper
}

// NewEnhanceUsecase 创建增强 usecase。
func NewEnhancementUsecase(enhancer *EnhancementEngine, epuc *EnhancementPolicyUsecase, eluc *EnhancementLogUsecase, logger log.Logger) *EnhancementUsecase {
	return &EnhancementUsecase{enhancer: enhancer, epuc: epuc, eluc: eluc, log: log.NewHelper(logger)}
}

// EnhanceResult 文本增强业务结果。
type EnhanceResult struct {
	OriginalText        string
	EnhancedText        string
	Changes             []*EnhancementChange
	StepTimings         map[string]time.Duration
	TotalTime           time.Duration
	StepSnapshots       []*StepSnapshot
	Status              int32
	ProcessingTimeMs    int64
	CleaningTimeMs      int64
	FillerTimeMs        int64
	VocabMatchTimeMs    int64
	AliasTimeMs         int64
	DeterministicTimeMs int64
	PinyinTimeMs        int64
	FuzzyTimeMs         int64
	ContextTimeMs       int64
	ErrorMessage        string
	SessionID           string
}

// EnhanceText 纯文本增强入口（对应 proto EnhanceText RPC）。
// session_id 后端统一生成（UUID v4 + 业务前缀）。
// profile_id 决定策略：0=租户默认；非 0=按场景关联策略。
func (uc *EnhancementUsecase) EnhanceText(ctx context.Context, req *pb.EnhanceTextRequest) (*EnhanceResult, error) {
	sessionID := id.NewSessionID(id.SessionIDPrefixEnhanceText)
	return uc.EnhanceTextWithSessionID(ctx, req, sessionID)
}

// EnhanceTextWithSessionID 纯文本增强入口（对应 proto EnhanceText RPC）。
func (uc *EnhancementUsecase) EnhanceTextWithSessionID(ctx context.Context, req *pb.EnhanceTextRequest, sessionID string) (*EnhanceResult, error) {
	policy, err := uc.ResolveEnhancePolicy(ctx, uc.epuc, req.GetProfileId())
	if err != nil {
		return nil, err
	}
	result, err := uc.enhancer.EnhanceWithPolicy(ctx, req.GetText(), policy)
	if err != nil {
		return nil, err
	}

	// 写增强日志（包含 8 步耗时 + step snapshots + 变更列表）
	changesJSON, _ := json.Marshal(result.Changes)
	snapshotsJSON, _ := json.Marshal(result.StepSnapshots)
	logData := &EnhancementLogData{
		SessionID:           sessionID,
		RawText:             result.RawText,
		EnhancedText:        result.EnhancedText,
		ChangesJSON:         string(changesJSON),
		StepSnapshotsJSON:   string(snapshotsJSON),
		ProcessingTimeMs:    result.TotalTime.Milliseconds(),
		CleaningTimeMs:      StepMs(result.StepTimings, "cleaning"),
		FillerTimeMs:        StepMs(result.StepTimings, "filler"),
		VocabMatchTimeMs:    StepMs(result.StepTimings, "vocabulary_matching"),
		AliasTimeMs:         StepMs(result.StepTimings, "alias_resolution"),
		DeterministicTimeMs: StepMs(result.StepTimings, "deterministic_replacement"),
		PinyinTimeMs:        StepMs(result.StepTimings, "pinyin_correction"),
		FuzzyTimeMs:         StepMs(result.StepTimings, "fuzzy_matching"),
		ContextTimeMs:       StepMs(result.StepTimings, "context_correction"),
		Status:              1,
	}
	// policy 可能为 nil（默认场景下未匹配策略），安全获取 Id/Mode/Name
	if policy != nil {
		logData.PolicyID = policy.Id
		logData.PolicyMode = policy.Mode
		logData.PolicyName = policy.Name
	}
	uc.eluc.Save(ctx, logData)

	return &EnhanceResult{
		OriginalText:        result.RawText,
		EnhancedText:        result.EnhancedText,
		Changes:             result.Changes,
		StepTimings:         result.StepTimings,
		TotalTime:           result.TotalTime,
		StepSnapshots:       result.StepSnapshots,
		Status:              1,
		ProcessingTimeMs:    result.TotalTime.Milliseconds(),
		CleaningTimeMs:      StepMs(result.StepTimings, "cleaning"),
		FillerTimeMs:        StepMs(result.StepTimings, "filler"),
		VocabMatchTimeMs:    StepMs(result.StepTimings, "vocabulary_matching"),
		AliasTimeMs:         StepMs(result.StepTimings, "alias_resolution"),
		DeterministicTimeMs: StepMs(result.StepTimings, "deterministic_replacement"),
		PinyinTimeMs:        StepMs(result.StepTimings, "pinyin_correction"),
		FuzzyTimeMs:         StepMs(result.StepTimings, "fuzzy_matching"),
		ContextTimeMs:       StepMs(result.StepTimings, "context_correction"),
		SessionID:           sessionID,
	}, nil
}

// ToProto 业务结果转 proto。
func (r *EnhanceResult) ToProto() *pb.EnhanceTextResponse {
	if r == nil {
		return nil
	}
	resp := &pb.EnhanceTextResponse{
		OriginalText:        r.OriginalText,
		EnhancedText:        r.EnhancedText,
		Changes:             make([]*pb.EnhanceChange, 0, len(r.Changes)),
		Status:              r.Status,
		ProcessingTimeMs:    r.ProcessingTimeMs,
		CleaningTimeMs:      r.CleaningTimeMs,
		FillerTimeMs:        r.FillerTimeMs,
		VocabMatchTimeMs:    r.VocabMatchTimeMs,
		AliasTimeMs:         r.AliasTimeMs,
		DeterministicTimeMs: r.DeterministicTimeMs,
		PinyinTimeMs:        r.PinyinTimeMs,
		FuzzyTimeMs:         r.FuzzyTimeMs,
		ContextTimeMs:       r.ContextTimeMs,
		ErrorMessage:        r.ErrorMessage,
		SessionId:           r.SessionID,
	}
	for _, ch := range r.Changes {
		resp.Changes = append(resp.Changes, &pb.EnhanceChange{
			From:       ch.OriginalText,
			To:         ch.ResultText,
			Type:       ch.Type,
			Confidence: float32(ch.Confidence),
		})
	}
	return resp
}

// StepMs 从 stepTimings map 中取步骤耗时，找不到返回 0。
func StepMs(m map[string]time.Duration, key string) int64 {
	if d, ok := m[key]; ok {
		return d.Milliseconds()
	}
	return 0
}

// ResolveEnhancePolicy 解析文本增强方案：
//   - profileID > 0：按增强场景（Profile）绑定的策略执行
//
// 设计原则：增强策略只能通过场景 Profile 关联，不接受 policy_id 直传。
// 这与系统「场景关联策略」的设计一致——避免策略在多个场景间游离。
func (uc *EnhancementUsecase) ResolveEnhancePolicy(ctx context.Context, epuc *EnhancementPolicyUsecase, profileID uint32) (*pb.EnhancementPolicy, error) {
	if uc == nil {
		return nil, nil
	}
	// 1) profileID：按场景绑定的策略取
	if profileID != 0 {
		profile, err := uc.epuc.GetProfile(ctx, profileID)
		if err != nil {
			return nil, err
		}
		if profile == nil || profile.GetPolicyId() == 0 {
			return nil, nil
		}
		return uc.epuc.GetPolicy(ctx, profile.GetPolicyId())
	}
	// 默认增强方案：优先选择「高性能模式且启用口水词+别名+确定性替换」的策略；
	// 其次选启用核心步骤（口水词+别名+确定性替换）的策略；没有则 nil（执行全部步骤）。
	policies, _, err := uc.epuc.ListPolicies(ctx, &pb.ListPoliciesRequest{PageSize: 100})
	if err != nil {
		return nil, err
	}
	for _, p := range policies {
		if p.GetMode() == "HIGH_PERFORMANCE" && p.GetFillerRemoval() && p.GetAliasResolution() && p.GetDeterministicReplacement() {
			return p, nil
		}
	}
	for _, p := range policies {
		if p.GetFillerRemoval() && p.GetAliasResolution() && p.GetDeterministicReplacement() {
			return p, nil
		}
	}
	return nil, nil
}

// EnhancementAction 增强动作类型（开发说明第三十六节）。
const (
	ActionKeep    = "KEEP"
	ActionReplace = "REPLACE"
	ActionDelete  = "DELETE"
	ActionSuggest = "SUGGEST"
	ActionResolve = "RESOLVE"
)

// EnhancementChange 单条文本增强变更（开发说明第三十七节）。
// JSON tag 与 proto EnhanceChange 字段名保持一致，便于直接序列化到 enhancement_logs.changes_json
// 并由 GetRecordDetail 反序列化回 pb.EnhanceChange。
type EnhancementChange struct {
	OriginalText string  `json:"from,omitempty"`
	ResultText   string  `json:"to,omitempty"`
	Action       string  `json:"action,omitempty"`
	Type         string  `json:"type,omitempty"`
	Source       string  `json:"source,omitempty"`
	Confidence   float64 `json:"confidence,omitempty"`
	Reason       string  `json:"reason,omitempty"`
	Locked       bool    `json:"locked,omitempty"`
}

// StepSnapshot 单步增强快照（用于步骤图/分词明细展示）。
// JSON tag 与 proto EnhancementStepSnapshot 字段名保持一致。
type StepSnapshot struct {
	Step       string               `json:"step,omitempty"`
	Before     string               `json:"before,omitempty"`
	After      string               `json:"after,omitempty"`
	DurationMs int64                `json:"duration_ms,omitempty"`
	Skipped    bool                 `json:"skipped,omitempty"`
	Changes    []*EnhancementChange `json:"changes,omitempty"`
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
		&FillerStep{},                   // ② 口水词
		&VocabularyMatchingStep{},       // ③ 词库匹配
		&AliasResolutionStep{},          // ④ 别名解析
		&DeterministicReplacementStep{}, // ⑤ 确定性替换
		&PhraseStandardizationStep{},    // 短语标准化（确定性规则）
		&PinyinCorrectionStep{},         // ⑥ 拼音纠错（推断）
		&FuzzyMatchingStep{},            // ⑦ 模糊匹配（推断）
		&ContextCorrectionStep{},        // ⑧ 上下文纠错（推断）
	}
	return e
}

// Enhance 执行完整文本增强流水线（默认 STANDARD 策略：全部确定性 + 推断）。
func (e *EnhancementEngine) Enhance(ctx context.Context, rawText string) (*EnhancementResult, error) {
	return e.enhance(ctx, rawText, nil)
}

// EnhanceWithPolicy 按策略执行流水线（仅启用策略开启的步骤）。
func (e *EnhancementEngine) EnhanceWithPolicy(ctx context.Context, rawText string, policy *pb.EnhancementPolicy) (*EnhancementResult, error) {
	return e.enhance(ctx, rawText, policy)
}

func (e *EnhancementEngine) enhance(ctx context.Context, rawText string, policy *pb.EnhancementPolicy) (*EnhancementResult, error) {
	totalStart := time.Now()
	vc, err := e.vocab.Build(ctx, authn.GetAuthUserTenantID(ctx))
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
		// 逐个记录实际清洗片段（重复标点/连续空格/控制字符），避免把整段文本塞入变更导致前端 Tag 撑大
		for _, s := range ctrlChars.FindAllString(c.Text, -1) {
			c.Changes = append(c.Changes, &EnhancementChange{
				OriginalText: s, ResultText: "", Action: ActionDelete,
				Type: "CLEAN", Source: "SYSTEM", Confidence: 1.0, Locked: true,
			})
		}
		for _, s := range multiSpace.FindAllString(c.Text, -1) {
			if s != " " {
				c.Changes = append(c.Changes, &EnhancementChange{
					OriginalText: s, ResultText: " ", Action: ActionReplace,
					Type: "CLEAN", Source: "SYSTEM", Confidence: 1.0, Locked: true,
				})
			}
		}
		for _, s := range multiPunct.FindAllString(c.Text, -1) {
			if len([]rune(s)) > 1 {
				c.Changes = append(c.Changes, &EnhancementChange{
					OriginalText: s, ResultText: string([]rune(s)[0]), Action: ActionReplace,
					Type: "CLEAN", Source: "SYSTEM", Confidence: 1.0, Locked: true,
				})
			}
		}
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
	var removed []string
	changed := false
	for i, r := range runes {
		if strongFillers[string(r)] {
			// 仅删除句首或标点/空白后的口水词
			if i == 0 || isPunctOrSpace(runes[i-1]) {
				removed = append(removed, string(r))
				changed = true
				continue
			}
		}
		out = append(out, r)
	}
	if changed {
		// 逐个记录被删除的口水词，避免把整段文本塞入变更
		for _, f := range removed {
			c.Changes = append(c.Changes, &EnhancementChange{
				OriginalText: f, ResultText: "", Action: ActionDelete,
				Type: "FILLER", Source: "SYSTEM", Confidence: 1.0, Locked: true,
			})
		}
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
