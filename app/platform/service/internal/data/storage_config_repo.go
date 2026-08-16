package data

import (
	"context"
	"fmt"
	"time"

	pb "backend-service/api/platform/service/v1"

	"backend-service/app/platform/service/internal/biz"
	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/storageconfig"
	entviewer "backend-service/app/platform/service/internal/data/ent/viewer"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/objectstorage"
	"backend-service/pkg/utils/convert"
	"backend-service/pkg/utils/crypto"

	"google.golang.org/protobuf/proto"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var _ biz.StorageConfigRepo = (*storageConfigRepo)(nil)

type storageConfigRepo struct {
	Data  *Data
	crypt *storageCrypto
	log   *log.Helper
}

// ───────────────────────────── Crypto ─────────────────────────────

type storageCrypto struct {
	key []byte
}

// NewStorageCrypto creates a crypto helper for storage config encryption.
func NewStorageCrypto() *storageCrypto {
	defaultKey := "storage-config-key-32bytes-replace"
	key := []byte(defaultKey)
	if len(key) != 32 {
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	}
	return &storageCrypto{key: key}
}

func (c *storageCrypto) encrypt(plaintext []byte) (string, error) {
	if len(c.key) == 0 {
		return string(plaintext), nil
	}
	return crypto.Encrypt(plaintext, c.key)
}

func (c *storageCrypto) decrypt(encoded string) ([]byte, error) {
	if len(c.key) == 0 {
		return []byte(encoded), nil
	}
	return crypto.Decrypt(encoded, c.key)
}

func NewStorageConfigRepo(data *Data, crypt *storageCrypto, logger log.Logger) biz.StorageConfigRepo {
	return &storageConfigRepo{
		Data:  data,
		crypt: crypt,
		log:   log.NewHelper(log.With(logger, "module", "data/storage-config")),
	}
}

// ───────────────────────────── CRUD ─────────────────────────────

func (r *storageConfigRepo) Save(ctx context.Context, g *pb.StorageConfig) (*pb.StorageConfig, error) {
	tenantID, err := r.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	encrypted, err := r.crypt.encrypt([]byte(g.GetConfigJson()))
	if err != nil {
		return nil, fmt.Errorf("encrypt config: %w", err)
	}
	row, err := r.Data.DB(ctx).StorageConfig.Create().
		SetTenantID(tenantID).
		SetName(g.GetName()).
		SetProvider(g.GetProvider()).
		SetNillablePurpose(nilIfEmpty(g.GetPurpose())).
		SetBucket(g.GetBucket()).
		SetIsDefault(g.GetIsDefault()).
		SetConfigJSON(encrypted).
		SetHealthStatus("unknown").
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.entToProto(row), nil
}

func (r *storageConfigRepo) Update(ctx context.Context, g *pb.StorageConfig) (*pb.StorageConfig, error) {
	m := r.Data.DB(ctx).StorageConfig.UpdateOneID(g.GetId())
	if g.GetName() != "" {
		m.SetName(g.GetName())
	}
	if g.GetProvider() != "" {
		m.SetProvider(g.GetProvider())
	}
	if g.Purpose != nil {
		m.SetPurpose(g.GetPurpose())
	}
	m.SetBucket(g.GetBucket())
	m.SetIsDefault(g.GetIsDefault())
	if g.GetConfigJson() != "" {
		encrypted, err := r.crypt.encrypt([]byte(g.GetConfigJson()))
		if err != nil {
			return nil, fmt.Errorf("encrypt config: %w", err)
		}
		m.SetConfigJSON(encrypted)
	}
	m.SetUpdatedAt(time.Now())
	row, err := m.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.entToProto(row), nil
}

func (r *storageConfigRepo) FindByID(ctx context.Context, id uint32) (*pb.StorageConfig, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	row, err := r.Data.DB(ctx).StorageConfig.Query().
		Where(storageconfig.IDEQ(id), storageconfig.DeletedAtIsNil()).
		Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("STORAGE_CONFIG_NOT_FOUND", "存储配置不存在")
	}
	if err != nil {
		return nil, err
	}
	return r.entToProto(row), nil
}

func (r *storageConfigRepo) ListConfigs(ctx context.Context, opts ...listing.Option) ([]*pb.StorageConfig, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	rows, err := r.Data.DB(ctx).StorageConfig.Query().
		Select(
			storageconfig.FieldID, storageconfig.FieldName,
			storageconfig.FieldProvider, storageconfig.FieldPurpose,
			storageconfig.FieldBucket, storageconfig.FieldIsDefault,
			storageconfig.FieldConfigJSON, storageconfig.FieldHealthStatus,
			storageconfig.FieldCreatedAt, storageconfig.FieldUpdatedAt,
		).
		Where(storageconfig.DeletedAtIsNil()).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(rows, r.entToProto), nil
}

