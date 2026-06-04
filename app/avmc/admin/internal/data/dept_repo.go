package data

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/avmc/admin/v1"
	"backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/dept"
	"backend-service/app/avmc/admin/internal/data/ent/gen/user"
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
	status := enum.Status(0)
	if res.Status != nil {
		status = enum.Status(*res.Status)
	}
	return &pbCore.Dept{
		Id:        res.ID,
		Name:      res.Name,
		ParentId:  res.ParentID,
		Ancestors: convert.ToAny(res.Ancestors, func(i int) uint32 { return uint32(i) }),
		LeaderId:  res.LeaderID,
		Sort:      res.Sort,
		Status:    convert.EmptyToNil(status),
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

func (r *deptRepo) resolveAncestors(ctx context.Context, domainID, deptID, parentID uint32) ([]int, error) {
	if parentID == 0 {
		return []int{}, nil
	}
	if deptID != 0 && parentID == deptID {
		return nil, pb.ErrorBadRequest("部门不能以自身作为上级部门")
	}
	parent, err := r.Data.DB(ctx).Dept.Query().
		Where(dept.IDEQ(parentID), dept.DomainIDEQ(domainID)).
		Select(dept.FieldID, dept.FieldAncestors).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorBadRequest("上级部门不存在或不属于当前域")
		}
		return nil, err
	}
	for _, ancestorID := range parent.Ancestors {
		if deptID != 0 && uint32(ancestorID) == deptID {
			return nil, pb.ErrorBadRequest("部门层级不能形成循环")
		}
	}
	ancestors := append([]int(nil), parent.Ancestors...)
	return append(ancestors, int(parent.ID)), nil
}

func (r *deptRepo) validateLeader(ctx context.Context, domainID, leaderID uint32) error {
	if leaderID == 0 {
		return nil
	}
	exists, err := r.Data.DB(ctx).User.Query().
		Where(user.IDEQ(leaderID), user.DomainIDEQ(domainID)).
		Exist(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return pb.ErrorBadRequest("部门负责人不存在或不属于当前域")
	}
	return nil
}

