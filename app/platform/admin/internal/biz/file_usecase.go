package biz

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/objectstorage"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

const (
	FileStatusPending   int32 = 1
	FileStatusConfirmed int32 = 2
	FileStatusDeleted   int32 = 3

	defaultFileVisibility = "private"
	defaultFileBucket     = "tenant-files"
	defaultUploadTTL      = 15 * time.Minute
	defaultDownloadTTL    = 15 * time.Minute
)

type FileRepo interface {
	CreateUploadSession(context.Context, *pbCore.FileObject, string, time.Time) (*pbCore.FileObject, error)
	Get(context.Context, uint32) (*pbCore.FileObject, error)
	FindByIdempotencyKey(context.Context, string) (*pbCore.FileObject, error)
	List(context.Context, ...listing.Option) ([]*pbCore.FileObject, error)
	Count(context.Context, ...listing.Option) (int32, error)
	Confirm(context.Context, uint32, int64, string, string) (*pbCore.FileObject, error)
	Delete(context.Context, uint32) error
}

type FileUsecase struct {
	repo     FileRepo
	resolver StorageProviderResolver
	log      *log.Helper
}

func NewFileUsecase(repo FileRepo, resolver StorageProviderResolver, logger log.Logger) *FileUsecase {
	return &FileUsecase{repo: repo, resolver: resolver, log: log.NewHelper(logger)}
}

func (uc *FileUsecase) CreateUploadSession(ctx context.Context, req *pbCore.CreateFileUploadSessionRequest) (*pbCore.CreateFileUploadSessionResponse, error) {
	if req == nil {
		return nil, pb.ErrorBadRequest("上传请求不能为空")
	}
	fileName := strings.TrimSpace(req.GetFileName())
	if fileName == "" {
		return nil, pb.ErrorBadRequest("文件名不能为空")
	}
	visibility := normalizeFileVisibility(req.GetVisibility())
	idempotencyKey := strings.TrimSpace(req.GetIdempotencyKey())
	if idempotencyKey != "" {
		existing, err := uc.repo.FindByIdempotencyKey(ctx, idempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return uc.uploadSessionResponse(ctx, nil, existing, defaultUploadTTL)
		}
	}

	tenantID, err := currentTenantID(ctx)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().UTC().Add(defaultUploadTTL)
	if uc.resolver == nil {
		return nil, errors.BadRequest("FILE_STORAGE_NOT_CONFIGURED", "未配置可用的文件存储渠道")
	}
	provider, err := uc.resolver.ResolveDefault(ctx)
	if err != nil {
		return nil, err
	}
	bucket := provider.DefaultBucket
	if bucket == "" {
		bucket = defaultFileBucket
	}
	objectKey := buildObjectKey(tenantID, fileName)
	file := &pbCore.FileObject{
		TenantId:     &tenantID,
		FileName:     &fileName,
		ContentType:  fileStringPtr(defaultContentType(req.GetContentType())),
		Size:         fileInt64Ptr(req.GetSize()),
		Sha256:       fileStringPtr(strings.TrimSpace(req.GetSha256())),
		Provider:     fileStringPtr(provider.Type),
		ProviderId:   fileUint32Ptr(provider.ID),
		ProviderCode: fileStringPtr(provider.Code),
		Bucket:       &bucket,
		ObjectKey:    &objectKey,
		BusinessType: fileStringPtr(strings.TrimSpace(req.GetBusinessType())),
		BusinessId:   fileStringPtr(strings.TrimSpace(req.GetBusinessId())),
		Visibility:   &visibility,
		Status:       fileInt32Ptr(FileStatusPending),
		CreatedBy:    fileUint32Ptr(authn.GetAuthUserID(ctx)),
	}
	created, err := uc.repo.CreateUploadSession(ctx, file, idempotencyKey, expiresAt)
	if err != nil {
		return nil, err
	}
	return uc.uploadSessionResponse(ctx, provider, created, time.Until(expiresAt))
}

