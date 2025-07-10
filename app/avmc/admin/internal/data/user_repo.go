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
)

var _ biz.UserRepo = (*userRepo)(nil)

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
		Nickname: &res.Nickname,
		Realname: &res.Realname,
		Gender:   (*enum.Gender)(&res.Gender),
		Avatar:   &res.Avatar,
		Phone:    &res.Phone,
		Status:   &res.Status,
		Birthday: func() *string {
			if res.Birthday.IsZero() {
				return nil
			}
			birthday := res.Birthday.Format(time.DateOnly)
			return &birthday
		}(),
		// optional int32 gender = 7 [(validate.rules).int32.gte = 0,(gnostic.openapi.v3.property) = {description: "性别 0 未知 1 男 2 女"}];
		CreatedAt: func() *string {
			if res.CreatedAt.IsZero() {
				return nil
			}
			createdAt := res.CreatedAt.Format(time.DateTime)
			return &createdAt
		}(),
		UpdatedAt: func() *string {
			if res.UpdatedAt.IsZero() {
				return nil
			}
			updatedAt := res.UpdatedAt.Format(time.DateTime)
			return &updatedAt
		}(),
		Description: &res.Description,
	}
}

func (r *userRepo) toEnt(g *pbCore.User) *ent.User {
	return &ent.User{
		ID:       g.GetId(),
		Name:     g.GetName(),
		Email:    g.GetEmail(),
		Nickname: g.GetNickname(),
		Realname: g.GetRealname(),
		Birthday: func() time.Time {
			if g.GetBirthday() == "" {
				return time.Time{}
			}
			birthday, _ := time.Parse(time.DateOnly, g.GetBirthday())
			return birthday
		}(),
		Gender:      int32(g.GetGender()),
		Phone:       g.GetPhone(),
		Avatar:      g.GetAvatar(),
		Status:      int32(g.GetStatus()),
		Description: g.GetDescription(),
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

func (r *userRepo) ListByName(context.Context, string) ([]*pbCore.User, error) {
	return nil, nil
}
func (r *userRepo) ListByPhone(context.Context, string) ([]*pbCore.User, error) {
	return nil, nil
}

func (r *userRepo) ListAll(context.Context) ([]*pbCore.User, error) {
	return nil, nil
}
func (r *userRepo) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListUserResponse, error) {
	return nil, nil
}

func (r *userRepo) Delete(ctx context.Context, id uint32) error {
	return nil
}
