package data

import (
	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"context"
	"time"

	"github.com/go-kratos/aip-go/ents"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/dept"
	"backend-service/app/platform/admin/internal/data/ent/gen/menu"
	"backend-service/app/platform/admin/internal/data/ent/gen/role"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
)

var _ biz.RoleRepo = (*roleRepo)(nil)

type roleRepo struct {
	BaseRepo
	mpr *tenantMenuPermissionGroupRepo
}

// NewRoleRepo 创建角色仓库
func NewRoleRepo(data *Data, logger log.Logger) biz.RoleRepo {
	return &roleRepo{
		BaseRepo: NewBaseRepo(data, logger),
		mpr:      NewTenantMenuPermissionGroupRepo(data, logger).(*tenantMenuPermissionGroupRepo),
	}
}

func (r *roleRepo) validateMenuIDs(ctx context.Context, menuIDs []uint32) error {
	ids := make([]uint32, 0, len(menuIDs))
	seen := make(map[uint32]struct{}, len(menuIDs))
	for _, id := range menuIDs {
		if id == 0 {
			return pb.ErrorRolePermissionInvalid("菜单ID不能为空")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	count, err := r.Data.DB(ctx).Menu.Query().Where(menu.IDIn(ids...)).Count(ctx)
	if err != nil {
		return err
	}
	if count != len(ids) {
		return pb.ErrorRolePermissionInvalid("存在无效菜单ID")
	}
	if r.mpr != nil {
		if err := r.mpr.ValidateTenantMenuIDs(ctx, ids); err != nil {
			return err
		}
	}
	return nil
}

func (r *roleRepo) validateDeptIDs(ctx context.Context, deptIDs []uint32) error {
	ids := uniqueUint32(deptIDs)
	if len(ids) == 0 {
		return nil
	}
	count, err := r.Data.DB(ctx).Dept.Query().Where(dept.IDIn(ids...)).Count(ctx)
	if err != nil {
		return err
	}
	if count != len(ids) {
		return pb.ErrorBadRequest("存在无效或不属于当前租户的部门ID")
	}
	return nil
}

func validateDataScope(scope int32) error {
	if scope < 1 || scope > 5 {
		return pb.ErrorBadRequest("数据范围必须是 1 到 5")
	}
	return nil
}

// entToProto 将 ent.Role 转换为 pbCore.Role
func (r *roleRepo) entToProto(e *gen.Role) *pbCore.Role {
	if e == nil {
		return nil
	}
	status := pbEnum.Status(0)
	if e.Status != nil {
		status = pbEnum.Status(*e.Status)
	}
	return &pbCore.Role{
		Id:                e.ID,
		Name:              e.Name,
		DefaultRouter:     e.DefaultRouter,
		DataScope:         e.DataScope,
		Status:            &status,
		MenuCheckStrictly: e.MenuCheckStrictly,
		DeptCheckStrictly: e.DeptCheckStrictly,
		MenuIds:           roleMenuIDs(e),
		DeptIds:           roleDeptIDs(e),
		IsTenantAdmin:     e.IsTenantAdmin,
		CreatedAt:         convert.TimeValueToString(&e.CreatedAt, time.DateTime),
		UpdatedAt:         convert.TimeValueToString(&e.UpdatedAt, time.DateTime),
	}
}

// protoToEnt 将 pbCore.Role 转换为 ent.Role
func (r *roleRepo) protoToEnt(g *pbCore.Role) *gen.Role {
	return &gen.Role{
		ID:                g.GetId(),
		Name:              g.Name,
		DefaultRouter:     g.DefaultRouter,
		DataScope:         g.DataScope,
		Status:            (*int32)(g.Status),
		MenuCheckStrictly: g.MenuCheckStrictly,
		DeptCheckStrictly: g.DeptCheckStrictly,
		Edges: gen.RoleEdges{
			Menus:          roleMenus(g.GetMenuIds()),
			DataScopeDepts: roleDepts(g.GetDeptIds()),
		},
		IsTenantAdmin: g.GetIsTenantAdmin(),
	}
}

// Save 保存角色
func (r *roleRepo) Save(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	if g == nil || g.GetName() == "" {
		return nil, pb.ErrorRoleNameCannotBeEmpty("角色名称不能为空")
	}
	r.Log.Infof("保存角色: %s", g.GetName())
	ent := r.protoToEnt(g)
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if g.DataScope != nil {
		if err := validateDataScope(g.GetDataScope()); err != nil {
			return nil, err
		}
	}
	if err := r.validateMenuIDs(ctx, g.GetMenuIds()); err != nil {
		return nil, err
	}
	if err := r.validateDeptIDs(ctx, g.GetDeptIds()); err != nil {
		return nil, err
	}

	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)

	builder := tx.Role.Create().
		SetName(g.GetName()).
		SetNillableDefaultRouter(ent.DefaultRouter).
		SetNillableDataScope(ent.DataScope).
		SetNillableMenuCheckStrictly(ent.MenuCheckStrictly).
		SetNillableDeptCheckStrictly(ent.DeptCheckStrictly).
		SetNillableStatus(ent.Status)
	if len(g.GetMenuIds()) > 0 {
		builder.AddMenuIDs(g.GetMenuIds()...)
	}
	if len(g.GetDeptIds()) > 0 {
		builder.AddDataScopeDeptIDs(uniqueUint32(g.GetDeptIds())...)
	}
	res, err := builder.Save(ctx)
	if err != nil {
		r.Log.Errorf("保存角色失败: %v", err)
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorRoleAlreadyExists("角色名称已存在")
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	r.bumpTenantAuthorizationVersion(ctx, tenantID)
	result := r.entToProto(res)
	SyncRolePolicies(ctx, r.Data.DB(ctx), r.Data.authorizer, tenantID, res.ID)
	return result, nil
}

// Update 更新角色
func (r *roleRepo) Update(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	if g == nil || g.GetId() == 0 || g.GetName() == "" {
		return nil, pb.ErrorRoleInvalidId("角色ID和名称不能为空")
	}
	r.Log.Infof("更新角色 ID: %d", g.GetId())
	ent := r.protoToEnt(g)
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if g.DataScope != nil {
		if err := validateDataScope(g.GetDataScope()); err != nil {
			return nil, err
		}
	}
	if g.MenuIds != nil {
		if err := r.validateMenuIDs(ctx, g.MenuIds); err != nil {
			return nil, err
		}
	}
	if g.DeptIds != nil {
		if err := r.validateDeptIDs(ctx, g.DeptIds); err != nil {
			return nil, err
		}
	}

	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)

	current, err := findRoleForUpdate(ctx, tx.Client(), g.GetId())
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorRoleNotFound("角色不存在")
		}
		return nil, err
	}
	if current.IsTenantAdmin {
		return nil, pb.ErrorBadRequest("租户管理员角色由系统维护，不能编辑或停用")
	}

	builder := tx.Role.UpdateOneID(g.GetId()).
		SetName(g.GetName()).
		SetNillableDefaultRouter(ent.DefaultRouter).
		SetNillableDataScope(ent.DataScope).
		SetNillableMenuCheckStrictly(ent.MenuCheckStrictly).
		SetNillableDeptCheckStrictly(ent.DeptCheckStrictly).
		SetNillableStatus(ent.Status)
	if g.MenuIds != nil {
		builder.ClearMenus()
		if len(g.MenuIds) > 0 {
			builder.AddMenuIDs(g.MenuIds...)
		}
	}
	if g.DeptIds != nil {
		builder.ClearDataScopeDepts()
		if len(g.DeptIds) > 0 {
			builder.AddDataScopeDeptIDs(uniqueUint32(g.DeptIds)...)
		}
	}
	res, err := builder.Save(ctx)
	if err != nil {
		r.Log.Errorf("更新角色失败: %v", err)
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorRoleAlreadyExists("角色名称已存在")
		}
		if gen.IsNotFound(err) {
			return nil, pb.ErrorRoleNotFound("角色不存在")
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	r.bumpTenantAuthorizationVersion(ctx, tenantID)
	result := r.entToProto(res)
	SyncRolePolicies(ctx, r.Data.DB(ctx), r.Data.authorizer, tenantID, res.ID)
	return result, nil
}

