package biz

import (
	"context"
	"fmt"
	"time"

	pbenum "backend-service/api/common/enum"
	pb "backend-service/api/edai/service/v1"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/crypto"

	"github.com/go-kratos/kratos/v2/log"
)

// UserRepo 本地账号仓库。
type UserRepo interface {
	GetByUsername(ctx context.Context, username string) (*pb.User, string, error) // 返回用户与密码哈希
	GetByID(ctx context.Context, id uint32) (*pb.User, error)
	UpdateLastLogin(ctx context.Context, id uint32, at time.Time) error
	Create(ctx context.Context, user *pb.User, passwordHash string) (*pb.User, error)
	Count(ctx context.Context) (int, error)
}

// Authenticator 抽象 pkg/auth 的 JWT 能力，便于测试替换。
type Authenticator interface {
	CreateToken(ctx context.Context, claims authn.AuthClaims, expiration time.Duration) (string, error)
	ValidateToken(ctx context.Context, token string) (*authn.AuthClaims, error)
	RefreshToken(ctx context.Context, token string) (string, error)
	RevokeToken(ctx context.Context, token string) error
}

// AuthUsecase 账号登录与令牌签发。
//
// 4-2 ADR-013 决策 4：EDAI 自持账号与 JWT，不复用 platform 的用户/会话/Casbin。
type AuthUsecase struct {
	repo   UserRepo
	auth   Authenticator
	access time.Duration
	log    *log.Helper
}

// NewAuthUsecase 创建认证 usecase。
func NewAuthUsecase(repo UserRepo, auth Authenticator, accessExpiration time.Duration, logger log.Logger) *AuthUsecase {
	if accessExpiration <= 0 {
		accessExpiration = 7 * 24 * time.Hour
	}
	return &AuthUsecase{
		repo:   repo,
		auth:   auth,
		access: accessExpiration,
		log:    log.NewHelper(log.With(logger, "module", "edai/biz/auth")),
	}
}

// Login 校验账号密码并签发令牌。
func (uc *AuthUsecase) Login(ctx context.Context, username, password string) (*pb.TokenPayload, *pb.User, error) {
	user, hash, err := uc.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, nil, err
	}
	// 账号不存在与密码错误返回同一错误，避免账号枚举
	if user == nil || hash == "" || !crypto.CheckPasswordHash(password, hash) {
		return nil, nil, pb.ErrorInvalidCredentials("账号或密码错误")
	}
	if user.GetStatus() == pbenum.Status_STATUS_DISABLED {
		return nil, nil, pb.ErrorAccountDisabled("账号已被禁用")
	}

	claims := authn.AuthClaims{
		"sub":      fmt.Sprintf("%d", user.GetId()),
		"username": user.GetUsername(),
		"role":     user.GetRole(),
		"tenant":   "1",
	}
	accessToken, err := uc.auth.CreateToken(ctx, claims, uc.access)
	if err != nil {
		return nil, nil, pb.ErrorInternalServerError("签发访问令牌失败")
	}
	refreshToken, err := uc.auth.CreateToken(ctx, claims, uc.access*10)
	if err != nil {
		return nil, nil, pb.ErrorInternalServerError("签发刷新令牌失败")
	}
	if err := uc.repo.UpdateLastLogin(ctx, user.GetId(), time.Now()); err != nil {
		// 登录成功但记录失败不阻断
		uc.log.Warnf("update last_login failed: %v", err)
	}
	return &pb.TokenPayload{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(uc.access.Seconds()),
	}, user, nil
}

// Refresh 用刷新令牌换取新的访问令牌。
func (uc *AuthUsecase) Refresh(ctx context.Context, refreshToken string) (*pb.TokenPayload, error) {
	claims, err := uc.auth.ValidateToken(ctx, refreshToken)
	if err != nil || claims == nil {
		return nil, pb.ErrorNotLoggedIn("刷新令牌无效或已过期")
	}
	accessToken, err := uc.auth.CreateToken(ctx, *claims, uc.access)
	if err != nil {
		return nil, pb.ErrorInternalServerError("签发访问令牌失败")
	}
	return &pb.TokenPayload{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(uc.access.Seconds()),
	}, nil
}

// Logout 撤销刷新令牌。
func (uc *AuthUsecase) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	if err := uc.auth.RevokeToken(ctx, refreshToken); err != nil {
		uc.log.Warnf("revoke token failed: %v", err)
	}
	return nil
}

// CurrentUser 返回上下文中的登录用户。
func (uc *AuthUsecase) CurrentUser(ctx context.Context) (*pb.User, error) {
	userID := authn.GetAuthUserID(ctx)
	if userID == 0 {
		return nil, pb.ErrorNotLoggedIn("未登录")
	}
	user, err := uc.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, pb.ErrorResourceNotFound("用户不存在")
	}
	return user, nil
}

// 角色 → 静态权限码。
//
// EDAI 没有平台菜单权限组（4-2 ADR-013），权限码仅用于前端 v-access 隐藏入口。
// 后端在写接口上按角色断言，不依赖这些码。
var roleAccessCodes = map[string][]string{
	"admin":    {"edai:source:read", "edai:source:write", "edai:source:delete", "edai:asset:read", "edai:task:manage"},
	"operator": {"edai:source:read", "edai:source:write", "edai:asset:read"},
	"viewer":   {"edai:source:read", "edai:asset:read"},
}

// AccessCodesFor 返回角色对应的权限码；未知角色返回只读。
func (uc *AuthUsecase) AccessCodesFor(role string) []string {
	if codes, ok := roleAccessCodes[role]; ok {
		return codes
	}
	return roleAccessCodes["viewer"]
}
