package biz

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/proto"

	pb "backend-service/api/platform/service/v1"
	"backend-service/pkg/notifier"
	"backend-service/pkg/utils/convert"
)

// 通知渠道类型常量（业务维度：通知模板选渠道）。
const (
	NotificationChannelInApp   = "in-app"
	NotificationChannelSMS     = "sms"
	NotificationChannelEmail   = "email"
	NotificationChannelWebhook = "webhook"
	NotificationChannelPush    = "push"
)

// 通知提供商类型常量（技术维度：具体服务商，对应 notifier 工厂注册名）。
const (
	NotificationProviderAliyunSMS = "aliyun-sms"
	NotificationProviderYunpian   = "yunpian"
	NotificationProviderJPush     = "jpush"
	NotificationProviderGetui     = "getui"
)

// NotificationProviderRepo 通知渠道配置仓储。
type NotificationProviderRepo interface {
	Create(context.Context, *pb.NotificationProvider) (*pb.NotificationProvider, error)
	Update(context.Context, *pb.NotificationProvider) (*pb.NotificationProvider, error)
	Delete(context.Context, uint32) error
	Get(context.Context, uint32) (*pb.NotificationProvider, error)
	List(context.Context, *pb.ListNotificationProvidersRequest) ([]*pb.NotificationProvider, int32, error)
	SetDefault(context.Context, uint32) (*pb.NotificationProvider, error)
	// ResolveChannel 按渠道解析启用配置（默认配置优先），返回含密钥的配置和 configJSON。
	ResolveChannel(context.Context, string) (*pb.NotificationProvider, json.RawMessage, error)
}

// NotificationProviderUsecase 通知渠道配置业务逻辑。
type NotificationProviderUsecase struct {
	repo NotificationProviderRepo
	log  *log.Helper
}

func NewNotificationProviderUsecase(repo NotificationProviderRepo, logger log.Logger) *NotificationProviderUsecase {
	return &NotificationProviderUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *NotificationProviderUsecase) Create(ctx context.Context, item *pb.NotificationProvider) (*pb.NotificationProvider, error) {
	if err := validateNotificationProvider(item, true); err != nil {
		return nil, err
	}
	return uc.repo.Create(ctx, normalizeNotificationProvider(item, true))
}

func (uc *NotificationProviderUsecase) Update(ctx context.Context, id uint32, item *pb.NotificationProvider) (*pb.NotificationProvider, error) {
	if id == 0 {
		return nil, errors.BadRequest("NOTIFICATION_PROVIDER_ID_REQUIRED", "通知渠道配置ID不能为空")
	}
	if item == nil {
		return nil, errors.BadRequest("NOTIFICATION_PROVIDER_REQUIRED", "通知渠道配置不能为空")
	}
	item.Id = id
	if err := validateNotificationProvider(item, false); err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, normalizeNotificationProvider(item, false))
}

