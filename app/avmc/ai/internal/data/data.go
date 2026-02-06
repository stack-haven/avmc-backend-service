package data

import (
	"backend-service/app/avmc/ai/internal/biz"
	"backend-service/app/avmc/ai/internal/conf"
	"backend-service/app/avmc/ai/internal/data/ent/gen"
	_ "backend-service/app/avmc/ai/internal/data/ent/gen/runtime"
	"context"
	"fmt"

	"github.com/bwmarrin/snowflake"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	redisotel "github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	// entrapper "github.com/casbin/ent-adapter"

	// casbinmodel "github.com/casbin/casbin/v2/model"

	_ "github.com/go-sql-driver/mysql"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData, NewTransaction, NewSnowflake,
	NewEntClient, NewRedisClient,
	NewChatRepo,
)

// Data .
type Data struct {
	// TODO wrapped database client
	db  *gen.Client
	rdb *redis.Client
	sf  *snowflake.Node
}

// NewData .
func NewData(
	c *conf.Data,
	db *gen.Client,
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
