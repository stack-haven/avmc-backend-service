package biz

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/proto"

	pb "backend-service/api/platform/service/v1"

	"backend-service/pkg/objectstorage"
	"backend-service/pkg/utils/convert"
)

const (
	StorageProviderTypeS3Compatible = "s3-compatible"
	StorageProviderTypeLocal        = "local"
	StorageProviderTypeAliyunOSS    = "aliyun-oss"
	StorageProviderTypeQiniuKodo    = "qiniu-kodo"
	StorageProviderTypeTencentCOS   = "tencent-cos"

	StorageProviderStatusEnabled  int32 = 1
	StorageProviderStatusDisabled int32 = 2

	defaultStorageProviderBucket = "tenant-files"
)

// SupportedStorageProviderTypes 返回全部受支持的存储渠道类型。
func SupportedStorageProviderTypes() []string {
	return []string{
		StorageProviderTypeS3Compatible,
		StorageProviderTypeLocal,
		StorageProviderTypeAliyunOSS,
		StorageProviderTypeQiniuKodo,
		StorageProviderTypeTencentCOS,
	}
}

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
	Create(context.Context, *pb.StorageProvider) (*pb.StorageProvider, error)
	Update(context.Context, *pb.StorageProvider) (*pb.StorageProvider, error)
	Delete(context.Context, uint32) error
	Get(context.Context, uint32) (*pb.StorageProvider, error)
	List(context.Context, *pb.ListStorageProvidersRequest) ([]*pb.StorageProvider, int32, error)
	SetDefault(context.Context, uint32) (*pb.StorageProvider, error)
	MarkHealth(context.Context, uint32, string) error
	BuildClient(context.Context, *pb.StorageProvider) (objectstorage.Client, error)
}

type StorageProviderUsecase struct {
	repo StorageProviderRepo
	log  *log.Helper
}

func NewStorageProviderUsecase(repo StorageProviderRepo, logger log.Logger) *StorageProviderUsecase {
	return &StorageProviderUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *StorageProviderUsecase) Create(ctx context.Context, item *pb.StorageProvider) (*pb.StorageProvider, error) {
	if err := validateStorageProvider(item, true); err != nil {
		return nil, err
	}
	return uc.repo.Create(ctx, normalizeStorageProvider(item, true))
}

func (uc *StorageProviderUsecase) Update(ctx context.Context, id uint32, item *pb.StorageProvider) (*pb.StorageProvider, error) {
	if id == 0 {
		return nil, errors.BadRequest("STORAGE_PROVIDER_ID_REQUIRED", "存储渠道ID不能为空")
	}
	if item == nil {
		return nil, errors.BadRequest("STORAGE_PROVIDER_REQUIRED", "存储渠道不能为空")
	}
	item.Id = id
	if err := validateStorageProvider(item, false); err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, normalizeStorageProvider(item, false))
}

