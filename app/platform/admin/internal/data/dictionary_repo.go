package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pb "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/dictionaryitem"
	"backend-service/app/platform/admin/internal/data/ent/gen/dictionarytype"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
)

type dictionaryRepo struct{ BaseRepo }

func NewDictionaryRepo(data *Data, logger log.Logger) biz.DictionaryRepo {
	return &dictionaryRepo{BaseRepo: NewBaseRepo(data, logger)}
}
func dictionaryTypeProto(row *gen.DictionaryType) *pb.DictionaryType {
	status := enum.Status(0)
	if row.Status != nil {
		status = enum.Status(*row.Status)
	}
	return &pb.DictionaryType{Id: row.ID, Name: row.Name, Code: row.Code, Status: &status, Sort: &row.Sort, Remark: &row.Remark, CreatedAt: convert.TimeValueToString(&row.CreatedAt, time.DateTime), UpdatedAt: convert.TimeValueToString(&row.UpdatedAt, time.DateTime)}
}
func dictionaryItemProto(row *gen.DictionaryItem) *pb.DictionaryItem {
	status := enum.Status(0)
	if row.Status != nil {
		status = enum.Status(*row.Status)
	}
	return &pb.DictionaryItem{Id: row.ID, TypeId: row.TypeID, Label: row.Label, Value: row.Value, Status: &status, Sort: &row.Sort, Color: &row.Color, Remark: &row.Remark, CreatedAt: convert.TimeValueToString(&row.CreatedAt, time.DateTime), UpdatedAt: convert.TimeValueToString(&row.UpdatedAt, time.DateTime)}
}
func (r *dictionaryRepo) ListTypes(ctx context.Context, req *pb.ListDictionaryTypesRequest) ([]*pb.DictionaryType, int32, error) {
	query := r.Data.DB(ctx).DictionaryType.Query().Where(dictionarytype.DeletedAtIsNil())
	if req.Name != nil {
		query.Where(dictionarytype.NameContains(*req.Name))
	}
	if req.Code != nil {
		query.Where(dictionarytype.CodeContains(*req.Code))
	}
	if req.Status != nil {
		query.Where(dictionarytype.StatusEQ(int32(*req.Status)))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.WithItems(func(q *gen.DictionaryItemQuery) {
		q.Where(dictionaryitem.DeletedAtIsNil()).Order(gen.Asc(dictionaryitem.FieldSort), gen.Asc(dictionaryitem.FieldID))
	}).Order(gen.Asc(dictionarytype.FieldSort), gen.Desc(dictionarytype.FieldID)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.DictionaryType, 0, len(rows))
	for _, row := range rows {
		item := dictionaryTypeProto(row)
		for _, child := range row.Edges.Items {
			item.Items = append(item.Items, dictionaryItemProto(child))
		}
		result = append(result, item)
	}
	return result, int32(total), nil
}
func (r *dictionaryRepo) GetType(ctx context.Context, id uint32) (*pb.DictionaryType, error) {
	row, err := r.Data.DB(ctx).DictionaryType.Query().Where(dictionarytype.IDEQ(id), dictionarytype.DeletedAtIsNil()).WithItems(func(q *gen.DictionaryItemQuery) {
		q.Where(dictionaryitem.DeletedAtIsNil()).Order(gen.Asc(dictionaryitem.FieldSort))
	}).Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("DICTIONARY_NOT_FOUND", "字典不存在")
	}
	if err != nil {
		return nil, err
	}
	result := dictionaryTypeProto(row)
	for _, child := range row.Edges.Items {
		result.Items = append(result.Items, dictionaryItemProto(child))
	}
	return result, nil
}
func (r *dictionaryRepo) CreateType(ctx context.Context, value *pb.DictionaryType) (*pb.DictionaryType, error) {
	row, err := r.Data.DB(ctx).DictionaryType.Create().SetName(value.GetName()).SetCode(value.GetCode()).SetStatus(int32(value.GetStatus())).SetSort(value.GetSort()).SetRemark(value.GetRemark()).Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("DICTIONARY_CODE_EXISTS", "字典编码已存在")
	}
	if err != nil {
		return nil, err
	}
	return dictionaryTypeProto(row), nil
}
func (r *dictionaryRepo) UpdateType(ctx context.Context, value *pb.DictionaryType) (*pb.DictionaryType, error) {
	row, err := r.Data.DB(ctx).DictionaryType.UpdateOneID(value.GetId()).SetName(value.GetName()).SetCode(value.GetCode()).SetStatus(int32(value.GetStatus())).SetSort(value.GetSort()).SetRemark(value.GetRemark()).Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("DICTIONARY_CODE_EXISTS", "字典编码已存在")
	}
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("DICTIONARY_NOT_FOUND", "字典不存在")
	}
	if err != nil {
		return nil, err
	}
	return dictionaryTypeProto(row), nil
}
func (r *dictionaryRepo) DeleteType(ctx context.Context, id uint32) error {
	count, err := r.Data.DB(ctx).DictionaryItem.Query().Where(dictionaryitem.TypeIDEQ(id), dictionaryitem.DeletedAtIsNil()).Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.Conflict("DICTIONARY_NOT_EMPTY", "请先删除字典项")
	}
	err = r.Data.DB(ctx).DictionaryType.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if gen.IsNotFound(err) {
		return errors.NotFound("DICTIONARY_NOT_FOUND", "字典不存在")
	}
	return err
}
func (r *dictionaryRepo) ListItems(ctx context.Context, req *pb.ListDictionaryItemsRequest) ([]*pb.DictionaryItem, error) {
	if _, err := r.GetType(ctx, req.GetTypeId()); err != nil {
		return nil, err
	}
	query := r.Data.DB(ctx).DictionaryItem.Query().Where(dictionaryitem.TypeIDEQ(req.GetTypeId()), dictionaryitem.DeletedAtIsNil())
	if req.Status != nil {
		query.Where(dictionaryitem.StatusEQ(int32(*req.Status)))
	}
	rows, err := query.Order(gen.Asc(dictionaryitem.FieldSort), gen.Asc(dictionaryitem.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*pb.DictionaryItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, dictionaryItemProto(row))
	}
	return result, nil
}
func (r *dictionaryRepo) CreateItem(ctx context.Context, value *pb.DictionaryItem) (*pb.DictionaryItem, error) {
	if _, err := r.GetType(ctx, value.GetTypeId()); err != nil {
		return nil, err
	}
	row, err := r.Data.DB(ctx).DictionaryItem.Create().SetTypeID(value.GetTypeId()).SetLabel(value.GetLabel()).SetValue(value.GetValue()).SetStatus(int32(value.GetStatus())).SetSort(value.GetSort()).SetColor(value.GetColor()).SetRemark(value.GetRemark()).Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("DICTIONARY_VALUE_EXISTS", "字典值已存在")
	}
	if err != nil {
		return nil, err
	}
	return dictionaryItemProto(row), nil
}
func (r *dictionaryRepo) UpdateItem(ctx context.Context, value *pb.DictionaryItem) (*pb.DictionaryItem, error) {
	if _, err := r.GetType(ctx, value.GetTypeId()); err != nil {
		return nil, err
	}
	row, err := r.Data.DB(ctx).DictionaryItem.UpdateOneID(value.GetId()).SetTypeID(value.GetTypeId()).SetLabel(value.GetLabel()).SetValue(value.GetValue()).SetStatus(int32(value.GetStatus())).SetSort(value.GetSort()).SetColor(value.GetColor()).SetRemark(value.GetRemark()).Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("DICTIONARY_VALUE_EXISTS", "字典值已存在")
	}
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("DICTIONARY_ITEM_NOT_FOUND", "字典项不存在")
	}
	if err != nil {
		return nil, err
	}
	return dictionaryItemProto(row), nil
}
func (r *dictionaryRepo) DeleteItem(ctx context.Context, id uint32) error {
	err := r.Data.DB(ctx).DictionaryItem.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if gen.IsNotFound(err) {
		return errors.NotFound("DICTIONARY_ITEM_NOT_FOUND", "字典项不存在")
	}
	return err
}
