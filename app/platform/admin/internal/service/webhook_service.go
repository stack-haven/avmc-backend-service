package service

import (
	"context"

	pb "backend-service/api/core/service/v1"
	v1 "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"

	"google.golang.org/protobuf/types/known/emptypb"
)

type WebhookServiceService struct {
	v1.UnimplementedWebhookServiceServer
	uc *biz.WebhookUsecase
}

func NewWebhookService(uc *biz.WebhookUsecase) *WebhookServiceService {
	return &WebhookServiceService{uc: uc}
}

// ---------------------------------------------------------------------------
// Subscription management
// ---------------------------------------------------------------------------

func (s *WebhookServiceService) ListWebhookSubscriptions(ctx context.Context, req *pb.ListWebhookSubscriptionsRequest) (*pb.ListWebhookSubscriptionsResponse, error) {
	items, total, err := s.uc.ListSubscriptions(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListWebhookSubscriptionsResponse{Items: items, Total: total}, nil
}

func (s *WebhookServiceService) GetWebhookSubscription(ctx context.Context, req *pb.GetWebhookSubscriptionRequest) (*pb.WebhookSubscription, error) {
	return s.uc.GetSubscription(ctx, req.GetId())
}

func (s *WebhookServiceService) CreateWebhookSubscription(ctx context.Context, req *pb.CreateWebhookSubscriptionRequest) (*pb.WebhookSubscription, error) {
	return s.uc.CreateSubscription(ctx, req.GetSubscription())
}

func (s *WebhookServiceService) UpdateWebhookSubscription(ctx context.Context, req *pb.UpdateWebhookSubscriptionRequest) (*pb.WebhookSubscription, error) {
	req.GetSubscription().Id = req.GetId()
	return s.uc.UpdateSubscription(ctx, req.GetSubscription())
}

func (s *WebhookServiceService) DeleteWebhookSubscription(ctx context.Context, req *pb.DeleteWebhookSubscriptionRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, s.uc.DeleteSubscription(ctx, req.GetId())
}

// ---------------------------------------------------------------------------
// Delivery log management
// ---------------------------------------------------------------------------

func (s *WebhookServiceService) ListWebhookDeliveryLogs(ctx context.Context, req *pb.ListWebhookDeliveryLogsRequest) (*pb.ListWebhookDeliveryLogsResponse, error) {
	items, total, err := s.uc.ListDeliveryLogs(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListWebhookDeliveryLogsResponse{Items: items, Total: total}, nil
}

func (s *WebhookServiceService) GetWebhookDeliveryLog(ctx context.Context, req *pb.GetWebhookDeliveryLogRequest) (*pb.WebhookDeliveryLog, error) {
	return s.uc.GetDeliveryLog(ctx, req.GetId())
}

func (s *WebhookServiceService) RetryWebhookDelivery(ctx context.Context, req *pb.RetryWebhookDeliveryRequest) (*pb.RetryWebhookDeliveryResponse, error) {
	return s.uc.RetryDelivery(ctx, req.GetId())
}
