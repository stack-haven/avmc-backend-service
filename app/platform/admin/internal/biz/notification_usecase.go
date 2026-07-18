package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"backend-service/api/common/enum"
	pb "backend-service/api/core/service/v1"
	"backend-service/pkg/auth/authn"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

const AsyncTaskTypeNotificationInApp = "notification.in_app.send"

type NotificationRepo interface {
	ListTemplates(context.Context, *pb.ListNotificationTemplatesRequest) ([]*pb.NotificationTemplate, int32, error)
	GetTemplate(context.Context, uint32) (*pb.NotificationTemplate, error)
	GetEnabledTemplateByCode(context.Context, uint32, string) (*pb.NotificationTemplate, error)
	CreateTemplate(context.Context, *pb.NotificationTemplate) (*pb.NotificationTemplate, error)
	UpdateTemplate(context.Context, *pb.NotificationTemplate) (*pb.NotificationTemplate, error)
	DeleteTemplate(context.Context, uint32) error
	CreateMessages(context.Context, []*pb.NotificationMessage) (int, error)
	ListMessages(context.Context, *pb.ListNotificationMessagesRequest) ([]*pb.NotificationMessage, int32, error)
	GetMessage(context.Context, uint32) (*pb.NotificationMessage, error)
	CountUnread(context.Context, uint32) (int32, error)
	MarkRead(context.Context, []uint32, uint32) error
}

type NotificationUsecase struct {
	repo  NotificationRepo
	tasks AsyncTaskRepo
	log   *log.Helper
}

func NewNotificationUsecase(repo NotificationRepo, tasks AsyncTaskRepo, logger log.Logger) *NotificationUsecase {
	return &NotificationUsecase{
		repo:  repo,
		tasks: tasks,
		log:   log.NewHelper(log.With(logger, "module", "biz/notification")),
	}
}

func (uc *NotificationUsecase) ListTemplates(ctx context.Context, req *pb.ListNotificationTemplatesRequest) ([]*pb.NotificationTemplate, int32, error) {
	return uc.repo.ListTemplates(ctx, req)
}

func (uc *NotificationUsecase) GetTemplate(ctx context.Context, id uint32) (*pb.NotificationTemplate, error) {
	return uc.repo.GetTemplate(ctx, id)
}

func (uc *NotificationUsecase) CreateTemplate(ctx context.Context, tpl *pb.NotificationTemplate) (*pb.NotificationTemplate, error) {
	if tpl.GetChannel() == pb.NotificationChannel_NOTIFICATION_CHANNEL_UNSPECIFIED {
		tpl.Channel = pb.NotificationChannel_NOTIFICATION_CHANNEL_IN_APP
	}
	if tpl.Status == nil || tpl.GetStatus() == enum.Status_STATUS_UNSPECIFIED {
		tpl.Status = enum.Status_STATUS_ENABLED.Enum()
	}
	return uc.repo.CreateTemplate(ctx, tpl)
}

func (uc *NotificationUsecase) UpdateTemplate(ctx context.Context, tpl *pb.NotificationTemplate) (*pb.NotificationTemplate, error) {
	if tpl.GetChannel() == pb.NotificationChannel_NOTIFICATION_CHANNEL_UNSPECIFIED {
		tpl.Channel = pb.NotificationChannel_NOTIFICATION_CHANNEL_IN_APP
	}
	if tpl.Status == nil || tpl.GetStatus() == enum.Status_STATUS_UNSPECIFIED {
		tpl.Status = enum.Status_STATUS_ENABLED.Enum()
	}
	return uc.repo.UpdateTemplate(ctx, tpl)
}

func (uc *NotificationUsecase) DeleteTemplate(ctx context.Context, id uint32) error {
	return uc.repo.DeleteTemplate(ctx, id)
}

