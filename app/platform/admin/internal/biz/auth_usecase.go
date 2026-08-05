package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pbCore "backend-service/api/core/service/v1"
	v1 "backend-service/api/platform/admin/v1"
	"backend-service/pkg/auth/authn"
)

// UserRepo is a Greater repo.
type AuthRepo interface {
	// Login 用户名密码登陆
	LoginByUsername(ctx context.Context, name, password string, tenantID uint32) (*v1.LoginResponse, error)
	// LoginByEmail 邮箱密码登陆
	LoginByEmail(ctx context.Context, email, password string, tenantID uint32) (*v1.LoginResponse, error)
	// Logout 登出
	Logout(context.Context, string) error
	// RefreshToken 刷新令牌
	RefreshToken(context.Context, string) (*v1.RefreshTokenResponse, error)
	// Register 注册用户
	Register(context.Context, string, string) error
	// Profile 获取用户简介信息
	Profile(context.Context, uint32) (*v1.ProfileResponse, error)
	// Codes 获取用户权限码
	Codes(context.Context, uint32) ([]string, error)
	// Menus 获取用户菜单
	Menus(context.Context, uint32) ([]*pbCore.Menu, error)
}

// AuthUsecase 业务用例结构体
// 包含日志记录器
type AuthUsecase struct {
	repo AuthRepo
	log  *log.Helper
}

// NewAuthUsecase 创建新的用户业务用例实例
// 参数：logger 日志记录器
// 返回值：用户业务用例实例指针
func NewAuthUsecase(logger log.Logger, repo AuthRepo) *AuthUsecase {
	return &AuthUsecase{
		log:  log.NewHelper(logger),
		repo: repo,
	}
}

// LoginByUsername 处理后台用户名登录业务逻辑。
//
// 注意：username/tenant_id 的格式校验（min_len、max_len、gt）由 proto 层
// buf.validate 完成，biz 层不应重复。本方法仅执行 proto 无法表达的**业务策略**
// 校验——密码字符类组合（大写+小写+数字+特殊字符）。
//
// 后续可扩展的 biz 层校验示例：
//  - 租户生命周期状态检查（是否 ACTIVE）
//  - 用户账号状态检查（是否被锁定/冻结）
//  - 登录地理位置异常检测
func (uc *AuthUsecase) LoginByUsername(ctx context.Context, name, password string, tenantID uint32) (*v1.LoginResponse, error) {
	uc.log.Infof("尝试用户名登录，tenant_id=%d", tenantID)
	// 业务策略：密码复杂度。proto 只校验长度，不校验字符类组合。
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	return uc.repo.LoginByUsername(ctx, name, password, tenantID)
}

// LoginByEmail 处理后台邮箱登录业务逻辑。
//
// email/tenant_id 的格式校验由 proto 层 buf.validate 完成（email: true、
// uint32.gt: 0），biz 层不重复。这里只做密码复杂度业务策略校验。
func (uc *AuthUsecase) LoginByEmail(ctx context.Context, email, password string, tenantID uint32) (*v1.LoginResponse, error) {
	uc.log.Infof("尝试邮箱登录，tenant_id=%d", tenantID)
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	return uc.repo.LoginByEmail(ctx, email, password, tenantID)
}

// RefreshToken 处理刷新令牌业务逻辑
// 参数：ctx 上下文，refreshToken 刷新令牌
// 返回值：刷新令牌响应结构体，错误信息
func (uc *AuthUsecase) RefreshToken(ctx context.Context, refreshToken string) (*v1.RefreshTokenResponse, error) {
	uc.log.Infof("尝试刷新令牌")
	return uc.repo.RefreshToken(ctx, refreshToken)
}

// Logout 处理后台登出业务逻辑
// 参数：ctx 上下文，accessToken 访问令牌
// 返回值：错误信息
func (uc *AuthUsecase) Logout(ctx context.Context) error {
	claims, ok := authn.AuthClaimsFromContext(ctx)
	if !ok || claims.GetID() == "" {
		return v1.ErrorAuthInvalidToken("当前会话无效")
	}
	uc.log.Infof("尝试登出")
	return uc.repo.Logout(ctx, claims.GetID())
}

// Register 处理注册业务逻辑
// 参数：ctx 上下文，name 用户名，password 密码
// 返回值：错误信息
func (uc *AuthUsecase) Register(ctx context.Context, name, password string) error {
	uc.log.Infof("尝试注册")
	return uc.repo.Register(ctx, name, password)
}

// Profile 处理登录用户简介信息业务逻辑
// 参数：ctx 上下文
// 返回值：登录用户简介信息响应结构体，错误信息
func (uc *AuthUsecase) Profile(ctx context.Context) (*v1.ProfileResponse, error) {
	// 这里实现具体的登录用户简介信息业务逻辑
	uc.log.Infof("尝试获取登录用户简介信息")
	userID := authn.GetAuthUserID(ctx)
	return uc.repo.Profile(ctx, userID)
}

// VbenProfile 处理登录用户简介信息业务逻辑
// 参数：ctx 上下文
// 返回值：登录用户简介信息响应结构体，错误信息
func (uc *AuthUsecase) VbenProfile(ctx context.Context) (*v1.VbenProfileResponse, error) {
	// 这里实现具体的登录用户简介信息业务逻辑
	uc.log.Infof("尝试获取登录用户简介信息")
	userID := authn.GetAuthUserID(ctx)
	profile, err := uc.repo.Profile(ctx, userID)
	if err != nil {
		uc.log.Errorf("获取登录用户简介信息失败: %v", err)
		return nil, err
	}
	return &v1.VbenProfileResponse{
		UserId:   profile.User.Id,
		Username: profile.User.Name,
		RealName: profile.User.Realname,
		Desc:     profile.User.Description,
		Avatar:   profile.User.Avatar,
		Role:     profile.Role,
		Roles:    profile.Roles,
	}, nil
}

// Codes 处理登录用户权限码业务逻辑
// 参数：ctx 上下文
// 返回值：登录用户权限码响应结构体，错误信息
func (uc *AuthUsecase) Codes(ctx context.Context) ([]string, error) {
	// 这里实现具体的登录用户权限码业务逻辑
	uc.log.Infof("尝试获取登录用户权限码")
	userID := authn.GetAuthUserID(ctx)
	return uc.repo.Codes(ctx, userID)
}

// Menus 处理登录用户菜单业务逻辑
// 参数：ctx 上下文
// 返回值：登录用户菜单响应结构体，错误信息
func (uc *AuthUsecase) Menus(ctx context.Context) ([]*pbCore.Menu, error) {
	// 这里实现具体的登录用户菜单业务逻辑
	uc.log.Infof("尝试获取登录用户菜单")
	userID := authn.GetAuthUserID(ctx)
	menus, err := uc.repo.Menus(ctx, userID)
	if err != nil {
		uc.log.Errorf("获取登录用户菜单失败: %v", err)
		return nil, err
	}
	uc.log.Infof("获取登录用户菜单成功, 数量: %d", len(menus))
	return menus, nil
}
