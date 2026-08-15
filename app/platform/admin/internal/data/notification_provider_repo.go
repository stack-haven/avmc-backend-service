package data

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/notificationprovider"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"

	// 触发通知提供商注册（按 provider_type 分发的渠道提供商）
	_ "backend-service/pkg/notifier/sms/aliyun"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var _ biz.NotificationProviderRepo = (*notificationProviderRepo)(nil)

type notificationProviderRepo struct {
	BaseRepo
}

func NewNotificationProviderRepo(data *Data, logger log.Logger) biz.NotificationProviderRepo {
	return &notificationProviderRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func notificationProviderProto(row *gen.NotificationProvider, includeSecret bool) *pbCore.NotificationProvider {
	if row == nil {
		return nil
	}
	status := row.Status
	if status == nil {
		status = convert.ToPointer(biz.StorageProviderStatusEnabled)
	}
	isDefault := row.IsDefault
	secretConfigured := row.AccessKeySecret != ""
	item := &pbCore.NotificationProvider{
		Id:               row.ID,
		Code:             row.Code,
		Name:             row.Name,
		Channel:          row.Channel,
		ProviderType:     &row.ProviderType,
		Endpoint:         &row.Endpoint,
		AccessKeyId:      &row.AccessKeyID,
		SignName:         &row.SignName,
		TemplateCode:     &row.TemplateCode,
		Status:           status,
		IsDefault:        &isDefault,
		Remark:           &row.Remark,
		SecretConfigured: &secretConfigured,
		CreatedAt:        convert.TimeValueToString(&row.CreatedAt, time.DateTime),
		UpdatedAt:        convert.TimeValueToString(&row.UpdatedAt, time.DateTime),
	}
	if includeSecret {
		item.AccessKeySecret = &row.AccessKeySecret
	}
	return item
}

func (r *notificationProviderRepo) Create(ctx context.Context, item *pbCore.NotificationProvider) (*pbCore.NotificationProvider, error) {
	systemCtx := entviewer.NewSystemContext(ctx)
	var result *pbCore.NotificationProvider
	err := r.Data.InTx(systemCtx, func(txCtx context.Context) error {
		if item.GetIsDefault() {
			if _, err := r.Data.DB(txCtx).NotificationProvider.Update().SetIsDefault(false).Save(txCtx); err != nil {
				return err
			}
		}
		row, err := r.Data.DB(txCtx).NotificationProvider.Create().
			SetCode(item.GetCode()).
			SetName(item.GetName()).
			SetChannel(item.GetChannel()).
			SetProviderType(item.GetProviderType()).
			SetEndpoint(item.GetEndpoint()).
			SetAccessKeyID(item.GetAccessKeyId()).
			SetAccessKeySecret(item.GetAccessKeySecret()).
			SetSignName(item.GetSignName()).
			SetTemplateCode(item.GetTemplateCode()).
			SetStatus(item.GetStatus()).
			SetIsDefault(item.GetIsDefault()).
			SetRemark(item.GetRemark()).
			Save(txCtx)
		if gen.IsConstraintError(err) {
			return errors.Conflict("NOTIFICATION_PROVIDER_CODE_EXISTS", "渠道配置编码已存在")
		}
		if err != nil {
			return err
		}
		result = notificationProviderProto(row, false)
		return nil
	})
	return result, err
}

func (r *notificationProviderRepo) Update(ctx context.Context, item *pbCore.NotificationProvider) (*pbCore.NotificationProvider, error) {
	systemCtx := entviewer.NewSystemContext(ctx)
	var result *pbCore.NotificationProvider
	err := r.Data.InTx(systemCtx, func(txCtx context.Context) error {
		old, err := r.getRow(txCtx, item.GetId())
		if err != nil {
			return err
		}
		if item.GetIsDefault() {
			if _, err = r.Data.DB(txCtx).NotificationProvider.Update().Where(notificationprovider.IDNEQ(item.GetId())).SetIsDefault(false).Save(txCtx); err != nil {
				return err
			}
		}
		update := old.Update().
			SetCode(item.GetCode()).
			SetName(item.GetName()).
			SetChannel(item.GetChannel()).
			SetProviderType(item.GetProviderType()).
			SetEndpoint(item.GetEndpoint()).
			SetAccessKeyID(item.GetAccessKeyId()).
			SetSignName(item.GetSignName()).
			SetTemplateCode(item.GetTemplateCode()).
			SetStatus(item.GetStatus()).
			SetIsDefault(item.GetIsDefault()).
			SetRemark(item.GetRemark())
		if item.AccessKeySecret != nil {
			update.SetAccessKeySecret(item.GetAccessKeySecret())
		}
		row, err := update.Save(txCtx)
		if gen.IsConstraintError(err) {
			return errors.Conflict("NOTIFICATION_PROVIDER_CODE_EXISTS", "渠道配置编码已存在")
		}
		if err != nil {
			return err
		}
		result = notificationProviderProto(row, false)
		return nil
	})
	return result, err
}

func (r *notificationProviderRepo) Delete(ctx context.Context, id uint32) error {
	systemCtx := entviewer.NewSystemContext(ctx)
	row, err := r.getRow(systemCtx, id)
	if err != nil {
		return err
	}
	return row.Update().
		SetDeletedAt(time.Now()).
		SetStatus(biz.StorageProviderStatusDisabled).
		SetIsDefault(false).
		Exec(systemCtx)
}

func (r *notificationProviderRepo) Get(ctx context.Context, id uint32) (*pbCore.NotificationProvider, error) {
	row, err := r.getRow(entviewer.NewSystemContext(ctx), id)
	if err != nil {
		return nil, err
	}
	return notificationProviderProto(row, false), nil
}

func (r *notificationProviderRepo) List(ctx context.Context, req *pbCore.ListNotificationProvidersRequest) ([]*pbCore.NotificationProvider, int32, error) {
	systemCtx := entviewer.NewSystemContext(ctx)
	query := r.Data.DB(systemCtx).NotificationProvider.Query().Where(notificationprovider.DeletedAtIsNil())
	if strings.TrimSpace(req.GetCode()) != "" {
		query.Where(notificationprovider.CodeContains(strings.TrimSpace(req.GetCode())))
	}
	if strings.TrimSpace(req.GetName()) != "" {
		query.Where(notificationprovider.NameContains(strings.TrimSpace(req.GetName())))
	}
	if strings.TrimSpace(req.GetChannel()) != "" {
		query.Where(notificationprovider.ChannelEQ(strings.TrimSpace(req.GetChannel())))
	}
	if req.GetStatus() > 0 {
		query.Where(notificationprovider.StatusEQ(req.GetStatus()))
	}
	total, err := query.Clone().Count(systemCtx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Desc(notificationprovider.FieldIsDefault), gen.Asc(notificationprovider.FieldID)).Offset(offset).Limit(size).All(systemCtx)
	if err != nil {
		return nil, 0, err
	}
	return convert.SliceToAny(rows, func(row *gen.NotificationProvider) *pbCore.NotificationProvider {
		return notificationProviderProto(row, false)
	}), int32(total), nil
}

func (r *notificationProviderRepo) SetDefault(ctx context.Context, id uint32) (*pbCore.NotificationProvider, error) {
	systemCtx := entviewer.NewSystemContext(ctx)
	var result *pbCore.NotificationProvider
	err := r.Data.InTx(systemCtx, func(txCtx context.Context) error {
		row, err := r.getRow(txCtx, id)
		if err != nil {
			return err
		}
		if providerStatus(row) != biz.StorageProviderStatusEnabled {
			return errors.Conflict("NOTIFICATION_PROVIDER_DISABLED", "禁用的渠道配置不能设为默认")
		}
		if _, err = r.Data.DB(txCtx).NotificationProvider.Update().Where(notificationprovider.ChannelEQ(row.Channel)).SetIsDefault(false).Save(txCtx); err != nil {
			return err
		}
		row, err = row.Update().SetIsDefault(true).Save(txCtx)
		if err != nil {
			return err
		}
		result = notificationProviderProto(row, false)
		return nil
	})
	return result, err
}

// ResolveChannel 按渠道解析启用配置（默认配置优先），返回含密钥的配置和供应商 configJSON。
func (r *notificationProviderRepo) ResolveChannel(ctx context.Context, channel string) (*pbCore.NotificationProvider, json.RawMessage, error) {
	systemCtx := entviewer.NewSystemContext(ctx)
	row, err := r.Data.DB(systemCtx).NotificationProvider.Query().
		Where(
			notificationprovider.DeletedAtIsNil(),
			notificationprovider.ChannelEQ(channel),
			notificationprovider.StatusEQ(biz.StorageProviderStatusEnabled),
			notificationprovider.IsDefaultEQ(true),
		).
		Only(systemCtx)
	if gen.IsNotFound(err) {
		row, err = r.Data.DB(systemCtx).NotificationProvider.Query().
			Where(
				notificationprovider.DeletedAtIsNil(),
				notificationprovider.ChannelEQ(channel),
				notificationprovider.StatusEQ(biz.StorageProviderStatusEnabled),
			).
			Order(gen.Asc(notificationprovider.FieldID)).
			First(systemCtx)
	}
	if gen.IsNotFound(err) {
		return nil, nil, errors.NotFound("NOTIFICATION_PROVIDER_NOT_FOUND", "未配置可用的通知渠道")
	}
	if err != nil {
		return nil, nil, err
	}
	item := notificationProviderProto(row, true)
	configJSON, err := notificationProviderObjectConfig(item)
	if err != nil {
		return nil, nil, err
	}
	return item, configJSON, nil
}

func (r *notificationProviderRepo) getRow(ctx context.Context, id uint32) (*gen.NotificationProvider, error) {
	row, err := r.Data.DB(ctx).NotificationProvider.Query().
		Where(notificationprovider.IDEQ(id), notificationprovider.DeletedAtIsNil()).
		Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("NOTIFICATION_PROVIDER_NOT_FOUND", "通知渠道配置不存在")
	}
	return row, err
}

