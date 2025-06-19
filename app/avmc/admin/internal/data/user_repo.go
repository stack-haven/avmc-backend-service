package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent"
	"backend-service/app/avmc/admin/internal/data/ent/user"
)

type userRepo struct {
	data *Data
	log  *log.Helper
}

// NewuserRepo .
func NewUserRepo(data *Data, logger log.Logger) biz.UserRepo {
	return &userRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *userRepo) toProto(res *ent.User) *pbCore.User {
	return &pbCore.User{
		Id:       res.ID,
		Name:     &res.Name,
		Email:    &res.Email,
		NickName: &res.Nickname,
		RealName: &res.Realname,
	}
}

func (r *userRepo) toEnt(g *pbCore.User) *ent.User {
	return &ent.User{
		ID:       g.GetId(),
		Name:     g.GetName(),
		Email:    g.GetEmail(),
		Nickname: g.GetNickName(),
		Realname: g.GetRealName(),
		// Birthday:  g.GetBirthday(),
		Gender: int(g.GetGender()),
		Phone:  g.GetPhone(),
		Avatar: g.GetAvatar(),
		Status: int(g.GetState()),
	}
}

func (r *userRepo) Save(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	return g, nil
}

func (r *userRepo) Update(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	return g, nil
}

func (r *userRepo) FindByID(ctx context.Context, id uint32) (*pbCore.User, error) {
	r.log.Infof("通过ID查询用户，ID：%d", id)
	res, err := r.data.DB(ctx).User.Query().Where(user.IDEQ(id)).Only(ctx)
	if err != nil {
		r.log.Errorf("通过ID查询用户失败，ID：%d，错误：%v", id, err)
		return nil, err
	}
	return r.toProto(res), nil
}

func (r *userRepo) ListByHello(context.Context, string) ([]*pbCore.User, error) {
	return nil, nil
}

func (r *userRepo) ListAll(context.Context) ([]*pbCore.User, error) {
	return nil, nil
}
