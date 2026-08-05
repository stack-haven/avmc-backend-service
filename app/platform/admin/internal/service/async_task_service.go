package service

import (
	"context"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
)

type AsyncTaskServiceService struct {
	pb.UnimplementedAsyncTaskServiceServer
	uc  *biz.AsyncTaskUsecase
	log *log.Helper
}

func NewAsyncTaskServiceService(uc *biz.AsyncTaskUsecase, logger log.Logger) *AsyncTaskServiceService {
	return &AsyncTaskServiceService{uc: uc, log: log.NewHelper(logger)}
}

func (s *AsyncTaskServiceService) ListAsyncTasks(ctx context.Context, req *pbCore.ListAsyncTasksRequest) (*pbCore.ListAsyncTasksResponse, error) {
	items, total, err := s.uc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &pbCore.ListAsyncTasksResponse{Items: items, Total: total}
	offset, err := strconv.Atoi(req.GetPageToken())
	if err != nil {
		offset = 0
	}
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
}

func (s *AsyncTaskServiceService) GetAsyncTaskStats(ctx context.Context, req *pbCore.GetAsyncTaskStatsRequest) (*pbCore.GetAsyncTaskStatsResponse, error) {
	stats, err := s.uc.Stats(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pbCore.GetAsyncTaskStatsResponse{Stats: stats}, nil
}

func (s *AsyncTaskServiceService) GetAsyncTask(ctx context.Context, req *pbCore.GetAsyncTaskRequest) (*pbCore.GetAsyncTaskResponse, error) {
	task, err := s.uc.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pbCore.GetAsyncTaskResponse{Task: task}, nil
}

func (s *AsyncTaskServiceService) CancelAsyncTask(ctx context.Context, req *pbCore.CancelAsyncTaskRequest) (*pbCore.CancelAsyncTaskResponse, error) {
	if err := s.uc.Cancel(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.CancelAsyncTaskResponse{}, nil
}

func (s *AsyncTaskServiceService) RetryAsyncTask(ctx context.Context, req *pbCore.RetryAsyncTaskRequest) (*pbCore.RetryAsyncTaskResponse, error) {
	task, err := s.uc.Retry(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pbCore.RetryAsyncTaskResponse{Task: task}, nil
}
