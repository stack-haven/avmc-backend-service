package biz

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/google/uuid"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/objectstorage"
	"backend-service/pkg/utils/convert"
)

const (
	FileStatusPending   int32 = 1
	FileStatusConfirmed int32 = 2
	FileStatusDeleted   int32 = 3

	defaultFileVisibility = "private"
	defaultFileBucket     = "tenant-files"
	defaultUploadTTL      = 15 * time.Minute
	defaultDownloadTTL    = 15 * time.Minute
	fileQuotaKey          = "files"
	storageBytesQuotaKey  = "storage.bytes"
)

type FileRepo interface {
	CreateUploadSession(context.Context, *pbCore.FileObject, string, time.Time) (*pbCore.FileObject, error)
	Get(context.Context, uint32) (*pbCore.FileObject, error)
	FindByIdempotencyKey(context.Context, string) (*pbCore.FileObject, error)
	List(context.Context, ...listing.Option) ([]*pbCore.FileObject, error)
	Count(context.Context, ...listing.Option) (int32, error)
	Confirm(context.Context, uint32, int64, string, string) (*pbCore.FileObject, error)
	Delete(context.Context, uint32) error
	UpdateFileName(context.Context, uint32, string) (*pbCore.FileObject, error)
	UpdateAfterReplace(context.Context, uint32, string, int64, string, string, string, string) (*pbCore.FileObject, error)
}

type FileAccessLogRepo interface {
	Append(context.Context, *pbCore.FileAccessLog) error
	List(context.Context, ...listing.Option) ([]*pbCore.FileAccessLog, error)
	Count(context.Context, ...listing.Option) (int32, error)
}

type FileUsecase struct {
	repo      FileRepo
	accessLog FileAccessLogRepo
	resolver  StorageProviderResolver
	quota     *ResourceQuotaUsecase
	log       *log.Helper
}

func NewFileUsecase(repo FileRepo, accessLog FileAccessLogRepo, resolver StorageProviderResolver, quota *ResourceQuotaUsecase, logger log.Logger) *FileUsecase {
	return &FileUsecase{repo: repo, accessLog: accessLog, resolver: resolver, quota: quota, log: log.NewHelper(logger)}
}

