package biz

import (
	v1 "backend-service/api/avmc/admin/v1"
	"context"
	"errors"

	"github.com/go-kratos/kratos/v2/log"
)

// 预定义错误
var (
	// ErrUnknown 未知错误
	ErrUnknown           = errors.New("unknown authentication error")
	ErrPasswordIncorrect = errors.New("auth failed: incorrect password")
)

// UserRepo is a Greater repo.
type AuthRepo interface {
	Login(ctx context.Context, name, password string, domainID uint32) (*v1.LoginResponse, error)
	Logout(context.Context) error
	RefreshToken(context.Context, string) (*v1.RefreshTokenResponse, error)
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

// Login 处理后台登录业务逻辑
// 参数：ctx 上下文，name 用户名，password 密码
// 返回值：登录响应结构体，错误信息
func (uc *AuthUsecase) Login(ctx context.Context, name, password string, domainID uint32) (*v1.LoginResponse, error) {
	// 这里实现具体的登录业务逻辑
	uc.log.Infof("尝试登录，用户名：%s", name)
	return uc.repo.Login(ctx, name, password, domainID)
}

// RefreshToken 处理刷新令牌业务逻辑
// 参数：ctx 上下文，refreshToken 刷新令牌
// 返回值：刷新令牌响应结构体，错误信息
func (uc *AuthUsecase) RefreshToken(ctx context.Context, refreshToken string) (*v1.RefreshTokenResponse, error) {
	// 这里实现具体的刷新令牌业务逻辑
	uc.log.Infof("尝试刷新令牌，刷新令牌：%s", refreshToken)
	return uc.repo.RefreshToken(ctx, refreshToken)
}

// Logout 处理后台登出业务逻辑
// 参数：ctx 上下文，accessToken 访问令牌
// 返回值：错误信息
func (uc *AuthUsecase) Logout(ctx context.Context) error {
	// 这里实现具体的登出业务逻辑
	uc.log.Infof("尝试登出")
	return uc.repo.Logout(ctx)
}

// Register 处理注册业务逻辑
// 参数：ctx 上下文，name 用户名，password 密码
// 返回值：错误信息
func (uc *AuthUsecase) Register(ctx context.Context, name, password string) error {
	// 这里实现具体的注册业务逻辑
	uc.log.Infof("尝试注册，用户名：%s", name)
	return nil
}
