package data

import (
	"backend-service/app/evie/service/internal/biz"
	"backend-service/app/evie/service/internal/conf"
	"backend-service/app/evie/service/internal/data/ent/gen"
	_ "backend-service/app/evie/service/internal/data/ent/gen/runtime"
	entviewer "backend-service/app/evie/service/internal/data/ent/viewer"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	redisotel "github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	// entrapper "github.com/casbin/ent-adapter"

	// casbinmodel "github.com/casbin/casbin/v2/model"

	"backend-service/pkg/audit"
	"backend-service/pkg/auth"
	authnEngine "backend-service/pkg/auth/authn"
	"backend-service/pkg/auth/loginattempt"
	"backend-service/pkg/utils"

	platformclient "backend-service/app/platform/service/client"
	authzEngine "backend-service/pkg/auth/authz"
	pkgHealth "backend-service/pkg/health"

	_ "github.com/go-sql-driver/mysql"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData, NewTransaction,
	NewEntClient, NewRedisClient,
	NewHealthChecker,
	NewLoginAttemptGuard,
	NewAuthenticator, NewAuthorizer, auth.NewAuthSecurity,
	auth.NewAuthToken,
	NewDictionaryRepo,
	NewHotwordRepo,
	NewASRRecordRepo,
	NewProviderConfigRepo,
	NewCorrectionRuleRepo,
	NewCorrectionLogRepo,
	NewFileCenterClient,
	NewAuditClient,
)

// Data .
type Data struct {
	db                    *gen.Client
	rdb                   *redis.Client
	permissionCacheBypass sync.Map
	authorizationCache    sync.Map
	authorizationStats    tenantAuthorizationCacheStats
	resourceQuotaStats    resourceQuotaStats
}

type tenantAuthorizationCacheStats struct {
	hits          atomic.Uint64
	misses        atomic.Uint64
	sets          atomic.Uint64
	bypasses      atomic.Uint64
	expired       atomic.Uint64
	clears        atomic.Uint64
	invalidations atomic.Uint64
}

type tenantAuthorizationCacheStatsSnapshot struct {
	Hits          uint64
	Misses        uint64
	Sets          uint64
	Bypasses      uint64
	Expired       uint64
	Clears        uint64
	Invalidations uint64
}

type resourceQuotaStats struct {
	consumes             atomic.Uint64
	releases             atomic.Uint64
	quotaExceeded        atomic.Uint64
	idempotencyConflicts atomic.Uint64
}

type resourceQuotaStatsSnapshot struct {
	Consumes             uint64
	Releases             uint64
	QuotaExceeded        uint64
	IdempotencyConflicts uint64
}

func (d *Data) tenantAuthorizationCacheStatsSnapshot() tenantAuthorizationCacheStatsSnapshot {
	if d == nil {
		return tenantAuthorizationCacheStatsSnapshot{}
	}
	return tenantAuthorizationCacheStatsSnapshot{
		Hits:          d.authorizationStats.hits.Load(),
		Misses:        d.authorizationStats.misses.Load(),
		Sets:          d.authorizationStats.sets.Load(),
		Bypasses:      d.authorizationStats.bypasses.Load(),
		Expired:       d.authorizationStats.expired.Load(),
		Clears:        d.authorizationStats.clears.Load(),
		Invalidations: d.authorizationStats.invalidations.Load(),
	}
}

func (d *Data) resourceQuotaStatsSnapshot() resourceQuotaStatsSnapshot {
	if d == nil {
		return resourceQuotaStatsSnapshot{}
	}
	return resourceQuotaStatsSnapshot{
		Consumes:             d.resourceQuotaStats.consumes.Load(),
		Releases:             d.resourceQuotaStats.releases.Load(),
		QuotaExceeded:        d.resourceQuotaStats.quotaExceeded.Load(),
		IdempotencyConflicts: d.resourceQuotaStats.idempotencyConflicts.Load(),
	}
}

// NewData .
func NewData(
	c *conf.Data,
	db *gen.Client,
	rdb *redis.Client,
	logger log.Logger,
) (*Data, func(), error) {
	log := log.NewHelper(log.With(logger, "data", "data/initialize"))
	cleanup := func() {
		log.Info("closing the data resources")
		if err := db.Close(); err != nil {
			log.Error(err)
		}
		if err := rdb.Close(); err != nil {
			log.Error(err)
		}
	}
	d := &Data{
		db:  db,
		rdb: rdb,
	}

	return d, cleanup, nil
}

