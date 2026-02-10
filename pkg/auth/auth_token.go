package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"

	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"
)

type Token struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type AuthData struct {
	Key   string `json:"key,omitempty"`
	Value uint32 `json:"value,omitempty"`
}

type AuthTokenInfo struct {
	Token       Token      `json:"token,omitempty"`
	Username    string     `json:"username,omitempty"`
	UserId      uint32     `json:"user_id,omitempty"`
	DomainId    uint32     `json:"domain_id,omitempty"`
	Roles       []AuthData `json:"roles,omitempty"`
	Permissions []AuthData `json:"permissions,omitempty"`
}

// AuthTokenRepo 认证令牌仓库结构体
// 包含Redis客户端、日志记录器、认证器、访问令牌和刷新令牌的键前缀
type AuthToken struct {
	authn.Authenticator
	rdb                   *redis.Client
	log                   *log.Helper
	accessTokenKeyPrefix  string
	refreshTokenKeyPrefix string
}

// NewAuthToken 创建认证令牌仓库实例
// 参数：data 数据访问层实例，authenticator 认证器实例，logger 日志记录器实例
// 返回：*AuthToken 认证令牌仓库实例
func NewAuthToken(
	rdb *redis.Client,
	logger log.Logger,
	authenticator authn.Authenticator,
) *AuthToken {
	log := log.NewHelper(log.With(logger, "module", "auth-token/cache"))
	const (
		accessTokenKeyPrefix  = "admin_uat_"
		refreshTokenKeyPrefix = "admin_urt_"
	)
	return &AuthToken{
		Authenticator:         authenticator,
		log:                   log,
		rdb:                   rdb,
		accessTokenKeyPrefix:  accessTokenKeyPrefix,
		refreshTokenKeyPrefix: refreshTokenKeyPrefix,
	}
}

// NewAuthToken 创建认证令牌仓库实例
// 参数：rdb Redis客户端实例，authenticator 认证器实例，logger 日志记录器实例，accessTokenKeyPrefix 访问令牌键前缀，refreshTokenKeyPrefix 刷新令牌键前缀
// 返回：*AuthToken 认证令牌仓库实例
func NewAuthTokenPrefix(
	rdb *redis.Client,
	logger log.Logger,
	authenticator authn.Authenticator,
	accessTokenKeyPrefix string,
	refreshTokenKeyPrefix string,
) *AuthToken {
	return &AuthToken{
		Authenticator:         authenticator,
		log:                   log.NewHelper(log.With(logger, "module", "auth/token/redis")),
		rdb:                   rdb,
		accessTokenKeyPrefix:  accessTokenKeyPrefix,
		refreshTokenKeyPrefix: refreshTokenKeyPrefix,
	}
}

// GenerateToken 创建令牌
func (r *AuthToken) GenerateToken(ctx context.Context, auth AuthTokenInfo) (accessToken string, refreshToken string, err error) {
	if accessToken = r.createAccessToken(auth.Username, auth.UserId, auth.DomainId); accessToken == "" {
		err = errors.New("create access token failed")
		return
	}
	if err = r.setAccessTokenToRedis(ctx, auth.UserId, accessToken, r.Authenticator.Options().TokenExpiration); err != nil {
		return
	}

	if refreshToken = r.createRefreshToken(auth.Username, auth.UserId, auth.DomainId); refreshToken == "" {
		err = errors.New("create refresh token failed")
		return
	}

	if err = r.setRefreshTokenToRedis(ctx, auth.UserId, refreshToken, r.Authenticator.Options().RefreshTokenExpiration); err != nil {
		return
	}

	return
}

// GenerateAccessToken 创建访问令牌
func (r *AuthToken) GenerateAccessToken(ctx context.Context, auth AuthTokenInfo) (accessToken string, err error) {
	if accessToken = r.createAccessToken(auth.Username, auth.UserId, auth.DomainId); accessToken == "" {
		err = errors.New("create access token failed")
		return
	}

	if err = r.setAccessTokenToRedis(ctx, auth.UserId, accessToken, 0); err != nil {
		return
	}

	return
}

// GenerateRefreshToken 创建刷新令牌
func (r *AuthToken) GenerateRefreshToken(ctx context.Context, auth AuthTokenInfo) (refreshToken string, err error) {
	if refreshToken = r.createRefreshToken(auth.Username, auth.UserId, auth.DomainId); refreshToken == "" {
		err = errors.New("create refresh token failed")
		return
	}

	if err = r.setRefreshTokenToRedis(ctx, auth.UserId, refreshToken, 0); err != nil {
		return
	}

	return
}

