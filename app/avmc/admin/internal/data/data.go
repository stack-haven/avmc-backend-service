package data

import (
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/conf"
	"context"
	"fmt"

	"github.com/bwmarrin/snowflake"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	redisotel "github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	entrapper "github.com/casbin/ent-adapter"

	// casbinmodel "github.com/casbin/casbin/v2/model"

	authnEngine "backend-service/pkg/auth/authn"
	authnJwt "backend-service/pkg/auth/authn/jwt"

	authzEngine "backend-service/pkg/auth/authz"
	authzCasbin "backend-service/pkg/auth/authz/casbin"

	"backend-service/app/avmc/admin/internal/data/ent"

	_ "github.com/go-sql-driver/mysql"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData, NewTransaction, NewSnowflake,
	NewEntClient, NewRedisClient,
	NewAuthenticator, NewAuthorizer, NewSecurityUser,
	NewAuthTokenRepo,
	NewAuthRepo,
	NewUserRepo,
	NewRoleRepo,
	NewMenuRepo,
	NewPostRepo,
	NewDeptRepo,
)

// Data .
type Data struct {
	// TODO wrapped database client
	db  *ent.Client
	rdb *redis.Client
	sf  *snowflake.Node
}

// NewData .
func NewData(
	c *conf.Data,
	db *ent.Client,
	rdb *redis.Client,
	sf *snowflake.Node,
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
		sf:  sf,
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
	tx := ent.TxFromContext(ctx)
	if tx != nil {
		return fn(ctx)
	}

	tx, err := d.db.Tx(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}

	if err = fn(ent.NewTxContext(ctx, tx)); err != nil {
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

func (d *Data) DB(ctx context.Context) *ent.Client {
	tx := ent.TxFromContext(ctx)
	if tx != nil {
		return tx.Client()
	}
	return d.db
}

// NewSnowflake 生成雪花算法id
func NewSnowflake(logger log.Logger) *snowflake.Node {
	l := log.NewHelper(log.With(logger, "module", "snowflake/data/initialize"))
	sf, err := snowflake.NewNode(1)
	if err != nil {
		l.Fatal("snowflake no init")
	}
	l.Infof("init snowflake ID：%s", sf.Generate())
	return sf
}

// NewRedisClient 创建Redis客户端
func NewRedisClient(cfg *conf.Data, logger log.Logger) (rdb *redis.Client) {
	l := log.NewHelper(log.With(logger, "module", "redis/data/initialize"))
	if rdb = redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.GetAddr(),
		Password:     cfg.Redis.GetPassword(),
		DB:           int(cfg.Redis.GetDb()),
		DialTimeout:  cfg.Redis.GetDialTimeout().AsDuration(),
		WriteTimeout: cfg.Redis.GetWriteTimeout().AsDuration(),
		ReadTimeout:  cfg.Redis.GetReadTimeout().AsDuration(),
	}); rdb == nil {
		l.Fatalf("failed opening connection to redis")
		return nil
	}

	// open tracing instrumentation.
	if cfg.Redis.GetEnableTracing() {
		if err := redisotel.InstrumentTracing(rdb); err != nil {
			l.Fatalf("failed open tracing: %s", err.Error())
			panic(err)
		}
	}

	// open metrics instrumentation.
	if cfg.Redis.GetEnableMetrics() {
		if err := redisotel.InstrumentMetrics(rdb); err != nil {
			l.Fatalf("failed open metrics: %s", err.Error())
			panic(err)
		}
	}
	return rdb
}

// NewAuthenticator 创建认证器
func NewAuthenticator(c *conf.Server, logger log.Logger) authnEngine.Authenticator {
	l := log.NewHelper(log.With(logger, "module", "authenticators/auth/initialize"))
	authenticator, err := authnJwt.NewAuthenticator(
		authnJwt.WithKey([]byte(c.Http.Middleware.Auth.Key)),
		authnJwt.WithSigningMethod(c.Http.Middleware.Auth.Method),
	)
	if err != nil {
		l.Fatalf("failed creating authentincator: %s", err.Error())
		panic(err)
	}
	return authenticator
}

// NewAuthorizer 创建权鉴器
func NewAuthorizer(cfg *conf.Data, logger log.Logger) authzEngine.Authorized {
	l := log.NewHelper(log.With(logger, "module", "authorizer/auth/initialize"))
	adapter, err := entrapper.NewAdapter(cfg.Database.Driver, cfg.Database.Source)
	if err != nil {
		l.Fatalf("failed creating adapter: %s", err.Error())
		panic(err)
	}
	// model, err := casbinmodel.NewModelFromString(authzCasbin.DefaultAbacModel)
	// if err != nil {
	// 	log.Fatalf("failed casbin model connection %v", err)
	// }

	engine, err := authzCasbin.NewAuthorized(
		context.Background(),
		authzCasbin.WithPolicyAdapter(adapter),
		// authzCasbin.WithModel(model),
	)
	if err != nil {
		l.Fatalf("failed creating engine: %s", err.Error())
		panic(err)
	}
	return engine
}
