package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
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

func (s *TenantServiceService) ListTenants(ctx context.Context, req *pb.ListTenantsRequest) (*pb.ListTenantsResponse, error) {
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
	resp := &pb.ListTenantsResponse{Total: count}
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

func (s *TenantServiceService) ListTenantSimples(ctx context.Context, req *pb.ListTenantSimplesRequest) (*pb.ListTenantSimplesResponse, error) {
	opts := []listing.Option{listing.LimitOption(int(req.GetPageSize()))}
	if req.GetName() != "" {
		opts = append(opts, listing.NameOption(req.GetName()))
	}
	tenants, err := s.uc.List(ctx, opts...)
	if err != nil {
		return nil, err
	}
	items := make([]*pb.TenantSimple, 0, len(tenants))
	for _, t := range tenants {
		items = append(items, &pb.TenantSimple{
			Id:   t.GetId(),
			Name: t.GetName(),
			Code: t.GetCode(),
		})
	}
	return &pb.ListTenantSimplesResponse{Items: items}, nil
}

func (s *TenantServiceService) GetTenant(ctx context.Context, req *pb.GetTenantRequest) (*pb.Tenant, error) {
	return s.uc.Get(ctx, req.GetId())
}

func (s *TenantServiceService) CreateTenant(ctx context.Context, req *pb.CreateTenantRequest) (*pb.CreateTenantResponse, error) {
	result, err := s.uc.Create(ctx, req.GetTenant(), req.GetInitialAdmin(), req.GetOperatorId())
	if err != nil {
		return nil, err
	}
	return &pb.CreateTenantResponse{
		Id:          result.Tenant.GetId(),
		Name:        result.Tenant.GetName(),
		AdminUserId: result.AdminUserID,
		AdminRoleId: result.AdminRoleID,
		RootDeptId:  result.RootDeptID,
	}, nil
}

func (s *TenantServiceService) UpdateTenant(ctx context.Context, req *pb.UpdateTenantRequest) (*pb.UpdateTenantResponse, error) {
	req.Tenant.Id = req.GetId()
	if _, err := s.uc.Update(ctx, req.GetTenant(), req.GetOperatorId()); err != nil {
		return nil, err
	}
	return &pb.UpdateTenantResponse{Id: req.GetId()}, nil
}

func (s *TenantServiceService) DeleteTenant(ctx context.Context, req *pb.DeleteTenantRequest) (*pb.DeleteTenantResponse, error) {
	if err := s.uc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeleteTenantResponse{Id: req.GetId()}, nil
}

func (s *TenantServiceService) UpdateTenantLifecycle(ctx context.Context, req *pb.UpdateTenantLifecycleRequest) (*pb.UpdateTenantLifecycleResponse, error) {
	tenant, err := s.uc.UpdateLifecycle(ctx, req.GetId(), req.GetLifecycleStatus())
	if err != nil {
		return nil, err
	}
	return &pb.UpdateTenantLifecycleResponse{Tenant: tenant}, nil
}

func (s *TenantServiceService) ListTenantAdmins(ctx context.Context, req *pb.ListTenantAdminsRequest) (*pb.ListTenantAdminsResponse, error) {
	items, err := s.uc.ListAdmins(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	return &pb.ListTenantAdminsResponse{Items: items}, nil
}

func (s *TenantServiceService) UpdateTenantAdmin(ctx context.Context, req *pb.UpdateTenantAdminRequest) (*pb.UpdateTenantAdminResponse, error) {
	admin, err := s.uc.UpdateAdmin(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateTenantAdminResponse{Admin: admin}, nil
}

func (s *TenantServiceService) ResetTenantAdminPassword(ctx context.Context, req *pb.ResetTenantAdminPasswordRequest) (*pb.ResetTenantAdminPasswordResponse, error) {
	if err := s.uc.ResetAdminPassword(ctx, req); err != nil {
		return nil, err
	}
	return &pb.ResetTenantAdminPasswordResponse{}, nil
}
