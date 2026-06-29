package data

import (
	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/menu"
	"backend-service/app/platform/admin/internal/data/ent/gen/menupermissiongroup"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenant"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenantpermissiongroup"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
	"context"
	"sort"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"
)

var _ biz.MenuPermissionGroupRepo = (*menuPermissionGroupRepo)(nil)

type menuPermissionGroupRepo struct {
	BaseRepo
	mr *menuRepo
}

func NewMenuPermissionGroupRepo(data *Data, logger log.Logger) biz.MenuPermissionGroupRepo {
	return &menuPermissionGroupRepo{
		BaseRepo: NewBaseRepo(data, logger),
		mr:       NewMenuRepo(data, logger).(*menuRepo),
	}
}

func (r *menuPermissionGroupRepo) validateMenuIDs(ctx context.Context, menuIDs []uint32) error {
	ids := uniquePositiveIDs(menuIDs)
	if len(ids) == 0 {
		return nil
	}
	count, err := r.Data.DB(ctx).Menu.Query().Where(menu.IDIn(ids...)).Count(ctx)
	if err != nil {
		return err
	}
	if count != len(ids) {
		return pb.ErrorBadRequest("存在无效菜单ID")
	}
	return nil
}

func (r *menuPermissionGroupRepo) entToProto(e *gen.MenuPermissionGroup) *pbCore.MenuPermissionGroup {
	if e == nil {
		return nil
	}
	status := pbEnum.Status(0)
	if e.Status != nil {
		status = pbEnum.Status(*e.Status)
	}
	return &pbCore.MenuPermissionGroup{
		Id:          e.ID,
		Name:        convert.ToPointer(e.Name),
		Code:        convert.ToPointer(e.Code),
		Status:      &status,
		IsSystem:    e.IsSystem,
		Sort:        e.Sort,
		Description: e.Description,
		Remark:      e.Remark,
		MenuIds:     menuPermissionGroupMenuIDs(e),
		TenantCount: convert.EmptyToNil(int32(len(e.Edges.TenantBindings))),
		CreatedAt:   convert.TimeValueToString(&e.CreatedAt, time.DateTime),
		UpdatedAt:   convert.TimeValueToString(&e.UpdatedAt, time.DateTime),
	}
}

func (r *menuPermissionGroupRepo) Save(ctx context.Context, g *pbCore.MenuPermissionGroup) (*pbCore.MenuPermissionGroup, error) {
	if g == nil || g.GetName() == "" || g.GetCode() == "" {
		return nil, pb.ErrorBadRequest("权限组名称和编码不能为空")
	}
	if err := r.validateMenuIDs(ctx, g.GetMenuIds()); err != nil {
		return nil, err
	}
	builder := r.Data.DB(ctx).MenuPermissionGroup.Create().
		SetName(g.GetName()).
		SetCode(g.GetCode()).
		SetNillableStatus((*int32)(g.Status)).
		SetNillableIsSystem(g.IsSystem).
		SetNillableSort(g.Sort).
		SetNillableDescription(g.Description).
		SetNillableRemark(g.Remark)
	if len(g.GetMenuIds()) > 0 {
		builder.AddMenuIDs(uniquePositiveIDs(g.GetMenuIds())...)
	}
	res, err := builder.Save(ctx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorBadRequest("权限组名称或编码已存在")
		}
		return nil, err
	}
	return r.FindByID(ctx, res.ID)
}

func (r *menuPermissionGroupRepo) Update(ctx context.Context, g *pbCore.MenuPermissionGroup) (*pbCore.MenuPermissionGroup, error) {
	if g == nil || g.GetId() == 0 || g.GetName() == "" || g.GetCode() == "" {
		return nil, pb.ErrorBadRequest("权限组ID、名称和编码不能为空")
	}
	if err := r.validateMenuIDs(ctx, g.GetMenuIds()); err != nil {
		return nil, err
	}
	builder := r.Data.DB(ctx).MenuPermissionGroup.UpdateOneID(g.GetId()).
		SetName(g.GetName()).
		SetCode(g.GetCode()).
		SetNillableStatus((*int32)(g.Status)).
		SetNillableIsSystem(g.IsSystem).
		SetNillableSort(g.Sort).
		SetNillableDescription(g.Description).
		SetNillableRemark(g.Remark).
		ClearMenus()
	if len(g.GetMenuIds()) > 0 {
		builder.AddMenuIDs(uniquePositiveIDs(g.GetMenuIds())...)
	}
	affectedTenantIDs, err := r.tenantIDsByGroup(ctx, g.GetId())
	if err != nil {
		return nil, err
	}
	res, err := builder.Save(ctx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorBadRequest("权限组名称或编码已存在")
		}
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("权限组不存在")
		}
		return nil, err
	}
	r.bumpTenantsPackageVersion(ctx, affectedTenantIDs)
	return r.FindByID(ctx, res.ID)
}

