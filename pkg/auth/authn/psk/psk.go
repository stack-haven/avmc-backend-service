package psk

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"strconv"
	"strings"
	"time"

	"backend-service/pkg/auth/authn"
)

// PSKAuthenticator 实现基于预共享密钥的认证器
type PSKAuthenticator struct {
	options *authn.Options
	keys    map[string]string // 密钥ID到密钥值的映射
}

// PSKProvider 实现预共享密钥认证提供者
type PSKProvider struct {
	options *authn.Options
}

// PSKOptions 定义PSK特定的选项
type PSKOptions struct {
	// Keys 预共享密钥映射，键为密钥ID，值为密钥值
	Keys map[string]string
	// DefaultKeyID 默认密钥ID
	DefaultKeyID string
	// TokenFormat 令牌格式，支持"basic"和"hmac"
	TokenFormat string
	// HMACAlgorithm HMAC算法，支持"sha256"、"sha512"等
	HMACAlgorithm string
}

// PSKUser 实现基于PSK的安全用户
type PSKUser struct {
	id       string
	username string
	roles    []string
	claims   map[string]interface{}
}

// NewPSKProvider 创建新的PSK认证提供者
func NewPSKProvider(options *authn.Options) *PSKProvider {
	return &PSKProvider{options: options}
}

// CreateAuthenticator 创建PSK认证器
func (p *PSKProvider) CreateAuthenticator() (authn.Authenticator, error) {
	// 获取PSK特定选项
	pskOpts, ok := p.options.ProviderOptions.(*PSKOptions)
	if !ok || pskOpts == nil {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidOptions, "invalid or missing PSK options")
	}

	// 验证必要的选项
	if len(pskOpts.Keys) == 0 {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidOptions, "missing PSK keys")
	}

	// 创建认证器
	authenticator := &PSKAuthenticator{
		options: p.options,
		keys:    pskOpts.Keys,
	}

	return authenticator, nil
}

// Name 返回提供者名称
func (p *PSKProvider) Name() string {
	return "psk"
}

// Authenticate 从上下文中提取并验证令牌
func (a *PSKAuthenticator) Authenticate(ctx context.Context) (*authn.AuthClaims, error) {
	// 从上下文中提取令牌
	token, err := authn.TokenFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 验证令牌
	return a.ValidateToken(ctx, token)
}

// ValidateToken 验证PSK令牌并返回声明
func (a *PSKAuthenticator) ValidateToken(ctx context.Context, tokenString string) (*authn.AuthClaims, error) {
	// 获取PSK特定选项
	pskOpts, ok := a.options.ProviderOptions.(*PSKOptions)
	if !ok || pskOpts == nil {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidOptions, "invalid or missing PSK options")
	}

	// 根据令牌格式验证
	switch strings.ToLower(pskOpts.TokenFormat) {
	case "hmac":
		return a.validateHMACToken(ctx, tokenString)
	case "basic", "":
		return a.validateBasicToken(ctx, tokenString)
	default:
		return nil, authn.NewAuthError(authn.ErrCodeInvalidOptions, fmt.Sprintf("unsupported token format: %s", pskOpts.TokenFormat))
	}
}

// validateBasicToken 验证基本令牌格式
func (a *PSKAuthenticator) validateBasicToken(ctx context.Context, tokenString string) (*authn.AuthClaims, error) {
	// 基本格式：keyID:key 或 key
	parts := strings.SplitN(tokenString, ":", 2)

	var keyID, key string
	if len(parts) == 2 {
		keyID = parts[0]
		key = parts[1]
	} else {
		// 获取PSK特定选项
		pskOpts, ok := a.options.ProviderOptions.(*PSKOptions)
		if !ok || pskOpts == nil {
			return nil, authn.NewAuthError(authn.ErrCodeInvalidOptions, "invalid or missing PSK options")
		}

		// 使用默认密钥ID
		keyID = pskOpts.DefaultKeyID
		key = tokenString
	}

	// 验证密钥
	expectedKey, ok := a.keys[keyID]
	if !ok {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid key ID")
	}

	if key != expectedKey {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid key")
	}

	// 创建认证声明
	claims := &authn.StandardClaims{
		Subject:   keyID,
		Issuer:    a.options.Issuer,
		Audience:  a.options.Audience,
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(a.options.TokenExpiration),
		Claims: map[string]interface{}{
			"auth_type": "psk",
			"key_id":    keyID,
		},
	}

	return claims, nil
}

// validateHMACToken 验证HMAC令牌格式
func (a *PSKAuthenticator) validateHMACToken(ctx context.Context, tokenString string) (*authn.AuthClaims, error) {
	// HMAC格式：keyID.timestamp.signature
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid HMAC token format")
	}

	keyID := parts[0]
	timestampStr := parts[1]
	signature := parts[2]

	// 验证密钥ID
	key, ok := a.keys[keyID]
	if !ok {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid key ID")
	}

	// 解析时间戳
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid timestamp")
	}

	// 检查令牌是否过期
	tokenTime := time.Unix(timestamp, 0)
	if time.Now().After(tokenTime.Add(a.options.TokenExpiration)) {
		return nil, authn.NewAuthError(authn.ErrCodeExpiredToken, "token has expired")
	}

	// 获取PSK特定选项
	pskOpts, ok := a.options.ProviderOptions.(*PSKOptions)
	if !ok || pskOpts == nil {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidOptions, "invalid or missing PSK options")
	}

	// 验证签名
	expectedSignature, err := a.generateHMAC(keyID+"."+timestampStr, key, pskOpts.HMACAlgorithm)
	if err != nil {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidSignature, fmt.Sprintf("failed to generate HMAC: %v", err))
	}

	if signature != expectedSignature {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidSignature, "invalid signature")
	}

	// 创建认证声明
	claims := &authn.StandardClaims{
		Subject:   keyID,
		Issuer:    a.options.Issuer,
		Audience:  a.options.Audience,
		IssuedAt:  tokenTime,
		ExpiresAt: tokenTime.Add(a.options.TokenExpiration),
		Claims: map[string]interface{}{
			"auth_type": "psk",
			"key_id":    keyID,
			"timestamp": timestamp,
		},
	}

	return claims, nil
}

