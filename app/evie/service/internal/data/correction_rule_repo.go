package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/evie/service/internal/biz"
	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/correctionrule"
)

type correctionRuleRepo struct{ BaseRepo }

// NewCorrectionRuleRepo 创建纠错规则仓库。
func NewCorrectionRuleRepo(data *Data, logger log.Logger) biz.CorrectionRuleRepo {
	return &correctionRuleRepo{BaseRepo: NewBaseRepo(data, logger)}
}

// List 查询租户全部启用的纠错规则。
func (r *correctionRuleRepo) List(ctx context.Context) ([]biz.CorrectionRule, error) {
	rows, err := r.Data.DB(ctx).CorrectionRule.Query().
		Where(correctionrule.DeletedAtIsNil()).
		Order(gen.Desc(correctionrule.FieldPriority)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]biz.CorrectionRule, 0, len(rows))
	for _, row := range rows {
		result = append(result, biz.CorrectionRule{
			Source:   row.Source,
			Target:   row.Target,
			Type:     row.Type,
			Priority: row.Priority,
		})
	}
	return result, nil
}
