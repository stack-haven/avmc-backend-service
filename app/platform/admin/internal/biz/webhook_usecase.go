package biz

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"backend-service/api/common/enum"
	pb "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

const (
	AsyncTaskTypeWebhookDeliver = "webhook.deliver"
	MaxWebhookSubscriptions     = 20
	WebhookDeliveryTimeout      = 10 * time.Second
)

// WebhookRepo defines the data access interface for webhook subscriptions and delivery logs.
type WebhookRepo interface {
	ListSubscriptions(context.Context, *pb.ListWebhookSubscriptionsRequest) ([]*pb.WebhookSubscription, int32, error)
	GetSubscription(context.Context, uint32) (*pb.WebhookSubscription, error)
	CreateSubscription(context.Context, *pb.WebhookSubscription) (*pb.WebhookSubscription, error)
	UpdateSubscription(context.Context, *pb.WebhookSubscription) (*pb.WebhookSubscription, error)
	DeleteSubscription(context.Context, uint32) error
	FindSubscriptionsByEvent(context.Context, uint32, pb.WebhookEventType) ([]*pb.WebhookSubscription, error)

	ListDeliveryLogs(context.Context, *pb.ListWebhookDeliveryLogsRequest) ([]*pb.WebhookDeliveryLog, int32, error)
	GetDeliveryLog(context.Context, uint32) (*pb.WebhookDeliveryLog, error)
	CreateDeliveryLog(context.Context, *pb.WebhookDeliveryLog) (*pb.WebhookDeliveryLog, error)
	UpdateDeliveryLog(context.Context, *pb.WebhookDeliveryLog) error
}

// WebhookUsecase handles webhook subscription management and event publishing.
type WebhookUsecase struct {
	repo      WebhookRepo
	tasks     AsyncTaskRepo
	httpDo    func(ctx context.Context, req *http.Request) (*http.Response, error)
	log       *log.Helper
}

func NewWebhookUsecase(repo WebhookRepo, tasks AsyncTaskRepo, logger log.Logger) *WebhookUsecase {
	return &WebhookUsecase{
		repo:  repo,
		tasks: tasks,
		httpDo: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			return (&http.Client{Timeout: WebhookDeliveryTimeout}).Do(req.WithContext(ctx))
		},
		log: log.NewHelper(log.With(logger, "module", "biz/webhook")),
	}
}

// ---------------------------------------------------------------------------
// Subscription CRUD
// ---------------------------------------------------------------------------

func (uc *WebhookUsecase) ListSubscriptions(ctx context.Context, req *pb.ListWebhookSubscriptionsRequest) ([]*pb.WebhookSubscription, int32, error) {
	return uc.repo.ListSubscriptions(ctx, req)
}

func (uc *WebhookUsecase) GetSubscription(ctx context.Context, id uint32) (*pb.WebhookSubscription, error) {
	return uc.repo.GetSubscription(ctx, id)
}

func (uc *WebhookUsecase) CreateSubscription(ctx context.Context, sub *pb.WebhookSubscription) (*pb.WebhookSubscription, error) {
	if sub == nil {
		return nil, errors.BadRequest("WEBHOOK_SUBSCRIPTION_REQUIRED", "订阅信息不能为空")
	}
	sub.Name = strings.TrimSpace(sub.GetName())
	sub.Url = strings.TrimSpace(sub.GetUrl())
	sub.Secret = strings.TrimSpace(sub.GetSecret())
	if sub.GetName() == "" {
		return nil, errors.BadRequest("WEBHOOK_NAME_REQUIRED", "订阅名称不能为空")
	}
	if sub.GetUrl() == "" || !strings.HasPrefix(sub.GetUrl(), "https://") {
		return nil, errors.BadRequest("WEBHOOK_URL_INVALID", "回调URL必须以https://开头")
	}
	if len(sub.GetSecret()) < 16 {
		return nil, errors.BadRequest("WEBHOOK_SECRET_TOO_SHORT", "签名密钥至少16个字符")
	}
	if len(sub.GetEventTypes()) == 0 {
		return nil, errors.BadRequest("WEBHOOK_EVENT_TYPES_REQUIRED", "至少选择一个事件类型")
	}

	// Check subscription limit per tenant
	tenantID, err := currentTenantID(ctx)
	if err != nil {
		return nil, err
	}
	existing, _, err := uc.repo.ListSubscriptions(ctx, &pb.ListWebhookSubscriptionsRequest{PageSize: 1})
	if err != nil {
		return nil, err
	}
	_ = tenantID
	if existing != nil && len(existing) >= MaxWebhookSubscriptions {
		return nil, errors.BadRequest("WEBHOOK_SUBSCRIPTION_LIMIT", fmt.Sprintf("每个租户最多 %d 个 Webhook 订阅", MaxWebhookSubscriptions))
	}

	if sub.Status == nil || sub.GetStatus() == enum.Status_STATUS_UNSPECIFIED {
		sub.Status = enum.Status_STATUS_ENABLED.Enum()
	}
	return uc.repo.CreateSubscription(ctx, sub)
}

