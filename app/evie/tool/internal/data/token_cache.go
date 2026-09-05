// Package data · token_cache.go
// TokenCache 薄包装：委托给 pkg/credential/redis.Provider，保留旧 API 兼容。
//
// qua token 在 Redis 中的 key 形态（Q2）：
//   key:   oauth2_access_token:<token>
//   value: {"tenantId":"...","id":"...","accessToken":"...","userId":"...",
//           "userType":2,"userInfo":{"nickname":"...","deptId":"..."},"expiresTime":1788491296083}
//
// 本文件保留：
//  1. AuthInfo：本工具视图（service 层 proto 反序列化用）
//  2. TokenCache：薄包装，构造时初始化 credential.Provider，Get 委托
//  3. ErrTokenNotFound / ErrTokenInvalid 错误类型（service 层引用）
//
// 新代码应直接用 pkg/credential + pkg/credential/redis。
package data

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"

	"backend-service/app/evie/tool/internal/conf"
	"backend-service/pkg/credential"
	credredis "backend-service/pkg/credential/redis"
)

// ErrTokenNotFound 表示 token 在 Redis 中不存在（已过期或被踢下线）。
var ErrTokenNotFound = credential.ErrTokenNotFound

// ErrTokenInvalid 表示 token value 反序列化失败。
var ErrTokenInvalid = credential.ErrTokenInvalid

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

	ClientID  string `json:"clientId"`
	Scopes    any    `json:"scopes"`      // 保留原样
	ExpiresAt int64  `json:"expiresTime"` // epoch ms
}

// TokenCache 提供 Bearer Token → AuthInfo 的查询能力（薄包装）。
type TokenCache struct {
	provider *credredis.Provider
	prefix   string
}

// NewTokenCache 创建 TokenCache（薄包装 credential/redis.Provider）。
//
//   rdb:    已连接 Redis（与 qua 共享 db）
//   prefix: Redis key 前缀，从 conf.Data.Redis.TokenKeyPrefix 注入
func NewTokenCache(rdb *redis.Client, redisConf *conf.Data_Redis) *TokenCache {
	prefix := redisConf.GetTokenKeyPrefix()
	if prefix == "" {
		prefix = "oauth2_access_token:"
	}
	provider, _ := credredis.New(credredis.Config{
		Client:    rdb,
		KeyPrefix: prefix,
		Fields: credredis.FieldMapper{
			TenantID:     "tenantId",
			UserID:       "userId",
			UserName:     "userInfo.nickname",
			DeptID:       "userInfo.deptId",
			UserType:     "userType",
			AccessToken:  "accessToken",
			RefreshToken: "refreshToken",
			Scopes:       "scopes",
			ExpiresAt:    "expiresTime",
		},
	})
	return &TokenCache{provider: provider, prefix: prefix}
}

// Key 拼接 Redis key（暴露给 test / 调试）。
func (t *TokenCache) Key(token string) string {
	return t.prefix + token
}

// Get 查询 AuthInfo（委托给 pkg/credential/redis.Provider 后转回本工具视图）。
//
// 错误语义：
//   credential.ErrTokenNotFound → ErrTokenNotFound
//   credential.ErrTokenInvalid  → ErrTokenInvalid
//   其它 → 原 error
func (c *TokenCache) Get(ctx context.Context, token string) (*AuthInfo, error) {
	id, err := c.provider.Authenticate(ctx, token)
	if err != nil {
		return nil, err
	}
	return identityToAuthInfo(id), nil
}

// identityToAuthInfo 把 credential.CallerIdentity 转为 data.AuthInfo。
//
// 保留原 AuthInfo JSON 形态供 service 层反序列化用。
func identityToAuthInfo(id *credential.CallerIdentity) *AuthInfo {
	if id == nil {
		return nil
	}
	info := &AuthInfo{
		TenantID:     id.TenantID,
		ID:           id.UserID,
		AccessToken:  id.AccessToken,
		RefreshToken: id.RefreshToken,
		UserID:       id.UserID,
		ExpiresAt:    id.ExpiresAt.UnixMilli(),
		Scopes:       id.Scopes,
	}
	info.UserType = id.UserType
	if m, ok := id.Raw.(map[string]any); ok {
		if ui, ok := m["userInfo"].(map[string]any); ok {
			info.UserInfo.Nickname = credLookupString(ui, "nickname")
			info.UserInfo.DeptID = credLookupString(ui, "deptId")
		}
		info.ClientID = credLookupString(m, "clientId")
	}
	if info.AccessToken == "" {
		info.AccessToken = id.AccessToken
	}
	return info
}

func credLookupString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		bs, _ := json.Marshal(int64(x))
		return string(bs)
	}
	return ""
}
