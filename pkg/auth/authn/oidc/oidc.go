package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"backend-service/pkg/auth/authn"
)

var _ authn.Authenticator = (*OIDCAuthenticator)(nil)

// OIDCAuthenticator 实现基于OpenID Connect的认证器
type OIDCAuthenticator struct {
	options      *authn.Options
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
}

// OIDCProvider 实现OpenID Connect认证提供者
type OIDCProvider struct {
	options *authn.Options
}

// OIDCOptions 定义OIDC特定的选项
type OIDCOptions struct {
	// ProviderURL OIDC提供者URL
	ProviderURL string
	// ClientID OAuth2客户端ID
	ClientID string
	// ClientSecret OAuth2客户端密钥
	ClientSecret string
	// RedirectURL OAuth2重定向URL
	RedirectURL string
	// Scopes OAuth2请求的作用域
	Scopes []string
}

// NewOIDCProvider 创建新的OIDC认证提供者
func NewOIDCProvider(options *authn.Options) *OIDCProvider {
	return &OIDCProvider{options: options}
}

// CreateAuthenticator 创建OIDC认证器
func (p *OIDCProvider) CreateAuthenticator() (authn.Authenticator, error) {
	// 获取OIDC特定选项
	oidcOpts, ok := p.options.ProviderOptions.(*OIDCOptions)
	if !ok || oidcOpts == nil {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidOptions, "invalid or missing OIDC options")
	}

	// 验证必要的选项
	if oidcOpts.ProviderURL == "" {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidOptions, "missing OIDC provider URL")
	}
	if oidcOpts.ClientID == "" {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidOptions, "missing OIDC client ID")
	}

	// 创建OIDC提供者
	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, oidcOpts.ProviderURL)
	if err != nil {
		return nil, authn.NewAuthError(authn.ErrCodeInitializationFailed, fmt.Sprintf("failed to initialize OIDC provider: %v", err))
	}

	// 创建ID令牌验证器
	verifier := provider.Verifier(&oidc.Config{
		ClientID: oidcOpts.ClientID,
	})

	// 创建OAuth2配置
	scopes := oidcOpts.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	oauth2Config := &oauth2.Config{
		ClientID:     oidcOpts.ClientID,
		ClientSecret: oidcOpts.ClientSecret,
		RedirectURL:  oidcOpts.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	// 创建认证器
	authenticator := &OIDCAuthenticator{
		options:      p.options,
		provider:     provider,
		verifier:     verifier,
		oauth2Config: oauth2Config,
	}

	return authenticator, nil
}

// Name 返回提供者名称
func (p *OIDCProvider) Name() string {
	return "oidc"
}

// Authenticate 从上下文中提取并验证令牌
func (a *OIDCAuthenticator) Authenticate(ctx context.Context) (*authn.AuthClaims, error) {
	// 从上下文中提取令牌
	token, err := authn.TokenFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 验证令牌
	return a.ValidateToken(ctx, token)
}

// ValidateToken 验证ID令牌并返回声明
func (a *OIDCAuthenticator) ValidateToken(ctx context.Context, tokenString string) (*authn.AuthClaims, error) {
	// 验证ID令牌
	idToken, err := a.verifier.Verify(ctx, tokenString)
	if err != nil {
		if strings.Contains(err.Error(), "token is expired") {
			return nil, authn.NewAuthError(authn.ErrCodeExpiredToken, "token has expired")
		}
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, fmt.Sprintf("invalid token: %v", err))
	}

	// 解析声明
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidClaims, fmt.Sprintf("failed to parse claims: %v", err))
	}

	// 创建认证声明
	authClaims := &authn.StandardClaims{
		Subject:   idToken.Subject,
		Issuer:    idToken.Issuer,
		Audience:  idToken.Audience,
		IssuedAt:  time.Unix(idToken.IssuedAt, 0),
		ExpiresAt: time.Unix(idToken.Expiry.Unix(), 0),
		Claims:    claims,
	}

	return authClaims, nil
}

// CreateToken 创建新的访问令牌（不适用于OIDC，需要重定向到提供者）
func (a *OIDCAuthenticator) CreateToken(ctx context.Context, subject string, customClaims map[string]interface{}) (string, error) {
	return "", authn.NewAuthError(authn.ErrCodeUnsupportedOperation, "direct token creation not supported by OIDC, use authorization code flow instead")
}

// RefreshToken 刷新访问令牌
func (a *OIDCAuthenticator) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	// 使用刷新令牌获取新的访问令牌
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	// 刷新令牌
	source := a.oauth2Config.TokenSource(ctx, token)
	newToken, err := source.Token()
	if err != nil {
		return "", authn.NewAuthError(authn.ErrCodeRefreshFailed, fmt.Sprintf("failed to refresh token: %v", err))
	}

	return newToken.AccessToken, nil
}

