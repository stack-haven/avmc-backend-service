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
	"backend-service/app/platform/admin/internal/data/ent/mixins"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
	"context"
	"sort"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"
)

var _ biz.TenantRepo = (*tenantRepo)(nil)

type tenantRepo struct {
	BaseRepo
}

func NewTenantRepo(data *Data, logger log.Logger) biz.TenantRepo {
	return &tenantRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func (r *tenantRepo) entToProto(e *gen.Tenant) *pbCore.Tenant {
	if e == nil {
		return nil
	}
	status := pbEnum.Status(0)
	if e.Status != nil {
		status = pbEnum.Status(*e.Status)
	}
	return &pbCore.Tenant{
		Id:              e.ID,
		Name:            convert.ToPointer(e.Name),
		Code:            convert.ToPointer(e.Code),
		Sort:            e.Sort,
		Status:          &status,
		Remark:          e.Remark,
		GroupIds:        tenantPermissionGroupIDs(e),
		Groups:          r.tenantPermissionGroups(e),
		CreatedAt:       convert.TimeValueToString(&e.CreatedAt, time.DateTime),
		UpdatedAt:       convert.TimeValueToString(&e.UpdatedAt, time.DateTime),
		LifecycleStatus: pbCore.TenantLifecycleStatus(e.LifecycleStatus).Enum(),
		ActivatedAt:     convert.TimeValueToString(e.ActivatedAt, time.DateTime),
		ExpiresAt:       convert.TimeValueToString(e.ExpiresAt, time.DateTime),
		SuspendedAt:     convert.TimeValueToString(e.SuspendedAt, time.DateTime),
		CancelledAt:     convert.TimeValueToString(e.CancelledAt, time.DateTime),
	}
}

func (r *tenantRepo) Provision(ctx context.Context, input *biz.TenantProvisioning) (*biz.TenantProvisioningResult, error) {
	if input == nil || input.Tenant == nil || input.AdminUsername == "" || input.AdminPasswordHash == "" {
		return nil, pb.ErrorBadRequest("租户和初始管理员信息不能为空")
	}
	g := input.Tenant
	if g.GetName() == "" || g.GetCode() == "" {
		return nil, pb.ErrorBadRequest("租户名称和编码不能为空")
	}
	systemCtx := entviewer.NewSystemContext(ctx)
	if err := r.validateGroupIDs(systemCtx, g.GetGroupIds()); err != nil {
		return nil, err
	}
	menuIDs, err := r.menuIDsByGroups(systemCtx, g.GetGroupIds())
	if err != nil {
		return nil, err
	}
	if len(menuIDs) == 0 {
		return nil, pb.ErrorBadRequest("业务套餐未包含有效菜单")
	}
	var expiresAt *time.Time
	if g.GetExpiresAt() != "" {
		value, parseErr := time.Parse(time.DateTime, g.GetExpiresAt())
		if parseErr != nil {
			return nil, pb.ErrorBadRequest("到期时间格式错误")
		}
		expiresAt = &value
	}
	now := time.Now()
	tx, err := r.Data.DB(systemCtx).Tx(systemCtx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)
	tenantRow, err := tx.Tenant.Create().
		SetName(g.GetName()).
		SetCode(g.GetCode()).
		SetNillableSort(g.Sort).
		SetStatus(int32(pbEnum.Status_STATUS_DISABLED)).
		SetLifecycleStatus(int32(pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_PENDING)).
		SetNillableExpiresAt(expiresAt).
		SetNillableRemark(g.Remark).
		Save(systemCtx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorBadRequest("租户名称或编码已存在")
		}
		return nil, err
	}
	if err := r.replaceTenantGroups(systemCtx, tx, tenantRow.ID, g.GetGroupIds(), input.OperatorID); err != nil {
		return nil, err
	}
	rootDept, err := tx.Dept.Create().
		SetTenantID(tenantRow.ID).
		SetName(g.GetName()).
		SetParentID(0).
		SetAncestors([]int{}).
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		Save(systemCtx)
	if err != nil {
		return nil, err
	}
	roleBuilder := tx.Role.Create().
		SetTenantID(tenantRow.ID).
		SetName("租户管理员").
		SetIsTenantAdmin(true).
		SetDataScope(1).
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddMenuIDs(menuIDs...)
	adminRole, err := roleBuilder.Save(systemCtx)
	if err != nil {
		return nil, err
	}
	adminBuilder := tx.User.Create().
		SetTenantID(tenantRow.ID).
		SetName(input.AdminUsername).
		SetPassword(input.AdminPasswordHash).
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddRoleIDs(adminRole.ID)
	if input.AdminRealname != "" {
		adminBuilder.SetRealname(input.AdminRealname)
	}
	if input.AdminEmail != "" {
		adminBuilder.SetEmail(input.AdminEmail)
	}
	adminUser, err := adminBuilder.Save(systemCtx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorBadRequest("初始管理员用户名或邮箱已存在")
		}
		return nil, err
	}
	if _, err = tx.Dept.UpdateOneID(rootDept.ID).SetLeaderID(adminUser.ID).Save(systemCtx); err != nil {
		return nil, err
	}
	if _, err = tx.Tenant.UpdateOneID(tenantRow.ID).
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetLifecycleStatus(int32(pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE)).
		SetActivatedAt(now).
		Save(systemCtx); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	r.bumpTenantPackageVersion(ctx, tenantRow.ID)
	created, err := r.FindByID(systemCtx, tenantRow.ID)
	if err != nil {
		return nil, err
	}
	return &biz.TenantProvisioningResult{
		Tenant: created, AdminUserID: adminUser.ID, AdminRoleID: adminRole.ID, RootDeptID: rootDept.ID,
	}, nil
}

