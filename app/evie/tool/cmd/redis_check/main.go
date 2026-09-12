// Package main · redis_check
//
// 诊断工具：检查 Bearer Token 是否在 Redis（qua 共享缓存）中存在。
// 用法：
//
//	go run ./cmd/redis_check <token>
//	REDIS_ADDR=host:6379 REDIS_PASSWORD=xxx go run ./cmd/redis_check <token>
//
// 退出码：
//
//	0  token 存在
//	1  token 不存在 / 连接失败
//	2  参数错误
//
// 这是人肉排查 401 TOKEN_INVALID 的第一把钥匙。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	token := flag.String("token", "", "Bearer token to check (or pass as 1st positional arg)")
	addr := flag.String("addr", "", "Redis addr (default 172.16.0.112:6379; env REDIS_ADDR overrides)")
	db := flag.Int("db", 14, "Redis db index")
	password := flag.String("password", "", "Redis password (env REDIS_PASSWORD)")
	keyPrefix := flag.String("prefix", "oauth2_access_token:", "Key prefix")
	flag.Parse()

	if *token == "" && flag.NArg() > 0 {
		*token = flag.Arg(0)
	}
	if *token == "" {
		fmt.Fprintln(os.Stderr, "usage: redis_check <token>  OR  redis_check -token=<token>")
		os.Exit(2)
	}
	if *addr == "" {
		*addr = os.Getenv("REDIS_ADDR")
	}
	if *addr == "" {
		*addr = "172.16.0.112:6379"
	}
	if *password == "" {
		*password = os.Getenv("REDIS_PASSWORD")
	}

	rc := redis.NewClient(&redis.Options{
		Addr:         *addr,
		Password:     *password,
		DB:           *db,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	})
	defer rc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rc.Ping(ctx).Err(); err != nil {
		fmt.Fprintf(os.Stderr, "redis ping failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ redis connected: %s db=%d\n", *addr, *db)

	key := *keyPrefix + *token
	val, err := rc.Get(ctx, key).Result()
	if err == redis.Nil {
		fmt.Printf("✗ key NOT found: %s\n", key)
		// 列同前缀 key 供排查
		keys, err := rc.Keys(ctx, *keyPrefix+"*").Result()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  redis KEYS failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  total %s* keys = %d\n", *keyPrefix, len(keys))
		for i, k := range keys {
			if i >= 5 {
				fmt.Printf("  ... (and %d more)\n", len(keys)-5)
				break
			}
			fmt.Printf("  - %s\n", k)
		}
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "redis get failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ FOUND key=%s\n", key)
	preview := strings.TrimSpace(val)
	if len(preview) > 300 {
		preview = preview[:300] + "..."
	}
	fmt.Printf("  value: %s\n", preview)
}
