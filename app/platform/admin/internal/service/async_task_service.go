package service

import (
	"context"
	"strconv"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
)

type AsyncTaskServiceService struct {
	pb.UnimplementedAsyncTaskServiceServer
	uc *biz.AsyncTaskUsecase
}

func NewAsyncTaskServiceService(uc *biz.AsyncTaskUsecase) *AsyncTaskServiceService {
	return &AsyncTaskServiceService{uc: uc}
}

func (s *AsyncTaskServiceService) ListAsyncTasks(ctx context.Context, req *pbCore.ListAsyncTasksRequest) (*pbCore.ListAsyncTasksResponse, error) {
	items, total, err := s.uc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &pbCore.ListAsyncTasksResponse{Items: items, Total: total}
	offset, _ := strconv.Atoi(req.GetPageToken())
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
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
