package biz

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/platform/service/v1"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"
)

// DeviceRepo 设备注册仓储。
type DeviceRepo interface {
	// Upsert 注册或更新设备（device_token 唯一，重复注册则更新绑定用户和活跃时间）。
	Upsert(context.Context, *pb.Device) (*pb.Device, error)
	// DeleteByToken 按设备令牌解绑（仅当前租户）。
	DeleteByToken(context.Context, string) error
	// ListByUser 查询用户设备列表。
	ListByUser(context.Context, uint32) ([]*pb.Device, error)
}

// DeviceUsecase 设备管理业务逻辑。
type DeviceUsecase struct {
	repo DeviceRepo
	log  *log.Helper
}

func NewDeviceUsecase(repo DeviceRepo, logger log.Logger) *DeviceUsecase {
	return &DeviceUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Register 注册当前用户的设备（APP 登录后上报 device token）。
func (uc *DeviceUsecase) Register(ctx context.Context, req *pb.RegisterDeviceRequest) (*pb.Device, error) {
	if req == nil || strings.TrimSpace(req.GetDeviceToken()) == "" {
		return nil, errors.BadRequest("DEVICE_TOKEN_REQUIRED", "设备令牌不能为空")
	}
	platform := strings.TrimSpace(strings.ToLower(req.GetPlatform()))
	if platform != "android" && platform != "ios" {
		return nil, errors.BadRequest("DEVICE_PLATFORM_INVALID", "平台类型必须为 android 或 ios")
	}
	userID := authn.GetAuthUserID(ctx)
	if userID == 0 {
		return nil, errors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	tenantID, err := currentTenantID(ctx)
	if err != nil {
		return nil, err
	}
	device := &pb.Device{
		TenantId:    tenantID,
		UserId:      userID,
		DeviceToken: strings.TrimSpace(req.GetDeviceToken()),
		Platform:    platform,
		AppKey:      strings.TrimSpace(req.GetAppKey()),
		DeviceName:  convert.EmptyToNil(req.GetDeviceName()),
		AppVersion:  convert.EmptyToNil(req.GetAppVersion()),
		Status:      convert.ToPointer(int32(1)),
	}
	return uc.repo.Upsert(ctx, device)
}

// Unregister 解绑当前用户的设备（登出/切换账号）。
func (uc *DeviceUsecase) Unregister(ctx context.Context, deviceToken string) error {
	if strings.TrimSpace(deviceToken) == "" {
		return errors.BadRequest("DEVICE_TOKEN_REQUIRED", "设备令牌不能为空")
	}
	return uc.repo.DeleteByToken(ctx, strings.TrimSpace(deviceToken))
}

// ListMyDevices 当前用户的设备列表。
func (uc *DeviceUsecase) ListMyDevices(ctx context.Context) ([]*pb.Device, error) {
	userID := authn.GetAuthUserID(ctx)
	if userID == 0 {
		return nil, errors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	return uc.repo.ListByUser(ctx, userID)
}

// ListUserDevices 按用户查询设备（推送时用，管理后台）。
func (uc *DeviceUsecase) ListUserDevices(ctx context.Context, userID uint32) ([]*pb.Device, error) {
	if userID == 0 {
		return nil, errors.BadRequest("DEVICE_USER_REQUIRED", "用户ID不能为空")
	}
	return uc.repo.ListByUser(ctx, userID)
}
