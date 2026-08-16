package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pb "backend-service/api/platform/service/v1"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/notifier"
	"backend-service/pkg/utils/convert"
)

// AsyncTaskTypeNotificationSend 通用通知发送任务（站内信/短信/邮件/Webhook）。
const AsyncTaskTypeNotificationSend = "notification.send"

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
		Channel:          NotificationChannelInApp,
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
	task, err := uc.enqueueNotification(ctx, tenantID, NotificationChannelInApp, len(req.GetRecipientUserIds()), 0, req.GetPriority(), payload, idempotencyKey)
	if err != nil {
		return nil, err
	}
	return &pb.SendInAppNotificationResponse{TaskId: task.GetId()}, nil
}

// SendNotification 通用通知发送（站内信/短信/邮件/Webhook）。
func (uc *NotificationUsecase) SendNotification(ctx context.Context, req *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
	if req == nil {
		return nil, errors.BadRequest("NOTIFICATION_REQUEST_REQUIRED", "通知请求不能为空")
	}
	channel := channelString(req.GetChannel())
	if channel == "" {
		return nil, errors.BadRequest("NOTIFICATION_CHANNEL_REQUIRED", "通知渠道不能为空")
	}
	if len(req.GetRecipientUserIds()) == 0 && len(req.GetPhones()) == 0 {
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
		Channel:          channel,
		TenantID:         tenantID,
		RecipientUserIDs: req.GetRecipientUserIds(),
		Phones:           req.GetPhones(),
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
		idempotencyKey = fmt.Sprintf("notification:%s:%d:%s:%s:%d", channel, tenantID, req.GetBusinessType(), req.GetBusinessId(), time.Now().UnixNano())
	}
	task, err := uc.enqueueNotification(ctx, tenantID, channel, len(req.GetRecipientUserIds()), len(req.GetPhones()), req.GetPriority(), payload, idempotencyKey)
	if err != nil {
		return nil, err
	}
	return &pb.SendNotificationResponse{TaskId: task.GetId()}, nil
}

// enqueueNotification 创建通用通知发送任务。
func (uc *NotificationUsecase) enqueueNotification(ctx context.Context, tenantID uint32, channel string, userCount, phoneCount int, priority int32, payload []byte, idempotencyKey string) (*pb.AsyncTask, error) {
	summary := fmt.Sprintf("通知发送：渠道 %s，用户 %d 个，手机号 %d 个", channel, userCount, phoneCount)
	return uc.tasks.Enqueue(ctx, &AsyncTaskSpec{
		TenantID:       &tenantID,
		TaskType:       AsyncTaskTypeNotificationSend,
		Queue:          "notification",
		Priority:       priority,
		Payload:        payload,
		PayloadSummary: summary,
		IdempotencyKey: idempotencyKey,
		MaxAttempts:    3,
		ScheduledAt:    time.Now(),
		CreatedBy:      convert.EmptyToNil(authn.GetAuthUserID(ctx)),
	})
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
	Channel          string   `json:"channel,omitempty"`
	TenantID         uint32   `json:"tenantId"`
	RecipientUserIDs []uint32 `json:"recipientUserIds"`
	Phones           []string `json:"phones,omitempty"`
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
	repo     NotificationRepo
	resolver *notificationSenderResolver
}

func NewNotificationAsyncTaskHandler(repo NotificationRepo, resolver *notificationSenderResolver) AsyncTaskHandler {
	return &notificationInAppHandler{repo: repo, resolver: resolver}
}

func (h *notificationInAppHandler) Type() string { return AsyncTaskTypeNotificationSend }

func (h *notificationInAppHandler) Handle(ctx context.Context, raw json.RawMessage) (string, error) {
	var payload inAppNotificationPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("decode notification payload: %w", err)
	}
	if payload.TenantID == 0 {
		return "", fmt.Errorf("tenant id is required")
	}
	channel := payload.Channel
	if channel == "" {
		channel = NotificationChannelInApp
	}
	if len(payload.RecipientUserIDs) == 0 && len(payload.Phones) == 0 {
		return "", fmt.Errorf("recipients or phones are required")
	}
	title, content, templateID, templateCode, err := h.resolveContent(ctx, &payload)
	if err != nil {
		return "", err
	}

	// 通过供应商抽象发送：resolver 解析 sender，站内信直写 DB，短信调用服务商。
	sender, err := h.resolver.ResolveByChannel(ctx, channel)
	if err != nil {
		return "", err
	}
	recipients := make([]notifier.Recipient, 0, len(payload.RecipientUserIDs)+len(payload.Phones))
	for _, recipientID := range payload.RecipientUserIDs {
		if recipientID == 0 {
			continue
		}
		recipients = append(recipients, notifier.Recipient{UserID: recipientID})
	}
	for _, phone := range payload.Phones {
		if strings.TrimSpace(phone) == "" {
			continue
		}
		recipients = append(recipients, notifier.Recipient{Phone: strings.TrimSpace(phone)})
	}
	var templateIDVal uint32
	if templateID != nil {
		templateIDVal = *templateID
	}
	result, err := sender.Send(ctx, notifier.Message{
		Channel:      channel,
		TenantID:     payload.TenantID,
		Title:        title,
		Content:      content,
		Recipients:   recipients,
		TemplateID:   templateIDVal,
		TemplateCode: templateCode,
		Priority:     payload.Priority,
		BusinessType: payload.BusinessType,
		BusinessID:   payload.BusinessID,
		SenderUserID: payload.SenderUserID,
		SenderName:   payload.SenderName,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("已发送 %d 条通知，失败 %d 条", result.SuccessCount, result.FailCount), nil
}

func (h *notificationInAppHandler) resolveContent(ctx context.Context, payload *inAppNotificationPayload) (string, string, *uint32, string, error) { //nolint:gocritic // return types clear from callers
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

// channelString 把通知渠道枚举转为 notifier 渠道标识。
func channelString(channel pb.NotificationChannel) string {
	switch channel {
	case pb.NotificationChannel_NOTIFICATION_CHANNEL_SMS:
		return NotificationChannelSMS
	case pb.NotificationChannel_NOTIFICATION_CHANNEL_EMAIL:
		return NotificationChannelEmail
	case pb.NotificationChannel_NOTIFICATION_CHANNEL_WEBHOOK:
		return NotificationChannelWebhook
	default:
		return NotificationChannelInApp
	}
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
