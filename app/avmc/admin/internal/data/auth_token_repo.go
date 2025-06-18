package data

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"

	authnEngine "backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"

	v1 "backend-service/api/avmc/admin/v1"
)

type authTokenRepo struct {
	rdb           *redis.Client
	log           *log.Helper
	authenticator authnEngine.Authenticator

	accessTokenKeyPrefix  string
	refreshTokenKeyPrefix string
}

// var _ authnEngine.TokenManager = (*authTokenRepo)(nil)

// // CreateToken implements authn.TokenManager.
// func (r *authTokenRepo) CreateToken(ctx context.Context, claims authnEngine.AuthClaims, expiration time.Duration) (string, error) {
// 	panic("unimplemented")
// }

// // RefreshToken implements authn.TokenManager.
// func (r *authTokenRepo) RefreshToken(ctx context.Context, token string) (string, error) {
// 	panic("unimplemented")
// }

// // RevokeToken implements authn.TokenManager.
// func (r *authTokenRepo) RevokeToken(ctx context.Context, token string) error {
// 	panic("unimplemented")
// }

// // ValidateToken implements authn.TokenManager.
// func (r *authTokenRepo) ValidateToken(ctx context.Context, token string) (*authnEngine.AuthClaims, error) {
// 	panic("unimplemented")
// }

func NewAuthTokenRepo(data *Data, authenticator authnEngine.Authenticator, logger log.Logger) *authTokenRepo {
	log := log.NewHelper(log.With(logger, "module", "auth-token/cache"))
	const (
		accessTokenKeyPrefix  = "admin_uat_"
		refreshTokenKeyPrefix = "admin_urt_"
	)
	return &authTokenRepo{
		log:                   log,
		rdb:                   data.rdb,
		authenticator:         authenticator,
		accessTokenKeyPrefix:  accessTokenKeyPrefix,
		refreshTokenKeyPrefix: refreshTokenKeyPrefix,
	}
}

func NewAuthToken(
	rdb *redis.Client,
	authenticator authnEngine.Authenticator,
	logger log.Logger,
	accessTokenKeyPrefix string,
	refreshTokenKeyPrefix string,
) *authTokenRepo {
	return &authTokenRepo{
		log:                   log.NewHelper(log.With(logger, "module", "auth-token/cache")),
		rdb:                   rdb,
		authenticator:         authenticator,
		accessTokenKeyPrefix:  accessTokenKeyPrefix,
		refreshTokenKeyPrefix: refreshTokenKeyPrefix,
	}
}

// createAccessJwtToken 生成JWT访问令牌
func (r *authTokenRepo) createAccessToken(_ string, userId uint32, domanId uint32) string {
	principal := authnEngine.AuthClaims{
		"sub":   convert.Unit32ToString(userId),
		"jti":   "",
		"dom":   convert.Unit32ToString(domanId),
		"scope": "",
	}

	signedToken, err := r.authenticator.CreateToken(context.Background(), principal)
	if err != nil {
		return ""
	}

	return signedToken
}

// createRefreshToken 生成刷新令牌
func (r *authTokenRepo) createRefreshToken(_ string, userId uint32, domanId uint32) string {
	// 刷新令牌信息中包含刷新过期时间
	authClaims := authnEngine.AuthClaims{
		"sub":         strconv.FormatUint(uint64(userId), 10),
		"dom":         convert.Unit32ToString(domanId),
		"refresh_exp": time.Now().Add(r.authenticator.Options().RefreshTokenExpiration),
	}
	token, err := r.authenticator.CreateToken(context.Background(), authClaims)
	if err != nil {
		return ""
	}
	return token
}

