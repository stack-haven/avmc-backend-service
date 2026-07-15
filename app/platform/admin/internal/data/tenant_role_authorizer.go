package data

import (
	"context"
	"io"
	"strconv"

	pbEnum "backend-service/api/common/enum"
	"backend-service/app/platform/admin/internal/authzpolicy"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/menu"
	"backend-service/app/platform/admin/internal/data/ent/gen/menupermissiongroup"
	"backend-service/app/platform/admin/internal/data/ent/gen/menupermissiongroupversion"
	"backend-service/app/platform/admin/internal/data/ent/gen/role"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenantpermissiongroup"
	"backend-service/app/platform/admin/internal/data/ent/gen/user"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"backend-service/pkg/auth/authz"

	"github.com/go-kratos/kratos/v2/log"
)

// tenantRoleAuthorizer keeps Casbin as the platform-policy engine and resolves
// ordinary tenant role permissions from the database source of truth.
type tenantRoleAuthorizer struct {
	authz.Authorizer
	db   *gen.Client
	repo BaseRepo
}

func newTenantRoleAuthorizer(base authz.Authorizer, db *gen.Client, data *Data) authz.Authorizer {
	if data == nil {
		data = &Data{db: db}
	}
	return &tenantRoleAuthorizer{Authorizer: base, db: db, repo: NewBaseRepo(data, log.NewStdLogger(io.Discard))}
}

func (a *tenantRoleAuthorizer) Enforce(
	ctx context.Context,
	sub authz.Subject,
	obj authz.Object,
	act authz.Action,
	tenant authz.Tenant,
) (bool, error) {
	if allowed, err := a.Authorizer.Enforce(ctx, sub, obj, act, tenant); err == nil && allowed {
		return true, nil
	}
	return a.enforceTenantRole(ctx, sub, obj, act, tenant)
}

func (a *tenantRoleAuthorizer) enforceTenantRole(
	ctx context.Context,
	sub authz.Subject,
	obj authz.Object,
	act authz.Action,
	tenant authz.Tenant,
) (bool, error) {
	if a.db == nil {
		return false, nil
	}
	userID, err := strconv.ParseUint(string(sub), 10, 32)
	if err != nil || userID == 0 {
		return false, nil
	}
	tenantID, err := strconv.ParseUint(string(tenant), 10, 32)
	if err != nil || tenantID == 0 {
		return false, nil
	}
	if !authzpolicy.MatchProtectedOperation(obj, act) {
		return false, nil
	}
	if allowed, ok := a.repo.getTenantRoleAuthorizationCache(ctx, uint32(tenantID), uint32(userID), string(obj), string(act)); ok {
		return allowed, nil
	}

	tenantCtx := entviewer.NewTenantContext(ctx, uint32(tenantID))
	activeUser, err := a.db.User.Query().
		Where(
			user.IDEQ(uint32(userID)),
			user.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)),
		).
		Exist(tenantCtx)
	if err != nil || !activeUser {
		return false, err
	}
	if authzpolicy.IsAuthenticatedSelfServiceOperation(obj, act) {
		return true, nil
	}

	roleMenuIDs, err := a.db.Menu.Query().
		Where(
			menu.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)),
			menu.AuthCodeEQ(string(obj)),
			menu.HasRolesWith(
				role.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)),
				role.HasUsersWith(user.IDEQ(uint32(userID))),
			),
		).
		IDs(tenantCtx)
	if err != nil {
		return false, err
	}
	if len(roleMenuIDs) == 0 {
		a.repo.setTenantRoleAuthorizationCache(ctx, uint32(tenantID), uint32(userID), string(obj), string(act), false)
		return false, nil
	}

	allowed, err := a.db.TenantPermissionGroup.Query().
		Where(
			tenantpermissiongroup.TenantIDEQ(uint32(tenantID)),
			tenantpermissiongroup.EnabledEQ(true),
			tenantpermissiongroup.HasGroupWith(
				menupermissiongroup.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)),
			),
			tenantpermissiongroup.Or(
				tenantpermissiongroup.HasVersionWith(
					menupermissiongroupversion.HasMenusWith(menu.IDIn(roleMenuIDs...)),
				),
				tenantpermissiongroup.And(
					tenantpermissiongroup.VersionIDIsNil(),
					tenantpermissiongroup.HasGroupWith(
						menupermissiongroup.HasMenusWith(menu.IDIn(roleMenuIDs...)),
					),
				),
			),
		).
		Exist(entviewer.NewSystemContext(ctx))
	if err == nil {
		a.repo.setTenantRoleAuthorizationCache(ctx, uint32(tenantID), uint32(userID), string(obj), string(act), allowed)
	}
	return allowed, err
}

func (a *tenantRoleAuthorizer) BatchEnforce(
	ctx context.Context,
	subjects []authz.Subject,
	objects []authz.Object,
	actions []authz.Action,
	tenants []authz.Tenant,
) ([]bool, error) {
	if len(subjects) != len(objects) || len(subjects) != len(actions) || len(subjects) != len(tenants) {
		return a.Authorizer.BatchEnforce(ctx, subjects, objects, actions, tenants)
	}
	results := make([]bool, len(subjects))
	for i := range subjects {
		allowed, err := a.Enforce(ctx, subjects[i], objects[i], actions[i], tenants[i])
		if err != nil {
			return nil, err
		}
		results[i] = allowed
	}
	return results, nil
}
