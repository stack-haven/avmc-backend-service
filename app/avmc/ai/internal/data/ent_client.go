package data

import (
	"backend-service/app/avmc/ai/internal/conf"
	"context"
	"time"

	"backend-service/app/avmc/ai/internal/data/ent/gen"
	"backend-service/app/avmc/ai/internal/data/ent/gen/intercept"
	"backend-service/app/avmc/ai/internal/data/ent/gen/migrate"

	_ "backend-service/app/avmc/ai/internal/data/ent/gen/runtime"

	// init mysql driver

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	_ "github.com/go-sql-driver/mysql"

	"github.com/go-kratos/kratos/v2/log"
)

// NewEntClient .
func NewEntClient(cfg *conf.Data, logger log.Logger) *gen.Client {
	l := log.NewHelper(log.With(logger, "module", "ent/data/initialize"))
	drv, err := sql.Open(cfg.Database.Driver, cfg.Database.Source)
	if err != nil {
		l.Fatalf("failed opening connection to %s: %v", cfg.Database.Driver, err)
		return nil
	}
	{
		db := drv.DB()
		// 连接池中最多保留的空闲连接数量
		db.SetMaxIdleConns(databaseMaxIdleConnections(cfg))
		// 连接池在同一时间打开连接的最大数量
		db.SetMaxOpenConns(databaseMaxOpenConnections(cfg))
		// 连接可重用的最大时间长度
		db.SetConnMaxLifetime(databaseConnectionMaxLifetime(cfg))
	}

	client := gen.NewClient(
		gen.Driver(drv),
		gen.Log(func(a ...any) {
			l.Debug(a...)
		}),
	)

	if cfg.Database.Debug {
		client = client.Debug()
	}

	// client.Use()
	client.Intercept(
		intercept.Func(func(ctx context.Context, q intercept.Query) error {
			// Limit all queries to 1000 records.
			// q.Limit(1000)
			if ent.QueryFromContext(ctx).Limit == nil {
				q.Limit(1000)
			}
			return nil
		}),
	)

	return client
}

func RunSchemaMigration(ctx context.Context, cfg *conf.Data, logger log.Logger) error {
	l := log.NewHelper(log.With(logger, "module", "ent/schema/migrate"))
	client := NewEntClient(cfg, logger)
	defer func() {
		if err := client.Close(); err != nil {
			l.Errorf("failed closing ent client: %v", err)
		}
	}()
	return client.Schema.Create(ctx, migrate.WithForeignKeys(false))
}

// NewEntData .
func NewEntData(cfg *conf.Data, logger log.Logger) *gen.Client {
	db, err := gen.Open(cfg.Database.Driver, cfg.Database.Source)
	if err != nil {
		log.Fatalf("failed opening connection to database: %v", err)
	}
	return db
}

func databaseMaxIdleConnections(cfg *conf.Data) int {
	if cfg != nil && cfg.Database != nil && cfg.Database.MaxIdleConnections > 0 {
		return int(cfg.Database.MaxIdleConnections)
	}
	return 10
}

func databaseMaxOpenConnections(cfg *conf.Data) int {
	if cfg != nil && cfg.Database != nil && cfg.Database.MaxOpenConnections > 0 {
		return int(cfg.Database.MaxOpenConnections)
	}
	return 50
}

func databaseConnectionMaxLifetime(cfg *conf.Data) time.Duration {
	if cfg != nil && cfg.Database != nil && cfg.Database.ConnectionMaxLifetime != nil {
		if d := cfg.Database.ConnectionMaxLifetime.AsDuration(); d > 0 {
			return d
		}
	}
	return 30 * time.Minute
}
