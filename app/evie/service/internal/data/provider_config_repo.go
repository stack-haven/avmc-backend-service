package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/asrproviderconfig"
)

type providerConfigRepo struct{ BaseRepo }

// NewProviderConfigRepo 创建供应商配置仓库。
func NewProviderConfigRepo(data *Data, logger log.Logger) biz.ProviderRepo {
	return &providerConfigRepo{BaseRepo: NewBaseRepo(data, logger)}
}

// providerConfigProto converts an Ent AsrProviderConfig to a proto TenantProviderConfig.
func providerConfigProto(row *gen.AsrProviderConfig) *pb.TenantProviderConfig {
	return &pb.TenantProviderConfig{
		ProviderName: row.ProviderName,
		IsActive:     row.IsActive,
		ConfigJson:   row.ConfigJSON,
		SampleRate:   int32(row.SampleRate),
		Language:     row.Language,
	}
}

// ListConfig 查询租户已配置的供应商。
func (r *providerConfigRepo) ListConfig(ctx context.Context) ([]*pb.TenantProviderConfig, error) {
	rows, err := r.Data.DB(ctx).AsrProviderConfig.Query().
		Order(gen.Asc(asrproviderconfig.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*pb.TenantProviderConfig, 0, len(rows))
	for _, row := range rows {
		result = append(result, providerConfigProto(row))
	}
	return result, nil
}

// UpsertConfig 新增或更新租户供应商配置。
func (r *providerConfigRepo) UpsertConfig(ctx context.Context, config *pb.TenantProviderConfig) (*pb.TenantProviderConfig, error) {
	existing, err := r.Data.DB(ctx).AsrProviderConfig.Query().
		Where(asrproviderconfig.ProviderNameEQ(config.GetProviderName())).
		Only(ctx)
	if err == nil {
		row, err := existing.Update().
			SetIsActive(config.GetIsActive()).
			SetConfigJSON(config.GetConfigJson()).
			SetSampleRate(int(config.GetSampleRate())).
			SetLanguage(config.GetLanguage()).
			Save(ctx)
		if err != nil {
			return nil, err
		}
		return providerConfigProto(row), nil
	}
	if !gen.IsNotFound(err) {
		return nil, err
	}
	row, err := r.Data.DB(ctx).AsrProviderConfig.Create().
		SetProviderName(config.GetProviderName()).
		SetIsActive(config.GetIsActive()).
		SetConfigJSON(config.GetConfigJson()).
		SetSampleRate(int(config.GetSampleRate())).
		SetLanguage(config.GetLanguage()).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return providerConfigProto(row), nil
}
