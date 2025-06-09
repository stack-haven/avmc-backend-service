package data

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	authnEngine "backend-service/pkg/auth/authn"

	v1 "backend-service/api/avmc/admin/v1"
)

const (
	ClaimFieldIssuer         = "iss" // 代表 JWT 的签发者。它是一个字符串或者 URL，用于标识是哪个实体（如服务器、服务提供商等）签发了这个 JWT。
	ClaimFieldSubject        = "sub" // 代表 JWT 的主题。通常是一个唯一标识符，用于标识 JWT 所涉及的主体，这个主体通常是用户，但也可以是其他实体，如设备等。
	ClaimFieldAudience       = "aud" // 代表 JWT 的受众。它指定了 JWT 的接收方，是一个或多个字符串或者 URL。
	ClaimFieldExpirationTime = "exp" // 代表 JWT 的过期时间。它是一个数字，表示从 1970 年 1 月 1 日 00:00:00 UTC 开始到过期时间的秒数。
	ClaimFieldNotBefore      = "nbf" // 代表 JWT 的生效时间。和exp类似，它是一个数字，表示从 1970 年 1 月 1 日 00:00:00 UTC 开始到生效时间的秒数。
	ClaimFieldIssuedAt       = "iat" // 代表 JWT 的签发时间。也是一个数字，表示从 1970 年 1 月 1 日 00:00:00 UTC 开始到签发时间的秒数。
	ClaimFieldJwtID          = "jti" // 代表 JWT 的唯一标识符。是一个字符串，用于唯一标识一个 JWT。

	ClaimFieldScope = "scope" // 代表 JWT 的权限范围。它是一个字符串或者字符串数组，用于标识 JWT 的权限范围。在一个 API 访问场景中，scope的值可能是["read:users", "write:posts"]。这意味着拥有此 JWT 的用户被授权读取用户信息和写入文章相关内容。通过这种方式，scope清晰地界定了用户凭借该令牌可以进行的操作范围。
)

type authTokenRepo struct {
	rdb           *redis.Client
	log           *log.Helper
	authenticator authnEngine.Authenticator

	accessTokenKeyPrefix  string
	refreshTokenKeyPrefix string
}

func NewAuthTokenRepo(data *Data, authenticator authnEngine.Authenticator, logger log.Logger) *authTokenRepo {
	log.NewHelper(log.With(logger, "module", "auth-token/cache"))
	const (
		userAccessTokenKeyPrefix  = "admin_uat_"
		userRefreshTokenKeyPrefix = "admin_urt_"
	)
	return NewAuthToken(data.rdb, authenticator, logger, userAccessTokenKeyPrefix, userRefreshTokenKeyPrefix)
}

func NewAuthToken(
	rdb *redis.Client,
	authenticator authnEngine.Authenticator,
	logger log.Logger,
	accessTokenKeyPrefix string,
	refreshTokenKeyPrefix string,
) *authTokenRepo {
	return &authTokenRepo{
		log:                   log.NewHelper(log.With(logger, "module", "user-token/cache")),
		rdb:                   rdb,
		authenticator:         authenticator,
		accessTokenKeyPrefix:  accessTokenKeyPrefix,
		refreshTokenKeyPrefix: refreshTokenKeyPrefix,
	}
}

// createAccessJwtToken 生成JWT访问令牌
func (r *authTokenRepo) createAccessToken(_ string, userId uint32) string {
	principal := authnEngine.AuthClaims{
		ClaimFieldSubject: strconv.FormatUint(uint64(userId), 10),
		// authnEngine.ClaimFieldScope:   make(authnEngine.ScopeSet),
	}

	signedToken, err := r.authenticator.CreateToken(context.Background(), principal)
	if err != nil {
		return ""
	}

	return signedToken
}

// createRefreshToken 生成刷新令牌
func (r *authTokenRepo) createRefreshToken() string {
	return uuid.New().String()
}

// GenerateToken 创建令牌
func (r *authTokenRepo) GenerateToken(ctx context.Context, auth *v1.Auth) (accessToken string, refreshToken string, err error) {
	if accessToken = r.createAccessToken(auth.GetName(), auth.GetId()); accessToken == "" {
		err = errors.New("create access token failed")
		return
	}

	if err = r.setAccessTokenToRedis(ctx, auth.GetId(), accessToken, 0); err != nil {
		return
	}

	if refreshToken = r.createRefreshToken(); refreshToken == "" {
		err = errors.New("create refresh token failed")
		return
	}

	if err = r.setRefreshTokenToRedis(ctx, auth.GetId(), refreshToken, 0); err != nil {
		return
	}

	return
}

// GenerateAccessToken 创建访问令牌
func (r *authTokenRepo) GenerateAccessToken(ctx context.Context, userId uint32, userName string) (accessToken string, err error) {
	if accessToken = r.createAccessToken(userName, userId); accessToken == "" {
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
	if refreshToken = r.createRefreshToken(); refreshToken == "" {
		err = errors.New("create refresh token failed")
		return
	}

	if err = r.setRefreshTokenToRedis(ctx, auth.GetId(), refreshToken, 0); err != nil {
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

func (r *authTokenRepo) setAccessTokenToRedis(ctx context.Context, userId uint32, token string, expires int32) error {
	key := fmt.Sprintf("%s%d", r.accessTokenKeyPrefix, userId)
	return r.rdb.Set(ctx, key, token, time.Duration(expires)).Err()
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

func (r *authTokenRepo) setRefreshTokenToRedis(ctx context.Context, userId uint32, token string, expires int32) error {
	key := fmt.Sprintf("%s%d", r.refreshTokenKeyPrefix, userId)
	return r.rdb.Set(ctx, key, token, time.Duration(expires)).Err()
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
