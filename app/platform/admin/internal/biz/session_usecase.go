package biz

import (
	"context"

	pb "backend-service/api/core/service/v1"
)

type SessionRepo interface {
	List(context.Context, *pb.ListSessionsRequest, string) ([]*pb.Session, int32, error)
	ListMine(context.Context, uint32, string) ([]*pb.Session, error)
	Revoke(context.Context, string) error
	RevokeUser(context.Context, uint32) error
	RevokeTenant(context.Context, uint32) error
}

type SessionUsecase struct{ repo SessionRepo }

func NewSessionUsecase(repo SessionRepo) *SessionUsecase {
	return &SessionUsecase{repo: repo}
}

func (uc *SessionUsecase) List(ctx context.Context, req *pb.ListSessionsRequest, currentSessionID string) ([]*pb.Session, int32, error) {
	return uc.repo.List(ctx, req, currentSessionID)
}

func (uc *SessionUsecase) ListMine(ctx context.Context, userID uint32, currentSessionID string) ([]*pb.Session, error) {
	return uc.repo.ListMine(ctx, userID, currentSessionID)
}

func (uc *SessionUsecase) Revoke(ctx context.Context, sessionID string) error {
	return uc.repo.Revoke(ctx, sessionID)
}
