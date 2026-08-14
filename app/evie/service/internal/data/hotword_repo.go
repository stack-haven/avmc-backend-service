package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/hotword"
)

type hotwordRepo struct{ BaseRepo }

// NewHotwordRepo 创建热词仓库。
func NewHotwordRepo(data *Data, logger log.Logger) biz.HotwordRepo {
	return &hotwordRepo{BaseRepo: NewBaseRepo(data, logger)}
}

// hotwordProto converts an Ent Hotword to a proto Hotword.
func hotwordProto(row *gen.Hotword) *pb.Hotword {
	return &pb.Hotword{
		Id:        row.ID,
		Word:      row.Word,
		Target:    row.Target,
		Weight:    float32(row.Weight),
		Category:  row.Category,
		CreatedAt: row.CreatedAt.Format(time.DateTime),
		UpdatedAt: row.UpdatedAt.Format(time.DateTime),
	}
}

// List 查询热词列表。
func (r *hotwordRepo) List(ctx context.Context, category string) ([]*pb.Hotword, error) {
	query := r.Data.DB(ctx).Hotword.Query()
	if category != "" {
		query.Where(hotword.CategoryEQ(category))
	}
	rows, err := query.Order(gen.Desc(hotword.FieldWeight), gen.Desc(hotword.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*pb.Hotword, 0, len(rows))
	for _, row := range rows {
		result = append(result, hotwordProto(row))
	}
	return result, nil
}

// Upsert 新增或更新热词。
func (r *hotwordRepo) Upsert(ctx context.Context, h *pb.Hotword) (*pb.Hotword, error) {
	if h.GetId() == 0 {
		row, err := r.Data.DB(ctx).Hotword.Create().
			SetWord(h.GetWord()).
			SetTarget(h.GetTarget()).
			SetWeight(float64(h.GetWeight())).
			SetCategory(h.GetCategory()).
			Save(ctx)
		if gen.IsConstraintError(err) {
			return nil, errors.Conflict("HOTWORD_EXISTS", "同租户下热词已存在")
		}
		if err != nil {
			return nil, err
		}
		return hotwordProto(row), nil
	}
	row, err := r.Data.DB(ctx).Hotword.UpdateOneID(h.GetId()).
		SetWord(h.GetWord()).
		SetTarget(h.GetTarget()).
		SetWeight(float64(h.GetWeight())).
		SetCategory(h.GetCategory()).
		Save(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("HOTWORD_NOT_FOUND", "热词不存在")
	}
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("HOTWORD_EXISTS", "同租户下热词已存在")
	}
	if err != nil {
		return nil, err
	}
	return hotwordProto(row), nil
}

// Delete 物理删除热词。
func (r *hotwordRepo) Delete(ctx context.Context, id uint32) error {
	if err := r.Data.DB(ctx).Hotword.DeleteOneID(id).Exec(ctx); gen.IsNotFound(err) {
		return errors.NotFound("HOTWORD_NOT_FOUND", "热词不存在")
	} else if err != nil {
		return err
	}
	return nil
}
