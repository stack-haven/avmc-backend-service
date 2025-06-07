package service

import (
	"context"

	pb "backend-service/api/version/service/v1"
	"backend-service/app/version/service/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type ReleaseService struct {
	pb.UnimplementedReleaseServiceServer
	ruc *biz.ReleaseUsecase
	log *log.Helper
}

func NewReleaseService(ruc *biz.ReleaseUsecase, logger log.Logger) *ReleaseService {
	return &ReleaseService{
		ruc: ruc,
		log: log.NewHelper(logger),
	}
}

func (s *ReleaseService) CreateRelease(ctx context.Context, req *pb.CreateReleaseRequest) (*pb.CreateReleaseResponse, error) {
	return &pb.CreateReleaseResponse{}, nil
}
func (s *ReleaseService) UpdateRelease(ctx context.Context, req *pb.UpdateReleaseRequest) (*pb.UpdateReleaseResponse, error) {
	return &pb.UpdateReleaseResponse{}, nil
}
func (s *ReleaseService) DeleteRelease(ctx context.Context, req *pb.DeleteReleaseRequest) (*pb.DeleteReleaseResponse, error) {
	return &pb.DeleteReleaseResponse{}, nil
}
func (s *ReleaseService) GetRelease(ctx context.Context, req *pb.GetReleaseRequest) (*pb.GetReleaseResponse, error) {
	return &pb.GetReleaseResponse{}, nil
}
func (s *ReleaseService) ListRelease(ctx context.Context, req *pb.ListReleaseRequest) (*pb.ListReleaseResponse, error) {
	return &pb.ListReleaseResponse{}, nil
}
