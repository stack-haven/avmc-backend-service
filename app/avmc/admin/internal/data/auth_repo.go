package data

import (
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent/user"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"
	"backend-service/pkg/utils/crypto"
	"context"
	"errors"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/avmc/admin/v1"
)

// AuthRepo 数据仓库结构体
// 包含日志记录器
type authRepo struct {
	data *Data
	log  *log.Helper
	atr  *authTokenRepo
}

// NewAuthRepo 创建新的用户数据仓库实例
// 参数：logger 日志记录器
// 返回值：用户数据仓库实例指针
func NewAuthRepo(data *Data, atr *authTokenRepo, logger log.Logger) biz.AuthRepo {
	return &authRepo{
		data: data,
		log:  log.NewHelper(logger),
		atr:  atr,
	}
}

// Login 处理后台登录数据操作
// 参数：ctx 上下文，name 用户名，password 密码
// 返回值：登录响应结构体，错误信息
func (r *authRepo) Login(ctx context.Context, name, password string) (*pb.LoginResponse, error) {
	// 这里实现具体的登录数据操作
	r.log.Infof("尝试登录数据操作，用户名：%s", name)
	res, err := r.data.DB(ctx).User.Query().Select(user.FieldPassword).Where(user.NameEQ(name)).Only(ctx)
	if err != nil {
		r.log.Errorf("登录数据操作失败，用户名：%s，错误：%v", name, err)
		return nil, err
	}
	if !crypto.CheckPasswordHash(password, res.Password) {
		r.log.Errorf("登录数据操作失败，用户名：%s，密码错误", name)
		return nil, biz.ErrPasswordIncorrect
	}
	accessToken, refreshToken, err := r.atr.GenerateToken(ctx, &pb.Auth{
		UserId:   res.ID,
		Username: name,
		// DomainId: ,
	})
	if err != nil {
		r.log.Errorf("登录数据操作失败，Token生成错误错误：%v", err)
		return nil, err
	}
	return &pb.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// RefreshToken 处理刷新令牌数据操作
// 参数：ctx 上下文，refreshToken 刷新令牌
// 返回值：刷新令牌响应结构体，错误信息
func (r *authRepo) RefreshToken(ctx context.Context, refreshToken string) (*pb.RefreshTokenResponse, error) {
	// 这里实现具体的刷新令牌数据操作
	r.log.Infof("尝试刷新令牌数据操作，刷新令牌：%s", refreshToken)
	// r.atr.GenerateRefreshToken(ctx, &pb.Auth{
	// 	Id:   "",
	// 	Name: "",
	// })
	return &pb.RefreshTokenResponse{
		AccessToken:  "",
		RefreshToken: "",
	}, nil
}

// Logout 处理后台登出数据操作
// 参数：ctx 上下文
// 返回值：错误信息
func (r *authRepo) Logout(ctx context.Context) error {
	// 这里实现具体的登出数据操作
	securityUser, success := authn.AuthUserFromContext(ctx)
	if !success {
		return errors.New("failed to parse user")
	}
	r.log.Infof("尝试登出数据操作，访问令牌：%s", securityUser.GetSubject())
	userId := convert.StringToUnit32(securityUser.GetSubject())
	return r.atr.RemoveToken(ctx, userId)
}

// Register 处理注册数据操作
// 参数：ctx 上下文，name 用户名，password 密码
// 返回值：错误信息
func (r *authRepo) Register(ctx context.Context, name, password string) error {
	// 这里实现具体的注册数据操作
	r.log.Infof("尝试注册数据操作，用户名：%s", name)
	_, err := r.data.DB(ctx).User.Create().SetName(name).SetPassword(password).Save(ctx)
	if err != nil {
		r.log.Errorf("注册数据操作失败，用户名：%s，错误：%v", name, err)
		return err
	}
	return nil
}
