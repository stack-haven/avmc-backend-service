package service

import (
	"context"

	pb "backend-service/api/avmc/admin/v1"
	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// RoleServiceService 角色服务结构体
// 包含业务用例和日志记录器
type RoleServiceService struct {
	pb.UnimplementedRoleServiceServer
	ruc *biz.RoleUsecase
	log *log.Helper
}

// NewRoleServiceService 创建新的角色服务实例
// 参数：ruc 角色业务用例实例，logger 日志记录器
// 返回值：角色服务实例指针
func NewRoleServiceService(ruc *biz.RoleUsecase, logger log.Logger) *RoleServiceService {
	return &RoleServiceService{
		ruc: ruc,
		log: log.NewHelper(logger),
	}
}

// ListRole 处理角色列表请求
// 参数：ctx 上下文，req 分页请求
// 返回值：角色列表响应，错误信息
func (s *RoleServiceService) ListRole(ctx context.Context, req *pbPagination.PagingRequest) (*pbCore.ListRoleResponse, error) {
	s.log.Infof("查询角色列表分页，分页请求：%v", req)
	return s.ruc.ListPage(ctx, req)
}

// GetRole 处理获取角色详情请求
// 参数：ctx 上下文，req 获取角色请求
// 返回值：角色详情，错误信息
func (s *RoleServiceService) GetRole(ctx context.Context, req *pbCore.GetRoleRequest) (*pbCore.Role, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorRoleInvalidId("角色ID不能为空")
	}
	s.log.Infof("获取角色详情，角色ID：%v", req.GetId())
	return s.ruc.Get(ctx, req.GetId())
}

// CreateRole 处理创建角色请求
// 参数：ctx 上下文，req 创建角色请求
// 返回值：创建角色响应，错误信息
func (s *RoleServiceService) CreateRole(ctx context.Context, req *pbCore.CreateRoleRequest) (*pbCore.CreateRoleResponse, error) {
	if req.GetRole() == nil {
		return nil, pb.ErrorRoleInvalidId("角色信息不能为空")
	}
	s.log.Infof("创建角色，角色信息：%v", req.Role)
	_, err := s.ruc.Create(ctx, req.Role)
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateRoleResponse{}, nil
}

// UpdateRole 处理更新角色请求
// 参数：ctx 上下文，req 更新角色请求
// 返回值：更新角色响应，错误信息
func (s *RoleServiceService) UpdateRole(ctx context.Context, req *pbCore.UpdateRoleRequest) (*pbCore.UpdateRoleResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorRoleInvalidId("角色ID不能为空")
	}
	if req.GetRole() == nil {
		return nil, pb.ErrorRoleInvalidId("角色信息不能为空")
	}
	s.log.Infof("更新角色，角色ID：%v，角色信息：%v", req.GetId(), req.GetRole())
	req.Role.Id = req.GetId()
	_, err := s.ruc.Update(ctx, req.GetRole())
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateRoleResponse{}, nil
}

// DeleteRole 处理删除角色请求
// 参数：ctx 上下文，req 删除角色请求
// 返回值：删除角色响应，错误信息
func (s *RoleServiceService) DeleteRole(ctx context.Context, req *pbCore.DeleteRoleRequest) (*pbCore.DeleteRoleResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorRoleInvalidId("角色ID不能为空")
	}
	s.log.Infof("删除角色，角色ID：%v", req.GetId())
	err := s.ruc.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pbCore.DeleteRoleResponse{}, nil
}
