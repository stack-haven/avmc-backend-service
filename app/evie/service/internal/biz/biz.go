package biz

import (
	"context"

	"github.com/google/wire"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	NewUserUsecase,
)

// SessionRepo TODO: move to session_usecase.go when session business logic created
type SessionRepo interface {
	RevokeUser(context.Context, uint32) error
}

type Transaction interface {
	InTx(context.Context, func(ctx context.Context) error) error
}