// GenerateToken 创建令牌
func (r *authTokenRepo) GenerateToken(ctx context.Context, auth *v1.Auth) (accessToken string, refreshToken string, err error) {
	if accessToken = r.createAccessToken(auth.GetUsername(), auth.GetUserId(), auth.DomainId); accessToken == "" {
		err = errors.New("create access token failed")
		return
	}
	if err = r.setAccessTokenToRedis(ctx, auth.GetUserId(), accessToken, r.authenticator.Options().TokenExpiration); err != nil {
		return
	}

	if refreshToken = r.createRefreshToken(auth.GetUsername(), auth.GetUserId(), 0); refreshToken == "" {
		err = errors.New("create refresh token failed")
		return
	}

	if err = r.setRefreshTokenToRedis(ctx, auth.GetUserId(), refreshToken, r.authenticator.Options().RefreshTokenExpiration); err != nil {
		return
	}

	return
}

// GenerateAccessToken 创建访问令牌
func (r *authTokenRepo) GenerateAccessToken(ctx context.Context, userId uint32, userName string) (accessToken string, err error) {
	if accessToken = r.createAccessToken(userName, userId, 0); accessToken == "" {
		err = errors.New("create access token failed")
		return
	}

	if err = r.setAccessTokenToRedis(ctx, userId, accessToken, 0); err != nil {
		return
	}

	return
}

// GenerateRefreshToken 创建刷新令牌
func (r *authTokenRepo) GenerateRefreshToken(ctx context.Context, auth *v1.Auth) (refreshToken string, err error) {
	if refreshToken = r.createRefreshToken(auth.GetUsername(), auth.GetUserId(), auth.GetDomainId()); refreshToken == "" {
		err = errors.New("create refresh token failed")
		return
	}

	if err = r.setRefreshTokenToRedis(ctx, auth.GetUserId(), refreshToken, 0); err != nil {
		return
	}

	return
}

// RemoveToken 移除所有令牌
func (r *authTokenRepo) RemoveToken(ctx context.Context, userId uint32) error {
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
func (r *authTokenRepo) GetAccessToken(ctx context.Context, userId uint32) string {
	return r.getAccessTokenFromRedis(ctx, userId)
}

// GetRefreshToken 获取刷新令牌
func (r *authTokenRepo) GetRefreshToken(ctx context.Context, userId uint32) string {
	return r.getRefreshTokenFromRedis(ctx, userId)
}

// IsExistAccessToken 访问令牌是否存在
func (r *authTokenRepo) IsExistAccessToken(ctx context.Context, userId uint32) bool {
	key := fmt.Sprintf("%s%d", r.accessTokenKeyPrefix, userId)
	n, err := r.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return n > 0
}

// IsExistRefreshToken 刷新令牌是否存在
func (r *authTokenRepo) IsExistRefreshToken(ctx context.Context, userId uint32) bool {
	key := fmt.Sprintf("%s%d", r.refreshTokenKeyPrefix, userId)
	n, err := r.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return n > 0
}

func (r *authTokenRepo) setAccessTokenToRedis(ctx context.Context, userId uint32, token string, expires time.Duration) error {
	key := fmt.Sprintf("%s%d", r.accessTokenKeyPrefix, userId)
	return r.rdb.Set(ctx, key, token, expires).Err()
}

func (r *authTokenRepo) getAccessTokenFromRedis(ctx context.Context, userId uint32) string {
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

func (r *authTokenRepo) deleteAccessTokenFromRedis(ctx context.Context, userId uint32) error {
	key := fmt.Sprintf("%s%d", r.accessTokenKeyPrefix, userId)
	return r.rdb.Del(ctx, key).Err()
}

func (r *authTokenRepo) setRefreshTokenToRedis(ctx context.Context, userId uint32, token string, expires time.Duration) error {
	key := fmt.Sprintf("%s%d", r.refreshTokenKeyPrefix, userId)
	return r.rdb.Set(ctx, key, token, expires).Err()
}

func (r *authTokenRepo) getRefreshTokenFromRedis(ctx context.Context, userId uint32) string {
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

func (r *authTokenRepo) deleteRefreshTokenFromRedis(ctx context.Context, userId uint32) error {
	key := fmt.Sprintf("%s%d", r.refreshTokenKeyPrefix, userId)
	return r.rdb.Del(ctx, key).Err()
}
