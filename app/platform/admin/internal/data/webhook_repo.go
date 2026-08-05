package data

import (
	"context"
	"strings"
	"time"

	"backend-service/api/common/enum"
	pb "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/webhookdeliverylog"
	"backend-service/app/platform/admin/internal/data/ent/gen/webhooksubscription"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

type webhookRepo struct{ BaseRepo }

func NewWebhookRepo(data *Data, logger log.Logger) biz.WebhookRepo {
	return &webhookRepo{BaseRepo: NewBaseRepo(data, logger)}
}

// ---------------------------------------------------------------------------
// Proto conversion helpers
// ---------------------------------------------------------------------------

func webhookSubscriptionProto(row *gen.WebhookSubscription) *pb.WebhookSubscription {
	if row == nil {
		return nil
	}
	status := enum.Status_STATUS_UNSPECIFIED
	if row.Status != nil {
		status = enum.Status(*row.Status)
	}
	var eventTypes []pb.WebhookEventType
	if len(row.EventTypes) > 0 {
		eventTypes = make([]pb.WebhookEventType, len(row.EventTypes))
		for i, v := range row.EventTypes {
			eventTypes[i] = pb.WebhookEventType(v)
		}
	}
	return &pb.WebhookSubscription{
		Id:         row.ID,
		TenantId:   &row.TenantID,
		Name:       row.Name,
		Url:        row.URL,
		Secret:     row.Secret,
		EventTypes: eventTypes,
		Status:     &status,
		CreatedAt:  convert.TimeValueToString(&row.CreatedAt, time.DateTime),
		UpdatedAt:  convert.TimeValueToString(&row.UpdatedAt, time.DateTime),
	}
}

func webhookDeliveryLogProto(row *gen.WebhookDeliveryLog) *pb.WebhookDeliveryLog {
	if row == nil {
		return nil
	}
	errMsg := ""
	if row.ErrorMessage != "" {
		errMsg = row.ErrorMessage
	}
	rspBody := ""
	if row.ResponseBody != "" {
		rspBody = row.ResponseBody
	}
	return &pb.WebhookDeliveryLog{
		Id:             row.ID,
		TenantId:       row.TenantID,
		SubscriptionId: row.SubscriptionID,
		EventId:        row.EventID,
		EventType:      pb.WebhookEventType(row.EventType),
		TargetUrl:      row.TargetURL,
		RequestBody:    row.RequestBody,
		ResponseCode:   convert.ToPointer(row.ResponseCode),
		ResponseBody:   convert.EmptyToNil(rspBody),
		DeliveryStatus: pb.WebhookDeliveryStatus(row.DeliveryStatus),
		AttemptNumber:  row.AttemptNumber,
		ErrorMessage:   convert.EmptyToNil(errMsg),
		CreatedAt:      convert.TimeValueToString(&row.CreatedAt, time.DateTime),
	}
}

// ---------------------------------------------------------------------------
// Subscription CRUD
// ---------------------------------------------------------------------------

func (r *webhookRepo) ListSubscriptions(ctx context.Context, req *pb.ListWebhookSubscriptionsRequest) ([]*pb.WebhookSubscription, int32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, 0, err
	}
	if req == nil {
		req = &pb.ListWebhookSubscriptionsRequest{}
	}
	query := r.Data.DB(ctx).WebhookSubscription.Query()
	if keyword := strings.TrimSpace(req.GetKeyword()); keyword != "" {
		query.Where(webhooksubscription.Or(
			webhooksubscription.NameContains(keyword),
			webhooksubscription.URLContains(keyword),
		))
	}
	// EventType filtering is applied at the application layer (JSON array contains)
	if req.Status != nil && req.GetStatus() != enum.Status_STATUS_UNSPECIFIED {
		query.Where(webhooksubscription.StatusEQ(int32(req.GetStatus())))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Desc(webhooksubscription.FieldCreatedAt)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return convert.SliceToAny(rows, webhookSubscriptionProto), int32(total), nil
}

func (r *webhookRepo) GetSubscription(ctx context.Context, id uint32) (*pb.WebhookSubscription, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	row, err := r.Data.DB(ctx).WebhookSubscription.Get(ctx, id)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("WEBHOOK_SUBSCRIPTION_NOT_FOUND", "Webhook 订阅不存在")
	}
	if err != nil {
		return nil, err
	}
	return webhookSubscriptionProto(row), nil
}

func (r *webhookRepo) CreateSubscription(ctx context.Context, value *pb.WebhookSubscription) (*pb.WebhookSubscription, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	row, err := r.Data.DB(ctx).WebhookSubscription.Create().
		SetName(value.GetName()).
		SetURL(value.GetUrl()).
		SetSecret(value.GetSecret()).
		SetEventTypes(eventTypeInt32s(value.GetEventTypes())).
		SetStatus(int32(value.GetStatus())).
		Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("WEBHOOK_SUBSCRIPTION_URL_EXISTS", "该 URL 已存在")
	}
	if err != nil {
		return nil, err
	}
	return webhookSubscriptionProto(row), nil
}

