package data

import (
	"context"
	"strings"
	"time"

	"backend-service/api/common/enum"
	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/notificationmessage"
	"backend-service/app/platform/service/internal/data/ent/gen/notificationtemplate"
	entviewer "backend-service/app/platform/service/internal/data/ent/viewer"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

type notificationRepo struct{ BaseRepo }

func NewNotificationRepo(data *Data, logger log.Logger) biz.NotificationRepo {
	return &notificationRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func notificationTemplateProto(row *gen.NotificationTemplate) *pb.NotificationTemplate {
	if row == nil {
		return nil
	}
	status := enum.Status_STATUS_UNSPECIFIED
	if row.Status != nil {
		status = enum.Status(*row.Status)
	}
	result := &pb.NotificationTemplate{
		Id:             row.ID,
		TenantId:       &row.TenantID,
		Code:           row.Code,
		Name:           row.Name,
		Channel:        pb.NotificationChannel(row.Channel),
		Title:          row.Title,
		Content:        row.Content,
		VariableSchema: &row.VariableSchema,
		Locale:         &row.Locale,
		Status:         &status,
		Remark:         &row.Remark,
		CreatedAt:      convert.TimeValueToString(&row.CreatedAt, time.DateTime),
		UpdatedAt:      convert.TimeValueToString(&row.UpdatedAt, time.DateTime),
	}
	return result
}

func notificationMessageProto(row *gen.NotificationMessage) *pb.NotificationMessage {
	if row == nil {
		return nil
	}
	result := &pb.NotificationMessage{
		Id:              row.ID,
		TenantId:        row.TenantID,
		RecipientUserId: row.RecipientUserID,
		TemplateId:      row.TemplateID,
		TemplateCode:    &row.TemplateCode,
		Channel:         pb.NotificationChannel(row.Channel),
		Title:           row.Title,
		Content:         row.Content,
		Status:          pb.NotificationMessageStatus(row.MessageStatus),
		Priority:        &row.Priority,
		BusinessType:    &row.BusinessType,
		BusinessId:      &row.BusinessID,
		SenderUserId:    row.SenderUserID,
		SenderName:      &row.SenderName,
		CreatedAt:       convert.TimeValueToString(&row.CreatedAt, time.DateTime),
		UpdatedAt:       convert.TimeValueToString(&row.UpdatedAt, time.DateTime),
	}
	setOptionalTime(&result.ReadAt, row.ReadAt)
	return result
}

func (r *notificationRepo) ListTemplates(ctx context.Context, req *pb.ListNotificationTemplatesRequest) ([]*pb.NotificationTemplate, int32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, 0, err
	}
	if req == nil {
		req = &pb.ListNotificationTemplatesRequest{}
	}
	query := r.Data.DB(ctx).NotificationTemplate.Query()
	if keyword := strings.TrimSpace(req.GetKeyword()); keyword != "" {
		query.Where(notificationtemplate.Or(
			notificationtemplate.CodeContains(keyword),
			notificationtemplate.NameContains(keyword),
			notificationtemplate.TitleContains(keyword),
		))
	}
	if req.Channel != nil && req.GetChannel() != pb.NotificationChannel_NOTIFICATION_CHANNEL_UNSPECIFIED {
		query.Where(notificationtemplate.ChannelEQ(int32(req.GetChannel())))
	}
	if req.Status != nil && req.GetStatus() != enum.Status_STATUS_UNSPECIFIED {
		query.Where(notificationtemplate.StatusEQ(int32(req.GetStatus())))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Desc(notificationtemplate.FieldCreatedAt)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return convert.SliceToAny(rows, notificationTemplateProto), int32(total), nil
}

func (r *notificationRepo) GetTemplate(ctx context.Context, id uint32) (*pb.NotificationTemplate, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	row, err := r.Data.DB(ctx).NotificationTemplate.Get(ctx, id)
	if err = r.MapNotFound(err, "NOTIFICATION_TEMPLATE_NOT_FOUND", "通知模板不存在"); err != nil {
		return nil, err
	}
	return notificationTemplateProto(row), nil
}

func (r *notificationRepo) GetEnabledTemplateByCode(ctx context.Context, tenantID uint32, code string) (*pb.NotificationTemplate, error) {
	if tenantID == 0 {
		return nil, errors.Forbidden("TENANT_CONTEXT_REQUIRED", "缺少有效的数据租户上下文")
	}
	ctx = entviewer.NewTenantContext(ctx, tenantID)
	row, err := r.Data.DB(ctx).NotificationTemplate.Query().
		Where(
			notificationtemplate.CodeEQ(code),
			notificationtemplate.ChannelEQ(int32(pb.NotificationChannel_NOTIFICATION_CHANNEL_IN_APP)),
			notificationtemplate.StatusEQ(int32(enum.Status_STATUS_ENABLED)),
		).
		Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("NOTIFICATION_TEMPLATE_NOT_FOUND", "通知模板不存在或未启用")
	}
	if err != nil {
		return nil, err
	}
	return notificationTemplateProto(row), nil
}

func (r *notificationRepo) CreateTemplate(ctx context.Context, value *pb.NotificationTemplate) (*pb.NotificationTemplate, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	row, err := r.Data.DB(ctx).NotificationTemplate.Create().
		SetCode(value.GetCode()).
		SetName(value.GetName()).
		SetChannel(int32(value.GetChannel())).
		SetTitle(value.GetTitle()).
		SetContent(value.GetContent()).
		SetVariableSchema(value.GetVariableSchema()).
		SetLocale(convert.DefaultString(value.GetLocale(), "zh-CN")).
		SetStatus(int32(value.GetStatus())).
		SetRemark(value.GetRemark()).
		Save(ctx)
	if err = r.MapConstraint(err, "NOTIFICATION_TEMPLATE_CODE_EXISTS", "通知模板编码已存在"); err != nil {
		return nil, err
	}
	return notificationTemplateProto(row), nil
}

func (r *notificationRepo) UpdateTemplate(ctx context.Context, value *pb.NotificationTemplate) (*pb.NotificationTemplate, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	row, err := r.Data.DB(ctx).NotificationTemplate.UpdateOneID(value.GetId()).
		SetCode(value.GetCode()).
		SetName(value.GetName()).
		SetChannel(int32(value.GetChannel())).
		SetTitle(value.GetTitle()).
		SetContent(value.GetContent()).
		SetVariableSchema(value.GetVariableSchema()).
		SetLocale(convert.DefaultString(value.GetLocale(), "zh-CN")).
		SetStatus(int32(value.GetStatus())).
		SetRemark(value.GetRemark()).
		Save(ctx)
	if err = r.MapConstraint(err, "NOTIFICATION_TEMPLATE_CODE_EXISTS", "通知模板编码已存在"); err != nil {
		return nil, err
	}
	if err = r.MapNotFound(err, "NOTIFICATION_TEMPLATE_NOT_FOUND", "通知模板不存在"); err != nil {
		return nil, err
	}
	return notificationTemplateProto(row), nil
}

func (r *notificationRepo) DeleteTemplate(ctx context.Context, id uint32) error {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return err
	}
	err := r.Data.DB(ctx).NotificationTemplate.DeleteOneID(id).Exec(ctx)
	if gen.IsNotFound(err) {
		return errors.NotFound("NOTIFICATION_TEMPLATE_NOT_FOUND", "通知模板不存在")
	}
	return err
}

