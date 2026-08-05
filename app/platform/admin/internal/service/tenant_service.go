package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/pkg/aip/listing"
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
		filtering.DeclareIdent("lifecycle_status", filtering.TypeInt),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}

	count, err := s.uc.Count(ctx, listing.FilterOption(params.Filter))
	if err != nil {
		return nil, err
	}
	resp := &pbCore.ListTenantsResponse{Total: count}
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
	return resp, nil
}

func (s *TenantServiceService) ListTenantSimples(ctx context.Context, req *pbCore.ListTenantSimplesRequest) (*pbCore.ListTenantSimplesResponse, error) {
	opts := []listing.Option{listing.LimitOption(int(req.GetPageSize()))}
	if req.GetName() != "" {
		opts = append(opts, listing.NameOption(req.GetName()))
	}
	tenants, err := s.uc.List(ctx, opts...)
	if err != nil {
		return nil, err
	}
	items := make([]*pbCore.TenantSimple, 0, len(tenants))
	for _, t := range tenants {
		items = append(items, &pbCore.TenantSimple{
			Id:   t.GetId(),
			Name: t.GetName(),
			Code: t.GetCode(),
		})
	}
	return &pbCore.ListTenantSimplesResponse{Items: items}, nil
}

func (s *TenantServiceService) GetTenant(ctx context.Context, req *pbCore.GetTenantRequest) (*pbCore.Tenant, error) {
	return s.uc.Get(ctx, req.GetId())
}

func (s *TenantServiceService) CreateTenant(ctx context.Context, req *pbCore.CreateTenantRequest) (*pbCore.CreateTenantResponse, error) {
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

func (s *TenantServiceService) UpdateTenant(ctx context.Context, req *pbCore.UpdateTenantRequest) (*pbCore.UpdateTenantResponse, error) {
	req.Tenant.Id = req.GetId()
	if _, err := s.uc.Update(ctx, req.GetTenant(), req.GetOperatorId()); err != nil {
		return nil, err
	}
	return &pbCore.UpdateTenantResponse{Id: req.GetId()}, nil
}

func (s *TenantServiceService) DeleteTenant(ctx context.Context, req *pbCore.DeleteTenantRequest) (*pbCore.DeleteTenantResponse, error) {
	if err := s.uc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.DeleteTenantResponse{Id: req.GetId()}, nil
}

func (s *TenantServiceService) UpdateTenantLifecycle(ctx context.Context, req *pbCore.UpdateTenantLifecycleRequest) (*pbCore.UpdateTenantLifecycleResponse, error) {
	tenant, err := s.uc.UpdateLifecycle(ctx, req.GetId(), req.GetLifecycleStatus())
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateTenantLifecycleResponse{Tenant: tenant}, nil
}

func (s *TenantServiceService) ListTenantAdmins(ctx context.Context, req *pbCore.ListTenantAdminsRequest) (*pbCore.ListTenantAdminsResponse, error) {
	items, err := s.uc.ListAdmins(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	return &pbCore.ListTenantAdminsResponse{Items: items}, nil
}

func (s *TenantServiceService) UpdateTenantAdmin(ctx context.Context, req *pbCore.UpdateTenantAdminRequest) (*pbCore.UpdateTenantAdminResponse, error) {
	admin, err := s.uc.UpdateAdmin(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateTenantAdminResponse{Admin: admin}, nil
}

func (s *TenantServiceService) ResetTenantAdminPassword(ctx context.Context, req *pbCore.ResetTenantAdminPasswordRequest) (*pbCore.ResetTenantAdminPasswordResponse, error) {
	if err := s.uc.ResetAdminPassword(ctx, req); err != nil {
		return nil, err
	}
	return &pbCore.ResetTenantAdminPasswordResponse{}, nil
}