func (uc *NotificationUsecase) SendInApp(ctx context.Context, req *pb.SendInAppNotificationRequest) (*pb.SendInAppNotificationResponse, error) {
	if req == nil || len(req.GetRecipientUserIds()) == 0 {
		return nil, errors.BadRequest("NOTIFICATION_RECIPIENT_REQUIRED", "通知接收人不能为空")
	}
	hasTemplate := strings.TrimSpace(req.GetTemplateCode()) != ""
	if !hasTemplate && (strings.TrimSpace(req.GetTitle()) == "" || strings.TrimSpace(req.GetContent()) == "") {
		return nil, errors.BadRequest("NOTIFICATION_CONTENT_REQUIRED", "未使用模板时通知标题和内容不能为空")
	}
	tenantID, err := currentTenantID(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(inAppNotificationPayload{
		TenantID:         tenantID,
		RecipientUserIDs: req.GetRecipientUserIds(),
		TemplateCode:     strings.TrimSpace(req.GetTemplateCode()),
		Title:            strings.TrimSpace(req.GetTitle()),
		Content:          strings.TrimSpace(req.GetContent()),
		Variables:        strings.TrimSpace(req.GetVariables()),
		Priority:         req.GetPriority(),
		BusinessType:     strings.TrimSpace(req.GetBusinessType()),
		BusinessID:       strings.TrimSpace(req.GetBusinessId()),
		SenderUserID:     authn.GetAuthUserID(ctx),
		SenderName:       currentUserName(ctx),
	})
	if err != nil {
		return nil, err
	}
	idempotencyKey := strings.TrimSpace(req.GetIdempotencyKey())
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("notification:in-app:%d:%s:%s:%d", tenantID, req.GetBusinessType(), req.GetBusinessId(), time.Now().UnixNano())
	}
	task, err := uc.tasks.Enqueue(ctx, &AsyncTaskSpec{
		TenantID:       &tenantID,
		TaskType:       AsyncTaskTypeNotificationInApp,
		Queue:          "notification",
		Priority:       req.GetPriority(),
		Payload:        payload,
		PayloadSummary: fmt.Sprintf("站内信接收人 %d 个", len(req.GetRecipientUserIds())),
		IdempotencyKey: idempotencyKey,
		MaxAttempts:    3,
		ScheduledAt:    time.Now(),
		CreatedBy:      notificationUint32Ptr(authn.GetAuthUserID(ctx)),
	})
	if err != nil {
		return nil, err
	}
	return &pb.SendInAppNotificationResponse{TaskId: task.GetId()}, nil
}

func (uc *NotificationUsecase) ListMessages(ctx context.Context, req *pb.ListNotificationMessagesRequest) ([]*pb.NotificationMessage, int32, error) {
	return uc.repo.ListMessages(ctx, req)
}

func (uc *NotificationUsecase) GetMessage(ctx context.Context, id uint32) (*pb.NotificationMessage, error) {
	return uc.repo.GetMessage(ctx, id)
}

