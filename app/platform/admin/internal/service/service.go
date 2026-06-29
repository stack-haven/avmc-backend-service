package service

import "github.com/google/wire"

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	NewAuthServiceService,
	NewTenantServiceService,
	NewUserServiceService,
	NewDeptServiceService,
	NewMenuServiceService,
	NewMenuPermissionGroupServiceService,
	NewTenantPermissionServiceService,
	NewRoleServiceService,
	NewPostServiceService,
	NewProjectServiceService,
	NewDictionaryServiceService,
	NewOperationLogServiceService,
	NewLoginLogServiceService,
	NewSessionServiceService,
)
