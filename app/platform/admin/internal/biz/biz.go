package biz

import (
	"context"

	"github.com/google/wire"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	NewAuthUsecase,
	NewUserUsecase,
	NewRoleUsecase,
	NewPostUsecase,
	NewMenuUsecase,
	NewMenuPermissionGroupUsecase,
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
	NewNotificationAsyncTaskHandler,
	NewAsyncTaskHandlers,
	NewAsyncTaskUsecase,
	NewWebhookUsecase,
)

type Transaction interface {
	InTx(context.Context, func(ctx context.Context) error) error
}

func NewStorageProviderResolver(repo StorageProviderRepo) StorageProviderResolver {
	return repo
}
