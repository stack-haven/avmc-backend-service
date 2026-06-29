package authzpolicy

import (
	"context"
	"strings"

	v1 "backend-service/api/platform/admin/v1"
	"backend-service/pkg/auth/authz"
)

type Operation struct {
	Object     authz.Object
	HTTPAction authz.Action
	GRPCAction authz.Action
}

func ProtectedOperations() []Operation {
	return []Operation{
		op(v1.OperationAuthServiceCodes, "GET"),
		op(v1.OperationAuthServiceLogout, "POST"),
		op(v1.OperationAuthServiceMenus, "GET"),
		op(v1.OperationAuthServiceProfile, "GET"),
		op(v1.OperationAuthServiceVbenProfile, "GET"),
		op(v1.OperationUserServiceCreateUser, "POST"),
		op(v1.OperationUserServiceDeleteUser, "DELETE"),
		op(v1.OperationUserServiceGetUser, "GET"),
		op(v1.OperationUserServiceListUsers, "GET"),
		op(v1.OperationUserServiceListUsersSimple, "GET"),
		op(v1.OperationUserServiceUpdateUser, "PUT"),
		op(v1.OperationUserServiceUpdateUserByStatus, "PUT"),
		op(v1.OperationDeptServiceCreateDept, "POST"),
		op(v1.OperationDeptServiceDeleteDept, "DELETE"),
		op(v1.OperationDeptServiceGetDept, "GET"),
		op(v1.OperationDeptServiceListDepts, "GET"),
		op(v1.OperationDeptServiceListDeptsTree, "GET"),
		op(v1.OperationDeptServiceUpdateDept, "PUT"),
		op(v1.OperationDeptServiceUpdateDeptByStatus, "PUT"),
		op(v1.OperationMenuServiceCreateMenu, "POST"),
		op(v1.OperationMenuServiceDeleteMenu, "DELETE"),
		op(v1.OperationMenuServiceExistMenuByName, "POST"),
		op(v1.OperationMenuServiceExistMenuByPath, "POST"),
		op(v1.OperationMenuServiceGetMenu, "GET"),
		op(v1.OperationMenuServiceListMenus, "GET"),
		op(v1.OperationMenuServiceListMenusAll, "GET"),
		op(v1.OperationMenuServiceListMenusTree, "GET"),
		op(v1.OperationMenuServiceUpdateMenu, "PUT"),
		op(v1.OperationMenuServiceUpdateMenuByStatus, "PUT"),
		op(v1.OperationTenantServiceCreateTenant, "POST"),
		op(v1.OperationTenantServiceDeleteTenant, "DELETE"),
		op(v1.OperationTenantServiceGetTenant, "GET"),
		op(v1.OperationTenantServiceListTenants, "GET"),
		op(v1.OperationTenantServiceUpdateTenant, "PUT"),
		op(v1.OperationTenantServiceUpdateTenantStatus, "PUT"),
		op(v1.OperationTenantServiceUpdateTenantLifecycle, "PUT"),
		op(v1.OperationMenuPermissionGroupServiceCreateMenuPermissionGroup, "POST"),
		op(v1.OperationMenuPermissionGroupServiceDeleteMenuPermissionGroup, "DELETE"),
		op(v1.OperationMenuPermissionGroupServiceGetMenuPermissionGroup, "GET"),
		op(v1.OperationMenuPermissionGroupServiceListMenuPermissionGroups, "GET"),
		op(v1.OperationMenuPermissionGroupServiceUpdateMenuPermissionGroup, "PUT"),
		op(v1.OperationMenuPermissionGroupServiceUpdateMenuPermissionGroupStatus, "PUT"),
		op(v1.OperationTenantPermissionServiceGetCurrentTenantEffectiveMenus, "GET"),
		op(v1.OperationTenantPermissionServiceGetTenantEffectiveMenus, "GET"),
		op(v1.OperationTenantPermissionServiceGetTenantPermissionGroups, "GET"),
		op(v1.OperationTenantPermissionServiceUpdateTenantPermissionGroups, "PUT"),
		op(v1.OperationRoleServiceCreateRole, "POST"),
		op(v1.OperationRoleServiceDeleteRole, "DELETE"),
		op(v1.OperationRoleServiceExistRoleByName, "POST"),
		op(v1.OperationRoleServiceGetRole, "GET"),
		op(v1.OperationRoleServiceListRoles, "GET"),
		op(v1.OperationRoleServiceUpdateRole, "PUT"),
		op(v1.OperationRoleServiceUpdateRoleByStatus, "PUT"),
		op(v1.OperationPostServiceCreatePost, "POST"),
		op(v1.OperationPostServiceDeletePost, "DELETE"),
		op(v1.OperationPostServiceGetPost, "GET"),
		op(v1.OperationPostServiceListPosts, "GET"),
		op(v1.OperationPostServiceUpdatePost, "PUT"),
		op(v1.OperationPostServiceUpdatePostByStatus, "PUT"),
		op(v1.OperationProjectServiceCreateProject, "POST"),
		op(v1.OperationProjectServiceDeleteProject, "DELETE"),
		op(v1.OperationProjectServiceGetProject, "GET"),
		op(v1.OperationProjectServiceListProjects, "GET"),
		op(v1.OperationProjectServiceUpdateProject, "PUT"),
		op(v1.OperationProjectServiceUpdateProjectByStatus, "PUT"),
		op(v1.OperationDictionaryServiceListDictionaryTypes, "GET"),
		op(v1.OperationDictionaryServiceGetDictionaryType, "GET"),
		op(v1.OperationDictionaryServiceCreateDictionaryType, "POST"),
		op(v1.OperationDictionaryServiceUpdateDictionaryType, "PUT"),
		op(v1.OperationDictionaryServiceDeleteDictionaryType, "DELETE"),
		op(v1.OperationDictionaryServiceListDictionaryItems, "GET"),
		op(v1.OperationDictionaryServiceCreateDictionaryItem, "POST"),
		op(v1.OperationDictionaryServiceUpdateDictionaryItem, "PUT"),
		op(v1.OperationDictionaryServiceDeleteDictionaryItem, "DELETE"),
		op(v1.OperationOperationLogServiceListOperationLogs, "GET"),
		op(v1.OperationOperationLogServiceGetOperationLog, "GET"),
		op(v1.OperationLoginLogServiceListLoginLogs, "GET"),
		op(v1.OperationLoginLogServiceGetLoginLog, "GET"),
		op(v1.OperationSessionServiceListSessions, "GET"),
		op(v1.OperationSessionServiceListMySessions, "GET"),
		op(v1.OperationSessionServiceRevokeSession, "DELETE"),
	}
}