func (uc *FileUsecase) UploadContent(ctx context.Context, req *pbCore.UploadFileContentRequest) (*pbCore.FileObject, error) {
	if req == nil || req.GetId() == 0 {
		return nil, pb.ErrorBadRequest("文件ID不能为空")
	}
	if len(req.GetContent()) == 0 {
		return nil, pb.ErrorBadRequest("文件内容不能为空")
	}
	file, err := uc.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if file.GetStatus() == FileStatusDeleted {
		return nil, pb.ErrorFileNotFound("文件不存在")
	}
	if file.GetProvider() != StorageProviderTypeLocal {
		return nil, errors.BadRequest("FILE_UPLOAD_METHOD_UNSUPPORTED", "当前存储渠道不支持平台代理上传")
	}
	if expires, err := parseDateTime(file.GetUploadExpiresAt()); err == nil && !expires.IsZero() && time.Now().After(expires) {
		return nil, errors.BadRequest("FILE_UPLOAD_SESSION_EXPIRED", "文件上传会话已过期")
	}
	provider, err := uc.resolver.ResolveSnapshot(ctx, file.GetProviderId(), file.GetProviderCode(), file.GetProvider())
	if err != nil {
		return nil, err
	}
	info, err := provider.Client.PutObject(ctx, file.GetBucket(), file.GetObjectKey(), bytes.NewReader(req.GetContent()), objectstorage.PutOptions{ContentType: defaultContentType(req.GetContentType())})
	if err != nil {
		return nil, pb.ErrorFileWriteError("写入本地文件失败: %v", err)
	}
	sha256 := strings.TrimSpace(req.GetSha256())
	if sha256 == "" {
		sha256 = info.ETag
	}
	return uc.repo.Confirm(ctx, req.GetId(), info.Size, sha256, info.ETag)
}

func (uc *FileUsecase) ConfirmUpload(ctx context.Context, req *pbCore.ConfirmFileUploadRequest) (*pbCore.FileObject, error) {
	if req == nil || req.GetId() == 0 {
		return nil, pb.ErrorBadRequest("文件ID不能为空")
	}
	file, err := uc.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if file.GetStatus() == FileStatusDeleted {
		return nil, pb.ErrorFileNotFound("文件不存在")
	}
	if file.GetStatus() == FileStatusConfirmed {
		return file, nil
	}
	if expires, err := parseDateTime(file.GetUploadExpiresAt()); err == nil && !expires.IsZero() && time.Now().After(expires) {
		return nil, errors.BadRequest("FILE_UPLOAD_SESSION_EXPIRED", "文件上传会话已过期")
	}
	size := req.GetSize()
	if size == 0 {
		size = file.GetSize()
	}
	return uc.repo.Confirm(ctx, req.GetId(), size, strings.TrimSpace(req.GetSha256()), strings.TrimSpace(req.GetEtag()))
}

func (uc *FileUsecase) Get(ctx context.Context, id uint32) (*pbCore.FileObject, error) {
	if id == 0 {
		return nil, pb.ErrorBadRequest("文件ID不能为空")
	}
	return uc.repo.Get(ctx, id)
}

func (uc *FileUsecase) List(ctx context.Context, opts ...listing.Option) ([]*pbCore.FileObject, error) {
	return uc.repo.List(ctx, opts...)
}

func (uc *FileUsecase) Count(ctx context.Context, opts ...listing.Option) (int32, error) {
	return uc.repo.Count(ctx, opts...)
}

func (uc *FileUsecase) PresignDownload(ctx context.Context, req *pbCore.PresignFileDownloadRequest) (*pbCore.PresignFileDownloadResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, pb.ErrorBadRequest("文件ID不能为空")
	}
	file, err := uc.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if file.GetStatus() != FileStatusConfirmed {
		return nil, pb.ErrorFileNotFound("文件未确认或已删除")
	}
	provider, err := uc.resolver.ResolveSnapshot(ctx, file.GetProviderId(), file.GetProviderCode(), file.GetProvider())
	if err != nil {
		return nil, err
	}
	ttl := defaultDownloadTTL
	if req.GetExpiresSeconds() > 0 {
		ttl = time.Duration(req.GetExpiresSeconds()) * time.Second
	}
	url, err := provider.Client.PresignGetObject(ctx, file.GetBucket(), file.GetObjectKey(), objectstorage.PresignOptions{Expires: ttl})
	if err != nil {
		return nil, pb.ErrorFileReadError("生成文件下载地址失败: %v", err)
	}
	return &pbCore.PresignFileDownloadResponse{
		DownloadUrl: url,
		ExpiresAt:   time.Now().UTC().Add(ttl).Format(time.DateTime),
	}, nil
}

