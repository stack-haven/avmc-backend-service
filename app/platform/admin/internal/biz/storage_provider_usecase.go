package biz

import (
	"context"
	"fmt"
	"strings"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/pkg/objectstorage"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

const (
	StorageProviderTypeS3Compatible = "s3-compatible"
	StorageProviderTypeLocal        = "local"

	StorageProviderStatusEnabled  int32 = 1
	StorageProviderStatusDisabled int32 = 2

	defaultStorageProviderBucket = "tenant-files"
)

type ResolvedStorageProvider struct {
	ID            uint32
	Code          string
	Type          string
	DefaultBucket string
	Client        objectstorage.Client
}

type StorageProviderResolver interface {
	ResolveDefault(context.Context) (*ResolvedStorageProvider, error)
	ResolveSnapshot(context.Context, uint32, string, string) (*ResolvedStorageProvider, error)
}

type StorageProviderRepo interface {
	StorageProviderResolver
	Create(context.Context, *pbCore.StorageProvider) (*pbCore.StorageProvider, error)
	Update(context.Context, *pbCore.StorageProvider) (*pbCore.StorageProvider, error)
	Delete(context.Context, uint32) error
	Get(context.Context, uint32) (*pbCore.StorageProvider, error)
	List(context.Context, *pbCore.ListStorageProvidersRequest) ([]*pbCore.StorageProvider, int32, error)
	SetDefault(context.Context, uint32) (*pbCore.StorageProvider, error)
	MarkHealth(context.Context, uint32, string) error
	BuildClient(context.Context, *pbCore.StorageProvider) (objectstorage.Client, error)
}

type StorageProviderUsecase struct {
	repo StorageProviderRepo
	log  *log.Helper
}

func NewStorageProviderUsecase(repo StorageProviderRepo, logger log.Logger) *StorageProviderUsecase {
	return &StorageProviderUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *StorageProviderUsecase) Create(ctx context.Context, item *pbCore.StorageProvider) (*pbCore.StorageProvider, error) {
	if err := validateStorageProvider(item, true); err != nil {
		return nil, err
	}
	return uc.repo.Create(ctx, normalizeStorageProvider(item, true))
}

func (uc *StorageProviderUsecase) Update(ctx context.Context, id uint32, item *pbCore.StorageProvider) (*pbCore.StorageProvider, error) {
	if id == 0 {
		return nil, pb.ErrorBadRequest("存储渠道ID不能为空")
	}
	if item == nil {
		return nil, pb.ErrorBadRequest("存储渠道不能为空")
	}
	item.Id = id
	if err := validateStorageProvider(item, false); err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, normalizeStorageProvider(item, false))
}

func (uc *StorageProviderUsecase) Delete(ctx context.Context, id uint32) error {
	if id == 0 {
		return pb.ErrorBadRequest("存储渠道ID不能为空")
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *StorageProviderUsecase) Get(ctx context.Context, id uint32) (*pbCore.StorageProvider, error) {
	if id == 0 {
		return nil, pb.ErrorBadRequest("存储渠道ID不能为空")
	}
	return uc.repo.Get(ctx, id)
}

func (uc *StorageProviderUsecase) List(ctx context.Context, req *pbCore.ListStorageProvidersRequest) ([]*pbCore.StorageProvider, int32, error) {
	if req == nil {
		req = &pbCore.ListStorageProvidersRequest{}
	}
	return uc.repo.List(ctx, req)
}

func (uc *StorageProviderUsecase) SetDefault(ctx context.Context, id uint32) (*pbCore.StorageProvider, error) {
	if id == 0 {
		return nil, pb.ErrorBadRequest("存储渠道ID不能为空")
	}
	return uc.repo.SetDefault(ctx, id)
}

func (uc *StorageProviderUsecase) Test(ctx context.Context, id uint32, item *pbCore.StorageProvider) (*pbCore.TestStorageProviderResponse, error) {
	if id > 0 {
		if _, err := uc.repo.ResolveSnapshot(ctx, id, "", ""); err != nil {
			_ = uc.repo.MarkHealth(ctx, id, "unhealthy")
			return &pbCore.TestStorageProviderResponse{Healthy: false, Message: err.Error()}, nil
		}
		_ = uc.repo.MarkHealth(ctx, id, "healthy")
		return &pbCore.TestStorageProviderResponse{Healthy: true, Message: "ok"}, nil
	}
	if err := validateStorageProvider(item, true); err != nil {
		return nil, err
	}
	provider := normalizeStorageProvider(item, true)
	if _, err := uc.repo.BuildClient(ctx, provider); err != nil {
		return &pbCore.TestStorageProviderResponse{Healthy: false, Message: err.Error()}, nil
	}
	return &pbCore.TestStorageProviderResponse{Healthy: true, Message: "ok"}, nil
}

func validateStorageProvider(item *pbCore.StorageProvider, create bool) error {
	if item == nil {
		return pb.ErrorBadRequest("存储渠道不能为空")
	}
	if strings.TrimSpace(item.GetCode()) == "" {
		return pb.ErrorBadRequest("存储渠道编码不能为空")
	}
	if strings.TrimSpace(item.GetName()) == "" {
		return pb.ErrorBadRequest("存储渠道名称不能为空")
	}
	providerType := normalizeStorageProviderType(item.GetType())
	switch providerType {
	case StorageProviderTypeS3Compatible:
		if strings.TrimSpace(item.GetEndpoint()) == "" {
			return pb.ErrorBadRequest("S3 endpoint 不能为空")
		}
		if create && strings.TrimSpace(item.GetSecretKey()) == "" {
			return pb.ErrorBadRequest("S3 secret key 不能为空")
		}
	case StorageProviderTypeLocal:
		if strings.TrimSpace(item.GetLocalBasePath()) == "" {
			return pb.ErrorBadRequest("本地存储根目录不能为空")
		}
	default:
		return errors.BadRequest("STORAGE_PROVIDER_TYPE_UNSUPPORTED", fmt.Sprintf("不支持的存储类型: %s", item.GetType()))
	}
	return nil
}

func normalizeStorageProvider(item *pbCore.StorageProvider, create bool) *pbCore.StorageProvider {
	clone := *item
	clone.Code = strings.TrimSpace(clone.Code)
	clone.Name = strings.TrimSpace(clone.Name)
	clone.Type = normalizeStorageProviderType(clone.Type)
	if clone.Status == nil || clone.GetStatus() == 0 {
		clone.Status = int32Ptr(StorageProviderStatusEnabled)
	}
	if strings.TrimSpace(clone.GetDefaultBucket()) == "" {
		clone.DefaultBucket = storageProviderStringPtr(defaultStorageProviderBucket)
	}
	if !create && strings.TrimSpace(clone.GetSecretKey()) == "" {
		clone.SecretKey = nil
	}
	if !create && strings.TrimSpace(clone.GetSessionToken()) == "" {
		clone.SessionToken = nil
	}
	return &clone
}

func normalizeStorageProviderType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "s3" || value == "minio" {
		return StorageProviderTypeS3Compatible
	}
	return value
}

func storageProviderStringPtr(value string) *string { return &value }

func int32Ptr(value int32) *int32 { return &value }