// NewTransaction 事务
func NewTransaction(data *Data) biz.Transaction {
	// return data.db
	return data
}

// InTx 执行事务
func (d *Data) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx := gen.TxFromContext(ctx)
	if tx != nil {
		return fn(ctx)
	}

	tx, err := d.db.Tx(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}

	if err = fn(gen.NewTxContext(ctx, tx)); err != nil {
		if err2 := tx.Rollback(); err2 != nil {
			return fmt.Errorf("rolling back transaction: %v (original error: %w)", err2, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return err
}

func (d *Data) DB(ctx context.Context) *gen.Client {
	tx := gen.TxFromContext(ctx)
	if tx != nil {
		return tx.Client()
	}
	return d.db
}

type platformHealthChecker struct {
	data *Data
}

func NewHealthChecker(data *Data) pkgHealth.Checker {
	return &platformHealthChecker{data: data}
}

func (c *platformHealthChecker) Ready(ctx context.Context) error {
	ctx = entviewer.NewSystemContext(ctx)
	var errs []error
	if c == nil || c.data == nil || c.data.db == nil {
		errs = append(errs, fmt.Errorf("database: unavailable"))
	} else if _, err := c.data.db.DictionaryWord.Query().Limit(1).Count(ctx); err != nil {
		errs = append(errs, fmt.Errorf("database: %w", err))
	}
	if c == nil || c.data == nil || c.data.rdb == nil {
		errs = append(errs, fmt.Errorf("redis: unavailable"))
	} else if err := c.data.rdb.Ping(ctx).Err(); err != nil {
		errs = append(errs, fmt.Errorf("redis: %w", err))
	}
	return errors.Join(errs...)
}

func (c *platformHealthChecker) Details(context.Context) map[string]any {
	if c == nil || c.data == nil {
		return nil
	}
	stats := c.data.tenantAuthorizationCacheStatsSnapshot()
	lookups := stats.Hits + stats.Misses + stats.Expired
	totalDecisions := lookups + stats.Bypasses
	quotaStats := c.data.resourceQuotaStatsSnapshot()
	quotaMutations := quotaStats.Consumes + quotaStats.Releases
	return map[string]any{
		"authorization_cache": map[string]any{
			"hits":          stats.Hits,
			"misses":        stats.Misses,
			"sets":          stats.Sets,
			"bypasses":      stats.Bypasses,
			"expired":       stats.Expired,
			"clears":        stats.Clears,
			"invalidations": stats.Invalidations,
			"hit_rate":      ratio(stats.Hits, lookups),
			"bypass_rate":   ratio(stats.Bypasses, totalDecisions),
		},
		"resource_quota": map[string]any{
			"consumes":              quotaStats.Consumes,
			"releases":              quotaStats.Releases,
			"quota_exceeded":        quotaStats.QuotaExceeded,
			"idempotency_conflicts": quotaStats.IdempotencyConflicts,
			"mutations":             quotaMutations,
			"exceeded_rate":         ratio(quotaStats.QuotaExceeded, quotaStats.Consumes),
			"conflict_rate":         ratio(quotaStats.IdempotencyConflicts, quotaMutations),
		},
	}
}

func ratio(value uint64, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(value) / float64(total)
}

func NewLoginAttemptGuard(rdb *redis.Client) (loginattempt.Guard, error) {
	opts, err := loginattempt.OptionsFromEnv("platform_ADMIN_LOGIN")
	if err != nil {
		return nil, err
	}
	return loginattempt.NewRedisGuard(rdb, opts), nil
}

// NewRedisClient 创建Redis客户端
func NewRedisClient(cfg *conf.Data, logger log.Logger) (*redis.Client, error) {
	l := log.NewHelper(log.With(logger, "module", "redis/data/initialize"))
	if cfg == nil || cfg.Redis == nil {
		return nil, fmt.Errorf("redis config is required")
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.GetAddr(),
		Password:     cfg.Redis.GetPassword(),
		DB:           int(cfg.Redis.GetDb()),
		DialTimeout:  utils.Duration(cfg.Redis.GetDialTimeout(), time.Second),
		WriteTimeout: utils.Duration(cfg.Redis.GetWriteTimeout(), 500*time.Millisecond),
		ReadTimeout:  utils.Duration(cfg.Redis.GetReadTimeout(), 500*time.Millisecond),
	})

	// open tracing instrumentation.
	if cfg.Redis.GetEnableTracing() {
		if err := redisotel.InstrumentTracing(rdb); err != nil {
			return nil, fmt.Errorf("opening redis tracing: %w", err)
		}
	}

	// open metrics instrumentation.
	if cfg.Redis.GetEnableMetrics() {
		if err := redisotel.InstrumentMetrics(rdb); err != nil {
			return nil, fmt.Errorf("opening redis metrics: %w", err)
		}
	}
	l.Infof("initialized redis client: addr=%s db=%d", cfg.Redis.GetAddr(), cfg.Redis.GetDb())
	return rdb, nil
}

// NewAuthenticator 创建认证器（JWT 本地验签，认证工厂已收敛到 pkg/auth）。
func NewAuthenticator(c *conf.Server, logger log.Logger, authSecurity *auth.AuthSecurity) (authnEngine.Authenticator, error) {
	if c == nil || c.Http == nil || c.Http.Middleware == nil || c.Http.Middleware.Auth == nil {
		return nil, fmt.Errorf("http auth config is required")
	}
	expires := utils.Duration(c.Http.Middleware.Auth.ExpiresTime, 7*24*time.Hour)
	// 令牌过期时间默认 7天
	if expires == 0 {
		expires = time.Hour * 24 * 7
	}
	return auth.NewAuthenticator(auth.AuthConfig{
		Key:               c.Http.Middleware.Auth.Key,
		Method:            c.Http.Middleware.Auth.Method,
		AccessExpiration:  expires,
		RefreshExpiration: expires * 10,
	}, authSecurity)
}

// NewAuthorizer 创建权鉴器。
// evie 作为产品服务不维护本地 Casbin 策略，鉴权通过 gRPC 委托给技术中台（platform/admin）的
// core.service.v1.AuthService.IsAuthorized，复用中台的租户用户权限体系。
func NewAuthorizer(cfg *conf.Client, logger log.Logger) (authzEngine.Enforcer, error) {
	if cfg == nil || cfg.Grpc == nil || cfg.Grpc.GetAddr() == "" {
		return nil, fmt.Errorf("client.grpc.addr is required for gRPC authorization delegation")
	}
	authorizer, err := platformclient.NewAuthorizer(context.Background(), cfg.Grpc.GetAddr())
	if err != nil {
		return nil, fmt.Errorf("creating gRPC authorizer: %w", err)
	}
	log.NewHelper(log.With(logger, "module", "authz/grpc")).
		Infof("initialized gRPC authorizer: endpoint=%s", cfg.Grpc.GetAddr())
	return authorizer, nil
}

// NewFileCenterClient 创建文件中心客户端（用于音频上传/预览）。
func NewFileCenterClient(cfg *conf.Client, logger log.Logger) (*platformclient.FileCenterClient, error) {
	if cfg == nil || cfg.Grpc == nil || cfg.Grpc.GetAddr() == "" {
		return nil, fmt.Errorf("client.grpc.addr is required for file center")
	}
	client, err := platformclient.NewFileCenterClient(context.Background(), cfg.Grpc.GetAddr())
	if err != nil {
		return nil, fmt.Errorf("creating file center client: %w", err)
	}
	log.NewHelper(log.With(logger, "module", "filecenter/grpc")).
		Infof("initialized file center client: endpoint=%s", cfg.Grpc.GetAddr())
	return client, nil
}

// NewAuditClient 创建审计客户端。
// evie 的操作审计通过 gRPC 委托给技术中台（platform/admin）的 OperationLogService，
// 复用中台的 append-only 审计与脱敏能力。
func NewAuditClient(cfg *conf.Client, logger log.Logger) (audit.Client, error) {
	if cfg == nil || cfg.Grpc == nil || cfg.Grpc.GetAddr() == "" {
		return nil, fmt.Errorf("client.grpc.addr is required for gRPC audit delegation")
	}
	client, err := platformclient.NewAuditClient(context.Background(), cfg.Grpc.GetAddr())
	if err != nil {
		return nil, fmt.Errorf("creating gRPC audit client: %w", err)
	}
	log.NewHelper(log.With(logger, "module", "audit/grpc")).
		Infof("initialized gRPC audit client: endpoint=%s", cfg.Grpc.GetAddr())
	return client, nil
}
