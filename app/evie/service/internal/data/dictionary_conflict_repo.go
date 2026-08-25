package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/evie/service/internal/biz"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionaryconflict"
)

type dictionaryConflictRepo struct{ BaseRepo }

// NewDictionaryConflictRepo 创建词库冲突记录仓库。
func NewDictionaryConflictRepo(data *Data, logger log.Logger) biz.DictionaryConflictRecorder {
	return &dictionaryConflictRepo{BaseRepo: NewBaseRepo(data, logger)}
}

// RecordConflict 记录一条词库冲突（幂等：同 input+scope+dictionary 已存在则跳过）。
func (r *dictionaryConflictRepo) RecordConflict(ctx context.Context, conflict *biz.DictionaryConflict) error {
	if conflict == nil || conflict.Input == "" {
		return nil
	}
	exists, err := r.Data.DB(ctx).DictionaryConflict.Query().
		Where(
			dictionaryconflict.InputEQ(conflict.Input),
			dictionaryconflict.SourceScopeEQ(conflict.SourceScope),
			dictionaryconflict.SourceDictionaryEQ(conflict.SourceDictionary),
		).
		Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = r.Data.DB(ctx).DictionaryConflict.Create().
		SetInput(conflict.Input).
		SetCandidate(conflict.Candidate).
		SetSourceScope(conflict.SourceScope).
		SetSourceDictionary(conflict.SourceDictionary).
		SetPriority(conflict.Priority).
		SetResolvedCandidate(conflict.ResolvedCandidate).
		Save(ctx)
	return err
}

// ListConflicts 查询全部冲突记录。
func (r *dictionaryConflictRepo) ListConflicts(ctx context.Context) ([]*biz.DictionaryConflict, error) {
	rows, err := r.Data.DB(ctx).DictionaryConflict.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*biz.DictionaryConflict, 0, len(rows))
	for _, row := range rows {
		result = append(result, &biz.DictionaryConflict{
			Input:             row.Input,
			Candidate:         row.Candidate,
			SourceScope:       row.SourceScope,
			SourceDictionary:  row.SourceDictionary,
			Priority:          row.Priority,
			ResolvedCandidate: row.ResolvedCandidate,
		})
	}
	return result, nil
}
