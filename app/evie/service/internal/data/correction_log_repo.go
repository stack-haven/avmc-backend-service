package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/evie/service/internal/biz"
)

type correctionLogRepo struct{ BaseRepo }

// NewCorrectionLogRepo 创建纠错记录仓库。
func NewCorrectionLogRepo(data *Data, logger log.Logger) biz.CorrectionLogRepo {
	return &correctionLogRepo{BaseRepo: NewBaseRepo(data, logger)}
}

// Save 保存一条纠错记录（只追加）。
func (r *correctionLogRepo) Save(ctx context.Context, log *biz.CorrectionLog) error {
	create := r.Data.DB(ctx).CorrectionLog.Create().
		SetSessionID(log.SessionID).
		SetOriginalText(log.OriginalText).
		SetCorrectedText(log.CorrectedText).
		SetChangesJSON(log.ChangesJSON).
		SetConfidence(log.Confidence).
		SetNeedConfirm(log.NeedConfirm).
		SetDurationMs(log.DurationMs).
		SetRuleHits(log.RuleHits).
		SetPinyinHits(log.PinyinHits).
		SetEntityHits(log.EntityHits).
		SetLlmHits(log.LLMHits)
	if log.UserID > 0 {
		create.SetUserID(log.UserID)
	}
	if _, err := create.Save(ctx); err != nil {
		return err
	}
	return nil
}
