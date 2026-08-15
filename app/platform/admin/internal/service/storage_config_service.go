package service

import (
	"context"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"

	"github.com/go-kratos/kratos/v2/errors"
	"go.einride.tech/aip/filtering"
	"google.golang.org/protobuf/proto"
)

type StorageConfigService struct {
	pb.UnimplementedStorageConfigServiceServer
	uc *biz.StorageConfigUsecase
}

func NewStorageConfigService(uc *biz.StorageConfigUsecase) *StorageConfigService {
	return &StorageConfigService{uc: uc}
}

func (s *StorageConfigService) CreateStorageConfig(ctx context.Context, req *pbCore.CreateStorageConfigRequest) (*pbCore.CreateStorageConfigResponse, error) {
	g := &pbCore.StorageConfig{
		Name:       req.Name,
		Provider:   req.Provider,
		Purpose:    req.Purpose,
		Bucket:     req.Bucket,
		IsDefault:  req.IsDefault,
		ConfigJson: req.ConfigJson,
	}
	cfg, err := s.uc.Create(ctx, g)
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateStorageConfigResponse{StorageConfig: cfg}, nil
}

func (s *StorageConfigService) UpdateStorageConfig(ctx context.Context, req *pbCore.UpdateStorageConfigRequest) (*pbCore.UpdateStorageConfigResponse, error) {
	g := &pbCore.StorageConfig{
		Id:         req.GetId(),
		Name:       req.Name,
		Provider:   req.Provider,
		Purpose:    req.Purpose,
		Bucket:     req.Bucket,
		IsDefault:  req.IsDefault,
		ConfigJson: req.ConfigJson,
	}
	cfg, err := s.uc.Update(ctx, g)
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateStorageConfigResponse{StorageConfig: cfg}, nil
}

func (s *StorageConfigService) DeleteStorageConfig(ctx context.Context, req *pbCore.DeleteStorageConfigRequest) (*pbCore.DeleteStorageConfigResponse, error) {
	if err := s.uc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.DeleteStorageConfigResponse{}, nil
}

func (s *StorageConfigService) GetStorageConfig(ctx context.Context, req *pbCore.GetStorageConfigRequest) (*pbCore.GetStorageConfigResponse, error) {
	cfg, err := s.uc.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pbCore.GetStorageConfigResponse{StorageConfig: cfg}, nil
}

func (s *StorageConfigService) ListStorageConfigs(ctx context.Context, req *pbCore.ListStorageConfigsRequest) (*pbCore.ListStorageConfigsResponse, error) {
	params, err := listing.ParseParams(
		req,
		filtering.DeclareIdent("name", filtering.TypeString),
		filtering.DeclareIdent("provider", filtering.TypeString),
		filtering.DeclareIdent("created_at", filtering.TypeTimestamp),
	)
	if err != nil {
		return nil, err
	}
	req.PageSize = (proto.Int32(int32(params.PageSize)))

	count, err := s.uc.CountConfigs(ctx, listing.FilterOption(params.Filter))
	if err != nil {
		return nil, err
	}

	resp := &pbCore.ListStorageConfigsResponse{Total: count}
	resp.Items, err = s.uc.ListConfigs(ctx,
		listing.FilterOption(params.Filter),
		listing.OrderByOption(params.OrderBy),
		listing.LimitOption(params.PageSize),
		listing.OffsetOption(int(params.PageToken.Offset)),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) >= params.PageSize {
		resp.NextPageToken = convert.ToPointer(params.PageToken.Next(req).String())
	}
	return resp, nil
}

func (s *StorageConfigService) SetDefaultStorageConfig(ctx context.Context, req *pbCore.SetDefaultStorageConfigRequest) (*pbCore.SetDefaultStorageConfigResponse, error) {
	tenantID := authn.GetAuthUserTenantID(ctx)
	if err := s.uc.SetDefault(ctx, tenantID, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.SetDefaultStorageConfigResponse{}, nil
}

func (s *StorageConfigService) TestStorageConfig(ctx context.Context, req *pbCore.TestStorageConfigRequest) (*pbCore.TestStorageConfigResponse, error) {
	if req.GetId() > 0 {
		if _, err := s.uc.Get(ctx, req.GetId()); err != nil {
			return nil, err
		}
		return &pbCore.TestStorageConfigResponse{Healthy: true, Message: "ok"}, nil
	}
	return nil, errors.BadRequest("TEST_STORAGE_CONFIG_INVALID", "请提供存储配置 ID")
}
