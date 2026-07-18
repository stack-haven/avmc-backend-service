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
		op(v1.OperationMenuPermissionGroupServiceListMenuPermissionGroupVersions, "GET"),
		op(v1.OperationMenuPermissionGroupServicePublishMenuPermissionGroupVersion, "POST"),
		op(v1.OperationMenuPermissionGroupServiceRollbackMenuPermissionGroupVersion, "POST"),
		op(v1.OperationTenantPermissionServiceCheckCurrentTenantResourceQuota, "GET"),
		op(v1.OperationTenantPermissionServiceConsumeCurrentTenantResourceQuota, "POST"),
		op(v1.OperationTenantPermissionServiceListCurrentTenantResourceQuotas, "GET"),
		op(v1.OperationTenantPermissionServiceReleaseCurrentTenantResourceQuota, "POST"),
		op(v1.OperationTenantPermissionServiceGetCurrentTenantCapabilities, "GET"),
		op(v1.OperationTenantPermissionServiceGetCurrentTenantEffectiveMenus, "GET"),
		op(v1.OperationTenantPermissionServiceGetTenantEffectiveMenus, "GET"),
		op(v1.OperationTenantPermissionServiceGetTenantPermissionGroups, "GET"),
		op(v1.OperationTenantPermissionServiceUpdateTenantPermissionGroups, "PUT"),
		op(v1.OperationTenantPermissionServiceUpdateTenantPermissionGroupVersion, "PUT"),
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
		op(v1.OperationParameterServiceListParameterDefinitions, "GET"),
		op(v1.OperationParameterServiceGetParameterDefinition, "GET"),
		op(v1.OperationParameterServiceCreateParameterDefinition, "POST"),
		op(v1.OperationParameterServiceUpdateParameterDefinition, "PUT"),
		op(v1.OperationParameterServiceDeleteParameterDefinition, "DELETE"),
		op(v1.OperationParameterServiceListCurrentTenantParameters, "GET"),
		op(v1.OperationParameterServiceSetCurrentTenantParameter, "PUT"),
		op(v1.OperationParameterServiceResetCurrentTenantParameter, "DELETE"),
		op(v1.OperationParameterServiceListTenantParameters, "GET"),
		op(v1.OperationParameterServiceSetTenantParameter, "PUT"),
		op(v1.OperationParameterServiceResetTenantParameter, "DELETE"),
		op(v1.OperationFileCenterServiceCreateFileUploadSession, "POST"),
		op(v1.OperationFileCenterServiceUploadFileContent, "POST"),
		op(v1.OperationFileCenterServiceConfirmFileUpload, "POST"),
		op(v1.OperationFileCenterServiceGetFileObject, "GET"),
		op(v1.OperationFileCenterServiceListFileObjects, "GET"),
		op(v1.OperationFileCenterServicePresignFileDownload, "GET"),
		op(v1.OperationFileCenterServiceDeleteFileObject, "DELETE"),
		op(v1.OperationStorageProviderServiceCreateStorageProvider, "POST"),
		op(v1.OperationStorageProviderServiceUpdateStorageProvider, "PUT"),
		op(v1.OperationStorageProviderServiceDeleteStorageProvider, "DELETE"),
		op(v1.OperationStorageProviderServiceGetStorageProvider, "GET"),
		op(v1.OperationStorageProviderServiceListStorageProviders, "GET"),
		op(v1.OperationStorageProviderServiceSetDefaultStorageProvider, "POST"),
		op(v1.OperationStorageProviderServiceTestStorageProvider, "POST"),
		op(v1.OperationAsyncTaskServiceListAsyncTasks, "GET"),
		op(v1.OperationAsyncTaskServiceGetAsyncTaskStats, "GET"),
		op(v1.OperationAsyncTaskServiceGetAsyncTask, "GET"),
		op(v1.OperationAsyncTaskServiceCancelAsyncTask, "POST"),
		op(v1.OperationAsyncTaskServiceRetryAsyncTask, "POST"),
	}
}

func MatchProtectedOperation(object authz.Object, action authz.Action) bool {
	for _, operation := range ProtectedOperations() {
		if operation.Object == object &&
			(operation.HTTPAction == action || operation.GRPCAction == action) {
			return true
		}
	}
	return false
}

