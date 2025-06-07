package data

import (
	"backend-service/app/version/service/internal/conf"
	"context"

	"backend-service/app/version/service/internal/data/ent"
	"backend-service/app/version/service/internal/data/ent/migrate"

	"github.com/go-kratos/kratos/v2/log"
	// init mysql driver
	"entgo.io/ent/dialect/sql"
	_ "github.com/go-sql-driver/mysql"
)

// NewEntClient .
func NewEntClient(cfg *conf.Data, logger log.Logger) *ent.Client {
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

	client := ent.NewClient(
		ent.Driver(drv),
		ent.Log(func(a ...any) {
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

	return client
}

// NewData .
func NewEntClient111(conf *conf.Data_Database, log *log.Helper) (*ent.Client, *sql.Driver, error) {
	drv, err := sql.Open(
		conf.Driver,
		conf.Source,
	)
	if err != nil {
		log.Errorf("failed opening connection to sqlite: %v", err)
		return nil, nil, err
	}
	// Run the auto migration tool.
	client := ent.NewClient(ent.Driver(drv))
	if err := client.Schema.Create(context.Background()); err != nil {
		log.Errorf("failed creating schema resources: %v", err)
		return nil, drv, err
	}
	return client, drv, nil
}