func (r *storageConfigRepo) CountConfigs(ctx context.Context, opts ...listing.Option) (int32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	o := listing.Options{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.Data.DB(ctx).StorageConfig.Query().
		Select(storageconfig.FieldID).
		Where(storageconfig.DeletedAtIsNil()).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *storageConfigRepo) Delete(ctx context.Context, id uint32) error {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return err
	}
	return r.Data.DB(ctx).StorageConfig.DeleteOneID(id).Exec(ctx)
}

func (r *storageConfigRepo) SetDefault(ctx context.Context, tenantID uint32, id uint32) error {
	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.StorageConfig.Update().
		Where(storageconfig.TenantIDEQ(tenantID), storageconfig.IsDefaultEQ(true)).
		SetIsDefault(false).
		Save(ctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.StorageConfig.UpdateOneID(id).
		SetIsDefault(true).
		SetUpdatedAt(time.Now()).
		Exec(ctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *storageConfigRepo) MarkHealth(ctx context.Context, id uint32, status string) error {
	return r.Data.DB(ctx).StorageConfig.UpdateOneID(id).
		SetHealthStatus(status).
		SetLastCheckedAt(time.Now()).
		SetUpdatedAt(time.Now()).
		Exec(ctx)
}

// ───────────────────────────── Resolver ─────────────────────────────

func (r *storageConfigRepo) ResolveDefault(ctx context.Context, tenantID uint32) (*biz.ResolvedStorage, error) {
	row, err := r.Data.DB(ctx).StorageConfig.Query().
		Where(
			storageconfig.TenantIDEQ(tenantID),
			storageconfig.IsDefaultEQ(true),
			storageconfig.DeletedAtIsNil(),
		).
		Only(ctx)
	if gen.IsNotFound(err) {
		row, err = r.Data.DB(ctx).StorageConfig.Query().
			Where(storageconfig.TenantIDEQ(tenantID), storageconfig.DeletedAtIsNil()).
			First(ctx)
		if gen.IsNotFound(err) {
			return nil, errors.NotFound("STORAGE_CONFIG_NOT_FOUND", "租户未配置存储")
		}
	}
	if err != nil {
		return nil, err
	}
	return r.resolveRow(row)
}

func (r *storageConfigRepo) ResolveByPurpose(ctx context.Context, tenantID uint32, purpose string) (*biz.ResolvedStorage, error) {
	row, err := r.Data.DB(ctx).StorageConfig.Query().
		Where(
			storageconfig.TenantIDEQ(tenantID),
			storageconfig.PurposeEQ(purpose),
			storageconfig.DeletedAtIsNil(),
		).
		First(ctx)
	if gen.IsNotFound(err) {
		return r.ResolveDefault(ctx, tenantID)
	}
	return r.resolveRow(row)
}

func (r *storageConfigRepo) resolveRow(row *gen.StorageConfig) (*biz.ResolvedStorage, error) {
	configJSON, err := r.crypt.decrypt(row.ConfigJSON)
	if err != nil {
		return nil, fmt.Errorf("decrypt config: %w", err)
	}
	client, err := objectstorage.NewClient(row.Provider, configJSON)
	if err != nil {
		return nil, fmt.Errorf("build storage client: %w", err)
	}
	return &biz.ResolvedStorage{
		ID:       row.ID,
		Name:     row.Name,
		Provider: row.Provider,
		Bucket:   row.Bucket,
		Client:   client,
	}, nil
}

// ───────────────────────────── ent ↔ proto ─────────────────────────────

// entToProto converts ent StorageConfig to proto StorageConfig.
// Decryption errors are logged and the config_json is masked.
func (r *storageConfigRepo) entToProto(row *gen.StorageConfig) *pb.StorageConfig {
	configJSON, err := r.crypt.decrypt(row.ConfigJSON)
	if err != nil {
		r.log.Warnf("decrypt storage config %d: %v", row.ID, err)
		configJSON = []byte("*** decrypt failed ***")
	}
	g := &pb.StorageConfig{
		Id:           row.ID,
		Name:         &row.Name,
		Provider:     &row.Provider,
		Bucket:       &row.Bucket,
		IsDefault:    &row.IsDefault,
		ConfigJson:   proto.String(string(configJSON)),
		HealthStatus: &row.HealthStatus,
		CreatedAt:    proto.String(row.CreatedAt.Format("2006-01-02 15:04:05")),
		UpdatedAt:    proto.String(row.UpdatedAt.Format("2006-01-02 15:04:05")),
	}
	if row.Purpose != "" {
		g.Purpose = &row.Purpose
	}
	return g
}

// ───────────────────────────── helpers ─────────────────────────────

func (r *storageConfigRepo) RequireTenantID(ctx context.Context) (uint32, error) {
	tenantID, ok := entviewer.TenantID(ctx)
	if !ok || tenantID == 0 {
		return 0, errors.Forbidden("TENANT_REQUIRED", "租户上下文缺失")
	}
	return tenantID, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