func (r *tenantRepo) RollbackProvisioning(ctx context.Context, result *biz.TenantProvisioningResult) error {
	if result == nil || result.Tenant == nil || result.Tenant.GetId() == 0 {
		return pb.ErrorBadRequest("租户开通补偿信息不能为空")
	}
	systemCtx := mixins.SkipSoftDelete(entviewer.NewSystemContext(ctx))
	tx, err := r.Data.DB(systemCtx).Tx(systemCtx)
	if err != nil {
		return err
	}
	defer rollback(tx, r.Log)

	if result.AdminUserID > 0 {
		if err = tx.User.DeleteOneID(result.AdminUserID).Exec(systemCtx); err != nil && !gen.IsNotFound(err) {
			return err
		}
	}
	if result.AdminRoleID > 0 {
		if err = tx.Role.DeleteOneID(result.AdminRoleID).Exec(systemCtx); err != nil && !gen.IsNotFound(err) {
			return err
		}
	}
	if result.RootDeptID > 0 {
		if err = tx.Dept.DeleteOneID(result.RootDeptID).Exec(systemCtx); err != nil && !gen.IsNotFound(err) {
			return err
		}
	}
	if _, err = tx.TenantPermissionGroup.Delete().
		Where(tenantpermissiongroup.TenantIDEQ(result.Tenant.GetId())).
		Exec(systemCtx); err != nil {
		return err
	}
	if err = tx.Tenant.DeleteOneID(result.Tenant.GetId()).Exec(systemCtx); err != nil && !gen.IsNotFound(err) {
		return err
	}
	return tx.Commit()
}

func (r *tenantRepo) menuIDsByGroups(ctx context.Context, groupIDs []uint32) ([]uint32, error) {
	groups, err := r.Data.DB(ctx).MenuPermissionGroup.Query().
		Where(
			menupermissiongroup.IDIn(uniquePositiveIDs(groupIDs)...),
			menupermissiongroup.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)),
		).
		WithMenus(func(q *gen.MenuQuery) {
			q.Where(menu.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED))).Select(menu.FieldID)
		}).
		All(ctx)
	if err != nil {
		return nil, err
	}
	idSet := make(map[uint32]struct{})
	for _, group := range groups {
		for _, item := range group.Edges.Menus {
			idSet[item.ID] = struct{}{}
		}
	}
	ids := make([]uint32, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (r *tenantRepo) Save(ctx context.Context, g *pbCore.Tenant, operatorID uint32) (*pbCore.Tenant, error) {
	if g == nil || g.GetName() == "" || g.GetCode() == "" {
		return nil, pb.ErrorBadRequest("租户名称和编码不能为空")
	}
	if err := r.validateGroupIDs(ctx, g.GetGroupIds()); err != nil {
		return nil, err
	}
	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)
	res, err := tx.Tenant.Create().
		SetName(g.GetName()).
		SetCode(g.GetCode()).
		SetNillableSort(g.Sort).
		SetNillableStatus((*int32)(g.Status)).
		SetNillableRemark(g.Remark).
		Save(ctx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorBadRequest("租户名称或编码已存在")
		}
		return nil, err
	}
	if err := r.replaceTenantGroups(ctx, tx, res.ID, g.GetGroupIds(), operatorID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	r.bumpTenantPackageVersion(ctx, res.ID)
	return r.FindByID(ctx, res.ID)
}

