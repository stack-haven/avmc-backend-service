package service

import "github.com/google/wire"

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	NewAuthServiceService,
	NewTenantServiceService,
	NewUserServiceService,
	NewDeptServiceService,
	NewMenuServiceService,
	NewTenantMenuPermissionGroupServiceService,
	NewRoleServiceService,
	NewPostServiceService,
	NewProjectServiceService,
	NewDictionaryServiceService,
	NewOperationLogServiceService,
	NewLoginLogServiceService,
	NewSessionServiceService,
	NewParameterServiceService,
	NewStorageProviderServiceService,
	NewFileCenterServiceService,
	NewNotificationServiceService,
	NewNotificationProviderServiceService,
	NewDeviceServiceService,
	NewAsyncTaskServiceService,
	NewWebhookService,
	NewStorageConfigService,
	NewAuthzService,
	NewCoreOperationLogService,
	NewCoreFileCenterService,
)