func (r *deptRepo) Save(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	if g == nil || g.GetName() == "" {
		return nil, pb.ErrorDeptNameCannotBeEmpty("部门名称不能为空")
	}
	r.Log.Infof("保存部门: %s", g.GetName())
	entDept := r.convertEnt(g)
	domainID, err := requireDomainID(ctx)
	if err != nil {
		return nil, err
	}
	ancestors, err := r.resolveAncestors(ctx, domainID, 0, g.GetParentId())
	if err != nil {
		return nil, err
	}
	if err := r.validateLeader(ctx, domainID, g.GetLeaderId()); err != nil {
		return nil, err
	}
	entDept.Ancestors = ancestors

	id, err := r.GetDeptExistByName(ctx, *entDept.Name)
	if err != nil {
		return nil, fmt.Errorf("checking dept name uniqueness: %w", err)
	}
	if id > 0 {
		return nil, pb.ErrorDeptNameAlreadyExists("部门名称已存在")
	}

	res, err := r.Data.DB(ctx).Dept.Create().
		SetDomainID(domainID).
		SetName(*entDept.Name).
		SetNillableParentID(entDept.ParentID).
		SetNillableLeaderID(entDept.LeaderID).
		SetNillableSort(entDept.Sort).
		SetNillableRemark(entDept.Remark).
		SetNillableStatus(entDept.Status).
		SetAncestors(entDept.Ancestors).
		Save(ctx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorDeptNameAlreadyExists("部门名称已存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *deptRepo) GetDeptExistByName(ctx context.Context, name string) (uint32, error) {
	domainID, err := requireDomainID(ctx)
	if err != nil {
		return 0, err
	}
	entDept, err := r.Data.DB(ctx).Dept.Query().Where(dept.Name(name), dept.DomainIDEQ(domainID)).Select(dept.FieldID).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return entDept.ID, nil
}

func (r *deptRepo) Update(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	if g == nil || g.GetId() == 0 || g.GetName() == "" {
		return nil, pb.ErrorDeptInvalidId("部门ID和名称不能为空")
	}
	entDept := r.convertEnt(g)
	domainID, err := requireDomainID(ctx)
	if err != nil {
		return nil, err
	}
	ancestors, err := r.resolveAncestors(ctx, domainID, g.GetId(), g.GetParentId())
	if err != nil {
		return nil, err
	}
	if err := r.validateLeader(ctx, domainID, g.GetLeaderId()); err != nil {
		return nil, err
	}
	entDept.Ancestors = ancestors
	id, err := r.GetDeptExistByName(ctx, *entDept.Name)
	if err != nil {
		return nil, fmt.Errorf("checking dept name uniqueness: %w", err)
	}
	if id > 0 && id != g.GetId() {
		return nil, pb.ErrorDeptNameAlreadyExists("部门名称已存在")
	}

	res, err := r.Data.DB(ctx).Dept.UpdateOneID(g.GetId()).
		Where(dept.DomainIDEQ(domainID)).
		SetName(*entDept.Name).
		SetNillableParentID(entDept.ParentID).
		SetNillableLeaderID(entDept.LeaderID).
		SetNillableSort(entDept.Sort).
		SetNillableRemark(entDept.Remark).
		SetNillableStatus(entDept.Status).
		SetAncestors(entDept.Ancestors).
		Save(ctx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorDeptNameAlreadyExists("部门名称已存在")
		}
		if gen.IsNotFound(err) {
			return nil, pb.ErrorDeptNotFound("部门不存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *deptRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Dept, error) {
	domainID, err := requireDomainID(ctx)
	if err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).Dept.Query().
		Where(dept.IDEQ(id), dept.DomainIDEQ(domainID)).Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorDeptNotFound("部门不存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *deptRepo) Delete(ctx context.Context, id uint32) error {
	domainID, err := requireDomainID(ctx)
	if err != nil {
		return err
	}
	hasChildren, err := r.Data.DB(ctx).Dept.Query().
		Where(dept.ParentIDEQ(id), dept.DomainIDEQ(domainID)).
		Exist(ctx)
	if err != nil {
		return err
	}
	if hasChildren {
		return pb.ErrorDeptCannotDeleteWithChildren("存在下级部门，无法删除")
	}
	err = r.Data.DB(ctx).Dept.UpdateOneID(id).Where(dept.DomainIDEQ(domainID)).SetDeletedAt(time.Now()).Exec(ctx)
	if gen.IsNotFound(err) {
		return pb.ErrorDeptNotFound("部门不存在")
	}
	return err
}

func (r *deptRepo) ListByName(ctx context.Context, name string) ([]*pbCore.Dept, error) {
	domainID, err := requireDomainID(ctx)
	if err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).Dept.Query().Where(dept.NameContains(name), dept.DomainIDEQ(domainID)).All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

func (r *deptRepo) CountDepts(ctx context.Context, opts ...biz.ListOption) (int32, error) {
	domainID, err := requireDomainID(ctx)
	if err != nil {
		return 0, err
	}
	o := biz.ListOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.Data.DB(ctx).Dept.Query().
		Select(dept.FieldID).
		Where(dept.DomainIDEQ(domainID), ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *deptRepo) ListAll(ctx context.Context) ([]*pbCore.Dept, error) {
	domainID, err := requireDomainID(ctx)
	if err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).Dept.Query().
		Select(dept.FieldID, dept.FieldName, dept.FieldParentID, dept.FieldStatus, dept.FieldRemark, dept.FieldLeaderID, dept.FieldSort, dept.FieldAncestors).
		Where(dept.DomainIDEQ(domainID)).
		Order(gen.Desc(dept.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

func (r *deptRepo) ListDepts(ctx context.Context, opts ...biz.ListOption) ([]*pbCore.Dept, error) {
	domainID, err := requireDomainID(ctx)
	if err != nil {
		return nil, err
	}
	o := biz.ListOptions{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	pos, err := r.Data.DB(ctx).Dept.Query().
		Select(dept.FieldID, dept.FieldName, dept.FieldParentID, dept.FieldStatus, dept.FieldRemark, dept.FieldLeaderID, dept.FieldSort, dept.FieldAncestors, dept.FieldCreatedAt, dept.FieldUpdatedAt).
		Where(dept.DomainIDEQ(domainID), ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(pos, r.convertProto), nil
}
