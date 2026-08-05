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

// TenantMenuPermissionGroupServiceService manages reusable tenant menu permission groups.
type TenantMenuPermissionGroupServiceService struct {
	pb.UnimplementedTenantMenuPermissionGroupServiceServer
	uc  *biz.TenantMenuPermissionGroupUsecase
	log *log.Helper
}

func NewTenantMenuPermissionGroupServiceService(uc *biz.TenantMenuPermissionGroupUsecase, logger log.Logger) *TenantMenuPermissionGroupServiceService {
	return &TenantMenuPermissionGroupServiceService{uc: uc, log: log.NewHelper(logger)}
}

func (s *TenantMenuPermissionGroupServiceService) ListTenantMenuPermissionGroups(ctx context.Context, req *pbCore.ListTenantMenuPermissionGroupsRequest) (*pbCore.ListTenantMenuPermissionGroupsResponse, error) {
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
	resp := pbCore.ListTenantMenuPermissionGroupsResponse{Total: count}
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

func (s *TenantMenuPermissionGroupServiceService) GetTenantMenuPermissionGroup(ctx context.Context, req *pbCore.GetTenantMenuPermissionGroupRequest) (*pbCore.TenantMenuPermissionGroup, error) {
	return s.uc.Get(ctx, req.GetId())
}

func (s *TenantMenuPermissionGroupServiceService) CreateTenantMenuPermissionGroup(ctx context.Context, req *pbCore.CreateTenantMenuPermissionGroupRequest) (*pbCore.CreateTenantMenuPermissionGroupResponse, error) {
	group, err := s.uc.Create(ctx, req.GetGroup())
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateTenantMenuPermissionGroupResponse{Id: group.GetId()}, nil
}

func (s *TenantMenuPermissionGroupServiceService) UpdateTenantMenuPermissionGroup(ctx context.Context, req *pbCore.UpdateTenantMenuPermissionGroupRequest) (*pbCore.UpdateTenantMenuPermissionGroupResponse, error) {
	req.Group.Id = req.GetId()
	if _, err := s.uc.Update(ctx, req.GetGroup()); err != nil {
		return nil, err
	}
	return &pbCore.UpdateTenantMenuPermissionGroupResponse{}, nil
}

func (s *TenantMenuPermissionGroupServiceService) DeleteTenantMenuPermissionGroup(ctx context.Context, req *pbCore.DeleteTenantMenuPermissionGroupRequest) (*pbCore.DeleteTenantMenuPermissionGroupResponse, error) {
	if err := s.uc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.DeleteTenantMenuPermissionGroupResponse{}, nil
}

func (s *TenantMenuPermissionGroupServiceService) UpdateTenantMenuPermissionGroupStatus(ctx context.Context, req *pbCore.UpdateTenantMenuPermissionGroupStatusRequest) (*pbCore.UpdateTenantMenuPermissionGroupStatusResponse, error) {
	if _, err := s.uc.UpdateStatus(ctx, req.GetId(), req.GetStatus()); err != nil {
		return nil, err
	}
	return &pbCore.UpdateTenantMenuPermissionGroupStatusResponse{}, nil
}

func (s *TenantMenuPermissionGroupServiceService) ListTenantMenuPermissionGroupVersions(ctx context.Context, req *pbCore.ListTenantMenuPermissionGroupVersionsRequest) (*pbCore.ListTenantMenuPermissionGroupVersionsResponse, error) {
	items, err := s.uc.ListVersions(ctx, req.GetGroupId())
	if err != nil {
		return nil, err
	}
	return &pbCore.ListTenantMenuPermissionGroupVersionsResponse{Items: items}, nil
}

func (s *TenantMenuPermissionGroupServiceService) PublishTenantMenuPermissionGroupVersion(ctx context.Context, req *pbCore.PublishTenantMenuPermissionGroupVersionRequest) (*pbCore.PublishTenantMenuPermissionGroupVersionResponse, error) {
	version, err := s.uc.PublishVersion(ctx, req.GetGroupId(), &pbCore.TenantMenuPermissionGroupVersion{
		MenuIds:        req.GetMenuIds(),
		ApiPermissions: req.GetApiPermissions(),
		FeatureFlags:   req.GetFeatureFlags(),
		ResourceQuotas: req.GetResourceQuotas(),
	}, req.GetChangeSummary(), req.GetOperatorId(), req.GetEffectiveAt())
	if err != nil {
		return nil, err
	}
	return &pbCore.PublishTenantMenuPermissionGroupVersionResponse{Version: version}, nil
}

func (s *TenantMenuPermissionGroupServiceService) RollbackTenantMenuPermissionGroupVersion(ctx context.Context, req *pbCore.RollbackTenantMenuPermissionGroupVersionRequest) (*pbCore.RollbackTenantMenuPermissionGroupVersionResponse, error) {
	version, err := s.uc.RollbackVersion(ctx, req.GetGroupId(), req.GetSourceVersionId(), req.GetChangeSummary(), req.GetOperatorId())
	if err != nil {
		return nil, err
	}
	return &pbCore.RollbackTenantMenuPermissionGroupVersionResponse{Version: version}, nil
}
