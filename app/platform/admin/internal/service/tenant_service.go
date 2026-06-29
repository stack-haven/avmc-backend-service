package service

import (
	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/pkg/aip/listing"
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"
)

// TenantServiceService manages platform tenants.
type TenantServiceService struct {
	pb.UnimplementedTenantServiceServer
	uc  *biz.TenantUsecase
	log *log.Helper
}

func NewTenantServiceService(uc *biz.TenantUsecase, logger log.Logger) *TenantServiceService {
	return &TenantServiceService{uc: uc, log: log.NewHelper(logger)}
}

func (s *TenantServiceService) ListTenants(ctx context.Context, req *pbCore.ListTenantsRequest) (*pbCore.ListTenantsResponse, error) {
	params, err := listing.ParseParams(
		req,
		filtering.DeclareIdent("name", filtering.TypeString),
		filtering.DeclareIdent("code", filtering.TypeString),
		filtering.DeclareIdent("status", filtering.TypeInt),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	req.PageSize = int32(params.PageSize)
	count, err := s.uc.Count(ctx, listing.FilterOption(params.Filter))
	if err != nil {
		return nil, err
	}
	resp := pbCore.ListTenantsResponse{Total: count}
	resp.Items, err = s.uc.List(ctx,
		listing.FilterOption(params.Filter),
		listing.OrderByOption(params.OrderBy),
		listing.LimitOption(params.PageSize),
		listing.OffsetOption(int(params.PageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) >= params.PageSize {
		resp.NextPageToken = params.PageToken.Next(req).String()
	}
	return &resp, nil
}

func (s *TenantServiceService) GetTenant(ctx context.Context, req *pbCore.GetTenantRequest) (*pbCore.Tenant, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	return s.uc.Get(ctx, req.GetId())
}

func (s *TenantServiceService) CreateTenant(ctx context.Context, req *pbCore.CreateTenantRequest) (*pbCore.CreateTenantResponse, error) {
	if req.GetTenant() == nil {
		return nil, pb.ErrorBadRequest("租户信息不能为空")
	}
	result, err := s.uc.Create(ctx, req.GetTenant(), req.GetInitialAdmin(), req.GetOperatorId())
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateTenantResponse{
		Id:          result.Tenant.GetId(),
		Name:        result.Tenant.GetName(),
		AdminUserId: result.AdminUserID,
		AdminRoleId: result.AdminRoleID,
		RootDeptId:  result.RootDeptID,
	}, nil
}

func (s *TenantServiceService) UpdateTenantLifecycle(ctx context.Context, req *pbCore.UpdateTenantLifecycleRequest) (*pbCore.UpdateTenantLifecycleResponse, error) {
	if req.GetId() == 0 || req.GetLifecycleStatus() == pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_UNSPECIFIED {
		return nil, pb.ErrorBadRequest("租户ID和生命周期状态不能为空")
	}
	tenant, err := s.uc.UpdateLifecycle(ctx, req.GetId(), req.GetLifecycleStatus())
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateTenantLifecycleResponse{Tenant: tenant}, nil
}

func (s *TenantServiceService) UpdateTenant(ctx context.Context, req *pbCore.UpdateTenantRequest) (*pbCore.UpdateTenantResponse, error) {
	if req.GetId() == 0 || req.GetTenant() == nil {
		return nil, pb.ErrorBadRequest("租户ID和信息不能为空")
	}
	req.Tenant.Id = req.GetId()
	if _, err := s.uc.Update(ctx, req.GetTenant(), req.GetOperatorId()); err != nil {
		return nil, err
	}
	return &pbCore.UpdateTenantResponse{Id: req.GetId()}, nil
}

func (s *TenantServiceService) DeleteTenant(ctx context.Context, req *pbCore.DeleteTenantRequest) (*pbCore.DeleteTenantResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	if err := s.uc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.DeleteTenantResponse{Id: req.GetId()}, nil
}

func (s *TenantServiceService) UpdateTenantStatus(ctx context.Context, req *pbCore.UpdateTenantStatusRequest) (*pbCore.UpdateTenantStatusResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	if req.GetStatus() == pbEnum.Status_STATUS_UNSPECIFIED {
		return nil, pb.ErrorBadRequest("租户状态不能为空")
	}
	if _, err := s.uc.UpdateStatus(ctx, req.GetId(), req.GetStatus()); err != nil {
		return nil, err
	}
	return &pbCore.UpdateTenantStatusResponse{}, nil
}
