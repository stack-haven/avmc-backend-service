package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/dept"
	"backend-service/pkg/utils/convert"
)

var _ biz.DeptRepo = (*deptRepo)(nil)

type deptRepo struct {
	BaseRepo
}

func NewDeptRepo(data *Data, logger log.Logger) biz.DeptRepo {
	return &deptRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func (r *deptRepo) convertProto(res *gen.Dept) *pbCore.Dept {
	return &pbCore.Dept{
		Id:        res.ID,
		Name:      res.Name,
		ParentId:  res.ParentID,
		Ancestors: convert.ToAny(res.Ancestors, func(i int) uint32 { return uint32(i) }),
		LeaderId:  res.LeaderID,
		Sort:      res.Sort,
		Status:    convert.EmptyToNil(enum.Status(*res.Status)),
		Remark:    res.Remark,
		CreatedAt: convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt: convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

func (r *deptRepo) convertEnt(g *pbCore.Dept) *gen.Dept {
	return &gen.Dept{
		ID:        g.GetId(),
		Name:      g.Name,
		ParentID:  g.ParentId,
		Ancestors: convert.ToAny(g.Ancestors, func(i uint32) int { return int(i) }),
		LeaderID:  g.LeaderId,
		Sort:      g.Sort,
		Status:    convert.ToPointer(int32(g.GetStatus())),
		Remark:    g.Remark,
	}
}

func (r *deptRepo) Save(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	r.Log.Infof("保存部门: %s", g.GetName())
	entDept := r.convertEnt(g)

	if id, _ := r.GetDeptExistByName(ctx, *entDept.Name); id > 0 {
		return nil, fmt.Errorf("dept name already exists")
	}

	res, err := r.Data.DB(ctx).Dept.Create().
		SetName(*entDept.Name).
		SetNillableParentID(entDept.ParentID).
		SetNillableLeaderID(entDept.LeaderID).
		SetNillableSort(entDept.Sort).
		SetNillableRemark(entDept.Remark).
		SetNillableStatus(entDept.Status).
		SetAncestors(entDept.Ancestors).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *deptRepo) GetDeptExistByName(ctx context.Context, name string) (uint32, error) {
	entDept, err := r.Data.DB(ctx).Dept.Query().Where(dept.Name(name)).Select(dept.FieldID).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return entDept.ID, nil
}

func (r *deptRepo) Update(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	entDept := r.convertEnt(g)
	id, _ := r.GetDeptExistByName(ctx, *entDept.Name)
	if id > 0 && id != g.GetId() {
		return nil, fmt.Errorf("dept name already exists")
	}

	res, err := r.Data.DB(ctx).Dept.UpdateOneID(g.GetId()).
		SetName(*entDept.Name).
		SetNillableParentID(entDept.ParentID).
		SetNillableLeaderID(entDept.LeaderID).
		SetNillableSort(entDept.Sort).
		SetNillableRemark(entDept.Remark).
		SetNillableStatus(entDept.Status).
		SetAncestors(entDept.Ancestors).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *deptRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Dept, error) {
	res, err := r.Data.DB(ctx).Dept.Query().
		Where(dept.IDEQ(id)).Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, errors.New("查询数据不存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *deptRepo) Delete(ctx context.Context, id uint32) error {
	return r.Data.DB(ctx).Dept.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
}

func (r *deptRepo) ListByName(ctx context.Context, name string) ([]*pbCore.Dept, error) {
	res, err := r.Data.DB(ctx).Dept.Query().Where(dept.NameContains(name)).All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

func (r *deptRepo) CountDepts(ctx context.Context, opts ...biz.ListOption) (int32, error) {
	o := biz.ListOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.Data.DB(ctx).Dept.Query().
		Select(dept.FieldID).
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *deptRepo) ListAll(ctx context.Context) ([]*pbCore.Dept, error) {
	res, err := r.Data.DB(ctx).Dept.Query().
		Select(dept.FieldID, dept.FieldName, dept.FieldParentID, dept.FieldStatus, dept.FieldRemark, dept.FieldLeaderID, dept.FieldSort).
		Order(gen.Desc(dept.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

func (r *deptRepo) ListDepts(ctx context.Context, opts ...biz.ListOption) ([]*pbCore.Dept, error) {
	o := biz.ListOptions{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	pos, err := r.Data.DB(ctx).Dept.Query().
		Select(dept.FieldID, dept.FieldName, dept.FieldParentID, dept.FieldStatus, dept.FieldRemark, dept.FieldLeaderID, dept.FieldSort, dept.FieldCreatedAt, dept.FieldUpdatedAt).
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(pos, r.convertProto), nil
}
