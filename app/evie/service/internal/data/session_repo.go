package data

import (
	"context"

	"backend-service/app/evie/service/internal/biz"
)

type sessionRepo struct{}

func NewSessionRepo() biz.SessionRepo {
	return &sessionRepo{}
}

func (r *sessionRepo) RevokeUser(_ context.Context, _ uint32) error {
	// TODO: implement session revocation
	return nil
}
