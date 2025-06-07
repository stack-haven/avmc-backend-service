package databases

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"

	// init mysql driver
	"entgo.io/ent/dialect/sql"
	_ "github.com/go-sql-driver/mysql"
)

// EntClientCreator 定义创建ent客户端的接口
type EntClient interface {
	Close() error
}
type EntCall func(*sql.Driver) error

type EntConfig struct {
	Driver                string
	Source                string
	Migrate               bool
	Debug                 bool
	MaxIdleConnections    int32
	MaxOpenConnections    int32
	ConnectionMaxLifetime time.Duration
}

// NewEntClient .
func NewEntClient[T EntClient](cfg *EntConfig, l *log.Helper) *T {
	drv, err := sql.Open(cfg.Driver, cfg.Source)
	if err != nil {
		l.Fatalf("failed opening connection to %s: %v", cfg.Driver, err)
		return nil
	}

	{
		db := drv.DB()
		// 连接池中最多保留的空闲连接数量
		db.SetMaxIdleConns(int(cfg.MaxIdleConnections))
		// 连接池在同一时间打开连接的最大数量
		db.SetMaxOpenConns(int(cfg.MaxOpenConnections))
		// 连接可重用的最大时间长度
		db.SetConnMaxLifetime(cfg.ConnectionMaxLifetime)
	}

	return nil
}
