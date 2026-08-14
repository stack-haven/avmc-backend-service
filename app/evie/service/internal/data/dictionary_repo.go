package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionaryalias"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionaryword"
	"backend-service/pkg/aip/listing"
)

type dictionaryRepo struct{ BaseRepo }

// NewDictionaryRepo 创建字典中心仓库。
func NewDictionaryRepo(data *Data, logger log.Logger) biz.DictionaryRepo {
	return &dictionaryRepo{BaseRepo: NewBaseRepo(data, logger)}
}

// dictionaryWordProto converts an Ent DictionaryWord to a proto DictionaryWord.
func dictionaryWordProto(row *gen.DictionaryWord) *pb.DictionaryWord {
	status := int32(0)
	if row.Status != nil {
		status = *row.Status
	}
	result := &pb.DictionaryWord{
		Id:        row.ID,
		Word:      row.Word,
		Level:     row.Level,
		Category:  row.Category,
		Source:    row.Source,
		Priority:  row.Priority,
		Status:    status,
		CreatedAt: row.CreatedAt.Format(time.DateTime),
		UpdatedAt: row.UpdatedAt.Format(time.DateTime),
	}
	if aliases := row.Edges.Aliases; len(aliases) > 0 {
		result.Aliases = make([]*pb.DictionaryAlias, 0, len(aliases))
		for _, a := range aliases {
			result.Aliases = append(result.Aliases, dictionaryAliasProto(a))
		}
	}
	return result
}

// dictionaryAliasProto converts an Ent DictionaryAlias to a proto DictionaryAlias.
func dictionaryAliasProto(row *gen.DictionaryAlias) *pb.DictionaryAlias {
	return &pb.DictionaryAlias{
		Id:     row.ID,
		WordId: row.WordID,
		Alias:  row.Alias,
		Pinyin: row.Pinyin,
		Weight: float32(row.Weight),
		Source: row.Source,
	}
}

// ListWords 分页查询标准词。
func (r *dictionaryRepo) ListWords(ctx context.Context, req *pb.ListWordsRequest) ([]*pb.DictionaryWord, int32, error) {
	query := r.Data.DB(ctx).DictionaryWord.Query().Where(dictionaryword.DeletedAtIsNil())
	if req.GetCategory() != "" {
		query.Where(dictionaryword.CategoryEQ(req.GetCategory()))
	}
	if req.GetKeyword() != "" {
		query.Where(dictionaryword.WordContains(req.GetKeyword()))
	}
	if req.GetStatus() != 0 {
		query.Where(dictionaryword.StatusEQ(req.GetStatus()))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.
		WithAliases(func(q *gen.DictionaryAliasQuery) {
			q.Where(dictionaryalias.DeletedAtIsNil()).Order(gen.Asc(dictionaryalias.FieldID))
		}).
		Order(gen.Desc(dictionaryword.FieldPriority), gen.Desc(dictionaryword.FieldID)).
		Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.DictionaryWord, 0, len(rows))
	for _, row := range rows {
		result = append(result, dictionaryWordProto(row))
	}
	return result, int32(total), nil
}

// GetWord 查询标准词详情（含别名）。
func (r *dictionaryRepo) GetWord(ctx context.Context, id uint32) (*pb.DictionaryWord, error) {
	row, err := r.Data.DB(ctx).DictionaryWord.Query().
		Where(dictionaryword.IDEQ(id), dictionaryword.DeletedAtIsNil()).
		WithAliases(func(q *gen.DictionaryAliasQuery) {
			q.Where(dictionaryalias.DeletedAtIsNil()).Order(gen.Asc(dictionaryalias.FieldID))
		}).
		Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("DICTIONARY_WORD_NOT_FOUND", "字典标准词不存在")
	}
	if err != nil {
		return nil, err
	}
	return dictionaryWordProto(row), nil
}

// CreateWord 创建标准词，并写入其别名（事务）。
func (r *dictionaryRepo) CreateWord(ctx context.Context, word *pb.DictionaryWord) (*pb.DictionaryWord, error) {
	var createdID uint32
	err := r.Data.InTx(ctx, func(ctx context.Context) error {
		row, err := r.Data.DB(ctx).DictionaryWord.Create().
			SetWord(word.GetWord()).
			SetLevel(word.GetLevel()).
			SetCategory(word.GetCategory()).
			SetSource(word.GetSource()).
			SetPriority(word.GetPriority()).
			Save(ctx)
		if gen.IsConstraintError(err) {
			return errors.Conflict("DICTIONARY_WORD_EXISTS", "同租户下标准词已存在")
		}
		if err != nil {
			return err
		}
		createdID = row.ID
		for _, a := range word.GetAliases() {
			aliasCreate := r.Data.DB(ctx).DictionaryAlias.Create().
				SetWordID(row.ID).
				SetAlias(a.GetAlias()).
				SetWeight(float64(a.GetWeight())).
				SetSource(a.GetSource())
			if a.GetPinyin() != "" {
				aliasCreate.SetPinyin(a.GetPinyin())
			}
			if _, err := aliasCreate.Save(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetWord(ctx, createdID)
}

// UpdateWord 更新标准词字段（别名另行管理）。
func (r *dictionaryRepo) UpdateWord(ctx context.Context, word *pb.DictionaryWord) (*pb.DictionaryWord, error) {
	update := r.Data.DB(ctx).DictionaryWord.UpdateOneID(word.GetId())
	if word.GetWord() != "" {
		update.SetWord(word.GetWord())
	}
	if word.GetCategory() != "" {
		update.SetCategory(word.GetCategory())
	}
	if word.GetPriority() != 0 {
		update.SetPriority(word.GetPriority())
	}
	if word.GetStatus() != 0 {
		update.SetStatus(word.GetStatus())
	}
	if _, err := update.Save(ctx); gen.IsNotFound(err) {
		return nil, errors.NotFound("DICTIONARY_WORD_NOT_FOUND", "字典标准词不存在")
	} else if gen.IsConstraintError(err) {
		return nil, errors.Conflict("DICTIONARY_WORD_EXISTS", "同租户下标准词已存在")
	} else if err != nil {
		return nil, err
	}
	return r.GetWord(ctx, word.GetId())
}

// DeleteWord 软删除标准词，并级联软删除别名。
func (r *dictionaryRepo) DeleteWord(ctx context.Context, id uint32) error {
	now := time.Now()
	return r.Data.InTx(ctx, func(ctx context.Context) error {
		if err := r.Data.DB(ctx).DictionaryWord.UpdateOneID(id).SetDeletedAt(now).Exec(ctx); gen.IsNotFound(err) {
			return errors.NotFound("DICTIONARY_WORD_NOT_FOUND", "字典标准词不存在")
		} else if err != nil {
			return err
		}
		if err := r.Data.DB(ctx).DictionaryAlias.Update().
			Where(dictionaryalias.WordIDEQ(id), dictionaryalias.DeletedAtIsNil()).
			SetDeletedAt(now).Exec(ctx); err != nil {
			return err
		}
		return nil
	})
}
