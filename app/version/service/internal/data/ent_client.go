package data

import (
	"backend-service/app/version/service/internal/conf"
	"context"
	"time"

	"backend-service/app/version/service/internal/data/ent"

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
		db.SetMaxIdleConns(databaseMaxIdleConnections(cfg))
		// 连接池在同一时间打开连接的最大数量
		db.SetMaxOpenConns(databaseMaxOpenConnections(cfg))
		// 连接可重用的最大时间长度
		db.SetConnMaxLifetime(databaseConnectionMaxLifetime(cfg))
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
