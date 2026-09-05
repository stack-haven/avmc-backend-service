// Package data · redis_client_helper_test.go
// 测试专用：构造一个直连指定地址的 *redis.Client。
package data

import (
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClientForTest 构造连接到 addr 的 redis.Client（导出供其他包 test 使用）。
func NewRedisClientForTest(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Network:      "tcp",
		Addr:         addr,
		Password:     "",
		DB:           0,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	})
}