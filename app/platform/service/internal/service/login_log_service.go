package service

import (
	"context"
	"strconv"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
)

type LoginLogServiceService struct {
	pb.UnimplementedLoginLogServiceServer
	uc *biz.LoginLogUsecase
}

func NewLoginLogServiceService(uc *biz.LoginLogUsecase) *LoginLogServiceService {
	return &LoginLogServiceService{uc: uc}
}

func (s *LoginLogServiceService) ListLoginLogs(ctx context.Context, req *pb.ListLoginLogsRequest) (*pb.ListLoginLogsResponse, error) {
	items, total, err := s.uc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListLoginLogsResponse{Items: items, Total: total}
	offset, err := strconv.Atoi(req.GetPageToken())
	if err != nil {
		offset = 0
	}
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
}

func (s *LoginLogServiceService) GetLoginLog(ctx context.Context, req *pb.GetLoginLogRequest) (*pb.LoginLog, error) {
	return s.uc.Get(ctx, req.GetId())
}
