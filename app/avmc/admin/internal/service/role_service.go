package service

import (
	"context"

	pb "backend-service/api/avmc/admin/v1"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type RoleServiceService struct {
	pb.UnimplementedRoleServiceServer
	ruc *biz.RoleUsecase
	log *log.Helper
}

func NewRoleServiceService(ruc *biz.RoleUsecase, logger log.Logger) *RoleServiceService {
	return &RoleServiceService{
		ruc: ruc,
		log: log.NewHelper(logger),
	}
}

func (s *RoleServiceService) ListRole(ctx context.Context, req *pbPagination.PagingRequest) (*pbCore.ListRoleResponse, error) {
	return &pbCore.ListRoleResponse{}, nil
}
func (s *RoleServiceService) GetRole(ctx context.Context, req *pbCore.GetRoleRequest) (*pbCore.Role, error) {
	return &pbCore.Role{}, nil
}
func (s *RoleServiceService) CreateRole(ctx context.Context, req *pbCore.CreateRoleRequest) (*pbCore.CreateRoleResponse, error) {
	return &pbCore.CreateRoleResponse{}, nil
}
func (s *RoleServiceService) UpdateRole(ctx context.Context, req *pbCore.UpdateRoleRequest) (*pbCore.UpdateRoleResponse, error) {
	return &pbCore.UpdateRoleResponse{}, nil
}
func (s *RoleServiceService) DeleteRole(ctx context.Context, req *pbCore.DeleteRoleRequest) (*pbCore.DeleteRoleResponse, error) {
	return &pbCore.DeleteRoleResponse{}, nil
}
