package service

import (
	"context"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/emptypb"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
)

type NotificationServiceService struct {
	pb.UnimplementedNotificationServiceServer
	uc  *biz.NotificationUsecase
	log *log.Helper
}

func NewNotificationServiceService(uc *biz.NotificationUsecase, logger log.Logger) *NotificationServiceService {
	return &NotificationServiceService{uc: uc, log: log.NewHelper(logger)}
}

func (s *NotificationServiceService) ListNotificationTemplates(ctx context.Context, req *pbCore.ListNotificationTemplatesRequest) (*pbCore.ListNotificationTemplatesResponse, error) {
	items, total, err := s.uc.ListTemplates(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &pbCore.ListNotificationTemplatesResponse{Items: items, Total: total}
	offset, err := strconv.Atoi(req.GetPageToken())
	if err != nil {
		offset = 0
	}
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp, nil
}

func (s *NotificationServiceService) GetNotificationTemplate(ctx context.Context, req *pbCore.GetNotificationTemplateRequest) (*pbCore.NotificationTemplate, error) {
	return s.uc.GetTemplate(ctx, req.GetId())
}

func (s *NotificationServiceService) CreateNotificationTemplate(ctx context.Context, req *pbCore.CreateNotificationTemplateRequest) (*pbCore.NotificationTemplate, error) {
	return s.uc.CreateTemplate(ctx, req.GetTemplate())
}

func (s *NotificationServiceService) UpdateNotificationTemplate(ctx context.Context, req *pbCore.UpdateNotificationTemplateRequest) (*pbCore.NotificationTemplate, error) {
	req.Template.Id = req.GetId()
	return s.uc.UpdateTemplate(ctx, req.GetTemplate())
}

func (s *NotificationServiceService) DeleteNotificationTemplate(ctx context.Context, req *pbCore.DeleteNotificationTemplateRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteTemplate(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *NotificationServiceService) SendInAppNotification(ctx context.Context, req *pbCore.SendInAppNotificationRequest) (*pbCore.SendInAppNotificationResponse, error) {
	return s.uc.SendInApp(ctx, req)
}

func (s *NotificationServiceService) ListNotificationMessages(ctx context.Context, req *pbCore.ListNotificationMessagesRequest) (*pbCore.ListNotificationMessagesResponse, error) {
	items, total, err := s.uc.ListMessages(ctx, req)
	if err != nil {
		return nil, err
	}
	return notificationMessageListResponse(req, items, total), nil
}

func (s *NotificationServiceService) GetNotificationMessage(ctx context.Context, req *pbCore.GetNotificationMessageRequest) (*pbCore.NotificationMessage, error) {
	return s.uc.GetMessage(ctx, req.GetId())
}

func (s *NotificationServiceService) ListMyNotifications(ctx context.Context, req *pbCore.ListNotificationMessagesRequest) (*pbCore.ListNotificationMessagesResponse, error) {
	items, total, err := s.uc.ListMyNotifications(ctx, req)
	if err != nil {
		return nil, err
	}
	return notificationMessageListResponse(req, items, total), nil
}

func (s *NotificationServiceService) CountMyUnreadNotifications(ctx context.Context, _ *pbCore.CountMyUnreadNotificationsRequest) (*pbCore.CountMyUnreadNotificationsResponse, error) {
	total, err := s.uc.CountMyUnread(ctx)
	if err != nil {
		return nil, err
	}
	return &pbCore.CountMyUnreadNotificationsResponse{Total: total}, nil
}

func (s *NotificationServiceService) MarkNotificationRead(ctx context.Context, req *pbCore.MarkNotificationReadRequest) (*emptypb.Empty, error) {
	if err := s.uc.MarkMyRead(ctx, []uint32{req.GetId()}); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *NotificationServiceService) MarkNotificationsRead(ctx context.Context, req *pbCore.MarkNotificationsReadRequest) (*emptypb.Empty, error) {
	if err := s.uc.MarkMyRead(ctx, req.GetIds()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func notificationMessageListResponse(req *pbCore.ListNotificationMessagesRequest, items []*pbCore.NotificationMessage, total int32) *pbCore.ListNotificationMessagesResponse {
	resp := &pbCore.ListNotificationMessagesResponse{Items: items, Total: total}
	if req == nil {
		return resp
	}
	offset, err := strconv.Atoi(req.GetPageToken())
	if err != nil {
		offset = 0
	}
	if offset+len(items) < int(total) {
		resp.NextPageToken = strconv.Itoa(offset + len(items))
	}
	return resp
}
