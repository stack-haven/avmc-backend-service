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

// MenuPermissionGroupServiceService manages reusable tenant menu permission groups.
type MenuPermissionGroupServiceService struct {
	pb.UnimplementedMenuPermissionGroupServiceServer
	uc  *biz.MenuPermissionGroupUsecase
	log *log.Helper
}

func NewMenuPermissionGroupServiceService(uc *biz.MenuPermissionGroupUsecase, logger log.Logger) *MenuPermissionGroupServiceService {
	return &MenuPermissionGroupServiceService{uc: uc, log: log.NewHelper(logger)}
}

func (s *MenuPermissionGroupServiceService) ListMenuPermissionGroups(ctx context.Context, req *pbCore.ListMenuPermissionGroupsRequest) (*pbCore.ListMenuPermissionGroupsResponse, error) {
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
	resp := pbCore.ListMenuPermissionGroupsResponse{Total: count}
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

func (s *MenuPermissionGroupServiceService) GetMenuPermissionGroup(ctx context.Context, req *pbCore.GetMenuPermissionGroupRequest) (*pbCore.MenuPermissionGroup, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorBadRequest("权限组ID不能为空")
	}
	return s.uc.Get(ctx, req.GetId())
}

func (s *MenuPermissionGroupServiceService) CreateMenuPermissionGroup(ctx context.Context, req *pbCore.CreateMenuPermissionGroupRequest) (*pbCore.CreateMenuPermissionGroupResponse, error) {
	if req.GetGroup() == nil {
		return nil, pb.ErrorBadRequest("权限组信息不能为空")
	}
	group, err := s.uc.Create(ctx, req.GetGroup())
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateMenuPermissionGroupResponse{Id: group.GetId()}, nil
}

func (s *MenuPermissionGroupServiceService) UpdateMenuPermissionGroup(ctx context.Context, req *pbCore.UpdateMenuPermissionGroupRequest) (*pbCore.UpdateMenuPermissionGroupResponse, error) {
	if req.GetId() == 0 || req.GetGroup() == nil {
		return nil, pb.ErrorBadRequest("权限组ID和信息不能为空")
	}
	req.Group.Id = req.GetId()
	if _, err := s.uc.Update(ctx, req.GetGroup()); err != nil {
		return nil, err
	}
	return &pbCore.UpdateMenuPermissionGroupResponse{}, nil
}

func (s *MenuPermissionGroupServiceService) DeleteMenuPermissionGroup(ctx context.Context, req *pbCore.DeleteMenuPermissionGroupRequest) (*pbCore.DeleteMenuPermissionGroupResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorBadRequest("权限组ID不能为空")
	}
	if err := s.uc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.DeleteMenuPermissionGroupResponse{}, nil
}

func (s *MenuPermissionGroupServiceService) UpdateMenuPermissionGroupStatus(ctx context.Context, req *pbCore.UpdateMenuPermissionGroupStatusRequest) (*pbCore.UpdateMenuPermissionGroupStatusResponse, error) {
	if req.GetId() == 0 {
		return nil, pb.ErrorBadRequest("权限组ID不能为空")
	}
	if req.GetStatus() == pbEnum.Status_STATUS_UNSPECIFIED {
		return nil, pb.ErrorBadRequest("权限组状态不能为空")
	}
	if _, err := s.uc.UpdateStatus(ctx, req.GetId(), req.GetStatus()); err != nil {
		return nil, err
	}
	return &pbCore.UpdateMenuPermissionGroupStatusResponse{}, nil
}

func (s *MenuPermissionGroupServiceService) ListMenuPermissionGroupVersions(ctx context.Context, req *pbCore.ListMenuPermissionGroupVersionsRequest) (*pbCore.ListMenuPermissionGroupVersionsResponse, error) {
	items, err := s.uc.ListVersions(ctx, req.GetGroupId())
	if err != nil {
		return nil, err
	}
	return &pbCore.ListMenuPermissionGroupVersionsResponse{Items: items}, nil
}

func (s *MenuPermissionGroupServiceService) PublishMenuPermissionGroupVersion(ctx context.Context, req *pbCore.PublishMenuPermissionGroupVersionRequest) (*pbCore.PublishMenuPermissionGroupVersionResponse, error) {
	version, err := s.uc.PublishVersion(ctx, req.GetGroupId(), req.GetMenuIds(), req.GetChangeSummary(), req.GetOperatorId(), req.GetEffectiveAt())
	if err != nil {
		return nil, err
	}
	return &pbCore.PublishMenuPermissionGroupVersionResponse{Version: version}, nil
}

func (s *MenuPermissionGroupServiceService) RollbackMenuPermissionGroupVersion(ctx context.Context, req *pbCore.RollbackMenuPermissionGroupVersionRequest) (*pbCore.RollbackMenuPermissionGroupVersionResponse, error) {
	version, err := s.uc.RollbackVersion(ctx, req.GetGroupId(), req.GetSourceVersionId(), req.GetChangeSummary(), req.GetOperatorId())
	if err != nil {
		return nil, err
	}
	return &pbCore.RollbackMenuPermissionGroupVersionResponse{Version: version}, nil
}
