package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
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

func (s *DeviceServiceService) RegisterDevice(ctx context.Context, req *pb.RegisterDeviceRequest) (*pb.Device, error) {
	return s.uc.Register(ctx, req)
}

func (s *DeviceServiceService) UnregisterDevice(ctx context.Context, req *pb.UnregisterDeviceRequest) (*pb.UnregisterDeviceResponse, error) {
	if err := s.uc.Unregister(ctx, req.GetDeviceToken()); err != nil {
		return nil, err
	}
	return &pb.UnregisterDeviceResponse{}, nil
}

func (s *DeviceServiceService) ListMyDevices(ctx context.Context, _ *pb.ListMyDevicesRequest) (*pb.ListMyDevicesResponse, error) {
	items, err := s.uc.ListMyDevices(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ListMyDevicesResponse{Items: items}, nil
}

func (s *DeviceServiceService) ListUserDevices(ctx context.Context, req *pb.ListUserDevicesRequest) (*pb.ListUserDevicesResponse, error) {
	items, err := s.uc.ListUserDevices(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	return &pb.ListUserDevicesResponse{Items: items}, nil
}