func (uc *FileUsecase) CreateUploadSession(ctx context.Context, req *pbCore.CreateFileUploadSessionRequest) (*pbCore.CreateFileUploadSessionResponse, error) {
	if req == nil {
		return nil, errors.BadRequest("FILE_UPLOAD_REQUEST_REQUIRED", "上传请求不能为空")
	}
	fileName := strings.TrimSpace(req.GetFileName())
	if fileName == "" {
		return nil, errors.BadRequest("FILE_NAME_REQUIRED", "文件名不能为空")
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
	if err := uc.checkUploadQuota(ctx, req.GetSize()); err != nil {
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
	partSize := req.GetPartSize()
	totalParts := req.GetTotalParts()

	// 分片上传模式：初始化分片会话（本地=临时目录，对象存储预留为 Multipart Upload）。
	var uploadID string
	if partSize > 0 {
		if totalParts <= 0 {
			return nil, errors.BadRequest("FILE_MULTIPART_PARTS_REQUIRED", "分片上传必须指定总分片数")
		}
		uploadID, err = provider.Client.CreateMultipartUpload(ctx, bucket, objectKey)
		if err != nil {
			return nil, errors.InternalServer("FILE_MULTIPART_INIT_FAILED", fmt.Sprintf("初始化分片上传失败: %v", err))
		}
	}

	file := &pbCore.FileObject{
		TenantId:     &tenantID,
		FileName:     &fileName,
		ContentType:  convert.ToPointer(defaultContentType(req.GetContentType())),
		Size:         convert.ToPointer(req.GetSize()),
		Sha256:       convert.ToPointer(strings.TrimSpace(req.GetSha256())),
		Provider:     convert.ToPointer(provider.Type),
		ProviderId:   convert.ToPointer(provider.ID),
		ProviderCode: convert.ToPointer(provider.Code),
		Bucket:       &bucket,
		ObjectKey:    &objectKey,
		BusinessType: convert.ToPointer(strings.TrimSpace(req.GetBusinessType())),
		BusinessId:   convert.ToPointer(strings.TrimSpace(req.GetBusinessId())),
		Visibility:   &visibility,
		Status:       convert.ToPointer(FileStatusPending),
		CreatedBy:    convert.ToPointer(authn.GetAuthUserID(ctx)),
		UploadId:     &uploadID,
		PartSize:     &partSize,
		TotalParts:   &totalParts,
	}
	created, err := uc.repo.CreateUploadSession(ctx, file, idempotencyKey, expiresAt)
	if err != nil {
		return nil, err
	}
	return uc.uploadSessionResponse(ctx, provider, created, time.Until(expiresAt))
}

func (uc *FileUsecase) UploadContent(ctx context.Context, req *pbCore.UploadFileContentRequest) (*pbCore.FileObject, error) {
	if req == nil || req.GetId() == 0 {
		return nil, errors.BadRequest("FILE_ID_REQUIRED", "文件ID不能为空")
	}
	if len(req.GetContent()) == 0 {
		return nil, errors.BadRequest("FILE_CONTENT_REQUIRED", "文件内容不能为空")
	}
	file, err := uc.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if file.GetStatus() == FileStatusDeleted {
		return nil, errors.NotFound("FILE_NOT_FOUND", "文件不存在")
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
		return nil, errors.InternalServer("FILE_WRITE_ERROR", fmt.Sprintf("写入本地文件失败: %v", err))
	}
	sha256 := strings.TrimSpace(req.GetSha256())
	if sha256 == "" {
		sha256 = info.ETag
	}
	return uc.confirmWithQuota(ctx, file, info.Size, sha256, info.ETag)
}

func (uc *FileUsecase) ConfirmUpload(ctx context.Context, req *pbCore.ConfirmFileUploadRequest) (*pbCore.FileObject, error) {
	if req == nil || req.GetId() == 0 {
		return nil, errors.BadRequest("FILE_ID_REQUIRED", "文件ID不能为空")
	}
	file, err := uc.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if file.GetStatus() == FileStatusDeleted {
		return nil, errors.NotFound("FILE_NOT_FOUND", "文件不存在")
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
	return uc.confirmWithQuota(ctx, file, size, strings.TrimSpace(req.GetSha256()), strings.TrimSpace(req.GetEtag()))
}

func (uc *FileUsecase) Get(ctx context.Context, id uint32) (*pbCore.FileObject, error) {
	if id == 0 {
		return nil, errors.BadRequest("FILE_ID_REQUIRED", "文件ID不能为空")
	}
	return uc.repo.Get(ctx, id)
}

func (uc *FileUsecase) List(ctx context.Context, opts ...listing.Option) ([]*pbCore.FileObject, error) {
	return uc.repo.List(ctx, opts...)
}

func (uc *FileUsecase) Count(ctx context.Context, opts ...listing.Option) (int32, error) {
	return uc.repo.Count(ctx, opts...)
}

func (uc *FileUsecase) ListAccessLogs(ctx context.Context, opts ...listing.Option) ([]*pbCore.FileAccessLog, error) {
	if uc.accessLog == nil {
		return nil, errors.BadRequest("FILE_ACCESS_LOG_NOT_CONFIGURED", "未配置文件访问日志仓储")
	}
	return uc.accessLog.List(ctx, opts...)
}

func (uc *FileUsecase) CountAccessLogs(ctx context.Context, opts ...listing.Option) (int32, error) {
	if uc.accessLog == nil {
		return 0, errors.BadRequest("FILE_ACCESS_LOG_NOT_CONFIGURED", "未配置文件访问日志仓储")
	}
	return uc.accessLog.Count(ctx, opts...)
}

func (uc *FileUsecase) PresignDownload(ctx context.Context, req *pbCore.PresignFileDownloadRequest) (*pbCore.PresignFileDownloadResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, errors.BadRequest("FILE_ID_REQUIRED", "文件ID不能为空")
	}
	file, err := uc.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if file.GetStatus() != FileStatusConfirmed {
		return nil, errors.NotFound("FILE_NOT_FOUND", "文件未确认或已删除")
	}
	provider, err := uc.resolver.ResolveSnapshot(ctx, file.GetProviderId(), file.GetProviderCode(), file.GetProvider())
	if err != nil {
		return nil, err
	}

	// 访问控制：public 文件优先返回公开 URL（长期有效），未配置公开域名时回退预签名。
	if file.GetVisibility() == "public" {
		if publicURL, publicErr := provider.Client.PublicURL(file.GetBucket(), file.GetObjectKey()); publicErr == nil && publicURL != "" {
			uc.appendAccessLog(ctx, file, "download", "success", "")
			return &pbCore.PresignFileDownloadResponse{DownloadUrl: publicURL}, nil
		}
	}

	ttl := defaultDownloadTTL
	if req.GetExpiresSeconds() > 0 {
		ttl = time.Duration(req.GetExpiresSeconds()) * time.Second
	}
	url, err := provider.Client.PresignGetObject(ctx, file.GetBucket(), file.GetObjectKey(), objectstorage.PresignOptions{Expires: ttl})
	if err != nil {
		uc.appendAccessLog(ctx, file, "download", "failure", err.Error())
		return nil, errors.InternalServer("FILE_READ_ERROR", fmt.Sprintf("生成文件下载地址失败: %v", err))
	}
	uc.appendAccessLog(ctx, file, "download", "success", "")
	return &pbCore.PresignFileDownloadResponse{
		DownloadUrl: url,
		ExpiresAt:   time.Now().UTC().Add(ttl).Format(time.DateTime),
	}, nil
}

// DownloadContent 后端代理读取文件内容（仅本地存储渠道）。
func (uc *FileUsecase) DownloadContent(ctx context.Context, id uint32) (*pbCore.DownloadFileContentResponse, error) {
	if id == 0 {
		return nil, errors.BadRequest("FILE_ID_REQUIRED", "文件ID不能为空")
	}
	file, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if file.GetStatus() != FileStatusConfirmed {
		return nil, errors.NotFound("FILE_NOT_FOUND", "文件未确认或已删除")
	}
	if file.GetProvider() != StorageProviderTypeLocal {
		return nil, errors.BadRequest("FILE_UPLOAD_METHOD_UNSUPPORTED", "当前存储渠道不支持后端代理下载")
	}
	provider, err := uc.resolver.ResolveSnapshot(ctx, file.GetProviderId(), file.GetProviderCode(), file.GetProvider())
	if err != nil {
		return nil, err
	}
	content, err := provider.Client.GetObject(ctx, file.GetBucket(), file.GetObjectKey())
	if err != nil {
		uc.appendAccessLog(ctx, file, "download", "failure", err.Error())
		return nil, errors.InternalServer("FILE_READ_ERROR", fmt.Sprintf("读取文件内容失败: %v", err))
	}
	uc.appendAccessLog(ctx, file, "download", "success", "")
	return &pbCore.DownloadFileContentResponse{
		Content:     content,
		ContentType: file.GetContentType(),
		FileName:    file.GetFileName(),
	}, nil
}

// UpdateFileName 更新文件显示名（不影响对象存储 key）。
func (uc *FileUsecase) UpdateFileName(ctx context.Context, id uint32, fileName string) (*pbCore.FileObject, error) {
	if id == 0 {
		return nil, errors.BadRequest("FILE_ID_REQUIRED", "文件ID不能为空")
	}
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return nil, errors.BadRequest("FILE_NAME_REQUIRED", "文件名不能为空")
	}
	file, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if file.GetStatus() == FileStatusDeleted {
		return nil, errors.NotFound("FILE_NOT_FOUND", "文件不存在")
	}
	return uc.repo.UpdateFileName(ctx, id, fileName)
}

// ReplaceContent 原地替换文件内容（保留文件 ID，替换二进制对象和元数据）。
func (uc *FileUsecase) ReplaceContent(ctx context.Context, req *pbCore.ReplaceFileContentRequest) (*pbCore.FileObject, error) {
	if req == nil || req.GetId() == 0 {
		return nil, errors.BadRequest("FILE_ID_REQUIRED", "文件ID不能为空")
	}
	if len(req.GetContent()) == 0 {
		return nil, errors.BadRequest("FILE_CONTENT_REQUIRED", "文件内容不能为空")
	}
	file, err := uc.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if file.GetStatus() != FileStatusConfirmed {
		return nil, errors.BadRequest("FILE_NOT_CONFIRMED", "仅已确认的文件可替换内容")
	}
	if file.GetProvider() != StorageProviderTypeLocal {
		return nil, errors.BadRequest("FILE_UPLOAD_METHOD_UNSUPPORTED", "当前存储渠道不支持平台代理上传")
	}
	provider, err := uc.resolver.ResolveSnapshot(ctx, file.GetProviderId(), file.GetProviderCode(), file.GetProvider())
	if err != nil {
		return nil, err
	}

	newSize := int64(len(req.GetContent()))
	contentType := defaultContentType(req.GetContentType())
	fileName := file.GetFileName()
	if strings.TrimSpace(req.GetFileName()) != "" {
		fileName = strings.TrimSpace(req.GetFileName())
	}

	// 配额：文件数量不变，仅存储字节从旧 size 变为新 size。
	if uc.quota != nil && newSize > file.GetSize() {
		storage, checkErr := uc.quota.CheckCurrent(ctx, storageBytesQuotaKey, newSize-file.GetSize())
		if checkErr != nil {
			return nil, checkErr
		}
		if !storage.GetAllowed() {
			return nil, errors.Forbidden("RESOURCE_QUOTA_EXCEEDED", "文件存储容量额度不足")
		}
	}

	// 删除旧对象，写入新对象（新 object_key，保持对象不可变语义）。
	if err = provider.Client.DeleteObject(ctx, file.GetBucket(), file.GetObjectKey()); err != nil {
		uc.appendAccessLog(ctx, file, "replace", "failure", err.Error())
		return nil, errors.InternalServer("FILE_DELETE_ERROR", fmt.Sprintf("删除旧文件对象失败: %v", err))
	}
	tenantID, tenantErr := currentTenantID(ctx)
	if tenantErr != nil {
		return nil, tenantErr
	}
	newObjectKey := buildObjectKey(tenantID, fileName)
	info, putErr := provider.Client.PutObject(ctx, file.GetBucket(), newObjectKey, bytes.NewReader(req.GetContent()), objectstorage.PutOptions{ContentType: contentType})
	if putErr != nil {
		uc.appendAccessLog(ctx, file, "replace", "failure", putErr.Error())
		return nil, errors.InternalServer("FILE_WRITE_ERROR", fmt.Sprintf("写入文件对象失败: %v", putErr))
	}

	updated, err := uc.repo.UpdateAfterReplace(ctx, req.GetId(), newObjectKey, info.Size, info.ETag, info.ETag, contentType, fileName)
	if err != nil {
		uc.appendAccessLog(ctx, file, "replace", "failure", err.Error())
		return nil, err
	}

	// 写入成功后调整配额占用：释放旧 size，占用新 size。
	if uc.quota != nil {
		if file.GetSize() > 0 {
			if _, releaseErr := uc.quota.ReleaseCurrent(ctx, storageBytesQuotaKey, file.GetSize(), fmt.Sprintf("file:replace:%d:old", file.GetId())); releaseErr != nil {
				uc.log.Warnf("release old file storage quota failed: %v", releaseErr)
			}
		}
		if newSize > 0 {
			if _, _, reserveErr := uc.quota.ReserveCurrent(ctx, storageBytesQuotaKey, newSize, fmt.Sprintf("file:replace:%d:new", file.GetId())); reserveErr != nil {
				uc.log.Warnf("reserve new file storage quota failed: %v", reserveErr)
			}
		}
	}

	uc.appendAccessLog(ctx, file, "replace", "success", "")
	return updated, nil
}

// UploadFilePart 上传单个分片（本地渠道代理接收；对象存储预留为预签名直传）。
func (uc *FileUsecase) UploadFilePart(ctx context.Context, req *pbCore.UploadFilePartRequest) (*pbCore.UploadFilePartResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, errors.BadRequest("FILE_ID_REQUIRED", "文件ID不能为空")
	}
	if req.GetPartNumber() <= 0 {
		return nil, errors.BadRequest("FILE_PART_NUMBER_INVALID", "分片序号必须大于 0")
	}
	if len(req.GetContent()) == 0 {
		return nil, errors.BadRequest("FILE_CONTENT_REQUIRED", "分片内容不能为空")
	}
	file, err := uc.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if file.GetStatus() != FileStatusPending {
		return nil, errors.BadRequest("FILE_NOT_PENDING", "仅待上传状态可接收分片")
	}
	if file.GetPartSize() <= 0 || file.GetUploadId() == "" {
		return nil, errors.BadRequest("FILE_NOT_MULTIPART", "当前文件不是分片上传会话")
	}
	if req.GetPartNumber() > file.GetTotalParts() {
		return nil, errors.BadRequest("FILE_PART_NUMBER_INVALID", "分片序号超出总分片数")
	}
	provider, err := uc.resolver.ResolveSnapshot(ctx, file.GetProviderId(), file.GetProviderCode(), file.GetProvider())
	if err != nil {
		return nil, err
	}
	etag, err := provider.Client.UploadPart(ctx, file.GetBucket(), file.GetObjectKey(), file.GetUploadId(), req.GetPartNumber(), bytes.NewReader(req.GetContent()), objectstorage.PutOptions{ContentType: file.GetContentType()})
	if err != nil {
		return nil, errors.InternalServer("FILE_PART_WRITE_ERROR", fmt.Sprintf("写入分片失败: %v", err))
	}
	return &pbCore.UploadFilePartResponse{Etag: etag, PartNumber: req.GetPartNumber()}, nil
}

