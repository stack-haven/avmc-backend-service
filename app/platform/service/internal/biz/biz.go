package biz

import (
	"context"

	"github.com/google/wire"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	NewAuthUsecase,
	NewTenantUsecase,
	NewUserUsecase,
	NewRoleUsecase,
	NewPostUsecase,
	NewMenuUsecase,
	NewTenantMenuPermissionGroupUsecase,
	NewDeptUsecase,
	NewProjectUsecase,
	NewDictionaryUsecase,
	NewOperationLogUsecase,
	NewLoginLogUsecase,
	NewSessionUsecase,
	NewParameterUsecase,
	NewResourceQuotaUsecase,
	NewStorageProviderUsecase,
	NewStorageProviderResolver,
	NewFileUsecase,
	NewNotificationUsecase,
	NewNotificationProviderUsecase,
	NewDeviceUsecase,
	NewNotificationSenderResolver,
	NewNotificationAsyncTaskHandler,
	NewAsyncTaskHandlers,
	NewAsyncTaskUsecase,
	NewWebhookUsecase,
	NewStorageConfigUsecase,
	NewStorageResolver,
)

type Transaction interface {
	InTx(context.Context, func(ctx context.Context) error) error
}

func NewStorageProviderResolver(repo StorageProviderRepo) StorageProviderResolver {
	return repo
}
