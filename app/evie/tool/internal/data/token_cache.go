// Package data · token_cache.go
// TokenCache：Bearer Token → qua Redis 验证。
//
// qua token 在 Redis 中的 key 形态（Q2）：
//   key:   oauth2_access_token:<token>
//   value: {"tenantId":"...","id":"...","accessToken":"...","userId":"...",
//           "userType":2,"userInfo":{"nickname":"...","deptId":"..."},"expiresTime":1788491296083}
//
// 本文件定义：
//   1. AuthInfo：从 qua JSON 反序列化得到的本工具视图（不依赖 quag 内部结构）
//   2. TokenCache：Redis GET 抽象，区分 redis.Nil / JSON 错误 / 连接故障
//
// 编译阶段处于 M0，本文件仅保留类型与构造函数骨架；
// M2 阶段实现 Get 方法 + SecurityUser/ctxKey。
package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"backend-service/app/evie/tool/internal/conf"
)

// ErrTokenNotFound 表示 token 在 Redis 中不存在（已过期或被踢下线）。
var ErrTokenNotFound = errors.New("token not found in redis")

// ErrTokenInvalid 表示 token value 反序列化失败。
var ErrTokenInvalid = errors.New("token value is invalid json")

// AuthInfo 本工具视图的 token 解析结果。
//
// 字段名严格对齐 Q2 确认的 JSON 形态（Spring Cloud 风格 camelCase）。
// ID 类字段一律 string，避免 uint32 截断（qua ID 常 > 2^32）。
type AuthInfo struct {
	TenantID     string `json:"tenantId"`
	ID           string `json:"id"` // qua 内部的「access token 主键 id」
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	UserID       string `json:"userId"`
	UserType     int32  `json:"userType"`

	// 嵌套 userInfo
	UserInfo struct {
		Nickname string `json:"nickname"`
		DeptID   string `json:"deptId"`
	} `json:"userInfo"`

	ClientID   string  `json:"clientId"`
	Scopes     any     `json:"scopes"`     // 保留原样
	ExpiresAt  int64   `json:"expiresTime"` // epoch ms
}

// TokenCache 提供 Bearer Token → AuthInfo 的查询能力。
type TokenCache struct {
	rdb    *redis.Client
	prefix string // 默认 "oauth2_access_token:"
}

// NewTokenCache 创建 TokenCache。
//
//   rdb:    已连接 Redis（与 qua 共享 db）
//   prefix: Redis key 前缀，从 conf.Data.Redis.TokenKeyPrefix 注入
func NewTokenCache(rdb *redis.Client, redisConf *conf.Data_Redis) *TokenCache {
	prefix := redisConf.GetTokenKeyPrefix()
	if prefix == "" {
		prefix = "oauth2_access_token:"
	}
	return &TokenCache{rdb: rdb, prefix: prefix}
}

// Key 拼接 Redis key（暴露给 test / 调试）。
func (t *TokenCache) Key(token string) string {
	return t.prefix + token
}

// Get 查询 AuthInfo（M2 阶段实现完整逻辑）。
//
// 错误语义：
//   redis.Nil           → ErrTokenNotFound
//   json.Unmarshal 失败 → ErrTokenInvalid
//   其它                → 包装原 error
func (c *TokenCache) Get(ctx context.Context, token string) (*AuthInfo, error) {
	raw, err := c.rdb.Get(ctx, c.Key(token)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("redis GET: %w", err)
	}
	var info AuthInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}
	return &info, nil
}