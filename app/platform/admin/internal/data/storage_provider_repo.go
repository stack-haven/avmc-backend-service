package data

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/conf"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/fileobject"
	"backend-service/app/platform/admin/internal/data/ent/gen/storageprovider"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/objectstorage"

	// 触发供应商注册
	_ "backend-service/pkg/objectstorage/local"
	_ "backend-service/pkg/objectstorage/s3"

	"backend-service/pkg/utils/convert"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var _ biz.StorageProviderRepo = (*storageProviderRepo)(nil)

type storageProviderRepo struct {
	BaseRepo
	oss *conf.OSS
}

func NewStorageProviderRepo(data *Data, cfg *conf.OSS, logger log.Logger) biz.StorageProviderRepo {
	return &storageProviderRepo{BaseRepo: NewBaseRepo(data, logger), oss: cfg}
}

func storageProviderProto(row *gen.StorageProvider, includeSecret bool) *pbCore.StorageProvider {
	if row == nil {
		return nil
	}
	status := row.Status
	if status == nil {
		status = storageProviderInt32Ptr(biz.StorageProviderStatusEnabled)
	}
	isDefault := row.IsDefault
	secretConfigured := row.SecretKey != "" || row.SessionToken != ""
	item := &pbCore.StorageProvider{
		Id:               row.ID,
		Code:             row.Code,
		Name:             row.Name,
		Type:             row.Type,
		Endpoint:         &row.Endpoint,
		Region:           &row.Region,
		AccessKey:        &row.AccessKey,
		UseSsl:           &row.UseSsl,
		ForcePathStyle:   &row.ForcePathStyle,
		PublicBaseUrl:    &row.PublicBaseURL,
		DefaultBucket:    &row.DefaultBucket,
		LocalBasePath:    &row.LocalBasePath,
		Status:           status,
		IsDefault:        &isDefault,
		HealthStatus:     &row.HealthStatus,
		Remark:           &row.Remark,
		SecretConfigured: &secretConfigured,
		CreatedAt:        convert.TimeValueToString(&row.CreatedAt, time.DateTime),
		UpdatedAt:        convert.TimeValueToString(&row.UpdatedAt, time.DateTime),
		DeletedAt:        convert.TimeValueToString(row.DeletedAt, time.DateTime),
		LastCheckedAt:    convert.TimeValueToString(row.LastCheckedAt, time.DateTime),
	}
	if includeSecret {
		item.SecretKey = &row.SecretKey
		item.SessionToken = &row.SessionToken
	}
	return item
}

func (r *storageProviderRepo) Create(ctx context.Context, item *pbCore.StorageProvider) (*pbCore.StorageProvider, error) {
	if item == nil {
		return nil, pb.ErrorBadRequest("存储渠道不能为空")
	}
	systemCtx := entviewer.NewSystemContext(ctx)
	var result *pbCore.StorageProvider
	err := r.Data.InTx(systemCtx, func(txCtx context.Context) error {
		if item.GetIsDefault() {
			if _, err := r.Data.DB(txCtx).StorageProvider.Update().SetIsDefault(false).Save(txCtx); err != nil {
				return err
			}
		}
		row, err := r.Data.DB(txCtx).StorageProvider.Create().
			SetCode(item.GetCode()).
			SetName(item.GetName()).
			SetType(item.GetType()).
			SetEndpoint(item.GetEndpoint()).
			SetRegion(item.GetRegion()).
			SetAccessKey(item.GetAccessKey()).
			SetSecretKey(item.GetSecretKey()).
			SetSessionToken(item.GetSessionToken()).
			SetUseSsl(item.GetUseSsl()).
			SetForcePathStyle(item.GetForcePathStyle()).
			SetPublicBaseURL(item.GetPublicBaseUrl()).
			SetDefaultBucket(item.GetDefaultBucket()).
			SetLocalBasePath(cleanLocalBasePath(item.GetLocalBasePath())).
			SetStatus(item.GetStatus()).
			SetIsDefault(item.GetIsDefault()).
			SetRemark(item.GetRemark()).
			Save(txCtx)
		if gen.IsConstraintError(err) {
			return errors.Conflict("STORAGE_PROVIDER_CODE_EXISTS", "存储渠道编码已存在")
		}
		if err != nil {
			return err
		}
		result = storageProviderProto(row, false)
		return nil
	})
	return result, err
}