func (r *notificationRepo) CreateMessages(ctx context.Context, values []*pb.NotificationMessage) (int, error) {
	if len(values) == 0 {
		return 0, nil
	}
	tenantID := values[0].GetTenantId()
	if tenantID == 0 {
		return 0, errors.Forbidden("TENANT_CONTEXT_REQUIRED", "缺少有效的数据租户上下文")
	}
	ctx = entviewer.NewTenantContext(ctx, tenantID)
	builders := make([]*gen.NotificationMessageCreate, 0, len(values))
	for _, value := range values {
		if value == nil || value.GetRecipientUserId() == 0 {
			continue
		}
		if value.GetTenantId() != tenantID {
			return 0, errors.Forbidden("NOTIFICATION_TENANT_MISMATCH", "通知消息租户不一致")
		}
		builders = append(builders, r.Data.DB(ctx).NotificationMessage.Create().
			SetRecipientUserID(value.GetRecipientUserId()).
			SetNillableTemplateID(value.TemplateId).
			SetTemplateCode(value.GetTemplateCode()).
			SetChannel(int32(value.GetChannel())).
			SetTitle(value.GetTitle()).
			SetContent(value.GetContent()).
			SetMessageStatus(int32(value.GetStatus())).
			SetPriority(value.GetPriority()).
			SetBusinessType(value.GetBusinessType()).
			SetBusinessID(value.GetBusinessId()).
			SetNillableSenderUserID(value.SenderUserId).
			SetSenderName(value.GetSenderName()))
	}
	if len(builders) == 0 {
		return 0, nil
	}
	if err := r.Data.DB(ctx).NotificationMessage.CreateBulk(builders...).Exec(ctx); err != nil {
		return 0, err
	}
	return len(builders), nil
}

