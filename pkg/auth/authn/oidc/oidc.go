package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"backend-service/pkg/auth/authn"
)

var _ authn.AuthProvider = (*OIDCProvider)(nil)

// OIDCProvider 实现OpenID Connect认证提供者
type OIDCProvider struct{}

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

// NewProvider 创建新的OIDC认证提供者
func NewProvider() authn.AuthProvider {
	return &OIDCProvider{}
}

// Name 返回提供者名称
func (p *OIDCProvider) Name() string {
	return "oidc"
}

// NewAuthenticator 创建新的认证器实例
func (p *OIDCProvider) NewAuthenticator(ctx context.Context, opts ...authn.Option) (authn.Authenticator, error) {
	// 创建OIDC认证器
	auth := new(OIDCAuthenticator)
	// 使用默认选项
	auth.options = authn.DefaultOptions()
	// 应用选项
	for _, opt := range opts {
		opt(&auth.options)
	}
	// 初始化认证器
	if err := auth.Init(ctx, opts...); err != nil {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidConfiguration, fmt.Sprintf("failed to initialize authenticator: %v", err), err)
	}
	return auth, nil
}

var _ authn.Authenticator = (*OIDCAuthenticator)(nil)

// OIDCAuthenticator 实现基于OpenID Connect的认证器
type OIDCAuthenticator struct {
	// options 配置选项
	options      authn.Options
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	// parseTokenFunc 解析令牌函数
	parseTokenFunc authn.ParseContextTokenFunc
}

// Init 初始化认证器
func (a *OIDCAuthenticator) Init(ctx context.Context, _ ...authn.Option) error {
	// 注意：选项已在NewAuthenticator中应用，此处不再处理

	// 获取OIDC特定选项
	providerOptions := a.options.ProviderOptions

	// 解析OIDC选项
	oidcOpts := &OIDCOptions{}
	if providerURL, ok := providerOptions["provider_url"].(string); ok {
		oidcOpts.ProviderURL = providerURL
	}
	if clientID, ok := providerOptions["client_id"].(string); ok {
		oidcOpts.ClientID = clientID
	}
	if clientSecret, ok := providerOptions["client_secret"].(string); ok {
		oidcOpts.ClientSecret = clientSecret
	}
	if redirectURL, ok := providerOptions["redirect_url"].(string); ok {
		oidcOpts.RedirectURL = redirectURL
	}
	if scopes, ok := providerOptions["scopes"].([]string); ok {
		oidcOpts.Scopes = scopes
	}

	// 验证必要的选项
	if oidcOpts.ProviderURL == "" {
		return authn.NewAuthError(authn.ErrCodeInvalidConfiguration, "missing OIDC provider URL", nil)
	}
	if oidcOpts.ClientID == "" {
		return authn.NewAuthError(authn.ErrCodeInvalidConfiguration, "missing OIDC client ID", nil)
	}

	// 创建OIDC提供者
	discoveryCtx := oidc.InsecureIssuerURLContext(ctx, oidcOpts.ProviderURL)
	provider, err := oidc.NewProvider(discoveryCtx, oidcOpts.ProviderURL)
	if err != nil {
		return authn.NewAuthError(authn.ErrCodeInvalidConfiguration, fmt.Sprintf("failed to initialize OIDC provider: %v", err), err)
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

	// 设置令牌解析函数
	a.parseTokenFunc = authn.ParseContextToken(authn.HeaderAuthorize, a.options.TokenHeadName)

	// 保存初始化结果
	a.provider = provider
	a.verifier = verifier
	a.oauth2Config = oauth2Config

	return nil
}

// Name 返回认证器名称
func (a *OIDCAuthenticator) Name() string {
	return "oidc"
}

// Options 返回认证器选项
func (a *OIDCAuthenticator) Options() authn.Options {
	return a.options
}

// Close 关闭认证器，释放资源
func (a *OIDCAuthenticator) Close() error {
	// 无需特殊清理
	return nil
}

// Authenticate 验证用户身份并返回认证声明
func (a *OIDCAuthenticator) Authenticate(ctx context.Context) (*authn.AuthClaims, error) {
	// 从上下文中解析令牌
	tokenString, err := a.parseTokenFunc(ctx)
	if err != nil {
		return nil, err
	}

	// 验证令牌
	return a.ValidateToken(ctx, tokenString)
}

// ValidateToken 验证ID令牌并返回声明
func (a *OIDCAuthenticator) ValidateToken(ctx context.Context, tokenString string) (*authn.AuthClaims, error) {
	// 验证ID令牌
	idToken, err := a.verifier.Verify(ctx, tokenString)
	if err != nil {
		if strings.Contains(err.Error(), "token is expired") {
			return nil, authn.NewAuthError(authn.ErrCodeExpiredToken, "token has expired", err)
		}
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, fmt.Sprintf("invalid token: %v", err), err)
	}

	// 解析声明
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, fmt.Sprintf("failed to parse claims: %v", err), err)
	}

	// 验证签发者
	if a.options.Issuer != "" {
		if idToken.Issuer != a.options.Issuer {
			return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid issuer", nil)
		}
	}

	// 验证接收者
	if len(a.options.Audience) > 0 {
		audOk := false
		for _, aud := range a.options.Audience {
			for _, tokenAud := range idToken.Audience {
				if aud == tokenAud {
					audOk = true
					break
				}
			}
			if audOk {
				break
			}
		}

		if !audOk {
			return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid audience", nil)
		}
	}

	// 创建认证声明
	authClaims := make(authn.AuthClaims)
	authClaims["sub"] = idToken.Subject
	authClaims["iss"] = idToken.Issuer
	authClaims["aud"] = idToken.Audience
	authClaims["iat"] = idToken.IssuedAt
	authClaims["exp"] = idToken.Expiry.Unix()

	// 添加其他声明
	for k, v := range claims {
		authClaims[k] = v
	}

	return &authClaims, nil
}

