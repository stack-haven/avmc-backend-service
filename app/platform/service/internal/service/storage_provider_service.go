package service

import (
	"context"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
)

type StorageProviderServiceService struct {
	pb.UnimplementedStorageProviderServiceServer
	uc *biz.StorageProviderUsecase
}

func NewStorageProviderServiceService(uc *biz.StorageProviderUsecase) *StorageProviderServiceService {
	return &StorageProviderServiceService{uc: uc}
}

func (s *StorageProviderServiceService) CreateStorageProvider(ctx context.Context, req *pb.CreateStorageProviderRequest) (*pb.CreateStorageProviderResponse, error) {
	item, err := s.uc.Create(ctx, req.GetProvider())
	if err != nil {
		return nil, err
	}
	return &pb.CreateStorageProviderResponse{Provider: item}, nil
}

func (s *StorageProviderServiceService) UpdateStorageProvider(ctx context.Context, req *pb.UpdateStorageProviderRequest) (*pb.UpdateStorageProviderResponse, error) {
	item, err := s.uc.Update(ctx, req.GetId(), req.GetProvider())
	if err != nil {
		return nil, err
	}
	return &pb.UpdateStorageProviderResponse{Provider: item}, nil
}

func (s *StorageProviderServiceService) DeleteStorageProvider(ctx context.Context, req *pb.DeleteStorageProviderRequest) (*pb.DeleteStorageProviderResponse, error) {
	if err := s.uc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeleteStorageProviderResponse{}, nil
}

func (s *StorageProviderServiceService) GetStorageProvider(ctx context.Context, req *pb.GetStorageProviderRequest) (*pb.StorageProvider, error) {
	return s.uc.Get(ctx, req.GetId())
}

func (s *StorageProviderServiceService) ListStorageProviders(ctx context.Context, req *pb.ListStorageProvidersRequest) (*pb.ListStorageProvidersResponse, error) {
	items, total, err := s.uc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListStorageProvidersResponse{Items: items, Total: total}, nil
}

func (s *StorageProviderServiceService) SetDefaultStorageProvider(ctx context.Context, req *pb.SetDefaultStorageProviderRequest) (*pb.SetDefaultStorageProviderResponse, error) {
	item, err := s.uc.SetDefault(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.SetDefaultStorageProviderResponse{Provider: item}, nil
}

func (s *StorageProviderServiceService) TestStorageProvider(ctx context.Context, req *pb.TestStorageProviderRequest) (*pb.TestStorageProviderResponse, error) {
	return s.uc.Test(ctx, req.GetId(), req.GetProvider())
}