// CreateToken 创建新的PSK令牌
func (a *PSKAuthenticator) CreateToken(ctx context.Context, subject string, customClaims map[string]interface{}) (string, error) {
	// 获取PSK特定选项
	pskOpts, ok := a.options.ProviderOptions.(*PSKOptions)
	if !ok || pskOpts == nil {
		return "", authn.NewAuthError(authn.ErrCodeInvalidOptions, "invalid or missing PSK options")
	}

	// 验证密钥ID
	key, ok := a.keys[subject]
	if !ok {
		// 如果subject不是有效的密钥ID，使用默认密钥ID
		subject = pskOpts.DefaultKeyID
		key, ok = a.keys[subject]
		if !ok {
			return "", authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid key ID and no default key ID")
		}
	}

	// 根据令牌格式创建
	switch strings.ToLower(pskOpts.TokenFormat) {
	case "hmac":
		// 创建HMAC令牌
		timestamp := time.Now().Unix()
		timestampStr := fmt.Sprintf("%d", timestamp)
		signature, err := a.generateHMAC(subject+"."+timestampStr, key, pskOpts.HMACAlgorithm)
		if err != nil {
			return "", authn.NewAuthError(authn.ErrCodeTokenCreationFailed, fmt.Sprintf("failed to generate HMAC: %v", err))
		}

		return subject + "." + timestampStr + "." + signature, nil
	case "basic", "":
		// 创建基本令牌
		return subject + ":" + key, nil
	default:
		return "", authn.NewAuthError(authn.ErrCodeInvalidOptions, fmt.Sprintf("unsupported token format: %s", pskOpts.TokenFormat))
	}
}

// RefreshToken 刷新PSK令牌
func (a *PSKAuthenticator) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	// 对于PSK，刷新令牌实际上是创建一个新令牌
	// 首先验证当前令牌
	claims, err := a.ValidateToken(ctx, refreshToken)
	if err != nil {
		return "", err
	}

	// 使用相同的subject创建新令牌
	return a.CreateToken(ctx, claims.GetSubject(), nil)
}

// RevokeToken 撤销PSK令牌
func (a *PSKAuthenticator) RevokeToken(ctx context.Context, token string) error {
	// PSK认证不支持令牌撤销，因为令牌是基于静态密钥的
	return authn.NewAuthError(authn.ErrCodeUnsupportedOperation, "token revocation not supported by PSK authenticator")
}

// ParseUserFromContext 从上下文中解析用户信息
func (a *PSKAuthenticator) ParseUserFromContext(ctx context.Context) (authn.SecurityUser, error) {
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
	user := &PSKUser{
		id:       claims.GetSubject(),
		username: claims.GetSubject(),
		roles:    []string{"api_client"},
		claims:   claimsData,
	}

	return user, nil
}

// generateHMAC 生成HMAC签名
func (a *PSKAuthenticator) generateHMAC(data, key, algorithm string) (string, error) {
	var h func() hash.Hash

	// 选择HMAC算法
	switch strings.ToLower(algorithm) {
	case "sha256", "":
		h = sha256.New
	case "sha512":
		h = sha512.New
	default:
		return "", fmt.Errorf("unsupported HMAC algorithm: %s", algorithm)
	}

	// 计算HMAC
	mac := hmac.New(h, []byte(key))
	mac.Write([]byte(data))
	signature := mac.Sum(nil)

	// 编码为十六进制字符串
	return hex.EncodeToString(signature), nil
}

// GetID 获取用户ID
func (u *PSKUser) GetID() string {
	return u.id
}

// GetUsername 获取用户名
func (u *PSKUser) GetUsername() string {
	return u.username
}

// GetEmail 获取用户邮箱
func (u *PSKUser) GetEmail() string {
	return ""
}

// GetName 获取用户姓名
func (u *PSKUser) GetName() string {
	return u.username
}

// GetRoles 获取用户角色
func (u *PSKUser) GetRoles() []string {
	return u.roles
}

// GetClaims 获取用户声明
func (u *PSKUser) GetClaims() map[string]interface{} {
	return u.claims
}

// IsEnabled 检查用户是否启用
func (u *PSKUser) IsEnabled() bool {
	// 默认启用
	return true
}

// IsAccountNonExpired 检查用户账户是否未过期
func (u *PSKUser) IsAccountNonExpired() bool {
	// 默认未过期
	return true
}

// IsAccountNonLocked 检查用户账户是否未锁定
func (u *PSKUser) IsAccountNonLocked() bool {
	// 默认未锁定
	return true
}

// IsCredentialsNonExpired 检查用户凭证是否未过期
func (u *PSKUser) IsCredentialsNonExpired() bool {
	// 默认未过期
	return true
}
