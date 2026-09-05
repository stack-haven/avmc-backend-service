// Package biz · lexnorm_engine.go
// 构造 lexnorm.Engine（per-tenant ProfileResolver + ASR 预设 Pipeline）。
//
// 设计：
//   - ProfileResolver 模式：每租户请求触发 lazy Build（VocabSyncer 已有的机制）
//   - Pipeline 顺序匹配 evie/tool 业务需求：
//     normalize → disfluency → alias → deterministic → pinyin → fuzzy_vocab → ctxproc
//   - 全局 Config：AutoApplyThreshold=0.95, SuggestThreshold=0.65（lexnorm DefaultConfig）
//   - per-category fuzzy 阈值由 FuzzyVocabConfig 处理（不写到 lexnorm Config）
package biz

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/alias"
	"github.com/stack-haven/lexnorm/processor/ctxproc"
	"github.com/stack-haven/lexnorm/processor/deterministic"
	"github.com/stack-haven/lexnorm/processor/disfluency"
	"github.com/stack-haven/lexnorm/processor/normalize"
	"github.com/stack-haven/lexnorm/processor/pinyin"

	"backend-service/app/evie/tool/internal/biz/processor"
	pkgpinyin "backend-service/pkg/pinyin"
)

// TenantProfileResolver 实现 lexnorm.ProfileResolver：按 tenantID 懒加载词库。
//
// 关键链路：
//   1. Resolve(ctx, "1889501240003497986") 由 Engine.Normalize 触发
//   2. 调 VocabularyBuilder.Build(ctx, tenantID)：
//      - 命中：直接返回 VocabularySnapshot
//      - miss：触发 lazySyncOnMiss（VocabSyncer.EnsureTenant），并发走 fallback/system
//   3. 把 VocabularySnapshot 转成 lexnorm.Lexicon（buildLexiconFromSnapshot）
//   4. 构造 Pipeline（每个 tenant 独立 Pipeline 实例，因为 Lexicon 不同）
//   5. 返回 lexnorm.Runtime
type TenantProfileResolver struct {
	builder *VocabularyBuilder
	cfg     lexnorm.Config
	logger  *log.Helper
}

// NewTenantProfileResolver 构造 resolver。
func NewTenantProfileResolver(builder *VocabularyBuilder, cfg lexnorm.Config, logger log.Logger) *TenantProfileResolver {
	return &TenantProfileResolver{
		builder: builder,
		cfg:     cfg,
		logger:  log.NewHelper(log.With(logger, "module", "lexnorm/resolver")),
	}
}

// Resolve 实现 lexnorm.ProfileResolver。
//
// 返回 *lexnorm.Runtime（Pipeline + Lexicon + Config 快照）。
func (r *TenantProfileResolver) Resolve(ctx context.Context, profileID lexnorm.ProfileID) (*lexnorm.Runtime, error) {
	tenantID := string(profileID)
	if tenantID == "" {
		tenantID = "default"
	}

	snap := r.builder.Build(ctx, tenantID)
	if snap == nil {
		return nil, fmt.Errorf("lexnorm/resolver: nil snapshot for tenant=%s", tenantID)
	}

	lex, err := buildLexiconFromSnapshot(snap, r.builder.systemDict, snap.Version)
	if err != nil {
		r.logger.Warnf("build lexicon failed for tenant=%s: %v (use empty lexicon)", tenantID, err)
		// 不中断请求：返回空 Lexicon，pipeline 仍能跑（只是没匹配）
		lex, _ = buildEmptyLexicon()
	}

	pipeline := r.buildPipeline(lex)
	return lexnorm.NewRuntime(profileID, lexnorm.ProfileBundle{
		Lexicon:  lex,
		Pipeline: pipeline,
		Config:   r.cfg,
	})
}

// buildPipeline 构造本租户的 pipeline。
func (r *TenantProfileResolver) buildPipeline(lex lexicon.Lexicon) lexnorm.Pipeline {
	return lexnorm.NewPipeline(
		normalize.New(),
		disfluency.New(),
		alias.New(lex),
		deterministic.New(lex),
		pinyin.New(lex, &defaultPinyinConverter{}),
		processor.NewFuzzyVocabProcessor(lex, processor.DefaultFuzzyVocabConfig()),
		ctxproc.New(),
	)
}

// buildEmptyLexicon 构造一个空的 Lexicon（HA 兜底用）。
func buildEmptyLexicon() (lexicon.Lexicon, error) {
	return lexicon.NewBuilderWithVersion("empty").Build()
}

// defaultPinyinConverter 桥接 backend-service/pkg/pinyin 到 lexnorm.PinyinConverter。
//
// 实现：
//   - 懒初始化 pkg/pinyin.Converter（一次性构造）
//   - 对 text 的每个 CJK 字符调 pkg/pinyin.Convert 拿拼音
//   - 返回该字符的所有拼音形式（去重）
type defaultPinyinConverter struct {
	once sync.Once
	conv pkgpinyin.Converter
}

func (p *defaultPinyinConverter) get() pkgpinyin.Converter {
	p.once.Do(func() {
		p.conv = pkgpinyin.NewConverter()
	})
	return p.conv
}

// ToPinyin 实现 lexnorm.PinyinConverter。
//
// 返回 text 中**每个字符**的拼音形式。
// lexnorm 的 pinyin processor 在 Process 中按 rune 扫描，对每个 char 调一次 ToPinyin(char)。
//
// 实现策略：
//   - 取每个字符的拼音（去重）
//   - 非 CJK 字符：原样返回（保持 passthrough 语义）
//   - go-pinyin 对非汉字返回空字符串，此处用原字符作 fallback
func (p *defaultPinyinConverter) ToPinyin(text string) []string {
	if text == "" {
		return nil
	}
	conv := p.get()
	// 只处理单字符场景（lexnorm pinyin 按 rune 调）
	r := []rune(text)
	if len(r) != 1 {
		return []string{text}
	}
	ch := string(r[0])
	// 非 CJK 跳过
	if r[0] < 0x4E00 || r[0] > 0x9FFF {
		return []string{ch}
	}
	res, err := conv.Convert(ch, false)
	if err != nil || res == nil || res.Pinyin == "" {
		return []string{ch}
	}
	// "ke" → ["ke"]; "ke fu" → ["ke", "fu"]; 多音字已天然用空格分开
	forms := strings.Fields(res.Pinyin)
	if len(forms) == 0 {
		return []string{ch}
	}
	return forms
}
