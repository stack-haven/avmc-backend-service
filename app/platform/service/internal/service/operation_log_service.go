package service

import (
	"context"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
)

type OperationLogServiceService struct {
	pb.UnimplementedOperationLogServiceServer
	uc  *biz.OperationLogUsecase
	log *log.Helper
}

func NewOperationLogServiceService(uc *biz.OperationLogUsecase, logger log.Logger) *OperationLogServiceService {
	return &OperationLogServiceService{uc: uc, log: log.NewHelper(logger)}
}

func (s *OperationLogServiceService) CreateOperationLog(ctx context.Context, req *pb.CreateOperationLogRequest) (*pb.CreateOperationLogResponse, error) {
	if err := s.uc.Record(ctx, req.GetEntry()); err != nil {
		return nil, err
	}
	return &pb.CreateOperationLogResponse{}, nil
}

func (s *OperationLogServiceService) ListOperationLogs(ctx context.Context, req *pb.ListOperationLogsRequest) (*pb.ListOperationLogsResponse, error) {
	items, total, err := s.uc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListOperationLogsResponse{Items: items, Total: total}
	offset, err := strconv.Atoi(req.GetPageToken())
	if err != nil {
		offset = 0
	}
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
}
func (s *OperationLogServiceService) GetOperationLog(ctx context.Context, req *pb.GetOperationLogRequest) (*pb.OperationLog, error) {
	return s.uc.Get(ctx, req.GetId())
}
