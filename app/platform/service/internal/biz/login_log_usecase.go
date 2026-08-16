package biz

import (
	"context"

	pb "backend-service/api/platform/service/v1"
)

type LoginLogRepo interface {
	Append(context.Context, *pb.LoginLog) error
	List(context.Context, *pb.ListLoginLogsRequest) ([]*pb.LoginLog, int32, error)
	Get(context.Context, uint64) (*pb.LoginLog, error)
}

type LoginLogUsecase struct{ repo LoginLogRepo }

func NewLoginLogUsecase(repo LoginLogRepo) *LoginLogUsecase {
	return &LoginLogUsecase{repo: repo}
}

func (uc *LoginLogUsecase) Record(ctx context.Context, event *pb.LoginLog) error {
	return uc.repo.Append(ctx, event)
}

func (uc *LoginLogUsecase) List(ctx context.Context, req *pb.ListLoginLogsRequest) ([]*pb.LoginLog, int32, error) {
	return uc.repo.List(ctx, req)
}

func (uc *LoginLogUsecase) Get(ctx context.Context, id uint64) (*pb.LoginLog, error) {
	return uc.repo.Get(ctx, id)
}