func (uc *NotificationUsecase) ListMyNotifications(ctx context.Context, req *pb.ListNotificationMessagesRequest) ([]*pb.NotificationMessage, int32, error) {
	if req == nil {
		req = &pb.ListNotificationMessagesRequest{}
	}
	userID := authn.GetAuthUserID(ctx)
	if userID == 0 {
		return nil, 0, errors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	req.RecipientUserId = &userID
	return uc.repo.ListMessages(ctx, req)
}

func (uc *NotificationUsecase) CountMyUnread(ctx context.Context) (int32, error) {
	userID := authn.GetAuthUserID(ctx)
	if userID == 0 {
		return 0, errors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	return uc.repo.CountUnread(ctx, userID)
}

func (uc *NotificationUsecase) MarkMyRead(ctx context.Context, ids []uint32) error {
	userID := authn.GetAuthUserID(ctx)
	if userID == 0 {
		return errors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	return uc.repo.MarkRead(ctx, ids, userID)
}

type inAppNotificationPayload struct {
	TenantID         uint32   `json:"tenantId"`
	RecipientUserIDs []uint32 `json:"recipientUserIds"`
	TemplateCode     string   `json:"templateCode,omitempty"`
	Title            string   `json:"title,omitempty"`
	Content          string   `json:"content,omitempty"`
	Variables        string   `json:"variables,omitempty"`
	Priority         int32    `json:"priority,omitempty"`
	BusinessType     string   `json:"businessType,omitempty"`
	BusinessID       string   `json:"businessId,omitempty"`
	SenderUserID     uint32   `json:"senderUserId,omitempty"`
	SenderName       string   `json:"senderName,omitempty"`
}

type notificationInAppHandler struct {
	repo NotificationRepo
}

func NewNotificationAsyncTaskHandler(repo NotificationRepo) AsyncTaskHandler {
	return &notificationInAppHandler{repo: repo}
}

func (h *notificationInAppHandler) Type() string { return AsyncTaskTypeNotificationInApp }

func (h *notificationInAppHandler) Handle(ctx context.Context, raw json.RawMessage) (string, error) {
	var payload inAppNotificationPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("decode notification payload: %w", err)
	}
	if payload.TenantID == 0 || len(payload.RecipientUserIDs) == 0 {
		return "", fmt.Errorf("tenant id and recipients are required")
	}
	title, content, templateID, templateCode, err := h.resolveContent(ctx, payload)
	if err != nil {
		return "", err
	}
	messages := make([]*pb.NotificationMessage, 0, len(payload.RecipientUserIDs))
	for _, recipientID := range payload.RecipientUserIDs {
		if recipientID == 0 {
			continue
		}
		messages = append(messages, &pb.NotificationMessage{
			TenantId:        payload.TenantID,
			RecipientUserId: recipientID,
			TemplateId:      templateID,
			TemplateCode:    notificationStringPtr(templateCode),
			Channel:         pb.NotificationChannel_NOTIFICATION_CHANNEL_IN_APP,
			Title:           title,
			Content:         content,
			Status:          pb.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_UNREAD,
			Priority:        &payload.Priority,
			BusinessType:    notificationStringPtr(payload.BusinessType),
			BusinessId:      notificationStringPtr(payload.BusinessID),
			SenderUserId:    notificationUint32Ptr(payload.SenderUserID),
			SenderName:      notificationStringPtr(payload.SenderName),
		})
	}
	count, err := h.repo.CreateMessages(ctx, messages)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("已生成 %d 条站内信", count), nil
}

func (h *notificationInAppHandler) resolveContent(ctx context.Context, payload inAppNotificationPayload) (string, string, *uint32, string, error) {
	title := payload.Title
	content := payload.Content
	var templateID *uint32
	templateCode := strings.TrimSpace(payload.TemplateCode)
	if templateCode != "" {
		tpl, err := h.repo.GetEnabledTemplateByCode(ctx, payload.TenantID, templateCode)
		if err != nil {
			return "", "", nil, "", err
		}
		id := tpl.GetId()
		templateID = &id
		title = tpl.GetTitle()
		content = tpl.GetContent()
	}
	var variables map[string]string
	if strings.TrimSpace(payload.Variables) != "" {
		if err := json.Unmarshal([]byte(payload.Variables), &variables); err != nil {
			return "", "", nil, "", fmt.Errorf("decode notification variables: %w", err)
		}
	}
	return renderNotificationText(title, variables), renderNotificationText(content, variables), templateID, templateCode, nil
}

func renderNotificationText(input string, variables map[string]string) string {
	for key, value := range variables {
		input = strings.ReplaceAll(input, "{{"+key+"}}", value)
	}
	return input
}

func currentUserName(ctx context.Context) string {
	if user, ok := authn.AuthUserFromContext(ctx); ok && user != nil {
		return user.Name()
	}
	return ""
}

func notificationStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func notificationUint32Ptr(value uint32) *uint32 {
	if value == 0 {
		return nil
	}
	return &value
}
