package databases

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr          string
	Password      string
	DB            int
	DialTimeout   time.Duration
	WriteTimeout  time.Duration
	ReadTimeout   time.Duration
	EnableTracing bool
	EnableMetrics bool
}

// NewRedisClient create go-redis client
func NewRedisClient(cfg *RedisConfig, logger log.Logger) (rdb *redis.Client) {
	l := log.NewHelper(log.With(logger, "module", "redis/data/service"))
	if rdb = redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		WriteTimeout: cfg.WriteTimeout,
		ReadTimeout:  cfg.ReadTimeout,
	}); rdb == nil {
		l.Fatalf("failed opening connection to redis")
		return nil
	}

	// open tracing instrumentation.
	if cfg.EnableTracing {
		if err := redisotel.InstrumentTracing(rdb); err != nil {
			l.Fatalf("failed open tracing: %s", err.Error())
			panic(err)
		}
	}

	// open metrics instrumentation.
	if cfg.EnableMetrics {
		if err := redisotel.InstrumentMetrics(rdb); err != nil {
			l.Fatalf("failed open metrics: %s", err.Error())
			panic(err)
		}
	}

	return rdb
}