// notificationProviderObjectConfig 按 provider_type 生成提供商 configJSON。
func notificationProviderObjectConfig(item *pbCore.NotificationProvider) (json.RawMessage, error) {
	cfg := map[string]interface{}{}
	switch item.GetProviderType() {
	case biz.NotificationProviderAliyunSMS:
		cfg["endpoint"] = item.GetEndpoint()
		cfg["access_key_id"] = item.GetAccessKeyId()
		cfg["access_key_secret"] = item.GetAccessKeySecret()
		cfg["sign_name"] = item.GetSignName()
		cfg["template_code"] = item.GetTemplateCode()
	case "":
		// 站内信无需配置。
	default:
		// 其他提供商（yunpian/jpush/getui）预留：先映射通用字段，未来按需扩展专属字段。
		cfg["endpoint"] = item.GetEndpoint()
		cfg["access_key_id"] = item.GetAccessKeyId()
		cfg["access_key_secret"] = item.GetAccessKeySecret()
		cfg["sign_name"] = item.GetSignName()
		cfg["template_code"] = item.GetTemplateCode()
	}
	return json.Marshal(cfg)
}

func providerStatus(row *gen.NotificationProvider) int32 {
	if row == nil || row.Status == nil {
		return biz.StorageProviderStatusEnabled
	}
	return *row.Status
}