func PoliciesForRole(role authz.Subject, tenant authz.Tenant) []authz.Policy {
	ops := ProtectedOperations()
	policies := make([]authz.Policy, 0, len(ops)*2)
	for _, op := range ops {
		policies = append(policies,
			authz.Policy{Subject: role, Object: op.Object, Action: op.HTTPAction, Tenant: tenant, Effect: authz.EffectAllow},
			authz.Policy{Subject: role, Object: op.Object, Action: op.GRPCAction, Tenant: tenant, Effect: authz.EffectAllow},
		)
	}
	return policies
}

func SyncSuperAdmin(ctx context.Context, authorizer authz.Authorizer, role authz.Subject, tenant authz.Tenant, users []authz.Subject) error {
	for _, policy := range PoliciesForRole(role, tenant) {
		if _, err := authorizer.AddPolicy(ctx, policy); err != nil {
			return err
		}
	}
	for _, user := range users {
		if user == "" {
			continue
		}
		if _, err := authorizer.AddRoleForUser(ctx, user, role, tenant); err != nil {
			return err
		}
	}
	return nil
}

func op(operation string, httpAction authz.Action) Operation {
	return Operation{
		Object:     authz.Object(operation),
		HTTPAction: httpAction,
		GRPCAction: authz.Action(lastSegment(operation)),
	}
}

func lastSegment(operation string) string {
	parts := strings.Split(operation, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return operation
}
