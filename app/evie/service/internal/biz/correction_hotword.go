package biz

import (
	"context"
	"strings"
)

// HotwordCorrector 热词纠错器：将识别文本中命中「热词原文」的片段替换为「期望识别结果」。
// 与规则纠错器（source→target）语义一致，但热词由业务方在热词管理页维护，
// 用于将 ASR 常误识别的词（如人名「田工」）纠正为期望词（「田华」）。
// 优先级 100，置信度 1.0（直接替换）。
type HotwordCorrector struct {
	repo HotwordRepo
}

// NewHotwordCorrector 创建热词纠错器。
func NewHotwordCorrector(repo HotwordRepo) *HotwordCorrector {
	return &HotwordCorrector{repo: repo}
}

func (h *HotwordCorrector) Name() string  { return "hotword" }
func (h *HotwordCorrector) Priority() int { return 100 }

// Correct 将文本中命中热词原文（word）的片段替换为期望结果（target）。
// 仅处理 target 非空且 target != word 的热词（纯热词增强暂未接引擎）。
func (h *HotwordCorrector) Correct(ctx context.Context, text string) ([]CorrectionCandidate, error) {
	hotwords, err := h.repo.List(ctx, "")
	if err != nil {
		return nil, err
	}
	var candidates []CorrectionCandidate
	for _, hw := range hotwords {
		word := hw.GetWord()
		target := hw.GetTarget()
		if word == "" || target == "" || word == target {
			continue
		}
		if strings.Contains(text, word) {
			candidates = append(candidates, CorrectionCandidate{
				From:       word,
				To:         target,
				Type:       "hotword",
				Confidence: 1.0,
				Source:     "hotword",
			})
		}
	}
	return candidates, nil
}
