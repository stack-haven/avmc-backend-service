package jwt

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"backend-service/pkg/auth/authn"
)

// JWTAuthenticator JWT认证器实现
type JWTAuthenticator struct {
	// options 配置选项
	options *authn.Options
	// signingMethod 签名方法
	signingMethod jwt.SigningMethod
	// signingKey 签名密钥
	signingKey interface{}
	// verificationKey 验证密钥
	verificationKey interface{}
	// parseTokenFunc 解析令牌函数
	parseTokenFunc authn.ParseContextTokenFunc
}

// JWTTokenInfo JWT令牌信息
type JWTTokenInfo struct {
	// AccessToken 访问令牌
	AccessToken string
	// RefreshToken 刷新令牌
	RefreshToken string
	// ExpiresAt 过期时间
	ExpiresAt time.Time
	// Claims 声明
	Claims map[string]interface{}
}

// GetAccessToken 获取访问令牌
func (t *JWTTokenInfo) GetAccessToken() string {
	return t.AccessToken
}

// GetRefreshToken 获取刷新令牌
func (t *JWTTokenInfo) GetRefreshToken() string {
	return t.RefreshToken
}

// GetExpiresAt 获取过期时间
func (t *JWTTokenInfo) GetExpiresAt() time.Time {
	return t.ExpiresAt
}

// GetClaims 获取令牌声明
func (t *JWTTokenInfo) GetClaims() map[string]interface{} {
	return t.Claims
}

// JWTProvider JWT认证提供者
type JWTProvider struct{}

// Name 获取提供者名称
func (p *JWTProvider) Name() string {
	return "jwt"
}

// NewAuthenticator 创建新的认证器实例
func (p *JWTProvider) NewAuthenticator(ctx context.Context, opts ...authn.Option) (authn.Authenticator, error) {
	// 创建JWT认证器
	auth := new(JWTAuthenticator)
	// 使用默认选项
	auth.options = authn.DefaultOptions()
	// 初始化认证器
	if err := auth.Init(ctx, opts); err != nil {
		return nil, err
	}
	return auth, nil
}

// Init 初始化认证器
func (a *JWTAuthenticator) Init(ctx context.Context, opts ...authn.Option) error {
	// 应用选项
	for _, opt := range opts {
		opt(a.options)
	}

	// 设置签名方法
	switch a.options.SigningMethod {
	case "HS256":
		a.signingMethod = jwt.SigningMethodHS256
	case "HS384":
		a.signingMethod = jwt.SigningMethodHS384
	case "HS512":
		a.signingMethod = jwt.SigningMethodHS512
	case "RS256":
		a.signingMethod = jwt.SigningMethodRS256
	case "RS384":
		a.signingMethod = jwt.SigningMethodRS384
	case "RS512":
		a.signingMethod = jwt.SigningMethodRS512
	case "ES256":
		a.signingMethod = jwt.SigningMethodES256
	case "ES384":
		a.signingMethod = jwt.SigningMethodES384
	case "ES512":
		a.signingMethod = jwt.SigningMethodES512
	default:
		return authn.NewAuthError(authn.ErrCodeInvalidConfiguration, "unsupported signing method", nil)
	}

	// 设置签名密钥
	if a.options.SigningKey == nil {
		return authn.NewAuthError(authn.ErrCodeInvalidConfiguration, "signing key is required", nil)
	}
	a.signingKey = a.options.SigningKey

	// 设置验证密钥
	if a.options.VerificationKey == nil {
		// 如果未设置验证密钥，使用签名密钥
		a.verificationKey = a.options.SigningKey
	} else {
		a.verificationKey = a.options.VerificationKey
	}

	// 设置令牌解析函数
	a.parseTokenFunc = authn.ParseContextToken(authn.HeaderAuthorize, a.options.TokenHeadName)

	return nil
}

// Authenticate 验证用户身份并返回认证声明
func (a *JWTAuthenticator) Authenticate(ctx context.Context) (*authn.AuthClaims, error) {
	// 从上下文中解析令牌
	tokenString, err := a.parseTokenFunc(ctx)
	if err != nil {
		return nil, err
	}

	// 验证令牌
	return a.ValidateToken(ctx, tokenString)
}

