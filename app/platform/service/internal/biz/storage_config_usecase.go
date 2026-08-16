package biz

import (
	"context"

	pb "backend-service/api/platform/service/v1"

	"backend-service/pkg/aip/listing"
	"backend-service/pkg/objectstorage"

	"github.com/go-kratos/kratos/v2/log"
)

// ───────────────────────────── Repo ─────────────────────────────

// StorageConfigRepo 租户存储配置数据访问
type StorageConfigRepo interface {
	Save(context.Context, *pb.StorageConfig) (*pb.StorageConfig, error)
	Update(context.Context, *pb.StorageConfig) (*pb.StorageConfig, error)
	FindByID(context.Context, uint32) (*pb.StorageConfig, error)
	CountConfigs(context.Context, ...listing.Option) (int32, error)
	ListConfigs(context.Context, ...listing.Option) ([]*pb.StorageConfig, error)
	Delete(context.Context, uint32) error
	SetDefault(context.Context, uint32, uint32) error // tenantID, configID
	MarkHealth(context.Context, uint32, string) error // id, status

	// Resolver
	ResolveDefault(context.Context, uint32) (*ResolvedStorage, error)
	ResolveByPurpose(context.Context, uint32, string) (*ResolvedStorage, error)
}

// ResolvedStorage 运行时已解析的存储客户端
type ResolvedStorage struct {
	ID       uint32
	Name     string
	Provider string
	Bucket   string
	Client   objectstorage.Client
}

// ───────────────────────────── Usecase ─────────────────────────────

// StorageConfigUsecase 租户存储配置业务逻辑
type StorageConfigUsecase struct {
	repo StorageConfigRepo
	log  *log.Helper
}

func NewStorageConfigUsecase(repo StorageConfigRepo, logger log.Logger) *StorageConfigUsecase {
	return &StorageConfigUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *StorageConfigUsecase) Create(ctx context.Context, g *pb.StorageConfig) (*pb.StorageConfig, error) {
	uc.log.WithContext(ctx).Infof("CreateStorageConfig: %s", g.GetName())
	return uc.repo.Save(ctx, g)
}

func (uc *StorageConfigUsecase) Update(ctx context.Context, g *pb.StorageConfig) (*pb.StorageConfig, error) {
	uc.log.WithContext(ctx).Infof("UpdateStorageConfig: %d", g.GetId())
	if _, err := uc.repo.FindByID(ctx, g.GetId()); err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, g)
}

func (uc *StorageConfigUsecase) Get(ctx context.Context, id uint32) (*pb.StorageConfig, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *StorageConfigUsecase) CountConfigs(ctx context.Context, opts ...listing.Option) (int32, error) {
	return uc.repo.CountConfigs(ctx, opts...)
}

func (uc *StorageConfigUsecase) ListConfigs(ctx context.Context, opts ...listing.Option) ([]*pb.StorageConfig, error) {
	return uc.repo.ListConfigs(ctx, opts...)
}

func (uc *StorageConfigUsecase) Delete(ctx context.Context, id uint32) error {
	uc.log.WithContext(ctx).Infof("DeleteStorageConfig: %d", id)
	if _, err := uc.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *StorageConfigUsecase) SetDefault(ctx context.Context, tenantID uint32, id uint32) error {
	uc.log.WithContext(ctx).Infof("SetDefaultStorageConfig: tenant=%d id=%d", tenantID, id)
	if _, err := uc.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return uc.repo.SetDefault(ctx, tenantID, id)
}

// ───────────────────────────── Resolver ─────────────────────────────

// StorageResolver 为业务层提供租户存储客户端
type StorageResolver struct {
	repo StorageConfigRepo
}

func NewStorageResolver(repo StorageConfigRepo) *StorageResolver {
	return &StorageResolver{repo: repo}
}

func (r *StorageResolver) Resolve(ctx context.Context, tenantID uint32) (*ResolvedStorage, error) {
	return r.repo.ResolveDefault(ctx, tenantID)
}

func (r *StorageResolver) ResolveByPurpose(ctx context.Context, tenantID uint32, purpose string) (*ResolvedStorage, error) {
	return r.repo.ResolveByPurpose(ctx, tenantID, purpose)
}
