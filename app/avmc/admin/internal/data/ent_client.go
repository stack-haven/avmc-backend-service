package data

import (
	"backend-service/app/avmc/admin/internal/conf"
	"context"

	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/intercept"
	"backend-service/app/avmc/admin/internal/data/ent/gen/migrate"

	_ "backend-service/app/avmc/admin/internal/data/ent/gen/runtime"

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
		db.SetMaxIdleConns(int(cfg.Database.MaxIdleConnections))
		// 连接池在同一时间打开连接的最大数量
		db.SetMaxOpenConns(int(cfg.Database.MaxOpenConnections))
		// 连接可重用的最大时间长度
		db.SetConnMaxLifetime(cfg.Database.ConnectionMaxLifetime.AsDuration())
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

	// 运行数据库迁移工具
	if cfg.Database.Migrate {
		if err = client.Schema.Create(
			context.Background(),
			migrate.WithForeignKeys(true),
			migrate.WithDropIndex(true),
			migrate.WithDropColumn(true),
			migrate.WithForeignKeys(false),
		); err != nil {
			l.Fatalf("failed creating schema resources: %v", err)
		}
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