func (uc *WebhookUsecase) UpdateSubscription(ctx context.Context, sub *pb.WebhookSubscription) (*pb.WebhookSubscription, error) {
	if sub == nil || sub.GetId() == 0 {
		return nil, errors.BadRequest("WEBHOOK_ID_REQUIRED", "订阅ID不能为空")
	}
	if sub.GetUrl() != "" && !strings.HasPrefix(sub.GetUrl(), "https://") {
		return nil, errors.BadRequest("WEBHOOK_URL_INVALID", "回调URL必须以https://开头")
	}
	if sub.GetSecret() != "" && len(sub.GetSecret()) < 16 {
		return nil, errors.BadRequest("WEBHOOK_SECRET_TOO_SHORT", "签名密钥至少16个字符")
	}
	if sub.Status == nil || sub.GetStatus() == enum.Status_STATUS_UNSPECIFIED {
		sub.Status = enum.Status_STATUS_ENABLED.Enum()
	}
	return uc.repo.UpdateSubscription(ctx, sub)
}

func (uc *WebhookUsecase) DeleteSubscription(ctx context.Context, id uint32) error {
	return uc.repo.DeleteSubscription(ctx, id)
}

// ---------------------------------------------------------------------------
// Event Publishing
// ---------------------------------------------------------------------------

