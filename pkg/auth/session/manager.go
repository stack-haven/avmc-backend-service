package session

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/redis/go-redis/v9"

	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"
	iputil "backend-service/pkg/utils/ip"
)

type Token struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type Data struct {
	Key   string `json:"key,omitempty"`
	Value uint32 `json:"value,omitempty"`
}

type Info struct {
	Token            Token      `json:"token,omitempty"`
	Username         string     `json:"username,omitempty"`
	UserId           uint32     `json:"user_id,omitempty"`
	TenantID         uint32     `json:"tenant_id,omitempty"`
	PlatformOperator bool       `json:"platform_operator,omitempty"`
	Roles            []Data     `json:"roles,omitempty"`
	Permissions      []Data     `json:"permissions,omitempty"`
	TenantExpiresAt  *time.Time `json:"tenant_expires_at,omitempty"`
}

type Session struct {
	ID           string    `json:"id"`
	TenantID     uint32    `json:"tenant_id"`
	UserID       uint32    `json:"user_id"`
	Username     string    `json:"username"`
	IP           string    `json:"ip"`
	UserAgent    string    `json:"user_agent"`
	CreatedAt    time.Time `json:"created_at"`
	LastActiveAt time.Time `json:"last_active_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

var ErrSessionNotFound = errors.New("session not found")

func (i Info) TenantIdentifier() uint32 {
	return i.TenantID
}

// AuthTokenRepo 认证令牌仓库结构体
// 包含Redis客户端、日志记录器、认证器、访问令牌和刷新令牌的键前缀
type Manager struct {
	authn.Authenticator
	store                 Store
	log                   *log.Helper
	accessTokenKeyPrefix  string
	refreshTokenKeyPrefix string
	sessionKeyPrefix      string
}

type Store interface {
	Set(ctx context.Context, key string, value string, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	SetAdd(ctx context.Context, key string, values ...string) error
	SetRemove(ctx context.Context, key string, values ...string) error
	SetMembers(ctx context.Context, key string) ([]string, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
}

type RedisStore struct {
	client *redis.Client
}

func (s RedisStore) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	return s.client.Set(ctx, key, value, expiration).Err()
}

func (s RedisStore) Get(ctx context.Context, key string) (string, error) {
	return s.client.Get(ctx, key).Result()
}

func (s RedisStore) Del(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

func (s RedisStore) Exists(ctx context.Context, key string) (bool, error) {
	n, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s RedisStore) SetAdd(ctx context.Context, key string, values ...string) error {
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	return s.client.SAdd(ctx, key, args...).Err()
}

func (s RedisStore) SetRemove(ctx context.Context, key string, values ...string) error {
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	return s.client.SRem(ctx, key, args...).Err()
}

func (s RedisStore) SetMembers(ctx context.Context, key string) ([]string, error) {
	return s.client.SMembers(ctx, key).Result()
}

func (s RedisStore) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return s.client.Expire(ctx, key, expiration).Err()
}

// NewManager 创建认证令牌仓库实例
// 参数：data 数据访问层实例，authenticator 认证器实例，logger 日志记录器实例
// 返回：*Manager 认证令牌仓库实例
func NewManager(
	rdb *redis.Client,
	logger log.Logger,
	authenticator authn.Authenticator,
) *Manager {
	log := log.NewHelper(log.With(logger, "module", "auth-token/cache"))
	const (
		accessTokenKeyPrefix  = "admin_uat_"
		refreshTokenKeyPrefix = "admin_urt_"
		sessionKeyPrefix      = "admin_session_"
	)
	return &Manager{
		Authenticator:         authenticator,
		log:                   log,
		store:                 RedisStore{client: rdb},
		accessTokenKeyPrefix:  accessTokenKeyPrefix,
		refreshTokenKeyPrefix: refreshTokenKeyPrefix,
		sessionKeyPrefix:      sessionKeyPrefix,
	}
}

// NewManager 创建认证令牌仓库实例
// 参数：rdb Redis客户端实例，authenticator 认证器实例，logger 日志记录器实例，accessTokenKeyPrefix 访问令牌键前缀，refreshTokenKeyPrefix 刷新令牌键前缀
// 返回：*Manager 认证令牌仓库实例
func NewManagerPrefix(
	rdb *redis.Client,
	logger log.Logger,
	authenticator authn.Authenticator,
	accessTokenKeyPrefix string,
	refreshTokenKeyPrefix string,
) *Manager {
	return &Manager{
		Authenticator:         authenticator,
		log:                   log.NewHelper(log.With(logger, "module", "auth/token/redis")),
		store:                 RedisStore{client: rdb},
		accessTokenKeyPrefix:  accessTokenKeyPrefix,
		refreshTokenKeyPrefix: refreshTokenKeyPrefix,
		sessionKeyPrefix:      "admin_session_",
	}
}

func newManagerWithStore(
	store Store,
	logger log.Logger,
	authenticator authn.Authenticator,
	accessTokenKeyPrefix string,
	refreshTokenKeyPrefix string,
) *Manager {
	return &Manager{
		Authenticator:         authenticator,
		log:                   log.NewHelper(log.With(logger, "module", "auth/token/store")),
		store:                 store,
		accessTokenKeyPrefix:  accessTokenKeyPrefix,
		refreshTokenKeyPrefix: refreshTokenKeyPrefix,
		sessionKeyPrefix:      "session_",
	}
}

// Authenticate validates the request token and verifies it is still the active session in Redis.
func (r *Manager) Authenticate(ctx context.Context) (*authn.AuthClaims, error) {
	tokenString, err := authn.ParseContextToken(authn.HeaderAuthorize, r.Authenticator.Options().TokenHeadName)(ctx)
	if err != nil {
		return nil, err
	}
	return r.ValidateToken(ctx, tokenString)
}

// ValidateToken validates a JWT and checks that its session is still active.
func (r *Manager) ValidateToken(ctx context.Context, tokenString string) (*authn.AuthClaims, error) {
	claims, err := r.Authenticator.ValidateToken(ctx, tokenString)
	if err != nil {
		return nil, err
	}
	if convert.StringToUnit32(claims.GetSubject()) == 0 {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidSubject, "invalid subject", nil)
	}
	sessionID := claims.GetID()
	if sessionID == "" {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "missing session id", nil)
	}
	stored := r.getSessionAccessToken(ctx, sessionID)
	if stored == "" || subtle.ConstantTimeCompare([]byte(stored), []byte(tokenString)) != 1 {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "token has been revoked", nil)
	}
	return claims, nil
}

// ValidateRefreshToken validates a refresh token and checks that it matches the active refresh token in Redis.
func (r *Manager) ValidateRefreshToken(ctx context.Context, tokenString string) (*authn.AuthClaims, error) {
	claims, err := r.Authenticator.ValidateToken(ctx, tokenString)
	if err != nil {
		return nil, err
	}
	if convert.StringToUnit32(claims.GetSubject()) == 0 {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidSubject, "invalid subject", nil)
	}
	sessionID := claims.GetID()
	if sessionID == "" {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "missing session id", nil)
	}
	stored := r.getSessionRefreshToken(ctx, sessionID)
	if stored == "" || subtle.ConstantTimeCompare([]byte(stored), []byte(tokenString)) != 1 {
		return nil, authn.NewAuthError(authn.ErrCodeInvalidToken, "refresh token has been revoked", nil)
	}
	return claims, nil
}

// GenerateToken 创建令牌
func (r *Manager) GenerateToken(ctx context.Context, auth Info) (accessToken string, refreshToken string, err error) {
	sessionID := tokenID()
	if accessToken = r.createAccessToken(auth, sessionID); accessToken == "" {
		err = errors.New("create access token failed")
		return
	}
	if refreshToken = r.createRefreshToken(auth, sessionID); refreshToken == "" {
		err = errors.New("create refresh token failed")
		return
	}
	if err = r.saveSession(ctx, auth, sessionID, accessToken, refreshToken); err != nil {
		return
	}
	return
}

// RotateSessionToken rotates both tokens while retaining the current session.
func (r *Manager) RotateSessionToken(ctx context.Context, auth Info, sessionID string) (accessToken string, refreshToken string, err error) {
	if sessionID == "" {
		return "", "", errors.New("session id is required")
	}
	if _, err = r.GetSession(ctx, sessionID); err != nil {
		return "", "", err
	}
	if accessToken = r.createAccessToken(auth, sessionID); accessToken == "" {
		return "", "", errors.New("create access token failed")
	}
	if refreshToken = r.createRefreshToken(auth, sessionID); refreshToken == "" {
		return "", "", errors.New("create refresh token failed")
	}
	accessExpiration := effectiveTokenExpiration(r.Authenticator.Options().TokenExpiration, auth.TenantExpiresAt)
	refreshExpiration := effectiveTokenExpiration(r.Authenticator.Options().RefreshTokenExpiration, auth.TenantExpiresAt)
	if err = r.store.Set(ctx, r.sessionAccessKey(sessionID), accessToken, accessExpiration); err != nil {
		return "", "", err
	}
	if err = r.store.Set(ctx, r.sessionRefreshKey(sessionID), refreshToken, refreshExpiration); err != nil {
		return "", "", err
	}
	session, getErr := r.GetSession(ctx, sessionID)
	if getErr == nil {
		session.LastActiveAt = time.Now()
		session.ExpiresAt = time.Now().Add(refreshExpiration)
		err = r.saveSessionMetadata(ctx, session)
		if err == nil {
			expiration := refreshExpiration
			_ = r.store.Expire(ctx, r.userSessionsKey(session.UserID), expiration)
			_ = r.store.Expire(ctx, r.tenantSessionsKey(session.TenantID), expiration)
		}
	}
	return
}

// GenerateAccessToken creates a standalone session and returns its access token.
func (r *Manager) GenerateAccessToken(ctx context.Context, auth Info) (accessToken string, err error) {
	accessToken, _, err = r.GenerateToken(ctx, auth)
	return accessToken, err
}

// GenerateRefreshToken creates a standalone session and returns its refresh token.
func (r *Manager) GenerateRefreshToken(ctx context.Context, auth Info) (refreshToken string, err error) {
	_, refreshToken, err = r.GenerateToken(ctx, auth)
	return refreshToken, err
}

// RemoveToken revokes all active sessions for one user.
func (r *Manager) RemoveToken(ctx context.Context, userId uint32) error {
	return r.RevokeUserSessions(ctx, 0, userId)
}

// GetAccessToken 获取访问令牌
func (r *Manager) GetAccessToken(ctx context.Context, userId uint32) string {
	sessions, err := r.ListUserSessions(ctx, 0, userId)
	if err != nil || len(sessions) == 0 {
		return ""
	}
	return r.getSessionAccessToken(ctx, sessions[0].ID)
}

// GetRefreshToken 获取刷新令牌
func (r *Manager) GetRefreshToken(ctx context.Context, userId uint32) string {
	sessions, err := r.ListUserSessions(ctx, 0, userId)
	if err != nil || len(sessions) == 0 {
		return ""
	}
	return r.getSessionRefreshToken(ctx, sessions[0].ID)
}

// IsExistAccessToken 访问令牌是否存在
func (r *Manager) IsExistAccessToken(ctx context.Context, userId uint32) bool {
	sessions, err := r.ListUserSessions(ctx, 0, userId)
	return err == nil && len(sessions) > 0
}

// IsExistRefreshToken 刷新令牌是否存在
func (r *Manager) IsExistRefreshToken(ctx context.Context, userId uint32) bool {
	sessions, err := r.ListUserSessions(ctx, 0, userId)
	if err != nil {
		return false
	}
	for _, session := range sessions {
		if ok, _ := r.store.Exists(ctx, r.sessionRefreshKey(session.ID)); ok {
			return true
		}
	}
	return false
}

// createAccessJwtToken 生成JWT访问令牌
func (r *Manager) createAccessToken(auth Info, sessionID string) string {
	principal := authn.AuthClaims{
		"jti":               sessionID,
		"sub":               convert.Unit32ToString(auth.UserId),
		"tenant":            convert.Unit32ToString(auth.TenantIdentifier()),
		"platform_operator": auth.PlatformOperator,
		"scope":             "",
		"nonce":             tokenID(),
	}

	signedToken, err := r.Authenticator.CreateToken(context.Background(), principal, effectiveTokenExpiration(r.Authenticator.Options().TokenExpiration, auth.TenantExpiresAt))
	if err != nil {
		return ""
	}

	return signedToken
}

// createRefreshToken 生成刷新令牌
func (r *Manager) createRefreshToken(auth Info, sessionID string) string {
	expiration := effectiveTokenExpiration(r.Authenticator.Options().RefreshTokenExpiration, auth.TenantExpiresAt)
	// 刷新令牌信息中包含刷新过期时间
	authClaims := authn.AuthClaims{
		"jti":               sessionID,
		"sub":               strconv.FormatUint(uint64(auth.UserId), 10),
		"tenant":            convert.Unit32ToString(auth.TenantIdentifier()),
		"platform_operator": auth.PlatformOperator,
		"nonce":             tokenID(),
		"refresh_exp":       time.Now().Add(expiration),
	}
	token, err := r.Authenticator.CreateToken(context.Background(), authClaims, expiration)
	if err != nil {
		return ""
	}
	return token
}

func (r *Manager) saveSession(ctx context.Context, auth Info, sessionID, accessToken, refreshToken string) error {
	now := time.Now()
	accessExpiration := effectiveTokenExpiration(r.Authenticator.Options().TokenExpiration, auth.TenantExpiresAt)
	refreshExpiration := effectiveTokenExpiration(r.Authenticator.Options().RefreshTokenExpiration, auth.TenantExpiresAt)
	session := &Session{
		ID:           sessionID,
		TenantID:     auth.TenantIdentifier(),
		UserID:       auth.UserId,
		Username:     auth.Username,
		IP:           sessionIP(ctx),
		UserAgent:    sessionUserAgent(ctx),
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    now.Add(refreshExpiration),
	}
	if err := r.store.Set(ctx, r.sessionAccessKey(sessionID), accessToken, accessExpiration); err != nil {
		return err
	}
	if err := r.store.Set(ctx, r.sessionRefreshKey(sessionID), refreshToken, refreshExpiration); err != nil {
		_ = r.store.Del(ctx, r.sessionAccessKey(sessionID))
		return err
	}
	if err := r.saveSessionMetadata(ctx, session); err != nil {
		_ = r.store.Del(ctx, r.sessionAccessKey(sessionID))
		_ = r.store.Del(ctx, r.sessionRefreshKey(sessionID))
		return err
	}
	userKey := r.userSessionsKey(auth.UserId)
	tenantKey := r.tenantSessionsKey(auth.TenantIdentifier())
	if err := r.store.SetAdd(ctx, userKey, sessionID); err != nil {
		_ = r.RevokeSession(ctx, 0, sessionID)
		return err
	}
	if err := r.store.SetAdd(ctx, tenantKey, sessionID); err != nil {
		_ = r.RevokeSession(ctx, 0, sessionID)
		return err
	}
	expiration := refreshExpiration
	_ = r.store.Expire(ctx, userKey, expiration)
	_ = r.store.Expire(ctx, tenantKey, expiration)
	return nil
}

func effectiveTokenExpiration(defaultExpiration time.Duration, tenantExpiresAt *time.Time) time.Duration {
	if tenantExpiresAt == nil {
		return defaultExpiration
	}
	remaining := time.Until(*tenantExpiresAt)
	if remaining <= 0 {
		return time.Second
	}
	if remaining < defaultExpiration {
		return remaining
	}
	return defaultExpiration
}

func (r *Manager) saveSessionMetadata(ctx context.Context, session *Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	expiration := time.Until(session.ExpiresAt)
	if expiration <= 0 {
		expiration = time.Second
	}
	return r.store.Set(ctx, r.sessionMetadataKey(session.ID), string(data), expiration)
}

func (r *Manager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	raw, err := r.store.Get(ctx, r.sessionMetadataKey(sessionID))
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *Manager) ListUserSessions(ctx context.Context, tenantID, userID uint32) ([]*Session, error) {
	ids, err := r.store.SetMembers(ctx, r.userSessionsKey(userID))
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	return r.loadSessions(ctx, ids, tenantID)
}

func (r *Manager) ListTenantSessions(ctx context.Context, tenantID uint32) ([]*Session, error) {
	ids, err := r.store.SetMembers(ctx, r.tenantSessionsKey(tenantID))
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	return r.loadSessions(ctx, ids, tenantID)
}

func (r *Manager) loadSessions(ctx context.Context, ids []string, tenantID uint32) ([]*Session, error) {
	result := make([]*Session, 0, len(ids))
	for _, id := range ids {
		session, err := r.GetSession(ctx, id)
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if tenantID > 0 && session.TenantID != tenantID {
			continue
		}
		result = append(result, session)
	}
	return result, nil
}

func (r *Manager) RevokeSession(ctx context.Context, tenantID uint32, sessionID string) error {
	session, err := r.GetSession(ctx, sessionID)
	if errors.Is(err, redis.Nil) {
		return ErrSessionNotFound
	}
	if err != nil {
		return err
	}
	if tenantID > 0 && session.TenantID != tenantID {
		return ErrSessionNotFound
	}
	if err := r.store.Del(ctx, r.sessionAccessKey(sessionID)); err != nil {
		return err
	}
	if err := r.store.Del(ctx, r.sessionRefreshKey(sessionID)); err != nil {
		return err
	}
	if err := r.store.Del(ctx, r.sessionMetadataKey(sessionID)); err != nil {
		return err
	}
	_ = r.store.SetRemove(ctx, r.userSessionsKey(session.UserID), sessionID)
	_ = r.store.SetRemove(ctx, r.tenantSessionsKey(session.TenantID), sessionID)
	return nil
}

func (r *Manager) RevokeUserSessions(ctx context.Context, tenantID, userID uint32) error {
	sessions, err := r.ListUserSessions(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if err := r.RevokeSession(ctx, tenantID, session.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Manager) RevokeTenantSessions(ctx context.Context, tenantID uint32) error {
	sessions, err := r.ListTenantSessions(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if err := r.RevokeSession(ctx, tenantID, session.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Manager) getSessionAccessToken(ctx context.Context, sessionID string) string {
	return r.getStoredToken(ctx, r.sessionAccessKey(sessionID))
}

func (r *Manager) getSessionRefreshToken(ctx context.Context, sessionID string) string {
	return r.getStoredToken(ctx, r.sessionRefreshKey(sessionID))
}

func (r *Manager) getStoredToken(ctx context.Context, key string) string {
	result, err := r.store.Get(ctx, key)
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			r.log.Errorf("get session token failed: %s", err.Error())
		}
		return ""
	}
	return result
}

func (r *Manager) sessionAccessKey(sessionID string) string {
	return r.accessTokenKeyPrefix + "session:" + sessionID
}

func (r *Manager) sessionRefreshKey(sessionID string) string {
	return r.refreshTokenKeyPrefix + "session:" + sessionID
}

func (r *Manager) sessionMetadataKey(sessionID string) string {
	return r.sessionKeyPrefix + sessionID
}

func (r *Manager) userSessionsKey(userID uint32) string {
	return fmt.Sprintf("%suser:%d", r.sessionKeyPrefix, userID)
}

func (r *Manager) tenantSessionsKey(tenantID uint32) string {
	return fmt.Sprintf("%stenant:%d", r.sessionKeyPrefix, tenantID)
}

func sessionIP(ctx context.Context) string {
	if info, ok := transport.FromServerContext(ctx); ok {
		if carrier, ok := info.(interface{ Request() *http.Request }); ok {
			if request := carrier.Request(); request != nil {
				if ip, err := iputil.GetIP(request); err == nil {
					return ip
				}
			}
		}
	}
	return iputil.FormContext(ctx)
}

func sessionUserAgent(ctx context.Context) string {
	if info, ok := transport.FromServerContext(ctx); ok {
		return info.RequestHeader().Get("User-Agent")
	}
	return ""
}

func tokenID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(b[:])
}