func (r *storageProviderRepo) Update(ctx context.Context, item *pbCore.StorageProvider) (*pbCore.StorageProvider, error) {
	systemCtx := entviewer.NewSystemContext(ctx)
	var result *pbCore.StorageProvider
	err := r.Data.InTx(systemCtx, func(txCtx context.Context) error {
		old, err := r.getRow(txCtx, item.GetId())
		if err != nil {
			return err
		}
		if item.GetIsDefault() {
			if _, err = r.Data.DB(txCtx).StorageProvider.Update().Where(storageprovider.IDNEQ(item.GetId())).SetIsDefault(false).Save(txCtx); err != nil {
				return err
			}
		}
		update := old.Update().
			SetCode(item.GetCode()).
			SetName(item.GetName()).
			SetType(item.GetType()).
			SetEndpoint(item.GetEndpoint()).
			SetRegion(item.GetRegion()).
			SetAccessKey(item.GetAccessKey()).
			SetUseSsl(item.GetUseSsl()).
			SetForcePathStyle(item.GetForcePathStyle()).
			SetPublicBaseURL(item.GetPublicBaseUrl()).
			SetDefaultBucket(item.GetDefaultBucket()).
			SetLocalBasePath(cleanLocalBasePath(item.GetLocalBasePath())).
			SetStatus(item.GetStatus()).
			SetIsDefault(item.GetIsDefault()).
			SetRemark(item.GetRemark())
		if item.SecretKey != nil {
			update.SetSecretKey(item.GetSecretKey())
		}
		if item.SessionToken != nil {
			update.SetSessionToken(item.GetSessionToken())
		}
		row, err := update.Save(txCtx)
		if gen.IsConstraintError(err) {
			return errors.Conflict("STORAGE_PROVIDER_CODE_EXISTS", "存储渠道编码已存在")
		}
		if err != nil {
			return err
		}
		result = storageProviderProto(row, false)
		return nil
	})
	return result, err
}

func (r *storageProviderRepo) Delete(ctx context.Context, id uint32) error {
	systemCtx := entviewer.NewSystemContext(ctx)
	count, err := r.Data.DB(systemCtx).FileObject.Query().
		Where(fileobject.ProviderIDEQ(id)).
		Count(systemCtx)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.Conflict("STORAGE_PROVIDER_IN_USE", "已有文件使用该存储渠道，不能删除")
	}
	row, err := r.getRow(systemCtx, id)
	if err != nil {
		return err
	}
	return row.Update().
		SetDeletedAt(time.Now()).
		SetStatus(biz.StorageProviderStatusDisabled).
		SetIsDefault(false).
		Exec(systemCtx)
}

func (r *storageProviderRepo) Get(ctx context.Context, id uint32) (*pbCore.StorageProvider, error) {
	row, err := r.getRow(entviewer.NewSystemContext(ctx), id)
	if err != nil {
		return nil, err
	}
	return storageProviderProto(row, false), nil
}

