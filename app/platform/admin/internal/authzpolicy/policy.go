package authzpolicy

import (
	v1 "backend-service/api/platform/admin/v1"
)

// IsPlatformControlOperation identifies operations that manage global platform
// resources or explicitly target another tenant. This is used by the middleware
// to distinguish platform control-plane operations from tenant data-plane ones.
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
		v1.OperationTenantServiceListTenantAdmins,
		v1.OperationTenantServiceResetTenantAdminPassword,
		v1.OperationTenantServiceUpdateTenant,
		v1.OperationTenantServiceUpdateTenantAdmin,
		v1.OperationTenantServiceUpdateTenantLifecycle,
		v1.OperationTenantMenuPermissionGroupServiceCreateTenantMenuPermissionGroup,
		v1.OperationTenantMenuPermissionGroupServiceDeleteTenantMenuPermissionGroup,
		v1.OperationTenantMenuPermissionGroupServiceGetTenantMenuPermissionGroup,
		v1.OperationTenantMenuPermissionGroupServiceListTenantMenuPermissionGroups,
		v1.OperationTenantMenuPermissionGroupServiceUpdateTenantMenuPermissionGroup,
		v1.OperationTenantMenuPermissionGroupServiceUpdateTenantMenuPermissionGroupStatus,
		v1.OperationTenantMenuPermissionGroupServiceListTenantMenuPermissionGroupVersions,
		v1.OperationTenantMenuPermissionGroupServicePublishTenantMenuPermissionGroupVersion,
		v1.OperationTenantMenuPermissionGroupServiceRollbackTenantMenuPermissionGroupVersion,
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
