package service

import (
	"context"

	pb "backend-service/api/avmc/admin/v1"
	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type DeptServiceService struct {
	pb.UnimplementedDeptServiceServer
	duc *biz.DeptUsecase
	log *log.Helper
}

func NewDeptServiceService(duc *biz.DeptUsecase, logger log.Logger) *DeptServiceService {
	return &DeptServiceService{
		duc: duc,
		log: log.NewHelper(logger),
	}
}

func (s *DeptServiceService) ListDept(ctx context.Context, req *pbPagination.PagingRequest) (*pbCore.ListDeptResponse, error) {
	return &pbCore.ListDeptResponse{}, nil
}
func (s *DeptServiceService) GetDept(ctx context.Context, req *pbCore.GetDeptRequest) (*pbCore.Dept, error) {
	return &pbCore.Dept{}, nil
}
func (s *DeptServiceService) CreateDept(ctx context.Context, req *pbCore.CreateDeptRequest) (*pbCore.CreateDeptResponse, error) {
	return &pbCore.CreateDeptResponse{}, nil
}
func (s *DeptServiceService) UpdateDept(ctx context.Context, req *pbCore.UpdateDeptRequest) (*pbCore.UpdateDeptResponse, error) {
	return &pbCore.UpdateDeptResponse{}, nil
}
func (s *DeptServiceService) DeleteDept(ctx context.Context, req *pbCore.DeleteDeptRequest) (*pbCore.DeleteDeptResponse, error) {
	return &pbCore.DeleteDeptResponse{}, nil
}
