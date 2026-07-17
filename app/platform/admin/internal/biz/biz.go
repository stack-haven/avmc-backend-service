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
	NewMenuPermissionGroupUsecase,
	NewDeptUsecase,
	NewProjectUsecase,
	NewDictionaryUsecase,
	NewOperationLogUsecase,
	NewLoginLogUsecase,
	NewSessionUsecase,
	NewParameterUsecase,
	NewResourceQuotaUsecase,
	NewAsyncTaskHandlers,
	NewAsyncTaskUsecase,
)

type Transaction interface {
	InTx(context.Context, func(ctx context.Context) error) error
}
