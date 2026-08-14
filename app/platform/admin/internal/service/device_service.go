package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
)

// DeviceServiceService 设备管理服务。
type DeviceServiceService struct {
	pb.UnimplementedDeviceServiceServer
	uc  *biz.DeviceUsecase
	log *log.Helper
}

func NewDeviceServiceService(uc *biz.DeviceUsecase, logger log.Logger) *DeviceServiceService {
	return &DeviceServiceService{uc: uc, log: log.NewHelper(logger)}
}

func (s *DeviceServiceService) RegisterDevice(ctx context.Context, req *pbCore.RegisterDeviceRequest) (*pbCore.Device, error) {
	return s.uc.Register(ctx, req)
}

func (s *DeviceServiceService) UnregisterDevice(ctx context.Context, req *pbCore.UnregisterDeviceRequest) (*pbCore.UnregisterDeviceResponse, error) {
	if err := s.uc.Unregister(ctx, req.GetDeviceToken()); err != nil {
		return nil, err
	}
	return &pbCore.UnregisterDeviceResponse{}, nil
}

func (s *DeviceServiceService) ListMyDevices(ctx context.Context, _ *pbCore.ListMyDevicesRequest) (*pbCore.ListMyDevicesResponse, error) {
	items, err := s.uc.ListMyDevices(ctx)
	if err != nil {
		return nil, err
	}
	return &pbCore.ListMyDevicesResponse{Items: items}, nil
}

func (s *DeviceServiceService) ListUserDevices(ctx context.Context, req *pbCore.ListUserDevicesRequest) (*pbCore.ListUserDevicesResponse, error) {
	items, err := s.uc.ListUserDevices(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	return &pbCore.ListUserDevicesResponse{Items: items}, nil
}