// FindByID 根据 ID 查询
func (r *roleRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Role, error) {
	r.Log.Infof("查询角色 ID: %d", id)
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).Role.Query().
		Select(
			role.FieldID, role.FieldName,
			role.FieldDefaultRouter, role.FieldDataScope,
			role.FieldMenuCheckStrictly, role.FieldDeptCheckStrictly,
			role.FieldStatus, role.FieldIsTenantAdmin,
			role.FieldCreatedAt, role.FieldUpdatedAt,
		).
		WithMenus(func(q *gen.MenuQuery) {
			q.Select(menu.FieldID)
		}).
		WithDataScopeDepts(func(q *gen.DeptQuery) {
			q.Select(dept.FieldID)
		}).
		Where(role.ID(id)).
		First(ctx)
	if err != nil {
		r.Log.Errorf("查询角色失败 ID: %d, err: %v", id, err)
		if gen.IsNotFound(err) {
			return nil, pb.ErrorRoleNotFound("角色不存在")
		}
		return nil, err
	}
	return r.entToProto(res), nil
}

// Delete 软删除
func (r *roleRepo) Delete(ctx context.Context, id uint32) error {
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return err
	}
	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx, r.Log)
	current, err := findRoleForUpdate(ctx, tx.Client(), id)
	if err != nil {
		if gen.IsNotFound(err) {
			return pb.ErrorRoleNotFound("角色不存在")
		}
		return err
	}
	if current.IsTenantAdmin {
		return pb.ErrorBadRequest("租户管理员角色由系统维护，不能删除")
	}
	inUse, err := current.QueryUsers().Exist(ctx)
	if err != nil {
		return err
	}
	if inUse {
		return kerrors.Conflict("ROLE_IN_USE", "角色仍关联用户，无法删除")
	}
	err = tx.Role.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if gen.IsNotFound(err) {
		return pb.ErrorRoleNotFound("角色不存在")
	}
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	r.bumpTenantAuthorizationVersion(ctx, tenantID)
	RemoveRolePolicies(ctx, r.Data.authorizer, tenantID, id)
	return nil
}

