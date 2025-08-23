package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/user"
	"backend-service/pkg/utils/convert"
	"backend-service/pkg/utils/crypto"
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

// toProto 转换gen.User为pbCore.User
func (r *userRepo) toProto(res *gen.User) *pbCore.User {
	return &pbCore.User{
		Id:          res.ID,
		Name:        res.Name,
		Email:       res.Email,
		Nickname:    res.Nickname,
		Realname:    res.Realname,
		Gender:      (*enum.Gender)(res.Gender),
		Avatar:      res.Avatar,
		Description: res.Description,
		Phone:       res.Phone,
		Status:      (*enum.Status)(res.Status),
		Birthday:    convert.TimeValueToString(res.Birthday, time.DateOnly),
		CreatedAt:   convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt:   convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

// toEnt 转换pbCore.User为gen.User
func (r *userRepo) toEnt(g *pbCore.User) *gen.User {
	return &gen.User{
		ID:          g.GetId(),
		Name:        g.Name,
		Email:       g.Email,
		Nickname:    g.Nickname,
		Realname:    g.Realname,
		Birthday:    convert.StringValueToTime(g.Birthday, time.DateOnly),
		Phone:       g.Phone,
		Avatar:      g.Avatar,
		Description: g.Description,
		Password:    g.Password,
		Gender:      (*int32)(g.Gender),
		Status:      (*int32)(g.Status),
	}
}

// ExistByEmail 获取用户邮箱是否存在
// 参数：ctx 上下文，email 用户邮箱
// 返回值：用户ID，错误信息
func (r *userRepo) ExistByEmail(ctx context.Context, email string) (uint32, error) {
	r.log.Infof("获取用户邮箱是否存在，用户email：%v", email)
	entUser, err := r.data.DB(ctx).User.Query().Where(user.Email(email)).Select(user.FieldID).First(ctx)
	if err != nil {
		r.log.Errorf("获取用户邮箱是否存在失败，用户email：%v，错误：%v", email, err)
		return 0, err
	}
	return entUser.ID, nil
}

// ExistByName 获取用户名是否存在
// 参数：ctx 上下文，name 用户名
// 返回值：用户ID，错误信息
func (r *userRepo) ExistByName(ctx context.Context, name string) (uint32, error) {
	r.log.Infof("获取用户名是否存在，用户名：%v", name)
	entUser, err := r.data.DB(ctx).User.Query().Where(user.Name(name)).Select(user.FieldID).First(ctx)
	if err != nil {
		r.log.Errorf("获取用户名是否存在失败，用户名：%v，错误：%v", name, err)
		return 0, err
	}
	return entUser.ID, nil
}

// ExistByPhone 获取用户手机号是否存在
// 参数：ctx 上下文，phone 手机号
// 返回值：用户ID，错误信息
func (r *userRepo) ExistByPhone(ctx context.Context, phone string) (uint32, error) {
	r.log.Infof("获取用户手机号是否存在，手机号：%v", phone)
	entUser, err := r.data.DB(ctx).User.Query().Where(user.Phone(phone)).Select(user.FieldID).First(ctx)
	if err != nil {
		r.log.Errorf("获取用户手机号是否存在失败，手机号：%v，错误：%v", phone, err)
		return 0, err
	}
	return entUser.ID, nil
}

// Save 保存用户信息
// 参数：ctx 上下文，g 用户信息
// 返回值：用户信息，错误信息
func (r *userRepo) Save(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	r.log.Infof("保存用户，用户信息：%v", g)
	entUser := r.toEnt(g)
	builder := r.data.DB(ctx).User.Create()

	id, _ := r.ExistByName(ctx, *entUser.Name)
	if id > 0 {
		r.log.Errorf("用户名已存在，用户信息：%v", g)
		return nil, fmt.Errorf("user name already exists")
	}
	if entUser.Email != nil {
		id, _ = r.ExistByEmail(ctx, *entUser.Email)
		if id > 0 {
			r.log.Errorf("用户名已存在，用户信息：%v", g)
			return nil, fmt.Errorf("user email already exists")
		}
		builder = builder.SetNillableEmail(entUser.Email)
	}
	if entUser.Phone != nil {
		id, _ = r.ExistByPhone(ctx, *entUser.Phone)
		if id > 0 {
			r.log.Errorf("手机号已存在，用户信息：%v", g)
			return nil, fmt.Errorf("user phone already exists")
		}
		builder = builder.SetNillablePhone(entUser.Phone)
	}
	if g.Password != nil {
		hashPassword, _ := crypto.HashPassword(*entUser.Password)
		builder = builder.SetPassword(hashPassword)
	}
	res, err := builder.SetName(*entUser.Name).
		SetNillableEmail(entUser.Email).
		SetNillableNickname(entUser.Nickname).
		SetNillableRealname(entUser.Realname).
		SetNillableBirthday(entUser.Birthday).
		SetNillableGender(entUser.Gender).
		SetNillableAvatar(entUser.Avatar).
		SetNillableStatus(entUser.Status).
		SetNillableDescription(entUser.Description).
		Save(ctx)
	if err != nil {
		r.log.Errorf("保存用户失败，用户信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.toProto(res), nil
}

// Update 更新用户信息
// 参数：ctx 上下文，g 用户信息
// 返回值：用户信息，错误信息
func (r *userRepo) Update(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	r.log.Infof("更新用户，用户信息：%v", g)
	entUser := r.toEnt(g)
	builder := r.data.DB(ctx).User.UpdateOneID(g.GetId())
	if g.Name != nil {
		id, _ := r.ExistByName(ctx, *entUser.Name)
		if id > 0 && id != g.GetId() {
			r.log.Errorf("用户名已存在，用户信息：%v", g)
			return nil, fmt.Errorf("user name already exists")
		}
		builder = builder.SetName(*entUser.Name)
	}

	if entUser.Email != nil {
		id, _ := r.ExistByEmail(ctx, *entUser.Email)
		if id > 0 && id != g.GetId() {
			r.log.Errorf("用户名已存在，用户信息：%v", g)
			return nil, fmt.Errorf("user email already exists")
		}
		builder = builder.SetNillableEmail(entUser.Email)
	}
	if entUser.Phone != nil {
		id, _ := r.ExistByPhone(ctx, *entUser.Phone)
		if id > 0 && id != g.GetId() {
			r.log.Errorf("手机号已存在，用户信息：%v", g)
			return nil, fmt.Errorf("user phone already exists")
		}
		builder = builder.SetNillablePhone(entUser.Phone)
	}
	if g.Password != nil {
		hashPassword, _ := crypto.HashPassword(*entUser.Password)
		builder = builder.SetPassword(hashPassword)
	}
	res, err := builder.
		SetNillableNickname(entUser.Nickname).
		SetNillableRealname(entUser.Realname).
		SetNillableBirthday(entUser.Birthday).
		SetNillableGender(entUser.Gender).
		SetNillableAvatar(entUser.Avatar).
		SetNillableStatus(entUser.Status).
		SetNillableDescription(entUser.Description).
		Save(ctx)
	if err != nil {
		r.log.Errorf("更新用户失败，用户信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.toProto(res), nil
}

// FindByID 通过ID查询用户信息
// 参数：ctx 上下文，id 用户ID
// 返回值：用户信息，错误信息
func (r *userRepo) FindByID(ctx context.Context, id uint32) (*pbCore.User, error) {
	r.log.Infof("通过ID查询用户，ID：%d", id)
	res, err := r.data.DB(ctx).User.Query().
		// Select(user.FieldID, user.FieldName, user.FieldEmail, user.FieldNickname, user.FieldRealname, user.FieldGender, user.FieldAvatar, user.FieldDescription, user.FieldPhone, user.FieldStatus, user.FieldBirthday, user.FieldCreatedAt, user.FieldUpdatedAt).
		Where(user.IDEQ(id)).Only(ctx)
	fmt.Printf("%v", res)
	if err != nil {
		r.log.Errorf("通过ID查询用户失败，ID：%d，错误：%v", id, err)
		if gen.IsNotFound(err) {
			return nil, errors.New("查询数据不存在")
		}
		return nil, err
	}
	return r.toProto(res), nil
}

// Count 统计用户数量
// 参数：ctx 上下文
// 返回值：用户数量，错误信息
func (r *userRepo) Count(ctx context.Context, condition []string) (int64, error) {
	r.log.Infof("统计用户数量")
	entQuery := r.data.DB(ctx).User.Query()
	if len(condition) > 0 {

		// entQuery.Where(sql.Column(user.FieldName).)
	}
	count, err := entQuery.Count(ctx)
	if err != nil {
		r.log.Errorf("统计用户数量失败，错误：%v", err)
		return 0, err
	}
	return int64(count), nil
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
	res, err := r.data.DB(ctx).User.Query().Select(user.FieldID, user.FieldName).Order(gen.Desc(user.FieldID)).All(ctx)
	if err != nil {
		r.log.Errorf("查询所有用户列表失败，错误：%v", err)
		return nil, err
	}
	return convert.SliceToAny(res, r.toProto), nil
}

// ListPageSimple 查询用户简单列表分页
// 参数：ctx 上下文，pagination 分页请求
// 返回值：用户列表响应，错误信息
func (r *userRepo) ListPageSimple(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListUserResponse, error) {
	r.log.Infof("查询用户简单列表分页，分页请求：%v", pagination)
	count, err := r.data.DB(ctx).User.Query().Select(user.FieldID).Where().Count(ctx)
	if err != nil {
		r.log.Errorf("查询所有用户列表失败，错误：%v", err)
		return nil, err
	}
	res, err := r.data.DB(ctx).User.Query().
		Select(user.FieldID, user.FieldName).
		Where().
		Offset(int((pagination.GetPage() - 1) * pagination.GetPageSize())).
		Limit(int(pagination.GetPageSize())).
		Order(gen.Desc(user.FieldID)).
		All(ctx)
	if err != nil {
		r.log.Errorf("查询用户简单列表分页失败，分页请求：%v，错误：%v", pagination, err)
		return nil, err
	}
	return &pbCore.ListUserResponse{
		Items: convert.SliceToAny(res, r.toProto),
		Total: int32(count),
	}, nil
}

// ListPage 查询用户列表分页
// 参数：ctx 上下文，pagination 分页请求
// 返回值：用户列表响应，错误信息
func (r *userRepo) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListUserResponse, error) {
	r.log.Infof("查询用户列表分页，分页请求：%v", pagination)
	count, err := r.data.DB(ctx).User.Query().Select(user.FieldID).Count(ctx)
	if err != nil {
		r.log.Errorf("查询所有用户列表失败，错误：%v", err)
		return nil, err
	}
	res, err := r.data.DB(ctx).User.Query().
		Select(
			user.FieldID,
			user.FieldName,
			user.FieldEmail,
			user.FieldNickname,
			user.FieldRealname,
			user.FieldBirthday,
			user.FieldGender,
			user.FieldPhone,
			user.FieldAvatar,
			user.FieldStatus,
			user.FieldCreatedAt,
			user.FieldUpdatedAt,
		).
		Offset(int((pagination.GetPage() - 1) * pagination.GetPageSize())).
		Limit(int(pagination.GetPageSize())).
		Order(gen.Desc(user.FieldID)).
		All(ctx)
	if err != nil {
		r.log.Errorf("查询用户列表分页失败，分页请求：%v，错误：%v", pagination, err)
		return nil, err
	}
	return &pbCore.ListUserResponse{
		Items: convert.SliceToAny(res, r.toProto),
		Total: int32(count),
	}, nil
}

// Delete 删除用户
// 参数：ctx 上下文，id 用户ID
// 返回值：错误信息
func (r *userRepo) Delete(ctx context.Context, id uint32) error {
	r.log.Infof("删除用户，用户ID：%d", id)
	err := r.data.DB(ctx).User.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if err != nil {
		r.log.Errorf("删除用户失败，用户ID：%d，错误：%v", id, err)
		return err
	}
	return nil
}

func filterUser(q *gen.UserQuery, req *pbCore.User) {
	if req.GetName() != "" {
		q.Where(user.NameContains(req.GetName()))
	}
	if req.GetPhone() != "" {
		q.Where(user.PhoneContains(req.GetPhone()))
	}
	if req.GetEmail() != "" {
		q.Where(user.EmailContains(req.GetEmail()))
	}
	if req.GetStatus() != 0 {
		q.Where(user.Status(int32(*req.Status)))
	}
}