func (r *storageProviderRepo) List(ctx context.Context, req *pbCore.ListStorageProvidersRequest) ([]*pbCore.StorageProvider, int32, error) {
	systemCtx := entviewer.NewSystemContext(ctx)
	query := r.Data.DB(systemCtx).StorageProvider.Query().Where(storageprovider.DeletedAtIsNil())
	if req.Code != nil && strings.TrimSpace(req.GetCode()) != "" {
		query.Where(storageprovider.CodeContains(strings.TrimSpace(req.GetCode())))
	}
	if req.Name != nil && strings.TrimSpace(req.GetName()) != "" {
		query.Where(storageprovider.NameContains(strings.TrimSpace(req.GetName())))
	}
	if req.Type != nil && strings.TrimSpace(req.GetType()) != "" {
		query.Where(storageprovider.TypeEQ(strings.TrimSpace(req.GetType())))
	}
	if req.Status != nil && req.GetStatus() > 0 {
		query.Where(storageprovider.StatusEQ(req.GetStatus()))
	}
	if req.IsDefault != nil {
		query.Where(storageprovider.IsDefaultEQ(req.GetIsDefault()))
	}
	total, err := query.Clone().Count(systemCtx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Desc(storageprovider.FieldIsDefault), gen.Asc(storageprovider.FieldID)).Offset(offset).Limit(size).All(systemCtx)
	if err != nil {
		return nil, 0, err
	}
	return convert.SliceToAny(rows, func(row *gen.StorageProvider) *pbCore.StorageProvider {
		return storageProviderProto(row, false)
	}), int32(total), nil
}

func (r *storageProviderRepo) SetDefault(ctx context.Context, id uint32) (*pbCore.StorageProvider, error) {
	systemCtx := entviewer.NewSystemContext(ctx)
	var result *pbCore.StorageProvider
	err := r.Data.InTx(systemCtx, func(txCtx context.Context) error {
		row, err := r.getRow(txCtx, id)
		if err != nil {
			return err
		}
		if storageProviderStatus(row) != biz.StorageProviderStatusEnabled {
			return errors.Conflict("STORAGE_PROVIDER_DISABLED", "禁用的存储渠道不能设为默认")
		}
		if _, err = r.Data.DB(txCtx).StorageProvider.Update().SetIsDefault(false).Save(txCtx); err != nil {
			return err
		}
		row, err = row.Update().SetIsDefault(true).Save(txCtx)
		if err != nil {
			return err
		}
		result = storageProviderProto(row, false)
		return nil
	})
	return result, err
}

func (r *storageProviderRepo) MarkHealth(ctx context.Context, id uint32, health string) error {
	systemCtx := entviewer.NewSystemContext(ctx)
	return r.Data.DB(systemCtx).StorageProvider.UpdateOneID(id).
		SetHealthStatus(health).
		SetLastCheckedAt(time.Now()).
		Exec(systemCtx)
}

func (r *storageProviderRepo) ResolveDefault(ctx context.Context) (*biz.ResolvedStorageProvider, error) {
	systemCtx := entviewer.NewSystemContext(ctx)
	row, err := r.Data.DB(systemCtx).StorageProvider.Query().
		Where(storageprovider.DeletedAtIsNil(), storageprovider.StatusEQ(biz.StorageProviderStatusEnabled), storageprovider.IsDefaultEQ(true)).
		Only(systemCtx)
	if gen.IsNotFound(err) {
		row, err = r.Data.DB(systemCtx).StorageProvider.Query().
			Where(storageprovider.DeletedAtIsNil(), storageprovider.StatusEQ(biz.StorageProviderStatusEnabled)).
			Order(gen.Asc(storageprovider.FieldID)).
			First(systemCtx)
	}
	if gen.IsNotFound(err) && r.oss != nil && r.oss.Minio != nil {
		return r.resolveLegacyMinIO(ctx)
	}
	if gen.IsNotFound(err) {
		return nil, errors.BadRequest("FILE_STORAGE_NOT_CONFIGURED", "未配置可用的文件存储渠道")
	}
	if err != nil {
		return nil, err
	}
	return r.resolveRow(ctx, row)
}

func (r *storageProviderRepo) ResolveSnapshot(ctx context.Context, id uint32, code string, providerType string) (*biz.ResolvedStorageProvider, error) {
	systemCtx := entviewer.NewSystemContext(ctx)
	var row *gen.StorageProvider
	var err error
	if id > 0 {
		row, err = r.getRow(systemCtx, id)
	} else if strings.TrimSpace(code) != "" {
		row, err = r.Data.DB(systemCtx).StorageProvider.Query().
			Where(storageprovider.CodeEQ(strings.TrimSpace(code)), storageprovider.DeletedAtIsNil()).
			Only(systemCtx)
	} else {
		return r.ResolveDefault(ctx)
	}
	if gen.IsNotFound(err) && providerType == biz.StorageProviderTypeS3Compatible && r.oss != nil && r.oss.Minio != nil {
		return r.resolveLegacyMinIO(ctx)
	}
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("STORAGE_PROVIDER_NOT_FOUND", "存储渠道不存在")
	}
	if err != nil {
		return nil, err
	}
	return r.resolveRow(ctx, row)
}