// PublishEvent publishes a webhook event to all matching tenant subscriptions.
// It enqueues an async delivery task for each active subscription.
func (uc *WebhookUsecase) PublishEvent(ctx context.Context, req *pb.PublishWebhookEventRequest) error {
	if req == nil || req.GetEventType() == pb.WebhookEventType_WEBHOOK_EVENT_TYPE_UNSPECIFIED {
		return errors.BadRequest("WEBHOOK_EVENT_TYPE_REQUIRED", "事件类型不能为空")
	}
	if strings.TrimSpace(req.GetEventId()) == "" {
		return errors.BadRequest("WEBHOOK_EVENT_ID_REQUIRED", "事件ID不能为空")
	}
	if strings.TrimSpace(req.GetPayload()) == "" {
		return errors.BadRequest("WEBHOOK_PAYLOAD_REQUIRED", "事件载荷不能为空")
	}

	tenantID, err := currentTenantID(ctx)
	if err != nil {
		return err
	}

	subs, err := uc.repo.FindSubscriptionsByEvent(ctx, tenantID, req.GetEventType())
	if err != nil {
		return fmt.Errorf("find subscriptions: %w", err)
	}

	for _, sub := range subs {
		deliveryPayload, err := json.Marshal(webhookDeliveryTaskPayload{
			SubscriptionID: sub.GetId(),
			TenantID:       tenantID,
			EventID:        strings.TrimSpace(req.GetEventId()),
			EventType:      int32(req.GetEventType()),
			TargetURL:      sub.GetUrl(),
			Secret:         sub.GetSecret(),
			Payload:        strings.TrimSpace(req.GetPayload()),
		})
		if err != nil {
			uc.log.Errorf("marshal delivery payload: %v", err)
			continue
		}

		idempotencyKey := fmt.Sprintf("webhook:%d:%s", sub.GetId(), strings.TrimSpace(req.GetEventId()))
		_, err = uc.tasks.Enqueue(ctx, &AsyncTaskSpec{
			TenantID:       &tenantID,
			TaskType:       AsyncTaskTypeWebhookDeliver,
			Queue:          "webhook",
			Priority:       0,
			Payload:        deliveryPayload,
			PayloadSummary: fmt.Sprintf("Webhook 投递: %s → %s", req.GetEventType().String(), sub.GetUrl()),
			IdempotencyKey: idempotencyKey,
			MaxAttempts:    5,
			ScheduledAt:    time.Now(),
		})
		if err != nil {
			uc.log.Errorf("enqueue webhook delivery for subscription %d: %v", sub.GetId(), err)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Delivery Log queries
// ---------------------------------------------------------------------------

func (uc *WebhookUsecase) ListDeliveryLogs(ctx context.Context, req *pb.ListWebhookDeliveryLogsRequest) ([]*pb.WebhookDeliveryLog, int32, error) {
	return uc.repo.ListDeliveryLogs(ctx, req)
}

func (uc *WebhookUsecase) GetDeliveryLog(ctx context.Context, id uint32) (*pb.WebhookDeliveryLog, error) {
	return uc.repo.GetDeliveryLog(ctx, id)
}

// RetryDelivery marks a failed delivery and creates a new retry.
func (uc *WebhookUsecase) RetryDelivery(ctx context.Context, id uint32) (*pb.RetryWebhookDeliveryResponse, error) {
	logEntry, err := uc.repo.GetDeliveryLog(ctx, id)
	if err != nil {
		return nil, err
	}
	if logEntry.GetDeliveryStatus() != pb.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_FAILED {
		return nil, errors.BadRequest("WEBHOOK_DELIVERY_NOT_FAILED", "只有失败的投递才能重试")
	}

	// Get the subscription to re-deliver
	sub, err := uc.repo.GetSubscription(ctx, logEntry.GetSubscriptionId())
	if err != nil {
		return nil, err
	}

	tenantID, err := currentTenantID(ctx)
	if err != nil {
		return nil, err
	}

	deliveryPayload, err := json.Marshal(webhookDeliveryTaskPayload{
		SubscriptionID: sub.GetId(),
		TenantID:       tenantID,
		EventID:        logEntry.GetEventId(),
		EventType:      int32(logEntry.GetEventType()),
		TargetURL:      sub.GetUrl(),
		Secret:         sub.GetSecret(),
		Payload:        logEntry.GetRequestBody(),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal retry payload: %w", err)
	}

	newEventID := fmt.Sprintf("%s-retry-%d", logEntry.GetEventId(), time.Now().UnixMilli())
	idempotencyKey := fmt.Sprintf("webhook:retry:%d:%s", sub.GetId(), newEventID)
	task, err := uc.tasks.Enqueue(ctx, &AsyncTaskSpec{
		TenantID:       &tenantID,
		TaskType:       AsyncTaskTypeWebhookDeliver,
		Queue:          "webhook",
		Priority:       0,
		Payload:        deliveryPayload,
		PayloadSummary: fmt.Sprintf("Webhook 重试: event %s → %s", logEntry.GetEventId(), sub.GetUrl()),
		IdempotencyKey: idempotencyKey,
		MaxAttempts:    3,
		ScheduledAt:    time.Now(),
	})
	if err != nil {
		return nil, err
	}

	return &pb.RetryWebhookDeliveryResponse{NewDeliveryLogId: task.GetId()}, nil
}

// ---------------------------------------------------------------------------
// AsyncTask Handler — actual HTTP delivery
// ---------------------------------------------------------------------------

type webhookDeliveryTaskPayload struct {
	SubscriptionID uint32 `json:"subscriptionId"`
	TenantID       uint32 `json:"tenantId"`
	EventID        string `json:"eventId"`
	EventType      int32  `json:"eventType"`
	TargetURL      string `json:"targetUrl"`
	Secret         string `json:"secret"`
	Payload        string `json:"payload"`
}

type webhookDeliveryHandler struct {
	repo   WebhookRepo
	httpDo func(ctx context.Context, req *http.Request) (*http.Response, error)
	log    *log.Helper
}

func NewWebhookAsyncTaskHandler(repo WebhookRepo, logger log.Logger) AsyncTaskHandler {
	client := &http.Client{Timeout: WebhookDeliveryTimeout}
	return &webhookDeliveryHandler{
		repo:   repo,
		httpDo: func(ctx context.Context, req *http.Request) (*http.Response, error) {
			return client.Do(req.WithContext(ctx))
		},
		log:    log.NewHelper(log.With(logger, "module", "handler/webhook-deliver")),
	}
}

func (h *webhookDeliveryHandler) Type() string { return AsyncTaskTypeWebhookDeliver }

func (h *webhookDeliveryHandler) Handle(ctx context.Context, raw json.RawMessage) (string, error) {
	var payload webhookDeliveryTaskPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("decode webhook delivery payload: %w", err)
	}

	// Create delivery log entry
	logEntry := &pb.WebhookDeliveryLog{
		TenantId:        payload.TenantID,
		SubscriptionId:  payload.SubscriptionID,
		EventId:         payload.EventID,
		EventType:       pb.WebhookEventType(payload.EventType),
		TargetUrl:       payload.TargetURL,
		RequestBody:     payload.Payload,
		DeliveryStatus:  pb.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_PENDING,
		AttemptNumber:   1,
	}
	created, err := h.repo.CreateDeliveryLog(ctx, logEntry)
	if err != nil {
		return "", fmt.Errorf("create delivery log: %w", err)
	}

	// Build HMAC signature
	timestamp := time.Now().Unix()
	signature := hmacSHA256(payload.Secret, fmt.Sprintf("%d.%s", timestamp, payload.Payload))

	// Build HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, payload.TargetURL,
		bytes.NewBufferString(payload.Payload))
	if err != nil {
		return h.failDelivery(ctx, created, fmt.Sprintf("build request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event-ID", payload.EventID)
	req.Header.Set("X-Webhook-Event-Type", pb.WebhookEventType(payload.EventType).String())
	req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-Webhook-Signature", signature)

	// Deliver
	resp, err := h.httpDo(ctx, req)
	if err != nil {
		return h.failDelivery(ctx, created, fmt.Sprintf("http request: %v", err))
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))

	created.ResponseCode = toInt32(resp.StatusCode)
	created.ResponseBody = toStrPtr(string(responseBody))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		created.DeliveryStatus = pb.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_SUCCESS
		if err := h.repo.UpdateDeliveryLog(ctx, created); err != nil {
			h.log.Errorf("update delivery log %d: %v", created.GetId(), err)
		}
		return fmt.Sprintf("投递成功: HTTP %d", resp.StatusCode), nil
	}

	return h.failDelivery(ctx, created, fmt.Sprintf("HTTP %d", resp.StatusCode))
}

func (h *webhookDeliveryHandler) failDelivery(ctx context.Context, logEntry *pb.WebhookDeliveryLog, reason string) (string, error) {
	logEntry.DeliveryStatus = pb.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_FAILED
	logEntry.ErrorMessage = &reason
	if err := h.repo.UpdateDeliveryLog(ctx, logEntry); err != nil {
		h.log.Errorf("update failed delivery log %d: %v", logEntry.GetId(), err)
	}
	return "", fmt.Errorf("webhook delivery failed: %s", reason)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func hmacSHA256(secret string, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func VerifyWebhookSignature(secret, payload, expectedSig string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expected := []byte("sha256=" + hex.EncodeToString(mac.Sum(nil)))
	actual := []byte(expectedSig)
	return subtle.ConstantTimeCompare(expected, actual) == 1
}

func toInt32(n int) *int32 {
	v := int32(n)
	return &v
}

func toStrPtr(s string) *string {
	return &s
}
