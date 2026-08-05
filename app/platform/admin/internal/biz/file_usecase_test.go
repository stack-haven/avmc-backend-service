package biz

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/proto"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/objectstorage"
)

type fileRepoStub struct {
	byID            map[uint32]*pbCore.FileObject
	byIdempotency   map[string]*pbCore.FileObject
	createCalls     int
	confirmCalls    int
	deleteCalls     int
	nextID          uint32
	findByIDErr     error
	findByIdemError error
}

func newFileRepoStub() *fileRepoStub {
	return &fileRepoStub{
		byID:          map[uint32]*pbCore.FileObject{},
		byIdempotency: map[string]*pbCore.FileObject{},
		nextID:        1,
	}
}

func (r *fileRepoStub) CreateUploadSession(_ context.Context, file *pbCore.FileObject, idempotencyKey string, expiresAt time.Time) (*pbCore.FileObject, error) {
	r.createCalls++
	created := proto.Clone(file).(*pbCore.FileObject) //nolint:errcheck // proto.Clone does not return error
	created.Id = r.nextID
	created.UploadExpiresAt = stringPtr(expiresAt.UTC().Format(time.DateTime))
	r.nextID++
	r.byID[created.Id] = created
	if idempotencyKey != "" {
		r.byIdempotency[idempotencyKey] = created
	}
	return created, nil
}

func (r *fileRepoStub) Get(_ context.Context, id uint32) (*pbCore.FileObject, error) {
	if r.findByIDErr != nil {
		return nil, r.findByIDErr
	}
	file := r.byID[id]
	if file == nil {
		return nil, errors.NotFound("FILE_NOT_FOUND", "file not found")
	}
	return file, nil
}

func (r *fileRepoStub) FindByIdempotencyKey(_ context.Context, key string) (*pbCore.FileObject, error) {
	if r.findByIdemError != nil {
		return nil, r.findByIdemError
	}
	return r.byIdempotency[key], nil
}

func (*fileRepoStub) List(context.Context, ...listing.Option) ([]*pbCore.FileObject, error) {
	return nil, nil
}

func (*fileRepoStub) Count(context.Context, ...listing.Option) (int32, error) {
	return 0, nil
}

func (r *fileRepoStub) Confirm(_ context.Context, id uint32, size int64, sha256, etag string) (*pbCore.FileObject, error) {
	r.confirmCalls++
	file := r.byID[id]
	if file == nil {
		return nil, errors.NotFound("FILE_NOT_FOUND", "file not found")
	}
	file.Status = fileInt32Ptr(FileStatusConfirmed)
	file.Size = fileInt64Ptr(size)
	file.Sha256 = stringPtr(sha256)
	return file, nil
}

func (r *fileRepoStub) Delete(_ context.Context, id uint32) error {
	r.deleteCalls++
	file := r.byID[id]
	if file != nil {
		file.Status = fileInt32Ptr(FileStatusDeleted)
	}
	return nil
}

type fileStorageStub struct {
	presignPutCalls int
	presignGetCalls int
	deleteCalls     int
	providerType    string
	resolveErr      error
}

type fileAccessLogRepoStub struct {
	items []*pbCore.FileAccessLog
}

func (r *fileAccessLogRepoStub) Append(_ context.Context, value *pbCore.FileAccessLog) error {
	cloned := proto.Clone(value).(*pbCore.FileAccessLog) //nolint:errcheck // proto.Clone does not return error
	r.items = append(r.items, cloned)
	return nil
}

func (r *fileAccessLogRepoStub) List(context.Context, ...listing.Option) ([]*pbCore.FileAccessLog, error) {
	return r.items, nil
}

func (r *fileAccessLogRepoStub) Count(context.Context, ...listing.Option) (int32, error) {
	return int32(len(r.items)), nil
}

func (*fileStorageStub) PutObject(context.Context, string, string, io.Reader, objectstorage.PutOptions) (*objectstorage.ObjectInfo, error) {
	return nil, nil
}

func (s *fileStorageStub) DeleteObject(context.Context, string, string) error {
	s.deleteCalls++
	return nil
}