func (r *storageProviderRepo) BuildClient(_ context.Context, item *pbCore.StorageProvider) (objectstorage.Client, error) {
	configJSON, err := storageProviderObjectConfig(item)
	if err != nil {
		return nil, err
	}
	return objectstorage.NewClient(item.GetType(), configJSON)
}

func (r *storageProviderRepo) getRow(ctx context.Context, id uint32) (*gen.StorageProvider, error) {
	row, err := r.Data.DB(ctx).StorageProvider.Query().
		Where(storageprovider.IDEQ(id), storageprovider.DeletedAtIsNil()).
		Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("STORAGE_PROVIDER_NOT_FOUND", "存储渠道不存在")
	}
	return row, err
}

func (r *storageProviderRepo) resolveRow(ctx context.Context, row *gen.StorageProvider) (*biz.ResolvedStorageProvider, error) {
	item := storageProviderProto(row, true)
	client, err := r.BuildClient(ctx, item)
	if err != nil {
		return nil, err
	}
	return &biz.ResolvedStorageProvider{
		ID:            row.ID,
		Code:          row.Code,
		Type:          row.Type,
		DefaultBucket: row.DefaultBucket,
		Client:        client,
	}, nil
}

func (r *storageProviderRepo) resolveLegacyMinIO(_ context.Context) (*biz.ResolvedStorageProvider, error) {
	minio := r.oss.Minio
	configJSON, err := json.Marshal(map[string]interface{}{
		"endpoint":         minio.Endpoint,
		"access_key":       minio.AccessKey,
		"secret_key":       minio.SecretKey,
		"session_token":    minio.Token,
		"use_ssl":          minio.UseSsl,
		"force_path_style": true,
		"public_base_url":  minio.DownloadHost,
	})
	if err != nil {
		return nil, err
	}
	client, err := objectstorage.NewClient(biz.StorageProviderTypeS3Compatible, configJSON)
	if err != nil {
		return nil, err
	}
	return &biz.ResolvedStorageProvider{
		Code:          "legacy-minio",
		Type:          biz.StorageProviderTypeS3Compatible,
		DefaultBucket: "tenant-files",
		Client:        client,
	}, nil
}

func storageProviderObjectConfig(item *pbCore.StorageProvider) (json.RawMessage, error) {
	cfg := map[string]interface{}{
		"region":          item.GetRegion(),
		"use_ssl":         item.GetUseSsl(),
		"force_path_style": item.GetForcePathStyle(),
		"public_base_url":  item.GetPublicBaseUrl(),
	}
	switch item.GetType() {
	case biz.StorageProviderTypeS3Compatible:
		cfg["endpoint"] = item.GetEndpoint()
		cfg["access_key"] = item.GetAccessKey()
		cfg["secret_key"] = item.GetSecretKey()
		cfg["session_token"] = item.GetSessionToken()
	case biz.StorageProviderTypeLocal:
		cfg["base_path"] = item.GetLocalBasePath()
	}
	return json.Marshal(cfg)
}

func cleanLocalBasePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return value
	}
	return abs
}

func boolPtr(value bool) *bool { return &value }

func storageStringPtr(value string) *string { return &value }

func storageProviderInt32Ptr(value int32) *int32 { return &value }

const defaultStorageProviderBucket = "tenant-files"

func storageProviderStatus(row *gen.StorageProvider) int32 {
	if row == nil || row.Status == nil {
		return biz.StorageProviderStatusEnabled
	}
	return *row.Status
}
