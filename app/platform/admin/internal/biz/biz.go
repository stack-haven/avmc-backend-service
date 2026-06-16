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
	NewDeptUsecase,
	NewProjectUsecase,
)

type Transaction interface {
	InTx(context.Context, func(ctx context.Context) error) error
}