func (r *notificationRepo) ListMessages(ctx context.Context, req *pb.ListNotificationMessagesRequest) ([]*pb.NotificationMessage, int32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, 0, err
	}
	if req == nil {
		req = &pb.ListNotificationMessagesRequest{}
	}
	query := r.Data.DB(ctx).NotificationMessage.Query()
	if req.RecipientUserId != nil {
		query.Where(notificationmessage.RecipientUserIDEQ(req.GetRecipientUserId()))
	}
	if req.Status != nil && req.GetStatus() != pb.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_UNSPECIFIED {
		query.Where(notificationmessage.MessageStatusEQ(int32(req.GetStatus())))
	}
	if value := strings.TrimSpace(req.GetBusinessType()); value != "" {
		query.Where(notificationmessage.BusinessTypeEQ(value))
	}
	if req.Channel != nil && req.GetChannel() != pb.NotificationChannel_NOTIFICATION_CHANNEL_UNSPECIFIED {
		query.Where(notificationmessage.ChannelEQ(int32(req.GetChannel())))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Desc(notificationmessage.FieldCreatedAt)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return convert.SliceToAny(rows, notificationMessageProto), int32(total), nil
}

func (r *notificationRepo) GetMessage(ctx context.Context, id uint32) (*pb.NotificationMessage, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	row, err := r.Data.DB(ctx).NotificationMessage.Get(ctx, id)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("NOTIFICATION_MESSAGE_NOT_FOUND", "通知消息不存在")
	}
	if err != nil {
		return nil, err
	}
	return notificationMessageProto(row), nil
}

func (r *notificationRepo) CountUnread(ctx context.Context, recipientUserID uint32) (int32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	count, err := r.Data.DB(ctx).NotificationMessage.Query().
		Where(
			notificationmessage.RecipientUserIDEQ(recipientUserID),
			notificationmessage.MessageStatusEQ(int32(pb.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_UNREAD)),
		).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *notificationRepo) MarkRead(ctx context.Context, ids []uint32, recipientUserID uint32) error {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return err
	}
	now := time.Now()
	query := r.Data.DB(ctx).NotificationMessage.Update().
		Where(
			notificationmessage.RecipientUserIDEQ(recipientUserID),
			notificationmessage.MessageStatusEQ(int32(pb.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_UNREAD)),
		).
		SetMessageStatus(int32(pb.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_READ)).
		SetReadAt(now)
	if len(ids) > 0 {
		query.Where(notificationmessage.IDIn(ids...))
	}
	_, err := query.Save(ctx)
	return err
}
