package service

import (
	"context"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
)

type AsyncTaskServiceService struct {
	pb.UnimplementedAsyncTaskServiceServer
	uc  *biz.AsyncTaskUsecase
	log *log.Helper
}

func NewAsyncTaskServiceService(uc *biz.AsyncTaskUsecase, logger log.Logger) *AsyncTaskServiceService {
	return &AsyncTaskServiceService{uc: uc, log: log.NewHelper(logger)}
}

func (s *AsyncTaskServiceService) ListAsyncTasks(ctx context.Context, req *pb.ListAsyncTasksRequest) (*pb.ListAsyncTasksResponse, error) {
	items, total, err := s.uc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListAsyncTasksResponse{Items: items, Total: total}
	offset, err := strconv.Atoi(req.GetPageToken())
	if err != nil {
		offset = 0
	}
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
}

func (s *AsyncTaskServiceService) GetAsyncTaskStats(ctx context.Context, req *pb.GetAsyncTaskStatsRequest) (*pb.GetAsyncTaskStatsResponse, error) {
	stats, err := s.uc.Stats(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.GetAsyncTaskStatsResponse{Stats: stats}, nil
}

func (s *AsyncTaskServiceService) GetAsyncTask(ctx context.Context, req *pb.GetAsyncTaskRequest) (*pb.GetAsyncTaskResponse, error) {
	task, err := s.uc.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.GetAsyncTaskResponse{Task: task}, nil
}

func (s *AsyncTaskServiceService) CancelAsyncTask(ctx context.Context, req *pb.CancelAsyncTaskRequest) (*pb.CancelAsyncTaskResponse, error) {
	if err := s.uc.Cancel(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.CancelAsyncTaskResponse{}, nil
}

func (s *AsyncTaskServiceService) RetryAsyncTask(ctx context.Context, req *pb.RetryAsyncTaskRequest) (*pb.RetryAsyncTaskResponse, error) {
	task, err := s.uc.Retry(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.RetryAsyncTaskResponse{Task: task}, nil
}
