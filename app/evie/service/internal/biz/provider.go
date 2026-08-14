package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
)

// ProviderRepo 供应商配置仓库接口。
type ProviderRepo interface {
	ListConfig(ctx context.Context) ([]*pb.TenantProviderConfig, error)
	UpsertConfig(ctx context.Context, config *pb.TenantProviderConfig) (*pb.TenantProviderConfig, error)
}

// ProviderUsecase 供应商配置业务逻辑。
type ProviderUsecase struct {
	repo ProviderRepo
	log  *log.Helper
}

// NewProviderUsecase 创建供应商 usecase。
func NewProviderUsecase(repo ProviderRepo, logger log.Logger) *ProviderUsecase {
	return &ProviderUsecase{repo: repo, log: log.NewHelper(logger)}
}

// availableProviders 是平台级可用的 ASR 供应商能力声明（硬编码，B2 接入真实 Provider 后改为注册中心）。
var availableProviders = []*pb.ProviderInfo{
	{Name: "funasr", DeploymentMode: "self_hosted", Streaming: true, SupportedFormats: []string{"pcm", "wav"}, SampleRates: []int32{8000, 16000}, HotwordSupport: true},
	{Name: "whisper", DeploymentMode: "self_hosted", Streaming: false, SupportedFormats: []string{"wav", "mp3"}, SampleRates: []int32{16000}, HotwordSupport: false},
	{Name: "xunfei", DeploymentMode: "cloud_api", Streaming: true, SupportedFormats: []string{"pcm", "wav"}, SampleRates: []int32{8000, 16000}, HotwordSupport: true},
	{Name: "aliyun", DeploymentMode: "cloud_api", Streaming: true, SupportedFormats: []string{"pcm", "wav", "opus"}, SampleRates: []int32{8000, 16000}, HotwordSupport: true},
}

// ListAvailableProviders 返回平台可用的供应商能力列表。
func (uc *ProviderUsecase) ListAvailableProviders(_ context.Context) ([]*pb.ProviderInfo, error) {
	return availableProviders, nil
}

// GetTenantConfig 查询租户已配置的供应商。
func (uc *ProviderUsecase) GetTenantConfig(ctx context.Context) ([]*pb.TenantProviderConfig, error) {
	return uc.repo.ListConfig(ctx)
}

// UpdateTenantConfig 更新（upsert）租户供应商配置。
func (uc *ProviderUsecase) UpdateTenantConfig(ctx context.Context, config *pb.TenantProviderConfig) (*pb.TenantProviderConfig, error) {
	if config.GetProviderName() == "" {
		return nil, pb.ErrorBadRequest("供应商名称不能为空")
	}
	if config.GetSampleRate() == 0 {
		config.SampleRate = 16000
	}
	if config.GetLanguage() == "" {
		config.Language = "zh"
	}
	return uc.repo.UpsertConfig(ctx, config)
}