// RemoveToken 移除所有令牌
func (r *AuthToken) RemoveToken(ctx context.Context, userId uint32) error {
	var err error
	if err = r.deleteAccessTokenFromRedis(ctx, userId); err != nil {
		r.log.Errorf("remove user access token failed: [%v]", err)
	}

	if err = r.deleteRefreshTokenFromRedis(ctx, userId); err != nil {
		r.log.Errorf("remove user refresh token failed: [%v]", err)
	}

	return err
}

// GetAccessToken 获取访问令牌
func (r *AuthToken) GetAccessToken(ctx context.Context, userId uint32) string {
	return r.getAccessTokenFromRedis(ctx, userId)
}

// GetRefreshToken 获取刷新令牌
func (r *AuthToken) GetRefreshToken(ctx context.Context, userId uint32) string {
	return r.getRefreshTokenFromRedis(ctx, userId)
}

// IsExistAccessToken 访问令牌是否存在
func (r *AuthToken) IsExistAccessToken(ctx context.Context, userId uint32) bool {
	key := fmt.Sprintf("%s%d", r.accessTokenKeyPrefix, userId)
	n, err := r.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return n > 0
}

// IsExistRefreshToken 刷新令牌是否存在
func (r *AuthToken) IsExistRefreshToken(ctx context.Context, userId uint32) bool {
	key := fmt.Sprintf("%s%d", r.refreshTokenKeyPrefix, userId)
	n, err := r.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return n > 0
}

// createAccessJwtToken 生成JWT访问令牌
func (r *AuthToken) createAccessToken(_ string, userId uint32, domanId uint32) string {
	principal := authn.AuthClaims{
		"jti":   "",
		"sub":   convert.Unit32ToString(userId),
		"dom":   convert.Unit32ToString(domanId),
		"scope": "",
	}

	signedToken, err := r.Authenticator.CreateToken(context.Background(), principal, r.Authenticator.Options().TokenExpiration)
	if err != nil {
		return ""
	}

	return signedToken
}

// createRefreshToken 生成刷新令牌
func (r *AuthToken) createRefreshToken(_ string, userId uint32, domanId uint32) string {
	// 刷新令牌信息中包含刷新过期时间
	authClaims := authn.AuthClaims{
		"sub":         strconv.FormatUint(uint64(userId), 10),
		"dom":         convert.Unit32ToString(domanId),
		"refresh_exp": time.Now().Add(r.Authenticator.Options().RefreshTokenExpiration),
	}
	token, err := r.Authenticator.CreateToken(context.Background(), authClaims, r.Authenticator.Options().RefreshTokenExpiration)
	if err != nil {
		return ""
	}
	return token
}

func (r *AuthToken) setAccessTokenToRedis(ctx context.Context, userId uint32, token string, expires time.Duration) error {
	key := fmt.Sprintf("%s%d", r.accessTokenKeyPrefix, userId)
	return r.rdb.Set(ctx, key, token, expires).Err()
}

func (r *AuthToken) getAccessTokenFromRedis(ctx context.Context, userId uint32) string {
	key := fmt.Sprintf("%s%d", r.accessTokenKeyPrefix, userId)
	result, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			r.log.Errorf("get redis user access token failed: %s", err.Error())
		}
		return ""
	}
	return result
}

func (r *AuthToken) deleteAccessTokenFromRedis(ctx context.Context, userId uint32) error {
	key := fmt.Sprintf("%s%d", r.accessTokenKeyPrefix, userId)
	return r.rdb.Del(ctx, key).Err()
}

func (r *AuthToken) setRefreshTokenToRedis(ctx context.Context, userId uint32, token string, expires time.Duration) error {
	key := fmt.Sprintf("%s%d", r.refreshTokenKeyPrefix, userId)
	return r.rdb.Set(ctx, key, token, expires).Err()
}

func (r *AuthToken) getRefreshTokenFromRedis(ctx context.Context, userId uint32) string {
	key := fmt.Sprintf("%s%d", r.refreshTokenKeyPrefix, userId)
	result, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			r.log.Errorf("get redis user refresh token failed: %s", err.Error())
		}
		return ""
	}
	return result
}

func (r *AuthToken) deleteRefreshTokenFromRedis(ctx context.Context, userId uint32) error {
	key := fmt.Sprintf("%s%d", r.refreshTokenKeyPrefix, userId)
	return r.rdb.Del(ctx, key).Err()
}
