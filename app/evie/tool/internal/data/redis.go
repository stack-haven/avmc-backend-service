// Package data · redis.go
// Redis 客户端装配（共用 qua 系统的 Redis 实例，用于查询 oauth2_access_token:<token>）。
package data

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"backend-service/app/evie/tool/internal/conf"
)

// NewRedisClient 根据 conf.Data.Redis 创建 Redis 客户端。
// M0 阶段做最小可用装配；后续 M2 TokenCache 复用此 client。
func NewRedisClient(c *conf.Data) (*redis.Client, error) {
	if c == nil || c.Redis == nil {
		return nil, fmt.Errorf("data.redis is required")
	}
	rc := c.Redis
	readTimeout := 200 * time.Millisecond
	writeTimeout := 200 * time.Millisecond
	if rc.ReadTimeout != nil {
		readTimeout = rc.ReadTimeout.AsDuration()
	}
	if rc.WriteTimeout != nil {
		writeTimeout = rc.WriteTimeout.AsDuration()
	}
	client := redis.NewClient(&redis.Options{
		Network:      rc.GetNetwork(),
		Addr:         rc.GetAddr(),
		Password:     rc.GetPassword(),
		DB:           int(rc.GetDb()),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})
	// 启动时 PING 一次，强制连接失败时快速报错
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return client, nil
}