func findRoleForUpdate(ctx context.Context, client *gen.Client, id uint32) (*gen.Role, error) {
	item, err := client.Role.Query().Where(role.IDEQ(id)).ForUpdate().Only(ctx)
	if isSelectForUpdateUnsupported(err) {
		return client.Role.Query().Where(role.IDEQ(id)).Only(ctx)
	}
	return item, err
}

// ListAll 查询所有角色
func (r *roleRepo) ListAll(ctx context.Context) ([]*pbCore.Role, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).Role.Query().
		Select(role.FieldID, role.FieldName, role.FieldStatus, role.FieldIsTenantAdmin).
		WithMenus(func(q *gen.MenuQuery) {
			q.Select(menu.FieldID)
		}).
		WithDataScopeDepts(func(q *gen.DeptQuery) {
			q.Select(dept.FieldID)
		}).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.entToProto), nil
}

// CountRoles 角色计数
func (r *roleRepo) CountRoles(ctx context.Context, opts ...listing.Option) (int32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	o := listing.Options{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.Data.DB(ctx).Role.Query().
		Select(role.FieldID).
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

// ListRoles 分页查询角色
func (r *roleRepo) ListRoles(ctx context.Context, opts ...listing.Option) ([]*pbCore.Role, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	res, err := r.Data.DB(ctx).Role.Query().
		Select(
			role.FieldID, role.FieldName,
			role.FieldDefaultRouter, role.FieldDataScope,
			role.FieldMenuCheckStrictly, role.FieldDeptCheckStrictly,
			role.FieldStatus, role.FieldIsTenantAdmin,
			role.FieldCreatedAt, role.FieldUpdatedAt,
		).
		WithMenus(func(q *gen.MenuQuery) {
			q.Select(menu.FieldID)
		}).
		WithDataScopeDepts(func(q *gen.DeptQuery) {
			q.Select(dept.FieldID)
		}).
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.entToProto), nil
}

// ExistByName 判断角色名是否存在
func (r *roleRepo) ExistByName(ctx context.Context, name string, excludeID uint32) (bool, error) {
	if name == "" {
		return false, nil
	}
	if _, err := r.RequireTenantID(ctx); err != nil {
		return false, err
	}
	builder := r.Data.DB(ctx).Role.Query()
	if excludeID != 0 {
		builder = builder.Where(role.IDNotIn(excludeID))
	}
	_, err := builder.Select(role.FieldID).Where(role.Name(name)).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func roleMenus(ids []uint32) []*gen.Menu {
	menus := make([]*gen.Menu, 0, len(ids))
	for _, id := range ids {
		menus = append(menus, &gen.Menu{ID: id})
	}
	return menus
}

func roleMenuIDs(e *gen.Role) []uint32 {
	if e == nil || len(e.Edges.Menus) == 0 {
		return nil
	}
	ids := make([]uint32, 0, len(e.Edges.Menus))
	for _, m := range e.Edges.Menus {
		if m != nil {
			ids = append(ids, m.ID)
		}
	}
	return ids
}

func roleDepts(ids []uint32) []*gen.Dept {
	items := make([]*gen.Dept, 0, len(ids))
	for _, id := range ids {
		items = append(items, &gen.Dept{ID: id})
	}
	return items
}

func roleDeptIDs(e *gen.Role) []uint32 {
	if e == nil || len(e.Edges.DataScopeDepts) == 0 {
		return nil
	}
	ids := make([]uint32, 0, len(e.Edges.DataScopeDepts))
	for _, item := range e.Edges.DataScopeDepts {
		if item != nil {
			ids = append(ids, item.ID)
		}
	}
	return ids
}
