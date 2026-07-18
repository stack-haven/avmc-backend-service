package biz

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/objectstorage"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
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
	created := *file
	created.Id = r.nextID
	created.UploadExpiresAt = stringPtr(expiresAt.UTC().Format(time.DateTime))
	r.nextID++
	r.byID[created.Id] = &created
	if idempotencyKey != "" {
		r.byIdempotency[idempotencyKey] = &created
	}
	return &created, nil
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

func (r *fileRepoStub) Confirm(_ context.Context, id uint32, size int64, sha256 string, etag string) (*pbCore.FileObject, error) {
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

func TestFileUsecaseCreateUploadSessionUsesIdempotencyReplay(t *testing.T) {
	t.Parallel()

	repo := newFileRepoStub()
	storage := &fileStorageStub{}
	uc := NewFileUsecase(repo, storage, log.NewStdLogger(io.Discard))
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

func TestFileUsecaseCreateUploadSessionWorksWithoutStorageClient(t *testing.T) {
	t.Parallel()

	uc := NewFileUsecase(newFileRepoStub(), nil, log.NewStdLogger(io.Discard))
	resp, err := uc.CreateUploadSession(projectQuotaContext(), &pbCore.CreateFileUploadSessionRequest{FileName: stringPtr("avatar.png")})
	if err != nil {
		t.Fatalf("CreateUploadSession() error = %v", err)
	}
	if resp.GetUploadUrl() != "" {
		t.Fatalf("upload url = %q, want empty when storage is not configured", resp.GetUploadUrl())
	}
	if resp.GetUploadMethod() != "PUT" {
		t.Fatalf("upload method = %s, want PUT", resp.GetUploadMethod())
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
	uc := NewFileUsecase(repo, &fileStorageStub{}, log.NewStdLogger(io.Discard))
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
	uc := NewFileUsecase(repo, storage, log.NewStdLogger(io.Discard))
	if err := uc.Delete(projectQuotaContext(), 1); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if storage.deleteCalls != 1 || repo.deleteCalls != 1 {
		t.Fatalf("delete calls storage=%d repo=%d, want 1/1", storage.deleteCalls, repo.deleteCalls)
	}
}
