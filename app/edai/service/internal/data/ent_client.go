package data

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backend-service/app/edai/service/internal/data/ent/gen"
	"backend-service/pkg/conf"

	// ent 运行期/隐私/钩子支持
	_ "backend-service/app/edai/service/internal/data/ent/gen/runtime"

	entsql "entgo.io/ent/dialect/sql"

	// 驱动注册：SQLite（D5）与 MySQL（备用）
	_ "github.com/glebarez/go-sqlite"
	_ "github.com/go-sql-driver/mysql"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	defaultMaxIdleConns    = 10
	defaultMaxOpenConns    = 20
	defaultConnMaxLifetime = time.Hour
)

// NewEntClient 创建 Ent 客户端。
//
// D5（4-2 ADR-013）：V1.0 使用 SQLite + WAL，单文件库放在 EDAI 数据目录。
//
// 注意：不能直接用 ent 的 `sql.Open`，它对 sqlite 的 dialect 名只认 "sqlite3"，
// 而本项目用的纯 Go 驱动注册名是 "sqlite"。这里经 database/sql 打开后
// 用 entsql.OpenDB 包一层，避免驱动名与 dialect 名混用。
func NewEntClient(cfg *conf.Data, logger log.Logger) (*gen.Client, error) {
	l := log.NewHelper(log.With(logger, "module", "edai/data/ent"))
	if cfg == nil || cfg.Database == nil {
		return nil, fmt.Errorf("database config is required")
	}

	driverName := strings.TrimSpace(cfg.Database.Driver)
	entDialect, err := entDialectOf(driverName)
	if err != nil {
		return nil, err
	}

	source := cfg.Database.Source
	if entDialect == "sqlite3" {
		// 确保库文件所在目录存在，否则驱动会报 "unable to open database file: out of memory (14)"
		// ——该错误信息具有误导性，实际是目录不存在。
		if dir := sqliteFilePath(source); dir != "" {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return nil, fmt.Errorf("creating sqlite directory %s: %w", dir, err)
			}
		}
	}

	db, err := sql.Open(driverName, source)
	if err != nil {
		return nil, fmt.Errorf("opening connection to %s: %w", driverName, err)
	} // SQLite 单文件库在 WAL 下仍只允许一个写者，连接池设小可减少锁竞争；
	// 读写并发由 WAL 与 busy_timeout 承担。
	maxIdle := intOr(cfg.Database.MaxIdleConnections, defaultMaxIdleConns)
	maxOpen := intOr(cfg.Database.MaxOpenConnections, defaultMaxOpenConns)
	if entDialect == "sqlite3" {
		if maxIdle > 4 {
			maxIdle = 4
		}
		if maxOpen > 8 {
			maxOpen = 8
		}
	}
	db.SetMaxIdleConns(maxIdle)
	db.SetMaxOpenConns(maxOpen)
	lifetime := defaultConnMaxLifetime
	if cfg.Database.ConnectionMaxLifetime != nil && cfg.Database.ConnectionMaxLifetime.AsDuration() > 0 {
		lifetime = cfg.Database.ConnectionMaxLifetime.AsDuration()
	}
	db.SetConnMaxLifetime(lifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging %s: %w", driverName, err)
	}

	drv := entsql.OpenDB(entDialect, db)

	client := gen.NewClient(
		gen.Driver(drv),
		gen.Log(func(a ...any) { l.Debug(a...) }),
	)
	if cfg.Database.Debug {
		client = client.Debug()
	}
	l.Infof("initialized ent client: driver=%s dialect=%s", driverName, entDialect)
	return client, nil
}

// entDialectOf 把驱动名映射为 ent 的 dialect 名。
func entDialectOf(driver string) (string, error) {
	switch driver {
	case "sqlite", "sqlite3":
		return "sqlite3", nil
	case "mysql":
		return "mysql", nil
	case "postgres", "postgresql":
		return "postgres", nil
	default:
		return "", fmt.Errorf("unsupported database driver: %s", driver)
	}
}

// RunSchemaMigration 执行结构同步。
//
// 注意：这里**不开启** WithDropColumn / WithDropIndex。EDAI 是全新产品，
// 但一旦进入试点就有真实数据，隐式删除列/索引会造成数据丢失（参考 4-2 ADR-014
// 中对既有服务该风险的记录）。删除列请显式写迁移脚本执行。
func RunSchemaMigration(ctx context.Context, cfg *conf.Data, logger log.Logger) error {
	l := log.NewHelper(log.With(logger, "module", "edai/data/ent/migrate"))
	client, err := NewEntClient(cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			l.Errorf("closing ent client: %v", cerr)
		}
	}()
	if err := client.Schema.Create(ctx); err != nil {
		return err
	}
	return seedAdminUser(ctx, client, l)
}

func intOr(v int32, def int) int {
	if v > 0 {
		return int(v)
	}
	return def
}

// sqliteFilePath 从 SQLite DSN 中提取库文件所在目录。
//
// 支持的形式（glebarez/go-sqlite）：
//
//	./data/edai.db?_pragma=...
//	file:./data/edai.db?_pragma=...
//	内存库（:memory: / file::memory:）返回空串，表示无需建目录。
func sqliteFilePath(dsn string) string {
	s := strings.TrimSpace(dsn)
	s = strings.TrimPrefix(s, "file:")
	if idx := strings.IndexAny(s, "?"); idx >= 0 {
		s = s[:idx]
	}
	if s == "" || s == ":memory:" {
		return ""
	}
	dir := filepath.Dir(s)
	if dir == "." || dir == "/" {
		return ""
	}
	return dir
}
