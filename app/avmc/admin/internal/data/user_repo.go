package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent"
	"backend-service/app/avmc/admin/internal/data/ent/user"
	"backend-service/pkg/utils/convert"
)

var _ biz.UserRepo = (*userRepo)(nil)

// userRepo 结构体
// 包含数据访问层实例和日志记录器
type userRepo struct {
	data *Data
	log  *log.Helper
}

// NewUserRepo 创建新的用户仓库实例
// 参数：data 数据访问层实例，logger 日志记录器
// 返回值：用户仓库实例指针
func NewUserRepo(data *Data, logger log.Logger) biz.UserRepo {
	return &userRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// toProto 转换ent.User为pbCore.User
func (r *userRepo) toProto(res *ent.User) *pbCore.User {
	return &pbCore.User{
		Id:          res.ID,
		Name:        &res.Name,
		Email:       &res.Email,
		Nickname:    &res.Nickname,
		Realname:    &res.Realname,
		Gender:      (*enum.Gender)(&res.Gender),
		Avatar:      &res.Avatar,
		Description: &res.Description,
		Phone:       &res.Phone,
		Status:      &res.Status,
		Birthday:    convert.TimeValueToString(&res.Birthday, time.DateOnly),
		CreatedAt:   convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt:   convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

// toEnt 转换pbCore.User为ent.User
func (r *userRepo) toEnt(g *pbCore.User) *ent.User {
	return &ent.User{
		ID:          g.GetId(),
		Name:        g.GetName(),
		Email:       g.GetEmail(),
		Nickname:    g.GetNickname(),
		Realname:    g.GetRealname(),
		Birthday:    *convert.StringValueToTime(g.Birthday, time.DateOnly),
		Gender:      int32(g.GetGender()),
		Phone:       g.GetPhone(),
		Avatar:      g.GetAvatar(),
		Status:      int32(g.GetStatus()),
		Description: g.GetDescription(),
	}
}

// Save 保存用户信息
// 参数：ctx 上下文，g 用户信息
// 返回值：用户信息，错误信息
func (r *userRepo) Save(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	return g, nil
}

// Update 更新用户信息
// 参数：ctx 上下文，g 用户信息
// 返回值：用户信息，错误信息
func (r *userRepo) Update(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	return g, nil
}

// FindByID 通过ID查询用户信息
// 参数：ctx 上下文，id 用户ID
// 返回值：用户信息，错误信息
func (r *userRepo) FindByID(ctx context.Context, id uint32) (*pbCore.User, error) {
	r.log.Infof("通过ID查询用户，ID：%d", id)
	res, err := r.data.DB(ctx).User.Query().Where(user.IDEQ(id)).Only(ctx)
	if err != nil {
		r.log.Errorf("通过ID查询用户失败，ID：%d，错误：%v", id, err)
		return nil, err
	}
	return r.toProto(res), nil
}

// ListByName 通过用户名查询用户列表
// 参数：ctx 上下文，name 用户名
// 返回值：用户列表，错误信息
func (r *userRepo) ListByName(ctx context.Context, name string) ([]*pbCore.User, error) {
	r.log.Infof("通过用户名查询用户，用户名：%s", name)
	res, err := r.data.DB(ctx).User.Query().Where(user.NameContains(name)).All(ctx)
	if err != nil {
		r.log.Errorf("通过用户名查询用户失败，用户名：%s，错误：%v", name, err)
		return nil, err
	}
	return convert.SliceToAny(res, r.toProto), nil
}

// ListByPhone 通过手机号查询用户列表
// 参数：ctx 上下文，phone 手机号
// 返回值：用户列表，错误信息
func (r *userRepo) ListByPhone(ctx context.Context, phone string) ([]*pbCore.User, error) {
	r.log.Infof("通过手机号查询用户，手机号：%s", phone)
	res, err := r.data.DB(ctx).User.Query().Where(user.PhoneEQ(phone)).All(ctx)
	if err != nil {
		r.log.Errorf("通过手机号查询用户失败，手机号：%s，错误：%v", phone, err)
		return nil, err
	}
	return convert.SliceToAny(res, r.toProto), nil
}

// ListAll 查询所有用户列表
// 参数：ctx 上下文
// 返回值：用户列表，错误信息
func (r *userRepo) ListAll(ctx context.Context) ([]*pbCore.User, error) {
	r.log.Infof("查询所有用户列表")
	res, err := r.data.DB(ctx).User.Query().All(ctx)
	if err != nil {
		r.log.Errorf("查询所有用户列表失败，错误：%v", err)
		return nil, err
	}
	return convert.SliceToAny(res, r.toProto), nil
}

// ListPage 查询用户列表分页
// 参数：ctx 上下文，pagination 分页请求
// 返回值：用户列表响应，错误信息
func (r *userRepo) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListUserResponse, error) {
	r.log.Infof("查询用户列表分页，分页请求：%v", pagination)
	res, err := r.data.DB(ctx).User.Query().
		Offset(int((pagination.GetPage() - 1) * pagination.GetPageSize())).
		Limit(int(pagination.GetPageSize())).
		All(ctx)
	if err != nil {
		r.log.Errorf("查询用户列表分页失败，分页请求：%v，错误：%v", pagination, err)
		return nil, err
	}
	return &pbCore.ListUserResponse{
		Items: convert.SliceToAny(res, r.toProto),
		Total: int32(len(res)),
	}, nil
}

// Delete 删除用户
// 参数：ctx 上下文，id 用户ID
// 返回值：错误信息
func (r *userRepo) Delete(ctx context.Context, id uint32) error {
	return nil
}
