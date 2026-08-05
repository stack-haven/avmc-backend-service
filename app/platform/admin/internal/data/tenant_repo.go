package data

import (
	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/menu"
	"backend-service/app/platform/admin/internal/data/ent/gen/role"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenant"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenantmenupermissiongroup"
	"backend-service/app/platform/admin/internal/data/ent/gen/user"
	"backend-service/app/platform/admin/internal/data/ent/mixins"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
	"context"
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
	return &pbCore.Tenant{
		Id:              e.ID,
		Name:            convert.ToPointer(e.Name),
		Code:            convert.ToPointer(e.Code),
		Sort:            e.Sort,
		Remark:          e.Remark,
		GroupIds:        tenantGroupIDs(e),
		Groups:          r.tenantGroups(e),
		CreatedAt:       convert.TimeValueToString(&e.CreatedAt, time.DateTime),
		UpdatedAt:       convert.TimeValueToString(&e.UpdatedAt, time.DateTime),
		LifecycleStatus: pbCore.TenantLifecycleStatus(e.LifecycleStatus).Enum(),
		ActivatedAt:     convert.TimeValueToString(e.ActivatedAt, time.DateTime),
		ExpiresAt:       convert.TimeValueToString(e.ExpiresAt, time.DateTime),
		SuspendedAt:     convert.TimeValueToString(e.SuspendedAt, time.DateTime),
		CancelledAt:     convert.TimeValueToString(e.CancelledAt, time.DateTime),
		IsPlatform:      &e.IsPlatform,
	}
}

func tenantAdminToProto(e *gen.User) *pbCore.User {
	if e == nil {
		return nil
	}
	status := pbEnum.Status(0)
	if e.Status != nil {
		status = pbEnum.Status(*e.Status)
	}
	return &pbCore.User{
		Id:            e.ID,
		Name:          e.Name,
		Realname:      e.Realname,
		Email:         e.Email,
		Phone:         e.Phone,
		Status:        &status,
		IsTenantAdmin: true,
		CreatedAt:     convert.TimeValueToString(&e.CreatedAt, time.DateTime),
		UpdatedAt:     convert.TimeValueToString(&e.UpdatedAt, time.DateTime),
	}
}

func (r *tenantRepo) ListAdmins(ctx context.Context, tenantID uint32) ([]*pbCore.User, error) {
	systemCtx := entviewer.NewSystemContext(ctx)
	items, err := r.Data.DB(systemCtx).User.Query().
		Where(
			user.TenantIDEQ(tenantID),
			user.HasRolesWith(role.TenantIDEQ(tenantID), role.IsTenantAdminEQ(true)),
		).
		Order(gen.Asc(user.FieldID)).
		All(systemCtx)
	if err != nil {
		return nil, err
	}
	result := make([]*pbCore.User, 0, len(items))
	for _, item := range items {
		result = append(result, tenantAdminToProto(item))
	}
	return result, nil
}

func (r *tenantRepo) tenantAdmin(ctx context.Context, tenantID, adminUserID uint32) (*gen.User, error) {
	systemCtx := entviewer.NewSystemContext(ctx)
	item, err := r.Data.DB(systemCtx).User.Query().Where(
		user.IDEQ(adminUserID),
		user.TenantIDEQ(tenantID),
		user.HasRolesWith(role.TenantIDEQ(tenantID), role.IsTenantAdminEQ(true)),
	).Only(systemCtx)
	if gen.IsNotFound(err) {
		return nil, pb.ErrorBadRequest("目标账号不是该租户的管理员")
	}
	return item, err
}

func (r *tenantRepo) UpdateAdmin(ctx context.Context, tenantID, adminUserID uint32, realname, email, phone *string) (*pbCore.User, error) {
	systemCtx := entviewer.NewSystemContext(ctx)
	if _, err := r.tenantAdmin(systemCtx, tenantID, adminUserID); err != nil {
		return nil, err
	}
	update := r.Data.DB(systemCtx).User.UpdateOneID(adminUserID)
	if realname != nil {
		if *realname == "" {
			update.ClearRealname()
		} else {
			update.SetRealname(*realname)
		}
	}
	if email != nil {
		if *email == "" {
			update.ClearEmail()
		} else {
			update.SetEmail(*email)
		}
	}
	if phone != nil {
		if *phone == "" {
			update.ClearPhone()
		} else {
			update.SetPhone(*phone)
		}
	}
	item, err := update.Save(systemCtx)
	if gen.IsConstraintError(err) {
		return nil, pb.ErrorBadRequest("管理员邮箱或手机号已被使用")
	}
	if err != nil {
		return nil, err
	}
	return tenantAdminToProto(item), nil
}

