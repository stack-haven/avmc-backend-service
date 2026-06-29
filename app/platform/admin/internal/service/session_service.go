package service

import (
	"context"
	"strconv"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/pkg/auth/authn"
)

type SessionServiceService struct {
	pb.UnimplementedSessionServiceServer
	uc *biz.SessionUsecase
}

func NewSessionServiceService(uc *biz.SessionUsecase) *SessionServiceService {
	return &SessionServiceService{uc: uc}
}

func currentSessionID(ctx context.Context) string {
	claims, _ := authn.AuthClaimsFromContext(ctx)
	return claims.GetID()
}

func (s *SessionServiceService) ListSessions(ctx context.Context, req *pbCore.ListSessionsRequest) (*pbCore.ListSessionsResponse, error) {
	items, total, err := s.uc.List(ctx, req, currentSessionID(ctx))
	if err != nil {
		return nil, err
	}
	resp := &pbCore.ListSessionsResponse{Items: items, Total: total}
	offset, _ := strconv.Atoi(req.GetPageToken())
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
}

func (s *SessionServiceService) ListMySessions(ctx context.Context, _ *pbCore.ListMySessionsRequest) (*pbCore.ListMySessionsResponse, error) {
	items, err := s.uc.ListMine(ctx, authn.GetAuthUserID(ctx), currentSessionID(ctx))
	if err != nil {
		return nil, err
	}
	return &pbCore.ListMySessionsResponse{Items: items}, nil
}

func (s *SessionServiceService) RevokeSession(ctx context.Context, req *pbCore.RevokeSessionRequest) (*pbCore.RevokeSessionResponse, error) {
	if err := s.uc.Revoke(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.RevokeSessionResponse{}, nil
}