// ValidateToken 验证令牌的有效性
func (a *JWTAuthenticator) ValidateToken(ctx context.Context, tokenString string) (*authn.AuthClaims, error) {
	// 解析令牌
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if token.Method.Alg() != a.signingMethod.Alg() {
			return nil, authn.NewAuthError(
				authn.ErrCodeInvalidSignature,
				fmt.Sprintf("unexpected signing method: %v", token.Method.Alg()),
				nil,
			)
		}

		return a.verificationKey, nil
	})

	if token == nil {
		return nil, authn.ErrInvalidToken
	}

	// 处理解析错误
	if err != nil {
		// JWT v5 使用不同的错误处理方式
		switch {
		case errors.Is(err, jwt.ErrTokenMalformed):
			return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "token is malformed", err)
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, authn.NewAuthError(authn.ErrCodeExpiredToken, "token has expired", err)
		case errors.Is(err, jwt.ErrTokenNotValidYet):
			return nil, authn.NewAuthError(authn.ErrCodeNotBeforeTime, "token not valid yet", err)
		case errors.Is(err, jwt.ErrTokenSignatureInvalid):
			return nil, authn.NewAuthError(authn.ErrCodeInvalidSignature, "invalid token signature", err)
		default:
			return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "failed to parse token", err)
		}
		// return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "failed to parse token", err)
	}

	// 验证令牌有效性
	if !token.Valid {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid token", nil)
	}

	// 提取声明
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidClaims, "invalid token claims", nil)
	}

	// 验证签发者
	if a.options.Issuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != a.options.Issuer {
			return nil, authn.NewAuthError(authn.ErrCodeInvalidIssuer, "invalid issuer", nil)
		}
	}

	// 验证接收者
	if len(a.options.Audience) > 0 {
		audOk := false
		switch aud := claims["aud"].(type) {
		case string:
			for _, a := range a.options.Audience {
				if aud == a {
					audOk = true
					break
				}
			}
		case []string:
			for _, a := range a.options.Audience {
				for _, audItem := range aud {
					if audItem == a {
						audOk = true
						break
					}
				}
				if audOk {
					break
				}
			}
		case []interface{}:
			for _, a := range a.options.Audience {
				for _, audItem := range aud {
					if audStr, ok := audItem.(string); ok && audStr == a {
						audOk = true
						break
					}
				}
				if audOk {
					break
				}
			}
		}

		if !audOk {
			return nil, authn.NewAuthError(authn.ErrCodeInvalidAudience, "invalid audience", nil)
		}
	}

	// 转换为认证声明
	authClaims := make(authn.AuthClaims)
	for k, v := range claims {
		authClaims[k] = v
	}

	return &authClaims, nil
}

// CreateToken 创建新的身份令牌
func (a *JWTAuthenticator) CreateToken(ctx context.Context, claims authn.AuthClaims) (string, error) {
	// 创建JWT声明
	jwtClaims := jwt.MapClaims{}
	for k, v := range claims {
		jwtClaims[k] = v
	}

	// 设置标准声明
	now := time.Now()
	jwtClaims["iat"] = now.Unix()
	jwtClaims["exp"] = now.Add(a.options.TokenExpiration).Unix()

	if a.options.Issuer != "" {
		jwtClaims["iss"] = a.options.Issuer
	}

	if len(a.options.Audience) > 0 {
		jwtClaims["aud"] = a.options.Audience
	}

	// 创建令牌
	token := jwt.NewWithClaims(a.signingMethod, jwtClaims)

	// 签名令牌
	tokenString, err := token.SignedString(a.signingKey)
	if err != nil {
		return "", authn.NewAuthError(authn.ErrCodeTokenCreationFailed, "failed to sign token", err)
	}

	return tokenString, nil
}

// RefreshToken 刷新令牌，延长有效期
func (a *JWTAuthenticator) RefreshToken(ctx context.Context, token string) (string, error) {
	// 验证令牌
	claims, err := a.ValidateToken(ctx, token)
	if err != nil {
		// 如果是过期错误，我们可以继续刷新
		var authErr *authn.AuthError
		if errors.As(err, &authErr) && authErr.Code != authn.ErrCodeExpiredToken {
			return "", err
		}
	}

	// 创建新令牌
	return a.CreateToken(ctx, *claims)
}

// RevokeToken 撤销令牌，使其失效
func (a *JWTAuthenticator) RevokeToken(ctx context.Context, token string) error {
	// JWT本身不支持撤销，需要实现黑名单机制
	// 这里只是一个简单的实现，实际应用中应该使用Redis等存储黑名单
	if !a.options.EnableRevocation {
		return authn.NewAuthError(
			authn.ErrCodeTokenRevocationFailed,
			"token revocation is not enabled",
			nil,
		)
	}

	// 验证令牌
	claims, err := a.ValidateToken(ctx, token)
	if err != nil {
		return err
	}

	// 获取令牌ID
	jti := claims.GetID()
	if jti == "" {
		return authn.NewAuthError(
			authn.ErrCodeTokenRevocationFailed,
			"token does not have an ID",
			nil,
		)
	}

	// TODO: 将令牌ID添加到黑名单

	return nil
}

// Close 关闭认证器，释放资源
func (a *JWTAuthenticator) Close() error {
	// 无需特殊清理
	return nil
}

// Name 返回认证提供者的名称
func (a *JWTAuthenticator) Name() string {
	return "jwt"
}

// NewProvider 创建新的JWT认证提供者
func NewProvider() authn.AuthProvider {
	return &JWTProvider{}
}