func (r *tenantRepo) Update(ctx context.Context, g *pbCore.Tenant, operatorID uint32) (*pbCore.Tenant, error) {
	if g == nil || g.GetId() == 0 || g.GetName() == "" || g.GetCode() == "" {
		return nil, pb.ErrorBadRequest("租户ID、名称和编码不能为空")
	}
	if err := r.validateGroupIDs(ctx, g.GetGroupIds()); err != nil {
		return nil, err
	}
	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)
	builder := tx.Tenant.UpdateOneID(g.GetId()).
		SetName(g.GetName()).
		SetCode(g.GetCode()).
		SetNillableSort(g.Sort).
		SetNillableRemark(g.Remark)
	if g.ExpiresAt != nil {
		if g.GetExpiresAt() == "" {
			builder.ClearExpiresAt()
		} else {
			expiresAt, parseErr := time.Parse(time.DateTime, g.GetExpiresAt())
			if parseErr != nil {
				return nil, pb.ErrorBadRequest("到期时间格式错误")
			}
			builder.SetExpiresAt(expiresAt)
		}
	}
	res, err := builder.Save(ctx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorBadRequest("租户名称或编码已存在")
		}
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("租户不存在")
		}
		return nil, err
	}
	if g.GroupIds != nil {
		if err := r.replaceTenantGroups(ctx, tx, res.ID, g.GetGroupIds(), operatorID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if g.GroupIds != nil {
		r.bumpTenantPackageVersion(ctx, res.ID)
	}
	return r.FindByID(ctx, res.ID)
}

func (r *tenantRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Tenant, error) {
	res, err := r.withTenantGroups(r.Data.DB(ctx).Tenant.Query()).Where(tenant.IDEQ(id)).Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("租户不存在")
		}
		return nil, err
	}
	return r.entToProto(res), nil
}

func (r *tenantRepo) CountTenants(ctx context.Context, opts ...listing.Option) (int32, error) {
	o := listing.Options{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.Data.DB(ctx).Tenant.Query().
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *tenantRepo) ListTenants(ctx context.Context, opts ...listing.Option) ([]*pbCore.Tenant, error) {
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	res, err := r.withTenantGroups(r.Data.DB(ctx).Tenant.Query()).
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return ConvertSlice(res, r.entToProto), nil
}

func (r *tenantRepo) withTenantGroups(query *gen.TenantQuery) *gen.TenantQuery {
	return query.WithPermissionGroupBindings(func(q *gen.TenantPermissionGroupQuery) {
		q.Where(tenantpermissiongroup.EnabledEQ(true)).
			WithGroup(func(gq *gen.MenuPermissionGroupQuery) {
				gq.WithMenus(func(mq *gen.MenuQuery) { mq.Select(menu.FieldID) })
				gq.WithCurrentVersion()
				gq.WithTenantBindings()
			})
	})
}

func (r *tenantRepo) validateGroupIDs(ctx context.Context, groupIDs []uint32) error {
	ids := uniquePositiveIDs(groupIDs)
	if len(ids) == 0 {
		return nil
	}
	count, err := r.Data.DB(ctx).MenuPermissionGroup.Query().
		Where(
			menupermissiongroup.IDIn(ids...),
			menupermissiongroup.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)),
		).
		Count(ctx)
	if err != nil {
		return err
	}
	if count != len(ids) {
		return pb.ErrorBadRequest("存在无效业务套餐ID")
	}
	return nil
}

