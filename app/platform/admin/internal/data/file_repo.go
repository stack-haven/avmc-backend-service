package data

import (
	"context"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/fileobject"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
)

var _ biz.FileRepo = (*fileRepo)(nil)

type fileRepo struct {
	BaseRepo
}

func NewFileRepo(data *Data, logger log.Logger) biz.FileRepo {
	return &fileRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func fileObjectToProto(row *gen.FileObject) *pbCore.FileObject {
	if row == nil {
		return nil
	}
	return &pbCore.FileObject{
		Id:              row.ID,
		TenantId:        &row.TenantID,
		FileName:        &row.FileName,
		ContentType:     &row.ContentType,
		Size:            &row.Size,
		Sha256:          &row.Sha256,
		Provider:        &row.Provider,
		Bucket:          &row.Bucket,
		ObjectKey:       &row.ObjectKey,
		BusinessType:    &row.BusinessType,
		BusinessId:      &row.BusinessID,
		Visibility:      &row.Visibility,
		Status:          row.Status,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       convert.TimeValueToString(&row.CreatedAt, time.DateTime),
		UpdatedAt:       convert.TimeValueToString(&row.UpdatedAt, time.DateTime),
		DeletedAt:       convert.TimeValueToString(row.DeletedAt, time.DateTime),
		UploadExpiresAt: convert.TimeValueToString(&row.UploadExpiresAt, time.DateTime),
	}
}

func (r *fileRepo) CreateUploadSession(ctx context.Context, file *pbCore.FileObject, idempotencyKey string, expiresAt time.Time) (*pbCore.FileObject, error) {
	if file == nil || file.GetFileName() == "" {
		return nil, pb.ErrorBadRequest("文件名不能为空")
	}
	if _, err := requireTenantID(ctx); err != nil {
		return nil, err
	}
	builder := r.Data.DB(ctx).FileObject.Create().
		SetFileName(file.GetFileName()).
		SetContentType(file.GetContentType()).
		SetSize(file.GetSize()).
		SetSha256(file.GetSha256()).
		SetProvider(file.GetProvider()).
		SetBucket(file.GetBucket()).
		SetObjectKey(file.GetObjectKey()).
		SetBusinessType(file.GetBusinessType()).
		SetBusinessID(file.GetBusinessId()).
		SetVisibility(file.GetVisibility()).
		SetStatus(biz.FileStatusPending).
		SetUploadExpiresAt(expiresAt)
	if idempotencyKey != "" {
		builder.SetIdempotencyKey(idempotencyKey)
	}
	if file.GetCreatedBy() > 0 {
		builder.SetCreatedBy(file.GetCreatedBy())
	}
	row, err := builder.Save(ctx)
	if err != nil {
		if gen.IsConstraintError(err) && idempotencyKey != "" {
			return r.FindByIdempotencyKey(ctx, idempotencyKey)
		}
		return nil, err
	}
	return fileObjectToProto(row), nil
}

func (r *fileRepo) FindByIdempotencyKey(ctx context.Context, idempotencyKey string) (*pbCore.FileObject, error) {
	if idempotencyKey == "" {
		return nil, nil
	}
	if _, err := requireTenantID(ctx); err != nil {
		return nil, err
	}
	row, err := r.Data.DB(ctx).FileObject.Query().
		Where(fileobject.IdempotencyKeyEQ(idempotencyKey)).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return fileObjectToProto(row), nil
}

func (r *fileRepo) Get(ctx context.Context, id uint32) (*pbCore.FileObject, error) {
	if _, err := requireTenantID(ctx); err != nil {
		return nil, err
	}
	row, err := r.Data.DB(ctx).FileObject.Get(ctx, id)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorFileNotFound("文件不存在")
		}
		return nil, err
	}
	return fileObjectToProto(row), nil
}

func (r *fileRepo) List(ctx context.Context, opts ...listing.Option) ([]*pbCore.FileObject, error) {
	if _, err := requireTenantID(ctx); err != nil {
		return nil, err
	}
	query := r.Data.DB(ctx).FileObject.Query()
	options := applyFileListOptions(opts...)
	rows, err := query.
		Where(ents.ApplyFilter(options.Filter)).
		Order(ents.ApplyOrderBy(options.OrderBy)).
		Offset(options.Offset).Limit(options.Limit).
		Order(gen.Desc(fileobject.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return ConvertSlice(rows, fileObjectToProto), nil
}

func (r *fileRepo) Count(ctx context.Context, opts ...listing.Option) (int32, error) {
	if _, err := requireTenantID(ctx); err != nil {
		return 0, err
	}
	query := r.Data.DB(ctx).FileObject.Query()
	options := applyFileListOptions(opts...)
	count, err := query.
		Where(ents.ApplyFilter(options.Filter)).
		Count(ctx)
	return int32(count), err
}

func applyFileListOptions(opts ...listing.Option) listing.Options {
	options := listing.Options{Limit: listing.DefaultPageSize}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return options
}

func (r *fileRepo) Confirm(ctx context.Context, id uint32, size int64, sha256 string, etag string) (*pbCore.FileObject, error) {
	if _, err := requireTenantID(ctx); err != nil {
		return nil, err
	}
	builder := r.Data.DB(ctx).FileObject.UpdateOneID(id).
		SetStatus(biz.FileStatusConfirmed).
		SetConfirmedAt(time.Now())
	if size > 0 {
		builder.SetSize(size)
	}
	if sha256 != "" {
		builder.SetSha256(sha256)
	}
	if etag != "" {
		builder.SetEtag(etag)
	}
	row, err := builder.Save(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorFileNotFound("文件不存在")
		}
		return nil, err
	}
	return fileObjectToProto(row), nil
}

func (r *fileRepo) Delete(ctx context.Context, id uint32) error {
	if _, err := requireTenantID(ctx); err != nil {
		return err
	}
	err := r.Data.DB(ctx).FileObject.UpdateOneID(id).
		SetStatus(biz.FileStatusDeleted).
		SetDeletedAt(time.Now()).
		Exec(ctx)
	if err != nil && gen.IsNotFound(err) {
		return pb.ErrorFileNotFound("文件不存在")
	}
	return err
}
