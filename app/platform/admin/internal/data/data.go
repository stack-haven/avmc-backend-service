package data

import (
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/conf"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	_ "backend-service/app/platform/admin/internal/data/ent/gen/runtime"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	redisotel "github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/durationpb"

	// entrapper "github.com/casbin/ent-adapter"

	// casbinmodel "github.com/casbin/casbin/v2/model"

	"backend-service/pkg/auth"
	authnEngine "backend-service/pkg/auth/authn"
	authnJwt "backend-service/pkg/auth/authn/jwt"
	"backend-service/pkg/auth/loginattempt"

	authzEngine "backend-service/pkg/auth/authz"
	authzCasbin "backend-service/pkg/auth/authz/casbin"
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
	NewTenantRepo,
	NewAuthRepo,
	NewUserRepo,
	NewRoleRepo,
	NewMenuRepo,
	NewMenuPermissionGroupRepo,
	NewPostRepo,
	NewDeptRepo,
	NewProjectRepo,
	NewDictionaryRepo,
	NewOperationLogRepo,
	NewLoginLogRepo,
	NewSessionRepo,
	NewParameterRepo,
	NewAsyncTaskRepo,
	NewPermissionCacheInvalidator,
	NewTenantAdminPolicy,
)

// Data .
type Data struct {
	db                    *gen.Client
	rdb                   *redis.Client
	permissionCacheBypass sync.Map
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

func NewHealthChecker(data *Data) pkgHealth.Checker {
	return pkgHealth.CheckFunc(func(ctx context.Context) error {
		ctx = entviewer.NewSystemContext(ctx)
		var errs []error
		if _, err := data.db.User.Query().Limit(1).Count(ctx); err != nil {
			errs = append(errs, fmt.Errorf("database: %w", err))
		}
		if err := data.rdb.Ping(ctx).Err(); err != nil {
			errs = append(errs, fmt.Errorf("redis: %w", err))
		}
		return errors.Join(errs...)
	})
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
		DialTimeout:  configDuration(cfg.Redis.GetDialTimeout(), time.Second),
		WriteTimeout: configDuration(cfg.Redis.GetWriteTimeout(), 500*time.Millisecond),
		ReadTimeout:  configDuration(cfg.Redis.GetReadTimeout(), 500*time.Millisecond),
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

// NewAuthenticator 创建认证器
func NewAuthenticator(c *conf.Server, logger log.Logger, authSecurity *auth.AuthSecurity) (authnEngine.Authenticator, error) {
	if c == nil || c.Http == nil || c.Http.Middleware == nil || c.Http.Middleware.Auth == nil {
		return nil, fmt.Errorf("http auth config is required")
	}
	expires := configDuration(c.Http.Middleware.Auth.ExpiresTime, 7*24*time.Hour)
	// 令牌过期时间默认 7天
	if expires == 0 {
		expires = time.Hour * 24 * 7
	}
	// 刷新令牌过期时间 = 令牌过期时间 * 10
	refreshExpires := expires * 10
	// 使用jwt提供者
	provider := authnJwt.NewProvider()
	authenticator, err := provider.NewAuthenticator(
		context.Background(),
		authnEngine.WithSigningKey([]byte(c.Http.Middleware.Auth.Key)),
		authnEngine.WithSigningMethod(c.Http.Middleware.Auth.Method),
		authnEngine.WithTokenExpiration(expires),
		authnEngine.WithRefreshTokenExpiration(refreshExpires),
		authnEngine.WithUserFactory(authSecurity.NewSecurityUser),
	)
	if err != nil {
		return nil, fmt.Errorf("creating authenticator: %w", err)
	}
	return authenticator, nil
}

func configDuration(d *durationpb.Duration, fallback time.Duration) time.Duration {
	if d == nil {
		return fallback
	}
	v := d.AsDuration()
	if v <= 0 {
		return fallback
	}
	return v
}

// NewAuthorizer 创建权鉴器
func NewAuthorizer(cfg *conf.Data, db *gen.Client, logger log.Logger) (authzEngine.Authorizer, error) {
	// adapter, err := entrapper.NewAdapter(cfg.Database.Driver, cfg.Database.Source)
	// if err != nil {
	// 	l.Fatalf("failed creating adapter: %s", err.Error())
	// 	panic(err)
	// }
	// model, err := casbinmodel.NewModelFromString(authzCasbin.DefaultAbacModel)
	// if err != nil {
	// 	log.Fatalf("failed casbin model connection %v", err)
	// }

	if cfg == nil || cfg.Database == nil {
		return nil, fmt.Errorf("database config is required")
	}
	provider := authzCasbin.NewProvider()
	authorizer, err := provider.NewAuthorizer(
		context.Background(),
		authzEngine.WithAdapterType(authzEngine.AdapterMySQL),
		authzEngine.WithAdapterDSN(cfg.Database.Source),
	)

	if err != nil {
		return nil, fmt.Errorf("creating authorizer: %w", err)
	}
	return newTenantRoleAuthorizer(authorizer, db), nil
}