func (r *tenantRepo) replaceTenantGroups(ctx context.Context, tx *gen.Tx, tenantID uint32, groupIDs []uint32, operatorID uint32) error {
	existing, err := tx.TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.TenantIDEQ(tenantID)).
		All(ctx)
	if err != nil {
		return err
	}
	byGroup := make(map[uint32]*gen.TenantPermissionGroup, len(existing))
	for _, item := range existing {
		byGroup[item.GroupID] = item
	}
	for _, groupID := range uniquePositiveIDs(groupIDs) {
		if item, ok := byGroup[groupID]; ok {
			builder := tx.TenantPermissionGroup.UpdateOneID(item.ID).SetEnabled(true)
			if operatorID > 0 {
				builder.SetBoundBy(operatorID)
			}
			if _, err = builder.Save(ctx); err != nil {
				return err
			}
			delete(byGroup, groupID)
			continue
		}
		group, err := tx.MenuPermissionGroup.Get(ctx, groupID)
		if err != nil {
			return err
		}
		builder := tx.TenantPermissionGroup.Create().
			SetTenantID(tenantID).
			SetGroupID(groupID).
			SetEnabled(true).
			SetAutoUpgrade(true).
			SetNillableVersionID(group.CurrentVersionID)
		if operatorID > 0 {
			builder.SetBoundBy(operatorID)
		}
		if _, err := builder.Save(ctx); err != nil {
			return err
		}
	}
	for _, item := range byGroup {
		if err = tx.TenantPermissionGroup.DeleteOneID(item.ID).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *tenantRepo) tenantPermissionGroups(e *gen.Tenant) []*pbCore.MenuPermissionGroup {
	if e == nil || len(e.Edges.PermissionGroupBindings) == 0 {
		return nil
	}
	groups := make([]*pbCore.MenuPermissionGroup, 0, len(e.Edges.PermissionGroupBindings))
	for _, binding := range e.Edges.PermissionGroupBindings {
		if binding == nil || (binding.Enabled != nil && !*binding.Enabled) {
			continue
		}
		group, err := binding.Edges.GroupOrErr()
		if err != nil {
			continue
		}
		groups = append(groups, (&menuPermissionGroupRepo{BaseRepo: r.BaseRepo}).entToProto(group))
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].GetId() < groups[j].GetId() })
	return groups
}

func tenantPermissionGroupIDs(e *gen.Tenant) []uint32 {
	if e == nil || len(e.Edges.PermissionGroupBindings) == 0 {
		return nil
	}
	ids := make([]uint32, 0, len(e.Edges.PermissionGroupBindings))
	for _, binding := range e.Edges.PermissionGroupBindings {
		if binding == nil || (binding.Enabled != nil && !*binding.Enabled) || binding.GroupID == 0 {
			continue
		}
		ids = append(ids, binding.GroupID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (r *tenantRepo) Delete(ctx context.Context, id uint32) error {
	if err := r.Data.DB(ctx).Tenant.DeleteOneID(id).Exec(ctx); err != nil {
		if gen.IsNotFound(err) {
			return pb.ErrorResourceNotFound("租户不存在")
		}
		return err
	}
	return nil
}

func (r *tenantRepo) UpdateStatus(ctx context.Context, id uint32, status pbEnum.Status) (*pbCore.Tenant, error) {
	res, err := r.Data.DB(ctx).Tenant.UpdateOneID(id).
		SetStatus(int32(status)).
		Save(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("租户不存在")
		}
		return nil, err
	}
	return r.entToProto(res), nil
}

func (r *tenantRepo) UpdateLifecycle(ctx context.Context, id uint32, lifecycle pbCore.TenantLifecycleStatus) (*pbCore.Tenant, error) {
	now := time.Now()
	builder := r.Data.DB(ctx).Tenant.UpdateOneID(id).
		SetLifecycleStatus(int32(lifecycle))
	if lifecycle == pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE {
		builder.SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
			SetActivatedAt(now).
			ClearSuspendedAt().
			ClearCancelledAt()
	} else {
		builder.SetStatus(int32(pbEnum.Status_STATUS_DISABLED))
		switch lifecycle {
		case pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED:
			builder.SetSuspendedAt(now)
		case pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED:
			builder.SetCancelledAt(now)
		}
	}
	if _, err := builder.Save(ctx); err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("租户不存在")
		}
		return nil, err
	}
	return r.FindByID(ctx, id)
}