// ListFileParts 查询已上传分片（断点续传）。本地扫描临时目录；对象存储预留为 ListParts。
func (uc *FileUsecase) ListFileParts(ctx context.Context, id uint32) (*pbCore.ListFilePartsResponse, error) {
	if id == 0 {
		return nil, errors.BadRequest("FILE_ID_REQUIRED", "文件ID不能为空")
	}
	file, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if file.GetPartSize() <= 0 || file.GetUploadId() == "" {
		return nil, errors.BadRequest("FILE_NOT_MULTIPART", "当前文件不是分片上传会话")
	}
	provider, err := uc.resolver.ResolveSnapshot(ctx, file.GetProviderId(), file.GetProviderCode(), file.GetProvider())
	if err != nil {
		return nil, err
	}
	parts, err := provider.Client.ListMultipartParts(ctx, file.GetBucket(), file.GetObjectKey(), file.GetUploadId())
	if err != nil {
		return nil, errors.InternalServer("FILE_PARTS_LIST_ERROR", fmt.Sprintf("查询已上传分片失败: %v", err))
	}
	pbParts := make([]*pbCore.FilePart, len(parts))
	for i, p := range parts {
		pbParts[i] = &pbCore.FilePart{PartNumber: p.PartNumber, Etag: p.ETag}
	}
	return &pbCore.ListFilePartsResponse{
		Parts:      pbParts,
		TotalParts: file.GetTotalParts(),
		PartSize:   file.GetPartSize(),
	}, nil
}