func (r *webhookRepo) UpdateSubscription(ctx context.Context, value *pb.WebhookSubscription) (*pb.WebhookSubscription, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	upd := r.Data.DB(ctx).WebhookSubscription.UpdateOneID(value.GetId())
	if value.GetName() != "" {
		upd.SetName(value.GetName())
	}
	if value.GetUrl() != "" {
		upd.SetURL(value.GetUrl())
	}
	if value.GetSecret() != "" {
		upd.SetSecret(value.GetSecret())
	}
	if len(value.GetEventTypes()) > 0 {
		upd.SetEventTypes(eventTypeInt32s(value.GetEventTypes()))
	}
	upd.SetStatus(int32(value.GetStatus()))
	row, err := upd.Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("WEBHOOK_SUBSCRIPTION_URL_EXISTS", "该 URL 已存在")
	}
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("WEBHOOK_SUBSCRIPTION_NOT_FOUND", "Webhook 订阅不存在")
	}
	if err != nil {
		return nil, err
	}
	return webhookSubscriptionProto(row), nil
}

func (r *webhookRepo) DeleteSubscription(ctx context.Context, id uint32) error {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return err
	}
	err := r.Data.DB(ctx).WebhookSubscription.DeleteOneID(id).Exec(ctx)
	if gen.IsNotFound(err) {
		return errors.NotFound("WEBHOOK_SUBSCRIPTION_NOT_FOUND", "Webhook 订阅不存在")
	}
	return err
}

func (r *webhookRepo) FindSubscriptionsByEvent(ctx context.Context, tenantID uint32, eventType pb.WebhookEventType) ([]*pb.WebhookSubscription, error) {
	// Find all active subscriptions for the tenant that include the given event type
	rows, err := r.Data.DB(ctx).WebhookSubscription.Query().
		Where(
			webhooksubscription.TenantIDEQ(tenantID),
			webhooksubscription.StatusEQ(int32(enum.Status_STATUS_ENABLED)),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	// Filter at application layer: check if event_type is in the JSON array
	result := make([]*pb.WebhookSubscription, 0, len(rows))
	for _, row := range rows {
		sub := webhookSubscriptionProto(row)
		for _, et := range sub.GetEventTypes() {
			if et == eventType {
				result = append(result, sub)
				break
			}
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Delivery Log CRUD
// ---------------------------------------------------------------------------

func (r *webhookRepo) ListDeliveryLogs(ctx context.Context, req *pb.ListWebhookDeliveryLogsRequest) ([]*pb.WebhookDeliveryLog, int32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, 0, err
	}
	if req == nil {
		req = &pb.ListWebhookDeliveryLogsRequest{}
	}
	query := r.Data.DB(ctx).WebhookDeliveryLog.Query()
	if req.SubscriptionId != nil && req.GetSubscriptionId() > 0 {
		query.Where(webhookdeliverylog.SubscriptionIDEQ(req.GetSubscriptionId()))
	}
	if req.EventType != nil && req.GetEventType() != pb.WebhookEventType_WEBHOOK_EVENT_TYPE_UNSPECIFIED {
		query.Where(webhookdeliverylog.EventTypeEQ(int32(req.GetEventType())))
	}
	if req.DeliveryStatus != nil && req.GetDeliveryStatus() != pb.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_UNSPECIFIED {
		query.Where(webhookdeliverylog.DeliveryStatusEQ(int32(req.GetDeliveryStatus())))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Desc(webhookdeliverylog.FieldCreatedAt)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return convert.SliceToAny(rows, webhookDeliveryLogProto), int32(total), nil
}

func (r *webhookRepo) GetDeliveryLog(ctx context.Context, id uint32) (*pb.WebhookDeliveryLog, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	row, err := r.Data.DB(ctx).WebhookDeliveryLog.Get(ctx, id)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("WEBHOOK_DELIVERY_LOG_NOT_FOUND", "投递记录不存在")
	}
	if err != nil {
		return nil, err
	}
	return webhookDeliveryLogProto(row), nil
}

func (r *webhookRepo) CreateDeliveryLog(ctx context.Context, value *pb.WebhookDeliveryLog) (*pb.WebhookDeliveryLog, error) {
	row, err := r.Data.DB(ctx).WebhookDeliveryLog.Create().
		SetSubscriptionID(value.GetSubscriptionId()).
		SetEventID(value.GetEventId()).
		SetEventType(int32(value.GetEventType())).
		SetTargetURL(value.GetTargetUrl()).
		SetRequestBody(value.GetRequestBody()).
		SetDeliveryStatus(int32(value.GetDeliveryStatus())).
		SetAttemptNumber(value.GetAttemptNumber()).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return webhookDeliveryLogProto(row), nil
}

func (r *webhookRepo) UpdateDeliveryLog(ctx context.Context, value *pb.WebhookDeliveryLog) error {
	upd := r.Data.DB(ctx).WebhookDeliveryLog.UpdateOneID(value.GetId()).
		SetDeliveryStatus(int32(value.GetDeliveryStatus()))
	if value.ResponseCode != nil {
		upd.SetResponseCode(*value.ResponseCode)
	}
	if value.GetResponseBody() != "" {
		upd.SetResponseBody(value.GetResponseBody())
	}
	if value.ErrorMessage != nil && *value.ErrorMessage != "" {
		upd.SetErrorMessage(*value.ErrorMessage)
	}
	return upd.Exec(ctx)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func eventTypeInt32s(types []pb.WebhookEventType) []int32 {
	result := make([]int32, len(types))
	for i, v := range types {
		result[i] = int32(v)
	}
	return result
}