// CreateToken 创建新的访问令牌（不适用于OIDC，需要重定向到提供者）
func (a *OIDCAuthenticator) CreateToken(ctx context.Context, claims authn.AuthClaims) (string, error) {
	return "", authn.NewAuthError(authn.ErrCodeUnsupportedTokenType, "direct token creation not supported by OIDC, use authorization code flow instead", nil)
}

// RefreshToken 刷新访问令牌
func (a *OIDCAuthenticator) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	tokenSource := a.oauth2Config.TokenSource(ctx, &oauth2.Token{
		RefreshToken: refreshToken,
	})

	newToken, err := tokenSource.Token()
	if err != nil {
		return "", authn.NewAuthError(authn.ErrCodeExpiredToken, fmt.Sprintf("failed to refresh token: %v", err), err)
	}

	return newToken.AccessToken, nil
}

// RevokeToken 撤销令牌（如果OIDC提供者支持）
func (a *OIDCAuthenticator) RevokeToken(ctx context.Context, token string) error {
	// 检查是否启用了撤销功能
	if !a.options.EnableRevocation {
		return authn.NewAuthError(
			authn.ErrCodeInvalidConfiguration,
			"token revocation is not enabled",
			nil,
		)
	}

	// 获取OIDC特定选项
	providerOptions := a.options.ProviderOptions
	oidcOpts := &OIDCOptions{}
	if clientSecret, ok := providerOptions["client_secret"].(string); ok {
		oidcOpts.ClientSecret = clientSecret
	}

	if clientID, ok := providerOptions["client_id"].(string); ok {
		oidcOpts.ClientID = clientID
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
		return authn.NewAuthError(authn.ErrCodeInvalidConfiguration, "token revocation not supported by this OIDC provider", nil)
	}

	// 构建撤销请求
	data := fmt.Sprintf("token=%s&client_id=%s&client_secret=%s", token, oidcOpts.ClientID, oidcOpts.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, "POST", revocationEndpoint, strings.NewReader(data))
	if err != nil {
		return authn.NewAuthError(authn.ErrCodeInvalidConfiguration, fmt.Sprintf("failed to create revocation request: %v", err), err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 发送撤销请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return authn.NewAuthError(authn.ErrCodeInvalidConfiguration, fmt.Sprintf("failed to send revocation request: %v", err), err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return authn.NewAuthError(authn.ErrCodeInvalidConfiguration, fmt.Sprintf("token revocation failed: %s - %s", resp.Status, string(body)), nil)
	}

	return nil
}

// ParseUserFromContext 从上下文中解析用户信息
func (a *OIDCAuthenticator) ParseUserFromContext(ctx context.Context) (authn.SecurityUser, error) {
	// 从上下文中获取认证声明
	claims, ok := authn.AuthClaimsFromContext(ctx)
	if !ok || claims == nil {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid or missing auth claims", nil)
	}

	// 创建用户信息
	user := &OIDCUser{
		id:       claims.GetSubject(),
		username: getStringClaim(*claims, "preferred_username", claims.GetSubject()),
		email:    getStringClaim(*claims, "email", ""),
		name:     getStringClaim(*claims, "name", ""),
		roles:    getStringArrayClaim(*claims, "roles"),
		claims:   *claims,
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
		return "", "", authn.NewAuthError(authn.ErrCodeInvalidConfiguration, fmt.Sprintf("failed to exchange code for token: %v", err), nil)
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
	claims   authn.AuthClaims
}

// GetAction 获取用户动作
func (u *OIDCUser) GetAction() string {
	return ""
}

// GetID 获取用户ID
func (u *OIDCUser) GetID() string {
	return u.id
}

// GetSubject 获取主体标识（通常是用户ID）
func (u *OIDCUser) GetSubject() string {
	return u.id
}

// Name 获取Security Name
func (u *OIDCUser) Name() string {
	return "oidc_user"
}

// ParseFromContext 从上下文中解析用户信息
func (u *OIDCUser) ParseFromContext(ctx context.Context) error {
	// 从上下文中获取认证声明
	claims, ok := authn.AuthClaimsFromContext(ctx)
	if !ok || claims == nil {
		return authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid or missing auth claims", nil)
	}

	// 更新用户信息
	u.id = claims.GetSubject()
	u.username = getStringClaim(*claims, "preferred_username", u.id)
	u.email = getStringClaim(*claims, "email", "")
	u.name = getStringClaim(*claims, "name", "")
	u.roles = getStringArrayClaim(*claims, "roles")
	u.claims = *claims

	return nil
}

// GetUsername 获取用户名
func (u *OIDCUser) GetUsername() string {
	return u.username
}

// GetEmail 获取用户邮箱
func (u *OIDCUser) GetEmail() string {
	return u.email
}

// GetDomain 获取用户域
func (u *OIDCUser) GetDomain() string {
	// 从claims中获取domain，如果不存在则返回空字符串
	return getStringClaim(u.claims, "domain", "")
}

// GetObject 获取对象标识
func (u *OIDCUser) GetObject() string {
	// 从claims中获取object，如果不存在则返回空字符串
	return getStringClaim(u.claims, "object", "")
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
func getStringClaim(claims authn.AuthClaims, key string, defaultValue string) string {
	if claims == nil {
		return defaultValue
	}
	if value, ok := claims[key].(string); ok {
		return value
	}
	return defaultValue
}

// 辅助函数：从声明中获取字符串数组
func getStringArrayClaim(claims authn.AuthClaims, key string) []string {
	if claims == nil {
		return []string{}
	}
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
