package service

import (
	"context"
	"strconv"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
)

type LoginLogServiceService struct {
	pb.UnimplementedLoginLogServiceServer
	uc *biz.LoginLogUsecase
}

func NewLoginLogServiceService(uc *biz.LoginLogUsecase) *LoginLogServiceService {
	return &LoginLogServiceService{uc: uc}
}

func (s *LoginLogServiceService) ListLoginLogs(ctx context.Context, req *pbCore.ListLoginLogsRequest) (*pbCore.ListLoginLogsResponse, error) {
	items, total, err := s.uc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &pbCore.ListLoginLogsResponse{Items: items, Total: total}
	offset, err := strconv.Atoi(req.GetPageToken())
	if err != nil {
		offset = 0
	}
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
}

func (s *LoginLogServiceService) GetLoginLog(ctx context.Context, req *pbCore.GetLoginLogRequest) (*pbCore.LoginLog, error) {
	return s.uc.Get(ctx, req.GetId())
}