// CompleteFileUpload 完成分片上传：本地合并分片，对象存储预留为 CompleteMultipartUpload。
func (uc *FileUsecase) CompleteFileUpload(ctx context.Context, req *pbCore.CompleteFileUploadRequest) (*pbCore.FileObject, error) {
	if req == nil || req.GetId() == 0 {
		return nil, errors.BadRequest("FILE_ID_REQUIRED", "文件ID不能为空")
	}
	if len(req.GetParts()) == 0 {
		return nil, errors.BadRequest("FILE_PARTS_REQUIRED", "分片列表不能为空")
	}
	file, err := uc.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if file.GetStatus() != FileStatusPending {
		return nil, errors.BadRequest("FILE_NOT_PENDING", "仅待上传状态可完成上传")
	}
	if file.GetPartSize() <= 0 || file.GetUploadId() == "" {
		return nil, errors.BadRequest("FILE_NOT_MULTIPART", "当前文件不是分片上传会话")
	}
	if int32(len(req.GetParts())) != file.GetTotalParts() {
		return nil, errors.BadRequest("FILE_PARTS_COUNT_MISMATCH", fmt.Sprintf("分片数量不匹配：期望 %d，实际 %d", file.GetTotalParts(), len(req.GetParts())))
	}
	provider, err := uc.resolver.ResolveSnapshot(ctx, file.GetProviderId(), file.GetProviderCode(), file.GetProvider())
	if err != nil {
		return nil, err
	}
	parts := make([]objectstorage.MultipartPart, len(req.GetParts()))
	for i, p := range req.GetParts() {
		parts[i] = objectstorage.MultipartPart{PartNumber: p.GetPartNumber(), ETag: p.GetEtag()}
	}
	etag, err := provider.Client.CompleteMultipartUpload(ctx, file.GetBucket(), file.GetObjectKey(), file.GetUploadId(), parts)
	if err != nil {
		uc.appendAccessLog(ctx, file, "upload", "failure", err.Error())
		return nil, errors.InternalServer("FILE_MULTIPART_COMPLETE_FAILED", fmt.Sprintf("合并分片失败: %v", err))
	}
	// 本地渠道 etag 即整文件 sha256。
	sha256 := etag
	if strings.TrimSpace(file.GetSha256()) != "" {
		sha256 = file.GetSha256()
	}
	uc.appendAccessLog(ctx, file, "upload", "success", "")
	return uc.confirmWithQuota(ctx, file, file.GetSize(), sha256, etag)
}