func (uc *StorageProviderUsecase) Delete(ctx context.Context, id uint32) error {
	if id == 0 {
		return errors.BadRequest("STORAGE_PROVIDER_ID_REQUIRED", "存储渠道ID不能为空")
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *StorageProviderUsecase) Get(ctx context.Context, id uint32) (*pb.StorageProvider, error) {
	if id == 0 {
		return nil, errors.BadRequest("STORAGE_PROVIDER_ID_REQUIRED", "存储渠道ID不能为空")
	}
	return uc.repo.Get(ctx, id)
}

func (uc *StorageProviderUsecase) List(ctx context.Context, req *pb.ListStorageProvidersRequest) ([]*pb.StorageProvider, int32, error) {
	if req == nil {
		req = &pb.ListStorageProvidersRequest{}
	}
	return uc.repo.List(ctx, req)
}

func (uc *StorageProviderUsecase) SetDefault(ctx context.Context, id uint32) (*pb.StorageProvider, error) {
	if id == 0 {
		return nil, errors.BadRequest("STORAGE_PROVIDER_ID_REQUIRED", "存储渠道ID不能为空")
	}
	return uc.repo.SetDefault(ctx, id)
}

func (uc *StorageProviderUsecase) Test(ctx context.Context, id uint32, item *pb.StorageProvider) (*pb.TestStorageProviderResponse, error) {
	if id > 0 {
		if _, err := uc.repo.ResolveSnapshot(ctx, id, "", ""); err != nil {
			if markErr := uc.repo.MarkHealth(ctx, id, "unhealthy"); markErr != nil {
				uc.log.WithContext(ctx).Warnf("mark storage provider unhealthy failed: %v", markErr)
			}
			//nolint:nilerr // Test wraps error into response, not propagation
			return &pb.TestStorageProviderResponse{Healthy: false, Message: err.Error()}, nil
		}
		if err := uc.repo.MarkHealth(ctx, id, "healthy"); err != nil {
			uc.log.WithContext(ctx).Warnf("mark storage provider healthy failed: %v", err)
		}
		return &pb.TestStorageProviderResponse{Healthy: true, Message: "ok"}, nil
	}
	if err := validateStorageProvider(item, true); err != nil {
		return nil, err
	}
	provider := normalizeStorageProvider(item, true)
	if _, err := uc.repo.BuildClient(ctx, provider); err != nil {
		//nolint:nilerr // Test wraps error into response, not propagation
		return &pb.TestStorageProviderResponse{Healthy: false, Message: err.Error()}, nil
	}
	return &pb.TestStorageProviderResponse{Healthy: true, Message: "ok"}, nil
}

func validateStorageProvider(item *pb.StorageProvider, create bool) error {
	if item == nil {
		return errors.BadRequest("STORAGE_PROVIDER_REQUIRED", "存储渠道不能为空")
	}
	if strings.TrimSpace(item.GetCode()) == "" {
		return errors.BadRequest("STORAGE_PROVIDER_CODE_REQUIRED", "存储渠道编码不能为空")
	}
	if strings.TrimSpace(item.GetName()) == "" {
		return errors.BadRequest("STORAGE_PROVIDER_NAME_REQUIRED", "存储渠道名称不能为空")
	}
	providerType := normalizeStorageProviderType(item.GetType())
	switch providerType {
	case StorageProviderTypeS3Compatible:
		if strings.TrimSpace(item.GetEndpoint()) == "" {
			return errors.BadRequest("STORAGE_PROVIDER_ENDPOINT_REQUIRED", "S3 endpoint 不能为空")
		}
		if create && strings.TrimSpace(item.GetSecretKey()) == "" {
			return errors.BadRequest("STORAGE_PROVIDER_SECRET_KEY_REQUIRED", "S3 secret key 不能为空")
		}
	case StorageProviderTypeLocal:
		if strings.TrimSpace(item.GetLocalBasePath()) == "" {
			return errors.BadRequest("STORAGE_PROVIDER_LOCAL_PATH_REQUIRED", "本地存储根目录不能为空")
		}
	case StorageProviderTypeAliyunOSS:
		if strings.TrimSpace(item.GetEndpoint()) == "" {
			return errors.BadRequest("STORAGE_PROVIDER_ENDPOINT_REQUIRED", "阿里云 OSS endpoint 不能为空")
		}
		if create && (strings.TrimSpace(item.GetAccessKey()) == "" || strings.TrimSpace(item.GetSecretKey()) == "") {
			return errors.BadRequest("STORAGE_PROVIDER_SECRET_KEY_REQUIRED", "阿里云 OSS access key 和 secret key 不能为空")
		}
	case StorageProviderTypeQiniuKodo:
		if create && (strings.TrimSpace(item.GetAccessKey()) == "" || strings.TrimSpace(item.GetSecretKey()) == "") {
			return errors.BadRequest("STORAGE_PROVIDER_SECRET_KEY_REQUIRED", "七牛 Kodo access key 和 secret key 不能为空")
		}
	case StorageProviderTypeTencentCOS:
		if strings.TrimSpace(item.GetRegion()) == "" {
			return errors.BadRequest("STORAGE_PROVIDER_REGION_REQUIRED", "腾讯云 COS region 不能为空")
		}
		if create && (strings.TrimSpace(item.GetAccessKey()) == "" || strings.TrimSpace(item.GetSecretKey()) == "") {
			return errors.BadRequest("STORAGE_PROVIDER_SECRET_KEY_REQUIRED", "腾讯云 COS secret id 和 secret key 不能为空")
		}
	default:
		return errors.BadRequest("STORAGE_PROVIDER_TYPE_UNSUPPORTED", fmt.Sprintf("不支持的存储类型: %s", item.GetType()))
	}
	return nil
}

func normalizeStorageProvider(item *pb.StorageProvider, create bool) *pb.StorageProvider {
	clone := proto.Clone(item).(*pb.StorageProvider) //nolint:errcheck // proto.Clone does not return error
	clone.Code = strings.TrimSpace(clone.Code)
	clone.Name = strings.TrimSpace(clone.Name)
	clone.Type = normalizeStorageProviderType(clone.Type)
	if clone.Status == nil || clone.GetStatus() == 0 {
		clone.Status = convert.ToPointer(StorageProviderStatusEnabled)
	}
	if strings.TrimSpace(clone.GetDefaultBucket()) == "" {
		clone.DefaultBucket = convert.ToPointer(defaultStorageProviderBucket)
	}
	if !create && strings.TrimSpace(clone.GetSecretKey()) == "" {
		clone.SecretKey = nil
	}
	if !create && strings.TrimSpace(clone.GetSessionToken()) == "" {
		clone.SessionToken = nil
	}
	return clone
}

func normalizeStorageProviderType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "s3", "minio":
		return StorageProviderTypeS3Compatible
	case "oss", "aliyun":
		return StorageProviderTypeAliyunOSS
	case "kodo", "qiniu":
		return StorageProviderTypeQiniuKodo
	case "cos", "tencent":
		return StorageProviderTypeTencentCOS
	default:
		return value
	}
}