func (uc *NotificationProviderUsecase) Delete(ctx context.Context, id uint32) error {
	if id == 0 {
		return errors.BadRequest("NOTIFICATION_PROVIDER_ID_REQUIRED", "通知渠道配置ID不能为空")
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *NotificationProviderUsecase) Get(ctx context.Context, id uint32) (*pb.NotificationProvider, error) {
	if id == 0 {
		return nil, errors.BadRequest("NOTIFICATION_PROVIDER_ID_REQUIRED", "通知渠道配置ID不能为空")
	}
	return uc.repo.Get(ctx, id)
}

func (uc *NotificationProviderUsecase) List(ctx context.Context, req *pb.ListNotificationProvidersRequest) ([]*pb.NotificationProvider, int32, error) {
	if req == nil {
		req = &pb.ListNotificationProvidersRequest{}
	}
	return uc.repo.List(ctx, req)
}

func (uc *NotificationProviderUsecase) SetDefault(ctx context.Context, id uint32) (*pb.NotificationProvider, error) {
	if id == 0 {
		return nil, errors.BadRequest("NOTIFICATION_PROVIDER_ID_REQUIRED", "通知渠道配置ID不能为空")
	}
	return uc.repo.SetDefault(ctx, id)
}

func (uc *NotificationProviderUsecase) Test(ctx context.Context, id uint32, phone string) (*pb.TestNotificationProviderResponse, error) {
	if id == 0 {
		return nil, errors.BadRequest("NOTIFICATION_PROVIDER_ID_REQUIRED", "通知渠道配置ID不能为空")
	}
	item, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.GetChannel() == NotificationChannelInApp {
		return &pb.TestNotificationProviderResponse{Healthy: true, Message: "站内信为内置渠道，无需外部配置"}, nil
	}
	_, configJSON, err := uc.repo.ResolveChannel(ctx, item.GetChannel())
	if err != nil {
		return nil, err
	}
	sender, err := notifier.NewSender(item.GetProviderType(), configJSON)
	if err != nil {
		return &pb.TestNotificationProviderResponse{Healthy: false, Message: err.Error()}, nil
	}
	// 短信渠道发一条测试短信；其他渠道仅验证配置可构建。
	if item.GetChannel() == NotificationChannelSMS {
		if strings.TrimSpace(phone) == "" {
			return &pb.TestNotificationProviderResponse{Healthy: true, Message: "配置有效（未提供测试手机号，跳过发送）"}, nil
		}
		_, err = sender.Send(ctx, notifier.Message{
			Channel:    NotificationChannelSMS,
			Recipients: []notifier.Recipient{{Phone: phone}},
			Content:    "test",
		})
		if err != nil {
			return &pb.TestNotificationProviderResponse{Healthy: false, Message: err.Error()}, nil
		}
	}
	return &pb.TestNotificationProviderResponse{Healthy: true, Message: "ok"}, nil
}

func validateNotificationProvider(item *pb.NotificationProvider, create bool) error {
	if item == nil {
		return errors.BadRequest("NOTIFICATION_PROVIDER_REQUIRED", "通知渠道配置不能为空")
	}
	if strings.TrimSpace(item.GetCode()) == "" {
		return errors.BadRequest("NOTIFICATION_PROVIDER_CODE_REQUIRED", "渠道配置编码不能为空")
	}
	if strings.TrimSpace(item.GetName()) == "" {
		return errors.BadRequest("NOTIFICATION_PROVIDER_NAME_REQUIRED", "渠道配置名称不能为空")
	}
	channel := strings.TrimSpace(strings.ToLower(item.GetChannel()))
	switch channel {
	case NotificationChannelInApp:
		// 站内信无需外部服务商。
	default:
		// 短信/推送/邮件/Webhook 必须指定 provider_type（具体提供商）。
		if strings.TrimSpace(item.GetProviderType()) == "" {
			return errors.BadRequest("NOTIFICATION_PROVIDER_TYPE_REQUIRED", "外部渠道必须指定提供商类型")
		}
	}
	if channel == NotificationChannelSMS && create && strings.TrimSpace(item.GetAccessKeySecret()) == "" {
		return errors.BadRequest("NOTIFICATION_PROVIDER_SECRET_REQUIRED", "短信渠道访问密钥不能为空")
	}
	return nil
}

func normalizeNotificationProvider(item *pb.NotificationProvider, create bool) *pb.NotificationProvider {
	clone := proto.Clone(item).(*pb.NotificationProvider) //nolint:errcheck
	clone.Code = strings.TrimSpace(clone.Code)
	clone.Name = strings.TrimSpace(clone.Name)
	clone.Channel = strings.TrimSpace(strings.ToLower(clone.Channel))
	if clone.ProviderType != nil {
		pt := strings.TrimSpace(strings.ToLower(clone.GetProviderType()))
		clone.ProviderType = &pt
	}
	if !create && strings.TrimSpace(clone.GetAccessKeySecret()) == "" {
		clone.AccessKeySecret = nil
	}
	return clone
}

// notificationSenderResolver 按渠道解析通知发送器。
type notificationSenderResolver struct {
	providers NotificationProviderRepo
	notifRepo NotificationRepo
}

// NewNotificationSenderResolver 创建通知发送器解析器。
func NewNotificationSenderResolver(providers NotificationProviderRepo, notifRepo NotificationRepo) *notificationSenderResolver {
	return &notificationSenderResolver{providers: providers, notifRepo: notifRepo}
}

// ResolveByChannel 按渠道解析发送器。站内信直写 DB；短信/推送等按 provider_type 构造提供商。
// 所有渠道都先校验配置存在且启用（站内信作为内置渠道，通过预置配置控制启停）。
func (r *notificationSenderResolver) ResolveByChannel(ctx context.Context, channel string) (notifier.Sender, error) {
	switch channel {
	case NotificationChannelInApp:
		if _, _, err := r.providers.ResolveChannel(ctx, channel); err != nil {
			return nil, err
		}
		return &inAppSender{repo: r.notifRepo}, nil
	default:
		// 短信/推送/邮件/Webhook：按 provider_type 分发的渠道提供商。
		provider, configJSON, err := r.providers.ResolveChannel(ctx, channel)
		if err != nil {
			return nil, err
		}
		return notifier.NewSender(provider.GetProviderType(), configJSON)
	}
}

// inAppSender 站内信发送器：直接写 NotificationMessage 表。
type inAppSender struct {
	repo NotificationRepo
}

func (s *inAppSender) Channel() string { return NotificationChannelInApp }

func (s *inAppSender) Send(ctx context.Context, msg notifier.Message) (notifier.Result, error) {
	result := notifier.Result{}
	messages := make([]*pb.NotificationMessage, 0, len(msg.Recipients))
	for _, recipient := range msg.Recipients {
		if recipient.UserID == 0 {
			continue
		}
		messages = append(messages, &pb.NotificationMessage{
			TenantId:        msg.TenantID,
			RecipientUserId: recipient.UserID,
			TemplateId:      convert.EmptyToNil(msg.TemplateID),
			TemplateCode:    convert.EmptyToNil(msg.TemplateCode),
			Channel:         pb.NotificationChannel_NOTIFICATION_CHANNEL_IN_APP,
			Title:           msg.Title,
			Content:         msg.Content,
			Status:          pb.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_UNREAD,
			Priority:        &msg.Priority,
			BusinessType:    convert.EmptyToNil(msg.BusinessType),
			BusinessId:      convert.EmptyToNil(msg.BusinessID),
			SenderUserId:    convert.EmptyToNil(msg.SenderUserID),
			SenderName:      convert.EmptyToNil(msg.SenderName),
		})
	}
	count, err := s.repo.CreateMessages(ctx, messages)
	if err != nil {
		return result, err
	}
	result.SuccessCount = count
	return result, nil
}
