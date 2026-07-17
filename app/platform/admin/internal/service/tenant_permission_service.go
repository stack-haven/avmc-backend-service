package service

import (
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/pkg/auth/authn"
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

// TenantPermissionServiceService manages tenant permission group bindings.
type TenantPermissionServiceService struct {
	pb.UnimplementedTenantPermissionServiceServer
	uc  *biz.MenuPermissionGroupUsecase
	log *log.Helper
}

func NewTenantPermissionServiceService(uc *biz.MenuPermissionGroupUsecase, logger log.Logger) *TenantPermissionServiceService {
	return &TenantPermissionServiceService{uc: uc, log: log.NewHelper(logger)}
}

func (s *TenantPermissionServiceService) GetTenantPermissionGroups(ctx context.Context, req *pbCore.GetTenantPermissionGroupsRequest) (*pbCore.GetTenantPermissionGroupsResponse, error) {
	if req.GetTenantId() == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	groups, err := s.uc.GetTenantGroups(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	groupIDs := make([]uint32, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			groupIDs = append(groupIDs, group.GetId())
		}
	}
	bindings, err := s.uc.GetTenantGroupBindings(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	return &pbCore.GetTenantPermissionGroupsResponse{
		Groups:   groups,
		GroupIds: groupIDs,
		Bindings: bindings,
	}, nil
}

func (s *TenantPermissionServiceService) UpdateTenantPermissionGroups(ctx context.Context, req *pbCore.UpdateTenantPermissionGroupsRequest) (*pbCore.UpdateTenantPermissionGroupsResponse, error) {
	if req.GetTenantId() == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	if err := s.uc.UpdateTenantGroups(ctx, req.GetTenantId(), req.GetGroupIds(), req.GetOperatorId()); err != nil {
		return nil, err
	}
	return &pbCore.UpdateTenantPermissionGroupsResponse{}, nil
}

func (s *TenantPermissionServiceService) GetTenantEffectiveMenus(ctx context.Context, req *pbCore.GetTenantEffectiveMenusRequest) (*pbCore.GetTenantEffectiveMenusResponse, error) {
	if req.GetTenantId() == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	items, err := s.uc.GetTenantEffectiveMenus(ctx, req.GetTenantId(), req.GetParentId())
	if err != nil {
		return nil, err
	}
	return &pbCore.GetTenantEffectiveMenusResponse{Items: items}, nil
}

func (s *TenantPermissionServiceService) GetCurrentTenantEffectiveMenus(ctx context.Context, req *pbCore.GetCurrentTenantEffectiveMenusRequest) (*pbCore.GetTenantEffectiveMenusResponse, error) {
	tenantID := authn.GetAuthUserTenantID(ctx)
	if tenantID == 0 {
		return nil, pb.ErrorBadRequest("缺少当前租户上下文")
	}
	items, err := s.uc.GetTenantEffectiveMenus(ctx, tenantID, req.GetParentId())
	if err != nil {
		return nil, err
	}
	return &pbCore.GetTenantEffectiveMenusResponse{Items: items}, nil
}

func (s *TenantPermissionServiceService) GetCurrentTenantCapabilities(ctx context.Context, req *pbCore.GetCurrentTenantCapabilitiesRequest) (*pbCore.GetCurrentTenantCapabilitiesResponse, error) {
	tenantID := authn.GetAuthUserTenantID(ctx)
	if tenantID == 0 {
		return nil, pb.ErrorBadRequest("缺少当前租户上下文")
	}
	return s.uc.GetTenantCapabilities(ctx, tenantID)
}

func (s *TenantPermissionServiceService) UpdateTenantPermissionGroupVersion(ctx context.Context, req *pbCore.UpdateTenantPermissionGroupVersionRequest) (*pbCore.UpdateTenantPermissionGroupVersionResponse, error) {
	binding, err := s.uc.UpdateTenantGroupVersion(
		ctx,
		req.GetTenantId(),
		req.GetGroupId(),
		req.GetVersionId(),
		req.GetAutoUpgrade(),
		req.GetOperatorId(),
	)
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateTenantPermissionGroupVersionResponse{Binding: binding}, nil
}
