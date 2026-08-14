package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	corepb "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"
)

// CoreOperationLogService 为产品服务（evie 等）提供跨服务的操作审计委托。
// 它实现了 core.service.v1.OperationLogService（gRPC-only），供 pkg/audit/grpc
// 客户端调用 CreateOperationLog，复用中台的 append-only 审计与脱敏能力。
type CoreOperationLogService struct {
	corepb.UnimplementedOperationLogServiceServer
	uc  *biz.OperationLogUsecase
	log *log.Helper
}

// NewCoreOperationLogService 创建核心操作日志服务实例。
func NewCoreOperationLogService(uc *biz.OperationLogUsecase, logger log.Logger) *CoreOperationLogService {
	return &CoreOperationLogService{uc: uc, log: log.NewHelper(logger)}
}

// CreateOperationLog 记录一条操作日志（供内部服务通过 gRPC 调用）。
func (s *CoreOperationLogService) CreateOperationLog(ctx context.Context, req *corepb.CreateOperationLogRequest) (*corepb.CreateOperationLogResponse, error) {
	if err := s.uc.Record(ctx, req.GetEntry()); err != nil {
		return nil, err
	}
	return &corepb.CreateOperationLogResponse{}, nil
}

// ListOperationLogs 分页查询操作日志（保留完整接口，供内部服务复用）。
func (s *CoreOperationLogService) ListOperationLogs(ctx context.Context, req *corepb.ListOperationLogsRequest) (*corepb.ListOperationLogsResponse, error) {
	items, total, err := s.uc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return &corepb.ListOperationLogsResponse{Items: items, Total: total}, nil
}

// GetOperationLog 获取操作日志详情。
func (s *CoreOperationLogService) GetOperationLog(ctx context.Context, req *corepb.GetOperationLogRequest) (*corepb.OperationLog, error) {
	return s.uc.Get(ctx, req.GetId())
}