// AbortFileUpload 取消分片上传，清理已上传分片并删除文件记录。
func (uc *FileUsecase) AbortFileUpload(ctx context.Context, id uint32) error {
	if id == 0 {
		return errors.BadRequest("FILE_ID_REQUIRED", "文件ID不能为空")
	}
	file, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if file.GetStatus() != FileStatusPending {
		return nil
	}
	if file.GetUploadId() != "" {
		provider, resolveErr := uc.resolver.ResolveSnapshot(ctx, file.GetProviderId(), file.GetProviderCode(), file.GetProvider())
		if resolveErr == nil {
			if abortErr := provider.Client.AbortMultipartUpload(ctx, file.GetBucket(), file.GetObjectKey(), file.GetUploadId()); abortErr != nil {
				uc.log.Warnf("abort multipart upload failed: %v", abortErr)
			}
		}
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *FileUsecase) Delete(ctx context.Context, id uint32, idempotencyKey string) error {
	if id == 0 {
		return errors.BadRequest("FILE_ID_REQUIRED", "文件ID不能为空")
	}
	file, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if file.GetStatus() == FileStatusDeleted {
		return uc.releaseFileQuota(ctx, file, idempotencyKey)
	}
	if file.GetBucket() != "" && file.GetObjectKey() != "" {
		provider, resolveErr := uc.resolver.ResolveSnapshot(ctx, file.GetProviderId(), file.GetProviderCode(), file.GetProvider())
		if resolveErr != nil {
			uc.appendAccessLog(ctx, file, "delete", "failure", resolveErr.Error())
			return resolveErr
		}
		if err = provider.Client.DeleteObject(ctx, file.GetBucket(), file.GetObjectKey()); err != nil {
			uc.appendAccessLog(ctx, file, "delete", "failure", err.Error())
			return errors.InternalServer("FILE_DELETE_ERROR", fmt.Sprintf("删除对象存储文件失败: %v", err))
		}
	}
	if err = uc.repo.Delete(ctx, id); err != nil {
		uc.appendAccessLog(ctx, file, "delete", "failure", err.Error())
		return err
	}
	if err = uc.releaseFileQuota(ctx, file, idempotencyKey); err != nil {
		uc.appendAccessLog(ctx, file, "delete", "failure", err.Error())
		return err
	}
	uc.appendAccessLog(ctx, file, "delete", "success", "")
	return nil
}

func (uc *FileUsecase) checkUploadQuota(ctx context.Context, size int64) error {
	if uc.quota == nil {
		return nil
	}
	files, err := uc.quota.CheckCurrent(ctx, fileQuotaKey, 1)
	if err != nil {
		return err
	}
	if !files.GetAllowed() {
		return errors.Forbidden("RESOURCE_QUOTA_EXCEEDED", "文件数量额度不足")
	}
	if size > 0 {
		storage, err := uc.quota.CheckCurrent(ctx, storageBytesQuotaKey, size)
		if err != nil {
			return err
		}
		if !storage.GetAllowed() {
			return errors.Forbidden("RESOURCE_QUOTA_EXCEEDED", "文件存储容量额度不足")
		}
	}
	return nil
}

func (uc *FileUsecase) confirmWithQuota(ctx context.Context, file *pbCore.FileObject, size int64, sha256, etag string) (*pbCore.FileObject, error) {
	if file == nil {
		return nil, errors.NotFound("FILE_NOT_FOUND", "文件不存在")
	}
	var fileReservation *ResourceQuotaReservation
	var storageReservation *ResourceQuotaReservation
	var err error
	if uc.quota != nil {
		fileReservation, _, err = uc.quota.ReserveCurrent(ctx, fileQuotaKey, 1, fmt.Sprintf("file:confirm:%d:files", file.GetId()))
		if err != nil {
			return nil, err
		}
		if size > 0 {
			storageReservation, _, err = uc.quota.ReserveCurrent(ctx, storageBytesQuotaKey, size, fmt.Sprintf("file:confirm:%d:storage.bytes", file.GetId()))
			if err != nil {
				releaseQuotaReservation(ctx, fileReservation, uc.log)
				return nil, err
			}
		}
	}
	confirmed, err := uc.repo.Confirm(ctx, file.GetId(), size, sha256, etag)
	if err != nil {
		releaseQuotaReservation(ctx, storageReservation, uc.log)
		releaseQuotaReservation(ctx, fileReservation, uc.log)
		return nil, err
	}
	return confirmed, nil
}

func releaseQuotaReservation(ctx context.Context, reservation *ResourceQuotaReservation, logger *log.Helper) {
	if reservation == nil || reservation.IsReplay() {
		return
	}
	if _, err := reservation.Release(ctx); err != nil && logger != nil {
		logger.Warnf("release file quota reservation failed: %v", err)
	}
}

func (uc *FileUsecase) releaseFileQuota(ctx context.Context, file *pbCore.FileObject, idempotencyKey string) error {
	if uc.quota == nil || file == nil || file.GetStatus() != FileStatusConfirmed {
		return nil
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		idempotencyKey = fmt.Sprintf("file:delete:%d", file.GetId())
	}
	if _, err := uc.quota.ReleaseCurrent(ctx, fileQuotaKey, 1, idempotencyKey+":files"); err != nil {
		return err
	}
	if file.GetSize() > 0 {
		if _, err := uc.quota.ReleaseCurrent(ctx, storageBytesQuotaKey, file.GetSize(), idempotencyKey+":storage.bytes"); err != nil {
			return err
		}
	}
	return nil
}

func (uc *FileUsecase) appendAccessLog(ctx context.Context, file *pbCore.FileObject, action, result, message string) {
	if uc.accessLog == nil || file == nil {
		return
	}
	operatorID := authn.GetAuthUserID(ctx)
	operatorName := ""
	if user, ok := authn.AuthUserFromContext(ctx); ok && user != nil {
		operatorName = user.Name()
		if operatorName == "" {
			operatorName = user.GetSubject()
		}
	}
	clientIP, userAgent := fileRequestMeta(ctx)
	entry := &pbCore.FileAccessLog{
		FileId:       file.GetId(),
		FileName:     convert.ToPointer(file.GetFileName()),
		Action:       convert.ToPointer(action),
		OperatorId:   convert.ToPointer(operatorID),
		OperatorName: convert.ToPointer(operatorName),
		ClientIp:     convert.ToPointer(clientIP),
		UserAgent:    convert.ToPointer(userAgent),
		Result:       convert.ToPointer(result),
		Message:      convert.ToPointer(message),
	}
	if err := uc.accessLog.Append(ctx, entry); err != nil {
		uc.log.Warnf("append file access log failed: %v", err)
	}
}

func fileRequestMeta(ctx context.Context) (ip, userAgent string) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		headers := tr.RequestHeader()
		ip := strings.TrimSpace(headers.Get("X-Forwarded-For"))
		if comma := strings.Index(ip, ","); comma >= 0 {
			ip = strings.TrimSpace(ip[:comma])
		}
		if ip == "" {
			ip = strings.TrimSpace(headers.Get("X-Real-IP"))
		}
		return ip, strings.TrimSpace(headers.Get("User-Agent"))
	}
	return "", ""
}

func (uc *FileUsecase) uploadSessionResponse(ctx context.Context, provider *ResolvedStorageProvider, file *pbCore.FileObject, ttl time.Duration) (*pbCore.CreateFileUploadSessionResponse, error) {
	if file == nil {
		return nil, errors.NotFound("FILE_NOT_FOUND", "文件不存在")
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
			UploadUrl:    fmt.Sprintf("/admin/v1/files/%d/content", file.GetId()),
			UploadMethod: "POST",
			ExpiresAt:    time.Now().UTC().Add(ttl).Format(time.DateTime),
		}, nil
	}
	uploadURL, err := provider.Client.PresignPutObject(ctx, file.GetBucket(), file.GetObjectKey(), objectstorage.PresignOptions{
		ContentType: file.GetContentType(),
		Expires:     ttl,
	})
	if err != nil {
		return nil, errors.InternalServer("FILE_WRITE_ERROR", fmt.Sprintf("生成文件上传地址失败: %v", err))
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
