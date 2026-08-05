package service

import (
	"context"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
)

type OperationLogServiceService struct {
	pb.UnimplementedOperationLogServiceServer
	uc  *biz.OperationLogUsecase
	log *log.Helper
}

func NewOperationLogServiceService(uc *biz.OperationLogUsecase, logger log.Logger) *OperationLogServiceService {
	return &OperationLogServiceService{uc: uc, log: log.NewHelper(logger)}
}
func (s *OperationLogServiceService) ListOperationLogs(ctx context.Context, req *pbCore.ListOperationLogsRequest) (*pbCore.ListOperationLogsResponse, error) {
	items, total, err := s.uc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &pbCore.ListOperationLogsResponse{Items: items, Total: total}
	offset, err := strconv.Atoi(req.GetPageToken())
	if err != nil {
		offset = 0
	}
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
}
func (s *OperationLogServiceService) GetOperationLog(ctx context.Context, req *pbCore.GetOperationLogRequest) (*pbCore.OperationLog, error) {
	return s.uc.Get(ctx, req.GetId())
}