func (s *fileStorageStub) PresignGetObject(context.Context, string, string, objectstorage.PresignOptions) (string, error) {
	s.presignGetCalls++
	return "https://storage.local/download", nil
}

func (s *fileStorageStub) PresignPutObject(context.Context, string, string, objectstorage.PresignOptions) (string, error) {
	s.presignPutCalls++
	return "https://storage.local/upload", nil
}

func (*fileStorageStub) PublicURL(string, string) (string, error) {
	return "", nil
}

func (s *fileStorageStub) ResolveDefault(context.Context) (*ResolvedStorageProvider, error) {
	if s == nil || s.resolveErr != nil {
		if s == nil {
			return nil, errors.BadRequest("FILE_STORAGE_NOT_CONFIGURED", "未配置可用的文件存储渠道")
		}
		return nil, s.resolveErr
	}
	providerType := s.providerType
	if providerType == "" {
		providerType = StorageProviderTypeS3Compatible
	}
	return &ResolvedStorageProvider{
		ID:            1,
		Code:          "default-storage",
		Type:          providerType,
		DefaultBucket: defaultFileBucket,
		Client:        s,
	}, nil
}

func (s *fileStorageStub) ResolveSnapshot(context.Context, uint32, string, string) (*ResolvedStorageProvider, error) {
	return s.ResolveDefault(context.Background())
}

func TestFileUsecaseCreateUploadSessionUsesIdempotencyReplay(t *testing.T) {
	t.Parallel()

	repo := newFileRepoStub()
	storage := &fileStorageStub{}
	uc := NewFileUsecase(repo, nil, storage, nil, log.NewStdLogger(io.Discard))
	ctx := projectQuotaContext()
	req := &pbCore.CreateFileUploadSessionRequest{
		FileName:       stringPtr("report.pdf"),
		ContentType:    stringPtr("application/pdf"),
		IdempotencyKey: stringPtr("file-upload-1"),
	}

	first, err := uc.CreateUploadSession(ctx, req)
	if err != nil {
		t.Fatalf("CreateUploadSession() first error = %v", err)
	}
	second, err := uc.CreateUploadSession(ctx, req)
	if err != nil {
		t.Fatalf("CreateUploadSession() replay error = %v", err)
	}
	if first.File.GetId() != second.File.GetId() {
		t.Fatalf("replay file id = %d, want %d", second.File.GetId(), first.File.GetId())
	}
	if repo.createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", repo.createCalls)
	}
	if storage.presignPutCalls != 2 {
		t.Fatalf("presignPutCalls = %d, want 2", storage.presignPutCalls)
	}
	if !strings.Contains(first.File.GetObjectKey(), "tenants/10/") {
		t.Fatalf("object key = %s, want tenant scoped path", first.File.GetObjectKey())
	}
}

func TestFileUsecaseCreateUploadSessionRejectsMissingStorageProvider(t *testing.T) {
	t.Parallel()

	uc := NewFileUsecase(newFileRepoStub(), nil, nil, nil, log.NewStdLogger(io.Discard))
	if _, err := uc.CreateUploadSession(projectQuotaContext(), &pbCore.CreateFileUploadSessionRequest{FileName: stringPtr("avatar.png")}); err == nil {
		t.Fatal("CreateUploadSession() error = nil, want missing storage provider rejection")
	}
}

func TestFileUsecasePresignDownloadRequiresConfirmedFile(t *testing.T) {
	t.Parallel()

	repo := newFileRepoStub()
	repo.byID[1] = &pbCore.FileObject{
		Id:        1,
		FileName:  stringPtr("pending.txt"),
		Bucket:    stringPtr("tenant-files"),
		ObjectKey: stringPtr("pending.txt"),
		Status:    fileInt32Ptr(FileStatusPending),
	}
	uc := NewFileUsecase(repo, nil, &fileStorageStub{}, nil, log.NewStdLogger(io.Discard))
	if _, err := uc.PresignDownload(projectQuotaContext(), &pbCore.PresignFileDownloadRequest{Id: 1}); err == nil {
		t.Fatal("PresignDownload() error = nil, want pending file rejection")
	}
}

