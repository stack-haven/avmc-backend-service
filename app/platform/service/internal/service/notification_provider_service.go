package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
)

// NotificationProviderServiceService 通知渠道配置服务。
type NotificationProviderServiceService struct {
	pb.UnimplementedNotificationProviderServiceServer
	uc  *biz.NotificationProviderUsecase
	log *log.Helper
}

func NewNotificationProviderServiceService(uc *biz.NotificationProviderUsecase, logger log.Logger) *NotificationProviderServiceService {
	return &NotificationProviderServiceService{uc: uc, log: log.NewHelper(logger)}
}

func (s *NotificationProviderServiceService) CreateNotificationProvider(ctx context.Context, req *pb.CreateNotificationProviderRequest) (*pb.CreateNotificationProviderResponse, error) {
	provider, err := s.uc.Create(ctx, req.GetProvider())
	if err != nil {
		return nil, err
	}
	return &pb.CreateNotificationProviderResponse{Provider: provider}, nil
}

func (s *NotificationProviderServiceService) UpdateNotificationProvider(ctx context.Context, req *pb.UpdateNotificationProviderRequest) (*pb.UpdateNotificationProviderResponse, error) {
	provider, err := s.uc.Update(ctx, req.GetId(), req.GetProvider())
	if err != nil {
		return nil, err
	}
	return &pb.UpdateNotificationProviderResponse{Provider: provider}, nil
}

func (s *NotificationProviderServiceService) DeleteNotificationProvider(ctx context.Context, req *pb.DeleteNotificationProviderRequest) (*pb.DeleteNotificationProviderResponse, error) {
	if err := s.uc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeleteNotificationProviderResponse{}, nil
}

func (s *NotificationProviderServiceService) GetNotificationProvider(ctx context.Context, req *pb.GetNotificationProviderRequest) (*pb.NotificationProvider, error) {
	return s.uc.Get(ctx, req.GetId())
}

func (s *NotificationProviderServiceService) ListNotificationProviders(ctx context.Context, req *pb.ListNotificationProvidersRequest) (*pb.ListNotificationProvidersResponse, error) {
	items, total, err := s.uc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListNotificationProvidersResponse{Items: items, Total: total}, nil
}

func (s *NotificationProviderServiceService) SetDefaultNotificationProvider(ctx context.Context, req *pb.SetDefaultNotificationProviderRequest) (*pb.SetDefaultNotificationProviderResponse, error) {
	provider, err := s.uc.SetDefault(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pb.SetDefaultNotificationProviderResponse{Provider: provider}, nil
}

func (s *NotificationProviderServiceService) TestNotificationProvider(ctx context.Context, req *pb.TestNotificationProviderRequest) (*pb.TestNotificationProviderResponse, error) {
	return s.uc.Test(ctx, req.GetId(), req.GetPhone())
}