func (uc *FileUsecase) Delete(ctx context.Context, id uint32) error {
	if id == 0 {
		return pb.ErrorBadRequest("文件ID不能为空")
	}
	file, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if file.GetStatus() == FileStatusDeleted {
		return nil
	}
	if file.GetBucket() != "" && file.GetObjectKey() != "" {
		provider, resolveErr := uc.resolver.ResolveSnapshot(ctx, file.GetProviderId(), file.GetProviderCode(), file.GetProvider())
		if resolveErr != nil {
			return resolveErr
		}
		if err = provider.Client.DeleteObject(ctx, file.GetBucket(), file.GetObjectKey()); err != nil {
			return pb.ErrorFileDeleteError("删除对象存储文件失败: %v", err)
		}
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *FileUsecase) uploadSessionResponse(ctx context.Context, provider *ResolvedStorageProvider, file *pbCore.FileObject, ttl time.Duration) (*pbCore.CreateFileUploadSessionResponse, error) {
	if file == nil {
		return nil, pb.ErrorFileNotFound("文件不存在")
	}
	if ttl <= 0 {
		ttl = defaultUploadTTL
	}
	if provider == nil {
		if uc.resolver == nil {
			return nil, errors.BadRequest("FILE_STORAGE_NOT_CONFIGURED", "未配置可用的文件存储渠道")
		}
		var err error
		provider, err = uc.resolver.ResolveSnapshot(ctx, file.GetProviderId(), file.GetProviderCode(), file.GetProvider())
		if err != nil {
			return nil, err
		}
	}
	if provider.Type == StorageProviderTypeLocal {
		return &pbCore.CreateFileUploadSessionResponse{
			File:         file,
			UploadUrl:    fmt.Sprintf("/admin/v1/files/%d:content", file.GetId()),
			UploadMethod: "POST",
			ExpiresAt:    time.Now().UTC().Add(ttl).Format(time.DateTime),
		}, nil
	}
	uploadURL, err := provider.Client.PresignPutObject(ctx, file.GetBucket(), file.GetObjectKey(), objectstorage.PresignOptions{
		ContentType: file.GetContentType(),
		Expires:     ttl,
	})
	if err != nil {
		return nil, pb.ErrorFileWriteError("生成文件上传地址失败: %v", err)
	}
	return &pbCore.CreateFileUploadSessionResponse{
		File:         file,
		UploadUrl:    uploadURL,
		UploadMethod: "PUT",
		ExpiresAt:    time.Now().UTC().Add(ttl).Format(time.DateTime),
	}, nil
}

func buildObjectKey(tenantID uint32, fileName string) string {
	return path.Join("tenants", fmt.Sprintf("%d", tenantID), time.Now().UTC().Format("2006/01/02"), uuid.NewString()+sanitizeFileExt(fileName))
}

func defaultContentType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "application/octet-stream"
	}
	return value
}

func normalizeFileVisibility(value string) string {
	value = strings.TrimSpace(value)
	if value != "public" {
		return defaultFileVisibility
	}
	return value
}

var fileExtPattern = regexp.MustCompile(`^[.][A-Za-z0-9]{1,12}$`)

func sanitizeFileExt(fileName string) string {
	ext := path.Ext(fileName)
	if !fileExtPattern.MatchString(ext) {
		return ""
	}
	return strings.ToLower(ext)
}

func parseDateTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.ParseInLocation(time.DateTime, value, time.Local)
}

func fileStringPtr(value string) *string {
	return &value
}

func fileInt64Ptr(value int64) *int64 {
	return &value
}

func fileInt32Ptr(value int32) *int32 {
	return &value
}

func fileUint32Ptr(value uint32) *uint32 {
	return &value
}