func TestFileUsecaseDeleteRemovesObjectAndSoftDeletesMetadata(t *testing.T) {
	t.Parallel()

	repo := newFileRepoStub()
	repo.byID[1] = &pbCore.FileObject{
		Id:        1,
		FileName:  stringPtr("confirmed.txt"),
		Bucket:    stringPtr("tenant-files"),
		ObjectKey: stringPtr("confirmed.txt"),
		Status:    fileInt32Ptr(FileStatusConfirmed),
	}
	storage := &fileStorageStub{}
	uc := NewFileUsecase(repo, nil, storage, nil, log.NewStdLogger(io.Discard))
	if err := uc.Delete(projectQuotaContext(), 1, "delete-file-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if storage.deleteCalls != 1 || repo.deleteCalls != 1 {
		t.Fatalf("delete calls storage=%d repo=%d, want 1/1", storage.deleteCalls, repo.deleteCalls)
	}
}

func TestFileUsecaseConfirmConsumesFileQuotas(t *testing.T) {
	t.Parallel()

	repo := newFileRepoStub()
	repo.byID[1] = &pbCore.FileObject{
		Id:        1,
		FileName:  stringPtr("confirmed.txt"),
		Bucket:    stringPtr("tenant-files"),
		ObjectKey: stringPtr("confirmed.txt"),
		Size:      fileInt64Ptr(12),
		Status:    fileInt32Ptr(FileStatusPending),
	}
	quotaRepo := &resourceQuotaRepoStub{}
	quota := NewResourceQuotaUsecase(
		quotaRepo,
		&TenantMenuPermissionGroupRepoStub{caps: &pbCore.GetCurrentTenantCapabilitiesResponse{
			TenantId:       10,
			ResourceQuotas: map[string]int64{fileQuotaKey: 2, storageBytesQuotaKey: 100},
		}},
		log.NewStdLogger(io.Discard),
	)
	uc := NewFileUsecase(repo, nil, &fileStorageStub{}, quota, log.NewStdLogger(io.Discard))

	file, err := uc.ConfirmUpload(projectQuotaContext(), &pbCore.ConfirmFileUploadRequest{Id: 1, Size: fileInt64Ptr(12)})
	if err != nil {
		t.Fatalf("ConfirmUpload() error = %v", err)
	}
	if file.GetStatus() != FileStatusConfirmed {
		t.Fatalf("status = %d, want confirmed", file.GetStatus())
	}
	if quotaRepo.usages[fileQuotaKey].GetUsed() != 1 {
		t.Fatalf("files quota used = %d, want 1", quotaRepo.usages[fileQuotaKey].GetUsed())
	}
	if quotaRepo.usages[storageBytesQuotaKey].GetUsed() != 12 {
		t.Fatalf("storage quota used = %d, want 12", quotaRepo.usages[storageBytesQuotaKey].GetUsed())
	}
}

func TestFileUsecasePresignDownloadAppendsAccessLog(t *testing.T) {
	t.Parallel()

	repo := newFileRepoStub()
	repo.byID[1] = &pbCore.FileObject{
		Id:        1,
		FileName:  stringPtr("confirmed.txt"),
		Bucket:    stringPtr("tenant-files"),
		ObjectKey: stringPtr("confirmed.txt"),
		Status:    fileInt32Ptr(FileStatusConfirmed),
	}
	accessLog := &fileAccessLogRepoStub{}
	uc := NewFileUsecase(repo, accessLog, &fileStorageStub{}, nil, log.NewStdLogger(io.Discard))

	if _, err := uc.PresignDownload(projectQuotaContext(), &pbCore.PresignFileDownloadRequest{Id: 1}); err != nil {
		t.Fatalf("PresignDownload() error = %v", err)
	}
	if len(accessLog.items) != 1 {
		t.Fatalf("access log count = %d, want 1", len(accessLog.items))
	}
	if accessLog.items[0].GetAction() != "download" || accessLog.items[0].GetResult() != "success" {
		t.Fatalf("access log = %v", accessLog.items[0])
	}
}