func (r *menuPermissionGroupRepo) FindByID(ctx context.Context, id uint32) (*pbCore.MenuPermissionGroup, error) {
	res, err := r.Data.DB(ctx).MenuPermissionGroup.Query().
		Where(menupermissiongroup.IDEQ(id)).
		WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
		WithTenantBindings().
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("权限组不存在")
		}
		return nil, err
	}
	return r.entToProto(res), nil
}

func (r *menuPermissionGroupRepo) CountMenuPermissionGroups(ctx context.Context, opts ...listing.Option) (int32, error) {
	o := listing.Options{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.Data.DB(ctx).MenuPermissionGroup.Query().
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *menuPermissionGroupRepo) ListMenuPermissionGroups(ctx context.Context, opts ...listing.Option) ([]*pbCore.MenuPermissionGroup, error) {
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	res, err := r.Data.DB(ctx).MenuPermissionGroup.Query().
		WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
		WithTenantBindings().
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return ConvertSlice(res, r.entToProto), nil
}

func (r *menuPermissionGroupRepo) Delete(ctx context.Context, id uint32) error {
	group, err := r.Data.DB(ctx).MenuPermissionGroup.Query().
		Where(menupermissiongroup.IDEQ(id)).
		WithTenantBindings().
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return pb.ErrorResourceNotFound("权限组不存在")
		}
		return err
	}
	if convert.ToValue(group.IsSystem) {
		return pb.ErrorBadRequest("系统内置权限组不能删除")
	}
	if len(group.Edges.TenantBindings) > 0 {
		return pb.ErrorBadRequest("权限组仍被租户绑定，不能删除")
	}
	return r.Data.DB(ctx).MenuPermissionGroup.DeleteOneID(id).Exec(ctx)
}

func (r *menuPermissionGroupRepo) UpdateStatus(ctx context.Context, id uint32, status pbEnum.Status) (*pbCore.MenuPermissionGroup, error) {
	affectedTenantIDs, err := r.tenantIDsByGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).MenuPermissionGroup.UpdateOneID(id).
		SetStatus(int32(status)).
		Save(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("权限组不存在")
		}
		return nil, err
	}
	r.bumpTenantsPackageVersion(ctx, affectedTenantIDs)
	return r.FindByID(ctx, res.ID)
}

func (r *menuPermissionGroupRepo) GetTenantGroups(ctx context.Context, tenantID uint32) ([]*pbCore.MenuPermissionGroup, error) {
	if tenantID == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	bindings, err := r.Data.DB(ctx).TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.TenantIDEQ(tenantID), tenantpermissiongroup.EnabledEQ(true)).
		WithGroup(func(q *gen.MenuPermissionGroupQuery) {
			q.WithMenus(func(mq *gen.MenuQuery) { mq.Select(menu.FieldID) })
			q.WithTenantBindings()
		}).
		All(ctx)
	if err != nil {
		return nil, err
	}
	groups := make([]*pbCore.MenuPermissionGroup, 0, len(bindings))
	for _, binding := range bindings {
		group, err := binding.Edges.GroupOrErr()
		if err != nil {
			continue
		}
		groups = append(groups, r.entToProto(group))
	}
	return groups, nil
}

func (r *menuPermissionGroupRepo) UpdateTenantGroups(ctx context.Context, tenantID uint32, groupIDs []uint32, operatorID uint32) error {
	if tenantID == 0 {
		return pb.ErrorBadRequest("租户ID不能为空")
	}
	exists, err := r.Data.DB(ctx).Tenant.Query().Where(tenant.IDEQ(tenantID)).Exist(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return pb.ErrorResourceNotFound("租户不存在")
	}
	ids := uniquePositiveIDs(groupIDs)
	if len(ids) > 0 {
		count, err := r.Data.DB(ctx).MenuPermissionGroup.Query().Where(menupermissiongroup.IDIn(ids...)).Count(ctx)
		if err != nil {
			return err
		}
		if count != len(ids) {
			return pb.ErrorBadRequest("存在无效权限组ID")
		}
	}
	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx, r.Log)
	if _, err := tx.TenantPermissionGroup.Delete().
		Where(tenantpermissiongroup.TenantIDEQ(tenantID)).
		Exec(ctx); err != nil {
		return err
	}
	for _, id := range ids {
		builder := tx.TenantPermissionGroup.Create().
			SetTenantID(tenantID).
			SetGroupID(id).
			SetEnabled(true)
		if operatorID > 0 {
			builder.SetBoundBy(operatorID)
		}
		if _, err := builder.Save(ctx); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	r.bumpTenantPackageVersion(ctx, tenantID)
	return nil
}