func (r *tenantRepo) ResetAdminPassword(ctx context.Context, tenantID, adminUserID uint32, passwordHash string) error {
	systemCtx := entviewer.NewSystemContext(ctx)
	if _, err := r.tenantAdmin(systemCtx, tenantID, adminUserID); err != nil {
		return err
	}
	return r.Data.DB(systemCtx).User.UpdateOneID(adminUserID).SetPassword(passwordHash).Exec(systemCtx)
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
	groupIDs := g.GetGroupIds()
	if err := r.validateGroupIDs(systemCtx, groupIDs); err != nil {
		return nil, err
	}
	menuIDs, err := r.menuIDsByGroups(systemCtx, groupIDs)
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
		SetNillableRemark(g.Remark).
		SetLifecycleStatus(int32(pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_PENDING)).
		SetIsPlatform(false).
		SetNillableExpiresAt(expiresAt).
		AddGroupIDs(groupIDs...).
		Save(systemCtx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorBadRequest("租户名称或编码已存在")
		}
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
	if input.AdminRealName != "" {
		adminBuilder.SetRealname(input.AdminRealName)
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
		SetLifecycleStatus(int32(pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE)).
		SetActivatedAt(now).
		Save(systemCtx); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	created, err := r.FindByID(systemCtx, tenantRow.ID)
	if err != nil {
		return nil, err
	}
	// Sync Casbin policies for the admin role and user
	SyncRolePolicies(ctx, r.Data.DB(ctx), r.Data.authorizer, tenantRow.ID, adminRole.ID)
	SyncUserRoles(ctx, r.Data.DB(ctx), r.Data.authorizer, tenantRow.ID, adminUser.ID, []uint32{adminRole.ID})
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
	if err = tx.Tenant.DeleteOneID(result.Tenant.GetId()).Exec(systemCtx); err != nil && !gen.IsNotFound(err) {
		return err
	}
	return tx.Commit()
}

func (r *tenantRepo) Update(ctx context.Context, g *pbCore.Tenant, operatorID uint32) (*pbCore.Tenant, error) {
	if g == nil || g.GetId() == 0 || g.GetName() == "" || g.GetCode() == "" {
		return nil, pb.ErrorBadRequest("租户ID、名称和编码不能为空")
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
	if g.GroupIds != nil {
		builder.ClearGroups().AddGroupIDs(g.GetGroupIds()...)
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
	if err := tx.Commit(); err != nil {
		return nil, err
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
	return convert.SliceToAny(res, r.entToProto), nil
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

func (r *tenantRepo) UpdateLifecycle(ctx context.Context, id uint32, lifecycle pbCore.TenantLifecycleStatus) (*pbCore.Tenant, error) {
	now := time.Now()
	builder := r.Data.DB(ctx).Tenant.UpdateOneID(id).
		SetLifecycleStatus(int32(lifecycle))
	switch lifecycle {
	case pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE:
		builder.SetActivatedAt(now).
			ClearExpiresAt().
			ClearSuspendedAt().
			ClearCancelledAt()
	case pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED:
		builder.SetSuspendedAt(now)
	case pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_EXPIRED:
		builder.SetExpiresAt(now)
	case pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED:
		builder.SetCancelledAt(now)
	}
	if _, err := builder.Save(ctx); err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("租户不存在")
		}
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *tenantRepo) withTenantGroups(query *gen.TenantQuery) *gen.TenantQuery {
	return query.WithGroups(func(q *gen.TenantMenuPermissionGroupQuery) {
		q.WithMenus(func(mq *gen.MenuQuery) { mq.Select(menu.FieldID) })
		q.WithCurrentVersion()
	})
}

func (r *tenantRepo) validateGroupIDs(ctx context.Context, groupIDs []uint32) error {
	ids := uniquePositiveIDs(groupIDs)
	if len(ids) == 0 {
		return nil
	}
	count, err := r.Data.DB(ctx).TenantMenuPermissionGroup.Query().
		Where(tenantmenupermissiongroup.IDIn(ids...)).
		Count(ctx)
	if err != nil {
		return err
	}
	if count != len(ids) {
		return pb.ErrorBadRequest("存在无效业务套餐ID")
	}
	return nil
}

func (r *tenantRepo) menuIDsByGroups(ctx context.Context, groupIDs []uint32) ([]uint32, error) {
	ids := uniquePositiveIDs(groupIDs)
	if len(ids) == 0 {
		return nil, nil
	}
	groups, err := r.Data.DB(ctx).TenantMenuPermissionGroup.Query().
		Where(tenantmenupermissiongroup.IDIn(ids...)).
		WithCurrentVersion(func(q *gen.TenantMenuPermissionGroupVersionQuery) {
			q.WithMenus(func(mq *gen.MenuQuery) { mq.Select(menu.FieldID) })
		}).
		All(ctx)
	if err != nil {
		return nil, err
	}
	menuSet := make(map[uint32]struct{})
	for _, g := range groups {
		if g.Edges.CurrentVersion == nil {
			continue
		}
		for _, m := range g.Edges.CurrentVersion.Edges.Menus {
			menuSet[m.ID] = struct{}{}
		}
	}
	menuIDs := make([]uint32, 0, len(menuSet))
	for id := range menuSet {
		menuIDs = append(menuIDs, id)
	}
	return menuIDs, nil
}

func (r *tenantRepo) tenantGroups(e *gen.Tenant) []*pbCore.TenantMenuPermissionGroup {
	if e == nil || len(e.Edges.Groups) == 0 {
		return nil
	}
	groups := make([]*pbCore.TenantMenuPermissionGroup, 0, len(e.Edges.Groups))
	for _, g := range e.Edges.Groups {
		if g == nil {
			continue
		}
		groups = append(groups, (&tenantMenuPermissionGroupRepo{BaseRepo: r.BaseRepo}).entToProto(g))
	}
	return groups
}

func tenantGroupIDs(e *gen.Tenant) []uint32 {
	if e == nil || len(e.Edges.Groups) == 0 {
		return nil
	}
	ids := make([]uint32, 0, len(e.Edges.Groups))
	for _, g := range e.Edges.Groups {
		if g == nil {
			continue
		}
		ids = append(ids, g.ID)
	}
	return ids
}
