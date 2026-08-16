package service

import (
	"context"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
	"backend-service/pkg/auth/authn"
)

type SessionServiceService struct {
	pb.UnimplementedSessionServiceServer
	uc  *biz.SessionUsecase
	log *log.Helper
}

func NewSessionServiceService(uc *biz.SessionUsecase, logger log.Logger) *SessionServiceService {
	return &SessionServiceService{uc: uc, log: log.NewHelper(logger)}
}

func currentSessionID(ctx context.Context) string {
	claims, _ := authn.AuthClaimsFromContext(ctx)
	return claims.GetID()
}

func (s *SessionServiceService) ListSessions(ctx context.Context, req *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	items, total, err := s.uc.List(ctx, req, currentSessionID(ctx))
	if err != nil {
		return nil, err
	}
	resp := &pb.ListSessionsResponse{Items: items, Total: total}
	offset, err := strconv.Atoi(req.GetPageToken())
	if err != nil {
		offset = 0
	}
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
}

func (s *SessionServiceService) ListMySessions(ctx context.Context, _ *pb.ListMySessionsRequest) (*pb.ListMySessionsResponse, error) {
	items, err := s.uc.ListMine(ctx, authn.GetAuthUserID(ctx), currentSessionID(ctx))
	if err != nil {
		return nil, err
	}
	return &pb.ListMySessionsResponse{Items: items}, nil
}

func (s *SessionServiceService) RevokeSession(ctx context.Context, req *pb.RevokeSessionRequest) (*pb.RevokeSessionResponse, error) {
	if err := s.uc.Revoke(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.RevokeSessionResponse{}, nil
}