func IsAuthenticatedSelfServiceOperation(object authz.Object, action authz.Action) bool {
	switch object {
	case authz.Object(v1.OperationAuthServiceCodes),
		authz.Object(v1.OperationAuthServiceMenus),
		authz.Object(v1.OperationAuthServiceProfile),
		authz.Object(v1.OperationAuthServiceVbenProfile):
		return action == "GET" || action == authz.Action(lastSegment(string(object)))
	case authz.Object(v1.OperationAuthServiceLogout):
		return action == "POST" || action == authz.Action(lastSegment(string(object)))
	case authz.Object(v1.OperationSessionServiceListMySessions):
		return action == "GET" || action == authz.Action(lastSegment(string(object)))
	case authz.Object(v1.OperationTenantPermissionServiceGetCurrentTenantCapabilities):
		return action == "GET" || action == authz.Action(lastSegment(string(object)))
	case authz.Object(v1.OperationTenantPermissionServiceCheckCurrentTenantResourceQuota),
		authz.Object(v1.OperationTenantPermissionServiceListCurrentTenantResourceQuotas):
		return action == "GET" || action == authz.Action(lastSegment(string(object)))
	case authz.Object(v1.OperationTenantPermissionServiceConsumeCurrentTenantResourceQuota),
		authz.Object(v1.OperationTenantPermissionServiceReleaseCurrentTenantResourceQuota):
		return action == "POST" || action == authz.Action(lastSegment(string(object)))
	}
	return false
}

// IsPlatformControlOperation identifies operations that manage global platform
// resources or explicitly target another tenant.
func IsPlatformControlOperation(operation string) bool {
	switch operation {
	case v1.OperationMenuServiceCreateMenu,
		v1.OperationMenuServiceDeleteMenu,
		v1.OperationMenuServiceExistMenuByName,
		v1.OperationMenuServiceExistMenuByPath,
		v1.OperationMenuServiceGetMenu,
		v1.OperationMenuServiceListMenus,
		v1.OperationMenuServiceListMenusAll,
		v1.OperationMenuServiceListMenusTree,
		v1.OperationMenuServiceUpdateMenu,
		v1.OperationMenuServiceUpdateMenuByStatus,
		v1.OperationTenantServiceCreateTenant,
		v1.OperationTenantServiceDeleteTenant,
		v1.OperationTenantServiceGetTenant,
		v1.OperationTenantServiceListTenants,
		v1.OperationTenantServiceUpdateTenant,
		v1.OperationTenantServiceUpdateTenantStatus,
		v1.OperationTenantServiceUpdateTenantLifecycle,
		v1.OperationMenuPermissionGroupServiceCreateMenuPermissionGroup,
		v1.OperationMenuPermissionGroupServiceDeleteMenuPermissionGroup,
		v1.OperationMenuPermissionGroupServiceGetMenuPermissionGroup,
		v1.OperationMenuPermissionGroupServiceListMenuPermissionGroups,
		v1.OperationMenuPermissionGroupServiceUpdateMenuPermissionGroup,
		v1.OperationMenuPermissionGroupServiceUpdateMenuPermissionGroupStatus,
		v1.OperationMenuPermissionGroupServiceListMenuPermissionGroupVersions,
		v1.OperationMenuPermissionGroupServicePublishMenuPermissionGroupVersion,
		v1.OperationMenuPermissionGroupServiceRollbackMenuPermissionGroupVersion,
		v1.OperationTenantPermissionServiceGetTenantEffectiveMenus,
		v1.OperationTenantPermissionServiceGetTenantPermissionGroups,
		v1.OperationTenantPermissionServiceUpdateTenantPermissionGroups,
		v1.OperationTenantPermissionServiceUpdateTenantPermissionGroupVersion,
		v1.OperationParameterServiceListParameterDefinitions,
		v1.OperationParameterServiceGetParameterDefinition,
		v1.OperationParameterServiceCreateParameterDefinition,
		v1.OperationParameterServiceUpdateParameterDefinition,
		v1.OperationParameterServiceDeleteParameterDefinition,
		v1.OperationParameterServiceListTenantParameters,
		v1.OperationParameterServiceSetTenantParameter,
		v1.OperationParameterServiceResetTenantParameter,
		v1.OperationStorageProviderServiceCreateStorageProvider,
		v1.OperationStorageProviderServiceUpdateStorageProvider,
		v1.OperationStorageProviderServiceDeleteStorageProvider,
		v1.OperationStorageProviderServiceGetStorageProvider,
		v1.OperationStorageProviderServiceListStorageProviders,
		v1.OperationStorageProviderServiceSetDefaultStorageProvider,
		v1.OperationStorageProviderServiceTestStorageProvider,
		v1.OperationAsyncTaskServiceListAsyncTasks,
		v1.OperationAsyncTaskServiceGetAsyncTaskStats,
		v1.OperationAsyncTaskServiceGetAsyncTask,
		v1.OperationAsyncTaskServiceCancelAsyncTask,
		v1.OperationAsyncTaskServiceRetryAsyncTask:
		return true
	default:
		return false
	}
}

