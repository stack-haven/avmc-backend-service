package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/device"
	"backend-service/pkg/utils/convert"
)

var _ biz.DeviceRepo = (*deviceRepo)(nil)

type deviceRepo struct {
	BaseRepo
}

func NewDeviceRepo(data *Data, logger log.Logger) biz.DeviceRepo {
	return &deviceRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func deviceToProto(row *gen.Device) *pbCore.Device {
	if row == nil {
		return nil
	}
	return &pbCore.Device{
		Id:           row.ID,
		TenantId:     row.TenantID,
		UserId:       row.UserID,
		DeviceToken:  row.DeviceToken,
		Platform:     row.Platform,
		AppKey:       row.AppKey,
		DeviceName:   &row.DeviceName,
		AppVersion:   &row.AppVersion,
		Status:       row.Status,
		LastActiveAt: convert.TimeValueToString(&row.LastActiveAt, time.DateTime),
		CreatedAt:    convert.TimeValueToString(&row.CreatedAt, time.DateTime),
		UpdatedAt:    convert.TimeValueToString(&row.UpdatedAt, time.DateTime),
	}
}

// Upsert 注册或更新设备：同一租户内 device_token 唯一，重复注册更新绑定用户和活跃时间。
func (r *deviceRepo) Upsert(ctx context.Context, item *pbCore.Device) (*pbCore.Device, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	now := time.Now()
	existing, err := r.Data.DB(ctx).Device.Query().
		Where(device.DeviceTokenEQ(item.GetDeviceToken())).
		Only(ctx)
	if err != nil && !gen.IsNotFound(err) {
		return nil, err
	}
	if existing != nil {
		row, err := existing.Update().
			SetUserID(item.GetUserId()).
			SetPlatform(item.GetPlatform()).
			SetAppKey(item.GetAppKey()).
			SetDeviceName(item.GetDeviceName()).
			SetAppVersion(item.GetAppVersion()).
			SetStatus(item.GetStatus()).
			SetLastActiveAt(now).
			Save(ctx)
		if err != nil {
			return nil, err
		}
		return deviceToProto(row), nil
	}
	row, err := r.Data.DB(ctx).Device.Create().
		SetUserID(item.GetUserId()).
		SetDeviceToken(item.GetDeviceToken()).
		SetPlatform(item.GetPlatform()).
		SetAppKey(item.GetAppKey()).
		SetDeviceName(item.GetDeviceName()).
		SetAppVersion(item.GetAppVersion()).
		SetStatus(item.GetStatus()).
		SetLastActiveAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return deviceToProto(row), nil
}

func (r *deviceRepo) DeleteByToken(ctx context.Context, deviceToken string) error {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return err
	}
	_, err := r.Data.DB(ctx).Device.Delete().
		Where(device.DeviceTokenEQ(deviceToken)).
		Exec(ctx)
	return err
}

func (r *deviceRepo) ListByUser(ctx context.Context, userID uint32) ([]*pbCore.Device, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	rows, err := r.Data.DB(ctx).Device.Query().
		Where(device.UserIDEQ(userID)).
		Order(gen.Desc(device.FieldLastActiveAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(rows, deviceToProto), nil
}
