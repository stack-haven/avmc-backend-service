package service

import "github.com/google/wire"

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	NewAuthServiceService,
	NewUserServiceService,
	NewDeptServiceService,
	NewMenuServiceService,
	NewMenuPermissionGroupServiceService,
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
	NewAsyncTaskServiceService,
	NewWebhookService,
)
