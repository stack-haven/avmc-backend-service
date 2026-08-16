package data

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/aip-go/ents"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/dept"
	"backend-service/app/platform/service/internal/data/ent/gen/user"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
)

var _ biz.DeptRepo = (*deptRepo)(nil)

type deptRepo struct {
	BaseRepo
}

func NewDeptRepo(data *Data, logger log.Logger) biz.DeptRepo {
	return &deptRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func (r *deptRepo) convertProto(res *gen.Dept) *pb.Dept {
	status := enum.Status(0)
	if res.Status != nil {
		status = enum.Status(*res.Status)
	}
	return &pb.Dept{
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

func (r *deptRepo) convertEnt(g *pb.Dept) *gen.Dept {
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

func (r *deptRepo) resolveAncestors(ctx context.Context, deptID, parentID uint32) ([]int, error) {
	if parentID == 0 {
		return []int{}, nil
	}
	if deptID != 0 && parentID == deptID {
		return nil, pb.ErrorBadRequest("部门不能以自身作为上级部门")
	}
	parent, err := r.Data.DB(ctx).Dept.Query().
		Where(dept.IDEQ(parentID)).
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

func (r *deptRepo) validateLeader(ctx context.Context, leaderID uint32) error {
	if leaderID == 0 {
		return nil
	}
	exists, err := r.Data.DB(ctx).User.Query().
		Where(user.IDEQ(leaderID)).
		Exist(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return pb.ErrorBadRequest("部门负责人不存在或不属于当前域")
	}
	return nil
}

func (r *deptRepo) Save(ctx context.Context, g *pb.Dept) (*pb.Dept, error) {
	if g == nil || g.GetName() == "" {
		return nil, pb.ErrorDeptNameCannotBeEmpty("部门名称不能为空")
	}
	r.Log.Infof("保存部门: %s", g.GetName())
	entDept := r.convertEnt(g)
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	ancestors, err := r.resolveAncestors(ctx, 0, g.GetParentId())
	if err != nil {
		return nil, err
	}
	if err := r.validateLeader(ctx, g.GetLeaderId()); err != nil {
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
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	entDept, err := r.Data.DB(ctx).Dept.Query().Where(dept.Name(name)).Select(dept.FieldID).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return entDept.ID, nil
}

func (r *deptRepo) Update(ctx context.Context, g *pb.Dept) (*pb.Dept, error) {
	if g == nil || g.GetId() == 0 || g.GetName() == "" {
		return nil, pb.ErrorDeptInvalidId("部门ID和名称不能为空")
	}
	entDept := r.convertEnt(g)
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	ancestors, err := r.resolveAncestors(ctx, g.GetId(), g.GetParentId())
	if err != nil {
		return nil, err
	}
	if err := r.validateLeader(ctx, g.GetLeaderId()); err != nil {
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

	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)
	res, err := tx.Dept.UpdateOneID(g.GetId()).
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
	descendants, err := tx.Dept.Query().
		Select(dept.FieldID, dept.FieldAncestors).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, child := range descendants {
		index := -1
		for i, ancestorID := range child.Ancestors {
			if uint32(ancestorID) == g.GetId() {
				index = i
				break
			}
		}
		if index < 0 {
			continue
		}
		updatedAncestors := append([]int(nil), entDept.Ancestors...)
		updatedAncestors = append(updatedAncestors, int(g.GetId()))
		updatedAncestors = append(updatedAncestors, child.Ancestors[index+1:]...)
		if err = tx.Dept.UpdateOneID(child.ID).SetAncestors(updatedAncestors).Exec(ctx); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *deptRepo) FindByID(ctx context.Context, id uint32) (*pb.Dept, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).Dept.Query().
		Where(dept.IDEQ(id)).Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorDeptNotFound("部门不存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *deptRepo) Delete(ctx context.Context, id uint32) error {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return err
	}
	item, err := r.Data.DB(ctx).Dept.Query().Where(dept.IDEQ(id)).Select(dept.FieldID, dept.FieldParentID).Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return pb.ErrorDeptNotFound("部门不存在")
		}
		return err
	}
	if item.ParentID == nil || *item.ParentID == 0 {
		return kerrors.Conflict("PROTECTED_ROOT_DEPT", "组织根部门不可删除")
	}
	hasChildren, err := r.Data.DB(ctx).Dept.Query().
		Where(dept.ParentIDEQ(id)).
		Exist(ctx)
	if err != nil {
		return err
	}
	if hasChildren {
		return pb.ErrorDeptCannotDeleteWithChildren("存在下级部门，无法删除")
	}
	inUse, err := r.Data.DB(ctx).Dept.Query().
		Where(
			dept.IDEQ(id),
			dept.Or(dept.HasUsers(), dept.HasDataScopeRoles()),
		).
		Exist(ctx)
	if err != nil {
		return err
	}
	if inUse {
		return kerrors.Conflict("DEPT_IN_USE", "部门仍关联用户或角色数据范围，无法删除")
	}
	err = r.Data.DB(ctx).Dept.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if gen.IsNotFound(err) {
		return pb.ErrorDeptNotFound("部门不存在")
	}
	return err
}

func (r *deptRepo) GetDeleteImpact(ctx context.Context, id uint32) (*pb.GetDeptDeleteImpactResponse, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	item, err := r.Data.DB(ctx).Dept.Query().
		Where(dept.IDEQ(id)).
		WithUsers().
		WithChildren().
		WithDataScopeRoles().
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorDeptNotFound("部门不存在")
		}
		return nil, err
	}
	userCount := uint32(len(item.Edges.Users))
	hasChildren := len(item.Edges.Children) > 0
	isProtectedRoot := item.ParentID == nil || *item.ParentID == 0
	hasDataScopeRoles := len(item.Edges.DataScopeRoles) > 0
	name := ""
	if item.Name != nil {
		name = *item.Name
	}
	return &pb.GetDeptDeleteImpactResponse{
		Id:                   item.ID,
		Name:                 name,
		DirectUserCount:      userCount,
		HasChildren:          hasChildren,
		IsProtectedRoot:      isProtectedRoot,
		HasDataScopeRoles:    hasDataScopeRoles,
		CanDeleteDirectly:    userCount == 0 && !hasChildren && !isProtectedRoot && !hasDataScopeRoles,
		RequiresUserTransfer: userCount > 0 && !hasChildren && !isProtectedRoot && !hasDataScopeRoles,
	}, nil
}

func (r *deptRepo) TransferAndDelete(ctx context.Context, id, targetDeptID uint32) (uint32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	if id == targetDeptID {
		return 0, pb.ErrorBadRequest("接收部门不能是待删除部门")
	}
	var transferred uint32
	err := r.Data.InTx(ctx, func(txCtx context.Context) error {
		client := r.Data.DB(txCtx)
		source, err := client.Dept.Query().Where(dept.IDEQ(id)).WithChildren().WithDataScopeRoles().Only(txCtx)
		if err != nil {
			if gen.IsNotFound(err) {
				return pb.ErrorDeptNotFound("待删除部门不存在")
			}
			return err
		}
		if source.ParentID == nil || *source.ParentID == 0 {
			return kerrors.Conflict("PROTECTED_ROOT_DEPT", "组织根部门不可删除")
		}
		if len(source.Edges.Children) > 0 {
			return pb.ErrorDeptCannotDeleteWithChildren("存在下级部门，请先处理下级部门")
		}
		if len(source.Edges.DataScopeRoles) > 0 {
			return kerrors.Conflict("DEPT_DATA_SCOPE_IN_USE", "部门仍被角色数据范围引用，请先调整角色")
		}
		if _, err = client.Dept.Query().Where(dept.IDEQ(targetDeptID)).Only(txCtx); err != nil {
			if gen.IsNotFound(err) {
				return pb.ErrorBadRequest("接收部门不存在或不属于当前租户")
			}
			return err
		}
		count, err := client.User.Query().Where(user.DeptIDEQ(id)).Count(txCtx)
		if err != nil {
			return err
		}
		if _, err = client.User.Update().Where(user.DeptIDEQ(id)).SetDeptID(targetDeptID).Save(txCtx); err != nil {
			return err
		}
		if err = client.Dept.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(txCtx); err != nil {
			return err
		}
		transferred = uint32(count)
		return nil
	})
	return transferred, err
}

func (r *deptRepo) ListByName(ctx context.Context, name string) ([]*pb.Dept, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).Dept.Query().Where(dept.NameContains(name)).All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

func (r *deptRepo) CountDepts(ctx context.Context, opts ...listing.Option) (int32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	o := listing.Options{}
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

func (r *deptRepo) ListAll(ctx context.Context) ([]*pb.Dept, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).Dept.Query().
		Select(dept.FieldID, dept.FieldName, dept.FieldParentID, dept.FieldStatus, dept.FieldRemark, dept.FieldLeaderID, dept.FieldSort, dept.FieldAncestors).
		Order(gen.Desc(dept.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	items := convert.SliceToAny(res, r.convertProto)
	users, err := r.Data.DB(ctx).User.Query().Select(user.FieldDeptID).All(ctx)
	if err != nil {
		return nil, err
	}
	direct := make(map[uint32]uint32)
	for _, item := range users {
		if item.DeptID != nil {
			direct[*item.DeptID]++
		}
	}
	byID := make(map[uint32]*pb.Dept, len(items))
	for _, item := range items {
		item.DirectUserCount = direct[item.GetId()]
		item.TotalUserCount = item.DirectUserCount
		byID[item.GetId()] = item
	}
	for _, item := range items {
		if item.DirectUserCount == 0 {
			continue
		}
		parentID := item.GetParentId()
		visited := map[uint32]struct{}{item.GetId(): {}}
		for parentID != 0 {
			if _, exists := visited[parentID]; exists {
				break
			}
			visited[parentID] = struct{}{}
			parent := byID[parentID]
			if parent == nil {
				break
			}
			parent.TotalUserCount += item.DirectUserCount
			parentID = parent.GetParentId()
		}
	}
	return items, nil
}

func (r *deptRepo) ListDepts(ctx context.Context, opts ...listing.Option) ([]*pb.Dept, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	pos, err := r.Data.DB(ctx).Dept.Query().
		Select(dept.FieldID, dept.FieldName, dept.FieldParentID, dept.FieldStatus, dept.FieldRemark, dept.FieldLeaderID, dept.FieldSort, dept.FieldAncestors, dept.FieldCreatedAt, dept.FieldUpdatedAt).
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(pos, r.convertProto), nil
}
