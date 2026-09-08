// Package data · token_cache.go
// TokenCache 薄包装：委托给 evie/tool/internal/credential/redis.Provider，保留旧 API 兼容。
//
// qua token 在 Redis 中的 key 形态（Q2）：
//
//	key:   oauth2_access_token:<token>
//	value: {"tenantId":"...","id":"...","accessToken":"...","userId":"...",
//	        "userType":2,"userInfo":{"nickname":"...","deptId":"..."},"expiresTime":1788491296083}
//
// 本文件保留：
//  1. AuthInfo：本工具视图（service 层 proto 反序列化用）
//  2. TokenCache：薄包装，构造时初始化 credential.Provider，Get 委托
//  3. ErrTokenNotFound / ErrTokenInvalid 错误类型（service 层引用）
//
// 新代码应直接用 credential + evie/tool/internal/credential/redis。
package data

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/redis/go-redis/v9"

	"backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/pkg/credential"
	credredis "backend-service/app/evie/tool/pkg/credential/redis"
	staticpkg "backend-service/app/evie/tool/pkg/credential/static"
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
//	rdb:    已连接 Redis（与 qua 共享 db）
//	prefix: Redis key 前缀，从 conf.Data.Redis.TokenKeyPrefix 注入
func NewTokenCache(rdb *redis.Client, redisConf *conf.Data_Redis) TokenLookup {
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

// Get 查询 AuthInfo（委托给 evie/tool/internal/credential/redis.Provider 后转回本工具视图）。
//
// 错误语义：
//
//	credential.ErrTokenNotFound → ErrTokenNotFound
//	credential.ErrTokenInvalid  → ErrTokenInvalid
//	其它 → 原 error
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

// ---- Demo / 离线模式 ----

// TokenLookup 是 HTTP/gRPC 中间件所需的最小能力集合。
//
// *TokenCache（Redis） 与 *StaticTokenCache（demo）均实现该接口，
// 使得 NewTokenAuthMiddleware 可以在两者之间透明切换。
type TokenLookup interface {
	Get(ctx context.Context, token string) (*AuthInfo, error)
	Key(token string) string
}

// StaticTokenCache 走 static Provider 的 TokenLookup（demo / 离线模式）。
type StaticTokenCache struct {
	provider *staticpkg.Provider
}

// NewStaticTokenCache 从 conf.StaticCredential 构造 TokenLookup。
func NewStaticTokenCache(cfg *conf.StaticCredential) (TokenLookup, error) {
	if cfg == nil {
		return nil, errors.New("data: static credential config is nil")
	}
	users := make([]staticpkg.User, 0, len(cfg.Users))
	for _, u := range cfg.Users {
		users = append(users, staticpkg.User{
			Token:    u.Token,
			TenantID: u.TenantId,
			UserID:   u.UserId,
			UserName: u.UserName,
			UserType: u.UserType,
		})
	}
	sp, err := staticpkg.New(staticpkg.Config{Users: users, DefaultTenant: cfg.DefaultTenant})
	if err != nil {
		return nil, err
	}
	return &StaticTokenCache{provider: sp}, nil
}

// Get 查表（demo 用）。
func (c *StaticTokenCache) Get(_ context.Context, token string) (*AuthInfo, error) {
	id, err := c.provider.Authenticate(context.Background(), token)
	if err != nil {
		return nil, err
	}
	if id == nil {
		return nil, ErrTokenNotFound
	}
	return &AuthInfo{
		TenantID:    id.TenantID,
		ID:          id.UserID,
		AccessToken: id.AccessToken,
		UserID:      id.UserID,
		UserType:    id.UserType,
		// demo 模式不展开嵌套字段；真实 qua 流程仍走 Redis。
	}, nil
}

// Key 不用于 demo（保留接口满足）。
func (c *StaticTokenCache) Key(token string) string { return "static:" + token }
