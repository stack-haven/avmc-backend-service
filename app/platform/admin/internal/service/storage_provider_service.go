package service

import (
	"context"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
)

type StorageProviderServiceService struct {
	pb.UnimplementedStorageProviderServiceServer
	uc *biz.StorageProviderUsecase
}

func NewStorageProviderServiceService(uc *biz.StorageProviderUsecase) *StorageProviderServiceService {
	return &StorageProviderServiceService{uc: uc}
}

func (s *StorageProviderServiceService) CreateStorageProvider(ctx context.Context, req *pbCore.CreateStorageProviderRequest) (*pbCore.CreateStorageProviderResponse, error) {
	item, err := s.uc.Create(ctx, req.GetProvider())
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateStorageProviderResponse{Provider: item}, nil
}

func (s *StorageProviderServiceService) UpdateStorageProvider(ctx context.Context, req *pbCore.UpdateStorageProviderRequest) (*pbCore.UpdateStorageProviderResponse, error) {
	item, err := s.uc.Update(ctx, req.GetId(), req.GetProvider())
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateStorageProviderResponse{Provider: item}, nil
}

func (s *StorageProviderServiceService) DeleteStorageProvider(ctx context.Context, req *pbCore.DeleteStorageProviderRequest) (*pbCore.DeleteStorageProviderResponse, error) {
	if err := s.uc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.DeleteStorageProviderResponse{}, nil
}

func (s *StorageProviderServiceService) GetStorageProvider(ctx context.Context, req *pbCore.GetStorageProviderRequest) (*pbCore.StorageProvider, error) {
	return s.uc.Get(ctx, req.GetId())
}

func (s *StorageProviderServiceService) ListStorageProviders(ctx context.Context, req *pbCore.ListStorageProvidersRequest) (*pbCore.ListStorageProvidersResponse, error) {
	items, total, err := s.uc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pbCore.ListStorageProvidersResponse{Items: items, Total: total}, nil
}

func (s *StorageProviderServiceService) SetDefaultStorageProvider(ctx context.Context, req *pbCore.SetDefaultStorageProviderRequest) (*pbCore.SetDefaultStorageProviderResponse, error) {
	item, err := s.uc.SetDefault(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pbCore.SetDefaultStorageProviderResponse{Provider: item}, nil
}

func (s *StorageProviderServiceService) TestStorageProvider(ctx context.Context, req *pbCore.TestStorageProviderRequest) (*pbCore.TestStorageProviderResponse, error) {
	return s.uc.Test(ctx, req.GetId(), req.GetProvider())
}
