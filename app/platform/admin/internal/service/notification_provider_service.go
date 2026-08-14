package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
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

func (s *NotificationProviderServiceService) CreateNotificationProvider(ctx context.Context, req *pbCore.CreateNotificationProviderRequest) (*pbCore.CreateNotificationProviderResponse, error) {
	provider, err := s.uc.Create(ctx, req.GetProvider())
	if err != nil {
		return nil, err
	}
	return &pbCore.CreateNotificationProviderResponse{Provider: provider}, nil
}

func (s *NotificationProviderServiceService) UpdateNotificationProvider(ctx context.Context, req *pbCore.UpdateNotificationProviderRequest) (*pbCore.UpdateNotificationProviderResponse, error) {
	provider, err := s.uc.Update(ctx, req.GetId(), req.GetProvider())
	if err != nil {
		return nil, err
	}
	return &pbCore.UpdateNotificationProviderResponse{Provider: provider}, nil
}

func (s *NotificationProviderServiceService) DeleteNotificationProvider(ctx context.Context, req *pbCore.DeleteNotificationProviderRequest) (*pbCore.DeleteNotificationProviderResponse, error) {
	if err := s.uc.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pbCore.DeleteNotificationProviderResponse{}, nil
}

func (s *NotificationProviderServiceService) GetNotificationProvider(ctx context.Context, req *pbCore.GetNotificationProviderRequest) (*pbCore.NotificationProvider, error) {
	return s.uc.Get(ctx, req.GetId())
}

func (s *NotificationProviderServiceService) ListNotificationProviders(ctx context.Context, req *pbCore.ListNotificationProvidersRequest) (*pbCore.ListNotificationProvidersResponse, error) {
	items, total, err := s.uc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pbCore.ListNotificationProvidersResponse{Items: items, Total: total}, nil
}

func (s *NotificationProviderServiceService) SetDefaultNotificationProvider(ctx context.Context, req *pbCore.SetDefaultNotificationProviderRequest) (*pbCore.SetDefaultNotificationProviderResponse, error) {
	provider, err := s.uc.SetDefault(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &pbCore.SetDefaultNotificationProviderResponse{Provider: provider}, nil
}

func (s *NotificationProviderServiceService) TestNotificationProvider(ctx context.Context, req *pbCore.TestNotificationProviderRequest) (*pbCore.TestNotificationProviderResponse, error) {
	return s.uc.Test(ctx, req.GetId(), req.GetPhone())
}
