package data

import (
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent/user"
	"backend-service/pkg/utils/convert"
	"backend-service/pkg/utils/crypto"
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/avmc/admin/v1"
)

// AuthRepo 数据仓库结构体
// 包含日志记录器
type authRepo struct {
	data *Data
	log  *log.Helper
	atr  *authTokenRepo
	ur   *userRepo
}

// NewAuthRepo 创建新的用户数据仓库实例
// 参数：logger 日志记录器
// 返回值：用户数据仓库实例指针
func NewAuthRepo(data *Data, atr *authTokenRepo, logger log.Logger) biz.AuthRepo {
	return &authRepo{
		data: data,
		log:  log.NewHelper(logger),
		atr:  atr,
		ur:   NewUserRepo(data, logger).(*userRepo),
	}
}

// Login 处理后台登录数据操作
// 参数：ctx 上下文，name 用户名，password 密码
// 返回值：登录响应结构体，错误信息
func (r *authRepo) Login(ctx context.Context, name, password string, domainId uint32) (*pb.LoginResponse, error) {
	// 这里实现具体的登录数据操作
	r.log.Infof("尝试登录数据操作，��户名：%s", name)
	res, err := r.data.DB(ctx).User.Query().Select(user.FieldPassword, user.FieldName).Where(user.NameEQ(name), user.DomainIDEQ(domainId)).Only(ctx)
	if err != nil {
		r.log.Errorf("登录数据操作失败，用户名：%s，错误：%v", name, err)
		return nil, err
	}
	if !crypto.CheckPasswordHash(password, *res.Password) {
		r.log.Errorf("登录数据操作失败，用户名：%s，密码错误", name)
		return nil, biz.ErrPasswordIncorrect
	}
	accessToken, refreshToken, err := r.atr.GenerateToken(ctx, &pb.Auth{
		UserId:   res.ID,
		Username: name,
		DomainId: domainId,
	})
	if err != nil {
		r.log.Errorf("登录数据操作失败，Token生成错误错误：%v", err)
		return nil, err
	}
	// 拼装具体过期时间
	expires := convert.TimeValueToString(func(exp time.Duration) *time.Time {
		t := time.Now().Add(exp)
		return &t
	}(r.atr.authenticator.Options().TokenExpiration), time.RFC3339)
	return &pb.LoginResponse{
		Id:           res.ID,
		Name:         res.Name,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expires,
	}, nil
}

// RefreshToken 处理刷新令牌数据操作
// 参数：ctx 上下文，refreshToken 刷新令牌
// 返回值：刷新令牌响应结构体，错误信息
func (r *authRepo) RefreshToken(ctx context.Context, refreshToken string) (*pb.RefreshTokenResponse, error) {
	// 这里实现具体的刷新令牌数据操作
	r.log.Infof("尝试刷新令牌数据操作，刷新令牌：%s", refreshToken)
	// r.atr.IsExistRefreshToken(ctx, )
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
func (r *authRepo) Logout(ctx context.Context, userId uint32) error {
	// 这里实现具体的登出数据操作
	r.log.Infof("尝试登出数据操作，用户ID：%d", userId)
	return r.atr.RemoveToken(ctx, userId)
}

// Register 处理注册数据操作
// 参数：ctx 上下文，name 用户名，password 密码
// 返回值：错误信息
func (r *authRepo) Register(ctx context.Context, name, password string) error {
	// 这里实现具体的注册数据操作
	r.log.Infof("尝试注册数据操作，用户名：%s", name)
	hashPassword, _ := crypto.HashPassword(password)
	_, err := r.data.DB(ctx).User.Create().SetName(name).SetPassword(hashPassword).Save(ctx)
	if err != nil {
		r.log.Errorf("注册数据操作失败，用户名：%s，错误：%v", name, err)
		return err
	}
	return nil
}

// Profile 获取用户简介信息
// 参数：ctx 上下文，userId 用户ID
// 返回值：用户简介信息响应结构体，错误信息
func (r *authRepo) Profile(ctx context.Context, userId uint32) (*pb.ProfileResponse, error) {
	// 这里实现具体的获取用户简介信息数据操作
	r.log.Infof("尝试获取用户简介信息数据操作，用户ID：%d", userId)
	user, err := r.ur.FindByID(ctx, userId)
	if err != nil {
		r.log.Errorf("获取用户简介信息数据操作失败，用户ID：%d，错误：%v", userId, err)
		return nil, err
	}
	return &pb.ProfileResponse{
		User: user,
	}, nil
}

// Codes 获取用户权限码
// 参数：ctx 上下文，userId 用户ID
// 返回值：用户权限码响应结构体，错误信息
func (r *authRepo) Codes(ctx context.Context, userId uint32) ([]string, error) {
	// 这里实现具体的获取用户权限码数据操作
	r.log.Infof("尝试获取用户权限码数据操作，用户ID：%d", userId)
	return nil, nil
}