// RevokeToken 撤销令牌（如果OIDC提供者支持）
func (a *OIDCAuthenticator) RevokeToken(ctx context.Context, token string) error {
	// 获取OIDC特定选项
	oidcOpts, ok := a.options.ProviderOptions.(*OIDCOptions)
	if !ok || oidcOpts == nil {
		return authn.NewAuthError(authn.ErrCodeInvalidOptions, "invalid or missing OIDC options")
	}

	// 尝试从提供者元数据中获取撤销端点
	var revocationEndpoint string
	if a.provider != nil {
		var providerMetadata map[string]interface{}
		if err := a.provider.Claims(&providerMetadata); err == nil {
			if endpoint, ok := providerMetadata["revocation_endpoint"].(string); ok {
				revocationEndpoint = endpoint
			}
		}
	}

	// 如果没有撤销端点，返回错误
	if revocationEndpoint == "" {
		return authn.NewAuthError(authn.ErrCodeUnsupportedOperation, "token revocation not supported by this OIDC provider")
	}

	// 构建撤销请求
	data := fmt.Sprintf("token=%s&client_id=%s&client_secret=%s", token, oidcOpts.ClientID, oidcOpts.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, "POST", revocationEndpoint, strings.NewReader(data))
	if err != nil {
		return authn.NewAuthError(authn.ErrCodeRevocationFailed, fmt.Sprintf("failed to create revocation request: %v", err))
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 发送撤销请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return authn.NewAuthError(authn.ErrCodeRevocationFailed, fmt.Sprintf("failed to send revocation request: %v", err))
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return authn.NewAuthError(authn.ErrCodeRevocationFailed, fmt.Sprintf("token revocation failed: %s - %s", resp.Status, string(body)))
	}

	return nil
}

// ParseUserFromContext 从上下文中解析用户信息
func (a *OIDCAuthenticator) ParseUserFromContext(ctx context.Context) (authn.SecurityUser, error) {
	// 从上下文中获取认证声明
	claims, ok := authn.AuthClaimsFromContext(ctx)
	if !ok || claims == nil {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid or missing auth claims")
	}

	// 获取声明数据
	claimsData, ok := claims.GetClaims()
	if !ok {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidClaims, "invalid or missing claims data")
	}

	// 创建用户信息
	user := &OIDCUser{
		id:       claims.GetSubject(),
		username: getStringClaim(claimsData, "preferred_username", claims.GetSubject()),
		email:    getStringClaim(claimsData, "email", ""),
		name:     getStringClaim(claimsData, "name", ""),
		roles:    getStringArrayClaim(claimsData, "roles"),
		claims:   claimsData,
	}

	return user, nil
}

// GetAuthorizationURL 获取OIDC授权URL
func (a *OIDCAuthenticator) GetAuthorizationURL(state string, options map[string]interface{}) string {
	// 创建OAuth2授权URL
	authURL := a.oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	// 添加额外选项
	if options != nil {
		extraParams := make([]string, 0, len(options))
		for k, v := range options {
			extraParams = append(extraParams, fmt.Sprintf("%s=%v", k, v))
		}

		if len(extraParams) > 0 {
			if strings.Contains(authURL, "?") {
				authURL += "&" + strings.Join(extraParams, "&")
			} else {
				authURL += "?" + strings.Join(extraParams, "&")
			}
		}
	}

	return authURL
}

// ExchangeCodeForToken 使用授权码交换令牌
func (a *OIDCAuthenticator) ExchangeCodeForToken(ctx context.Context, code string) (string, string, error) {
	// 使用授权码交换令牌
	token, err := a.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return "", "", authn.NewAuthError(authn.ErrCodeExchangeFailed, fmt.Sprintf("failed to exchange code for token: %v", err))
	}

	// 返回访问令牌和刷新令牌
	return token.AccessToken, token.RefreshToken, nil
}

// OIDCUser 实现基于OIDC的安全用户
type OIDCUser struct {
	id       string
	username string
	email    string
	name     string
	roles    []string
	claims   map[string]interface{}
}

// GetID 获取用户ID
func (u *OIDCUser) GetID() string {
	return u.id
}

// GetUsername 获取用户名
func (u *OIDCUser) GetUsername() string {
	return u.username
}

// GetEmail 获取用户邮箱
func (u *OIDCUser) GetEmail() string {
	return u.email
}

// GetName 获取用户姓名
func (u *OIDCUser) GetName() string {
	return u.name
}

// GetRoles 获取用户角色
func (u *OIDCUser) GetRoles() []string {
	return u.roles
}

// GetClaims 获取用户声明
func (u *OIDCUser) GetClaims() map[string]interface{} {
	return u.claims
}

// IsEnabled 检查用户是否启用
func (u *OIDCUser) IsEnabled() bool {
	// 默认启用
	return true
}

// IsAccountNonExpired 检查用户账户是否未过期
func (u *OIDCUser) IsAccountNonExpired() bool {
	// 默认未过期
	return true
}

// IsAccountNonLocked 检查用户账户是否未锁定
func (u *OIDCUser) IsAccountNonLocked() bool {
	// 默认未锁定
	return true
}

// IsCredentialsNonExpired 检查用户凭证是否未过期
func (u *OIDCUser) IsCredentialsNonExpired() bool {
	// 默认未过期
	return true
}

// 辅助函数：从声明中获取字符串值
func getStringClaim(claims map[string]interface{}, key string, defaultValue string) string {
	if value, ok := claims[key].(string); ok {
		return value
	}
	return defaultValue
}

// 辅助函数：从声明中获取字符串数组
func getStringArrayClaim(claims map[string]interface{}, key string) []string {
	var result []string

	value, ok := claims[key]
	if !ok {
		return result
	}

	// 尝试解析为字符串数组
	switch v := value.(type) {
	case []string:
		return v
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
	case string:
		// 尝试解析JSON字符串
		var arr []string
		if err := json.Unmarshal([]byte(v), &arr); err == nil {
			return arr
		}
		// 尝试解析逗号分隔的字符串
		parts := strings.Split(v, ",")
		for _, part := range parts {
			result = append(result, strings.TrimSpace(part))
		}
	}

	return result
}