func (r *menuPermissionGroupRepo) GetTenantEffectiveMenuIDs(ctx context.Context, tenantID uint32) ([]uint32, error) {
	if tenantID == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	if ids, ok := r.getTenantEffectiveMenuIDsCache(ctx, tenantID); ok {
		return ids, nil
	}
	bindings, err := r.Data.DB(ctx).TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.TenantIDEQ(tenantID), tenantpermissiongroup.EnabledEQ(true)).
		WithGroup(func(q *gen.MenuPermissionGroupQuery) {
			q.Where(menupermissiongroup.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)))
			q.WithMenus(func(mq *gen.MenuQuery) {
				mq.Where(menu.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)))
				mq.Select(menu.FieldID)
			})
		}).
		All(ctx)
	if err != nil {
		return nil, err
	}
	idSet := make(map[uint32]struct{})
	for _, binding := range bindings {
		group, err := binding.Edges.GroupOrErr()
		if err != nil {
			continue
		}
		for _, m := range group.Edges.Menus {
			if m != nil {
				idSet[m.ID] = struct{}{}
			}
		}
	}
	ids := make([]uint32, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	r.setTenantEffectiveMenuIDsCache(ctx, tenantID, ids)
	return ids, nil
}

func (r *menuPermissionGroupRepo) tenantIDsByGroup(ctx context.Context, groupID uint32) ([]uint32, error) {
	if groupID == 0 {
		return nil, nil
	}
	bindings, err := r.Data.DB(ctx).TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.GroupIDEQ(groupID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uint32, 0, len(bindings))
	for _, binding := range bindings {
		if binding != nil && binding.TenantID > 0 {
			ids = append(ids, binding.TenantID)
		}
	}
	return uniquePositiveIDs(ids), nil
}

func (r *menuPermissionGroupRepo) bumpTenantsPackageVersion(ctx context.Context, tenantIDs []uint32) {
	for _, tenantID := range uniquePositiveIDs(tenantIDs) {
		r.bumpTenantPackageVersion(ctx, tenantID)
	}
}

func (r *menuPermissionGroupRepo) GetTenantEffectiveMenus(ctx context.Context, tenantID uint32, parentID uint32) ([]*pbCore.Menu, error) {
	ids, err := r.GetTenantEffectiveMenuIDs(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	menus, err := r.Data.DB(ctx).Menu.Query().
		Where(menu.IDIn(ids...), menu.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED))).
		Order(gen.Asc(menu.FieldSort, menu.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	menus, err = withMenuAncestors(ctx, r.Data.DB(ctx), menus)
	if err != nil {
		return nil, err
	}
	tree := buildMenuTree(convert.SliceToAny(menus, r.mr.convertProto))
	if parentID == 0 {
		return tree, nil
	}
	return filterMenuTreeByParent(tree, parentID), nil
}

func (r *menuPermissionGroupRepo) ValidateTenantMenuIDs(ctx context.Context, menuIDs []uint32) error {
	ids := uniquePositiveIDs(menuIDs)
	if len(ids) == 0 {
		return nil
	}
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return err
	}
	effectiveIDs, err := r.GetTenantEffectiveMenuIDs(ctx, tenantID)
	if err != nil {
		return err
	}
	allowed := make(map[uint32]struct{}, len(effectiveIDs))
	for _, id := range effectiveIDs {
		allowed[id] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := allowed[id]; !ok {
			return pb.ErrorRolePermissionInvalid("角色菜单超出租户权限组授权范围")
		}
	}
	return nil
}

func uniquePositiveIDs(ids []uint32) []uint32 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uint32]struct{}, len(ids))
	result := make([]uint32, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func menuPermissionGroupMenuIDs(e *gen.MenuPermissionGroup) []uint32 {
	if e == nil || len(e.Edges.Menus) == 0 {
		return nil
	}
	ids := make([]uint32, 0, len(e.Edges.Menus))
	for _, m := range e.Edges.Menus {
		if m != nil {
			ids = append(ids, m.ID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func filterMenuTreeByParent(items []*pbCore.Menu, parentID uint32) []*pbCore.Menu {
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.GetId() == parentID {
			return item.Children
		}
		if children := filterMenuTreeByParent(item.Children, parentID); len(children) > 0 {
			return children
		}
	}
	return nil
}
