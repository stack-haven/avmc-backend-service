package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
)

// ProviderServiceService 供应商配置管理服务。
type ProviderServiceService struct {
	pb.UnimplementedProviderServiceServer
	puc *biz.ProviderUsecase
	log *log.Helper
}

// NewProviderServiceService 创建供应商服务实例。
func NewProviderServiceService(puc *biz.ProviderUsecase, logger log.Logger) *ProviderServiceService {
	return &ProviderServiceService{puc: puc, log: log.NewHelper(logger)}
}

// ListAvailableProviders 查询可用供应商列表。
func (s *ProviderServiceService) ListAvailableProviders(ctx context.Context, _ *pb.ListAvailableProvidersRequest) (*pb.ListAvailableProvidersResponse, error) {
	providers, err := s.puc.ListAvailableProviders(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ListAvailableProvidersResponse{Providers: providers}, nil
}

// GetTenantConfig 查询租户供应商配置。
func (s *ProviderServiceService) GetTenantConfig(ctx context.Context, _ *pb.GetTenantConfigRequest) (*pb.GetTenantConfigResponse, error) {
	configs, err := s.puc.GetTenantConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetTenantConfigResponse{Configs: configs}, nil
}

// UpdateTenantConfig 更新租户供应商配置。
func (s *ProviderServiceService) UpdateTenantConfig(ctx context.Context, req *pb.UpdateTenantConfigRequest) (*pb.UpdateTenantConfigResponse, error) {
	config, err := s.puc.UpdateTenantConfig(ctx, req.GetConfig())
	if err != nil {
		return nil, err
	}
	return &pb.UpdateTenantConfigResponse{Config: config}, nil
}