func TenantOperations() []Operation {
	operations := ProtectedOperations()
	result := make([]Operation, 0, len(operations))
	for _, operation := range operations {
		if !IsPlatformControlOperation(string(operation.Object)) {
			result = append(result, operation)
		}
	}
	return result
}

func PoliciesForRole(role authz.Subject, tenant authz.Tenant) []authz.Policy {
	return policiesForOperations(role, tenant, TenantOperations())
}

func PoliciesForPlatformRole(role authz.Subject, tenant authz.Tenant) []authz.Policy {
	return policiesForOperations(role, tenant, ProtectedOperations())
}

func policiesForOperations(role authz.Subject, tenant authz.Tenant, ops []Operation) []authz.Policy {
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
	return syncAdmin(ctx, authorizer, role, tenant, users, PoliciesForRole(role, tenant))
}

func SyncPlatformAdmin(ctx context.Context, authorizer authz.Authorizer, role authz.Subject, tenant authz.Tenant, users []authz.Subject) error {
	return syncAdmin(ctx, authorizer, role, tenant, users, PoliciesForPlatformRole(role, tenant))
}

func SetAdminMembership(
	ctx context.Context,
	authorizer authz.Authorizer,
	tenant authz.Tenant,
	user authz.Subject,
	platform bool,
	enabled bool,
) error {
	role := authz.Subject("super_admin")
	desired := PoliciesForRole(role, tenant)
	if platform {
		desired = PoliciesForPlatformRole(role, tenant)
	}
	for _, policy := range PoliciesForPlatformRole(role, tenant) {
		if _, err := authorizer.RemovePolicy(ctx, policy); err != nil {
			return err
		}
	}
	for _, policy := range desired {
		if _, err := authorizer.AddPolicy(ctx, policy); err != nil {
			return err
		}
	}
	if enabled {
		_, err := authorizer.AddRoleForUser(ctx, user, role, tenant)
		return err
	}
	_, err := authorizer.DeleteRoleForUser(ctx, user, role, tenant)
	return err
}

func syncAdmin(ctx context.Context, authorizer authz.Authorizer, role authz.Subject, tenant authz.Tenant, users []authz.Subject, desired []authz.Policy) error {
	// Remove the previous complete policy set first so changing a tenant from
	// platform to business scope cannot leave stale control-plane grants.
	for _, policy := range PoliciesForPlatformRole(role, tenant) {
		if _, err := authorizer.RemovePolicy(ctx, policy); err != nil {
			return err
		}
	}
	for _, policy := range desired {
		if _, err := authorizer.AddPolicy(ctx, policy); err != nil {
			return err
		}
	}
	desiredUsers := make(map[authz.Subject]struct{}, len(users))
	for _, user := range users {
		if user != "" {
			desiredUsers[user] = struct{}{}
		}
	}
	existingUsers, err := authorizer.GetUsersForRole(ctx, role, tenant)
	if err != nil {
		return err
	}
	for _, user := range existingUsers {
		if _, ok := desiredUsers[user]; ok {
			continue
		}
		if _, err := authorizer.DeleteRoleForUser(ctx, user, role, tenant); err != nil {
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
