package service

import (
	"context"

	pb "backend-service/api/avmc/admin/v1"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
	"go.einride.tech/aip/pagination"
)

// RoleServiceService 角色服务
type RoleServiceService struct {
	pb.UnimplementedRoleServiceServer
	ruc *biz.RoleUsecase
	log *log.Helper
}

// NewRoleServiceService 创建新的角色服务实例
func NewRoleServiceService(ruc *biz.RoleUsecase, logger log.Logger) *RoleServiceService {
	return &RoleServiceService{
		ruc: ruc,
		log: log.NewHelper(logger),
	}
}

// ListRoles 角色列表
func (s *RoleServiceService) ListRoles(ctx context.Context, req *pbCore.ListRolesRequest) (*pbCore.ListRolesResponse, error) {
	s.log.Infof("查询角色列表，page_size=%d page_token=%s", req.GetPageSize(), req.GetPageToken())
	pageSize := biz.NormalizePageSize(req.GetPageSize())
	req.PageSize = int32(pageSize)
	declarations, err := filtering.NewDeclarations(
		filtering.DeclareStandardFunctions(),
		filtering.DeclareIdent("name", filtering.TypeString),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	filter, err := filtering.ParseFilter(req, declarations)
	if err != nil {
		return nil, err
	}
	pageToken, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, err
	}
	orderBy, err := ordering.ParseOrderBy(req)
	if err != nil {
		return nil, err
	}
	count, err := s.ruc.CountRoles(ctx, biz.ListFilter(filter))
	if err != nil {
		return nil, err
	}
	resp := pbCore.ListRolesResponse{Total: count}
	resp.Items, err = s.ruc.ListRoles(ctx,
		biz.ListFilter(filter), biz.ListOrderBy(orderBy),
		biz.ListLimit(pageSize), biz.ListOffset(int(pageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) >= pageSize {
		resp.NextPageToken = pageToken.Next(req).String()
	}
	return &resp, nil
}

// GetRole 获取角色详情
func (s *RoleServiceService) GetRole(ctx context.Context, req *pbCore.GetRoleRequest) (*pbCore.Role, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorRoleInvalidId("角色ID不能为空")
	}
	s.log.Infof("获取角色详情 ID: %d", req.GetId())
	return s.ruc.Get(ctx, req.GetId())
}

// CreateRole 创建角色
func (s *RoleServiceService) CreateRole(ctx context.Context, req *pbCore.CreateRoleRequest) (*pbCore.CreateRoleResponse, error) {
	if req.GetRole() == nil {
		return nil, pb.ErrorRoleInvalidId("角色信息不能为空")
	}
	s.log.Infof("创建角色: %v", req.GetRole().GetName())
	_, err := s.ruc.Create(ctx, req.GetRole())
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateRoleResponse{}, nil
}

// UpdateRole 更新角色
func (s *RoleServiceService) UpdateRole(ctx context.Context, req *pbCore.UpdateRoleRequest) (*pbCore.UpdateRoleResponse, error) {
	if req.GetId() == 0 || req.GetRole() == nil {
		return nil, pb.ErrorRoleInvalidId("角色ID和信息不能为空")
	}
	existing, err := s.GetRole(ctx, &pbCore.GetRoleRequest{Id: req.GetId()})
	if err != nil {
		return nil, err
	}
	fieldmask.Update(req.UpdateMask, existing, req.Role)
	s.log.Infof("更新角色 ID: %d", req.GetId())

	existing.Id = req.GetId()
	_, err = s.ruc.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateRoleResponse{}, nil
}

// DeleteRole 删除角色
func (s *RoleServiceService) DeleteRole(ctx context.Context, req *pbCore.DeleteRoleRequest) (*pbCore.DeleteRoleResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorRoleInvalidId("角色ID不能为空")
	}
	s.log.Infof("删除角色 ID: %d", req.GetId())
	if err := s.ruc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.DeleteRoleResponse{}, nil
}

// ExistRoleByName 判断角色名是否存在
func (s *RoleServiceService) ExistRoleByName(ctx context.Context, req *pbCore.ExistRoleByNameRequest) (*pbCore.ExistRoleByNameResponse, error) {
	if req.GetName() == "" {
		return nil, pb.ErrorRoleNameCannotBeEmpty("角色名不能为空")
	}
	s.log.Infof("判断角色名是否存在: %s", req.GetName())
	exist, err := s.ruc.ExistByName(ctx, req.GetName(), req.GetId())
	if err != nil {
		return nil, err
	}
	return &pbCore.ExistRoleByNameResponse{Exist: exist}, nil
}

// UpdateRoleByStatus 更新角色状态
func (s *RoleServiceService) UpdateRoleByStatus(ctx context.Context, req *pbCore.UpdateRoleByStatusRequest) (*pbCore.UpdateRoleByStatusResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorRoleInvalidId("角色ID不能为空")
	}
	if req.GetStatus() == 0 {
		return nil, pb.ErrorRoleStatusCannotBeEmpty("角色状态不能为空")
	}
	s.log.Infof("更新角色状态 ID: %d, status: %d", req.GetId(), req.GetStatus())
	_, err := s.ruc.UpdateStatus(ctx, req.GetId(), int32(req.GetStatus()))
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateRoleByStatusResponse{}, nil
}
