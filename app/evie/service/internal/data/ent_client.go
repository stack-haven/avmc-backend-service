package data

import (
	"backend-service/app/evie/service/internal/conf"
	"context"
	"fmt"
	"time"

	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionarycategory"
	"backend-service/app/evie/service/internal/data/ent/gen/intercept"
	"backend-service/app/evie/service/internal/data/ent/gen/migrate"
	entviewer "backend-service/app/evie/service/internal/data/ent/viewer"

	_ "backend-service/app/evie/service/internal/data/ent/gen/runtime"

	// init mysql driver

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	_ "github.com/go-sql-driver/mysql"

	"github.com/go-kratos/kratos/v2/log"
)

// NewEntClient .
func NewEntClient(cfg *conf.Data, logger log.Logger) (*gen.Client, error) {
	l := log.NewHelper(log.With(logger, "module", "ent/data/initialize"))
	if cfg == nil || cfg.Database == nil {
		return nil, fmt.Errorf("database config is required")
	}
	drv, err := sql.Open(cfg.Database.Driver, cfg.Database.Source)
	if err != nil {
		return nil, fmt.Errorf("opening connection to %s: %w", cfg.Database.Driver, err)
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
	l.Infof("initialized ent client: driver=%s", cfg.Database.Driver)

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

	return client, nil
}

func RunSchemaMigration(ctx context.Context, cfg *conf.Data, logger log.Logger) error {
	l := log.NewHelper(log.With(logger, "module", "ent/schema/migrate"))
	client, err := NewEntClient(cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := client.Close(); err != nil {
			l.Errorf("failed closing ent client: %v", err)
		}
	}()
	if err := client.Schema.Create(
		ctx,
		migrate.WithForeignKeys(false),
		// Migrations are explicit for Admin while the product is unreleased, so
		// obsolete pre-tenant columns/indexes can be removed instead of kept.
		migrate.WithDropIndex(true),
		migrate.WithDropColumn(true),
	); err != nil {
		return err
	}
	if err := seedBuiltinCategories(ctx, client, l); err != nil {
		return err
	}
	return backfillTenantAdminRoleMarker(ctx, client, l)
}

// seedBuiltinCategories 幂等插入内置词条分类（tenant_id=0 全局共享）。
func seedBuiltinCategories(ctx context.Context, client *gen.Client, l *log.Helper) error {
	builtins := []struct {
		code, name string
		sort       int32
	}{
		{"PERSON", "人物", 10},
		{"ORGANIZATION", "组织", 20},
		{"PRODUCT", "产品", 30},
		{"LOCATION", "地点", 40},
		{"PERSON_TITLE", "职务", 50},
		{"BUSINESS_TERM", "业务术语", 60},
		{"TECH_TERM", "技术术语", 70},
		{"OTHER", "其他", 99},
	}
	sysCtx := entviewer.NewSystemContext(ctx)
	for _, b := range builtins {
		exists, err := client.DictionaryCategory.Query().
			Where(dictionarycategory.CodeEQ(b.code), dictionarycategory.BuiltinEQ(true)).
			Exist(sysCtx)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := client.DictionaryCategory.Create().
			SetTenantID(0).
			SetCode(b.code).
			SetName(b.name).
			SetBuiltin(true).
			SetSort(b.sort).
			Save(sysCtx); err != nil {
			return err
		}
	}
	l.Infof("seeded %d builtin dictionary categories", len(builtins))
	return nil
}

func backfillTenantAdminRoleMarker(_ context.Context, _ *gen.Client, _ *log.Helper) error {
	// TODO: 添加 Role schema 后实现
	return nil
}

// NewEntData .
func NewEntData(cfg *conf.Data, logger log.Logger) (*gen.Client, error) {
	if cfg == nil || cfg.Database == nil {
		return nil, fmt.Errorf("database config is required")
	}
	db, err := gen.Open(cfg.Database.Driver, cfg.Database.Source)
	if err != nil {
		return nil, fmt.Errorf("opening connection to database: %w", err)
	}
	return db, nil
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
