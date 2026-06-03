package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/post"
	"backend-service/pkg/utils/convert"
)

var _ biz.PostRepo = (*postRepo)(nil)

type postRepo struct {
	BaseRepo
}

func NewPostRepo(data *Data, logger log.Logger) biz.PostRepo {
	return &postRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func (r *postRepo) convertProto(res *gen.Post) *pbCore.Post {
	return &pbCore.Post{
		Id:        res.ID,
		Name:      res.Name,
		Sort:      res.Sort,
		Status:    convert.EmptyToNil(enum.Status(*res.Status)),
		Remark:    res.Remark,
		CreatedAt: convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt: convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

func (r *postRepo) convertEnt(g *pbCore.Post) *gen.Post {
	return &gen.Post{
		ID:     g.GetId(),
		Name:   g.Name,
		Sort:   g.Sort,
		Status: convert.ToPointer(int32(g.GetStatus())),
		Remark: g.Remark,
	}
}

func (r *postRepo) Save(ctx context.Context, g *pbCore.Post) (*pbCore.Post, error) {
	r.Log.Infof("保存岗位: %s", g.GetName())
	entPost := r.convertEnt(g)

	if id, _ := r.GetPostExistByName(ctx, *entPost.Name); id > 0 {
		return nil, fmt.Errorf("post name already exists")
	}

	res, err := r.Data.DB(ctx).Post.Create().
		SetName(*entPost.Name).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *postRepo) GetPostExistByName(ctx context.Context, name string) (uint32, error) {
	entPost, err := r.Data.DB(ctx).Post.Query().Where(post.Name(name)).Select(post.FieldID).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return entPost.ID, nil
}

func (r *postRepo) Update(ctx context.Context, g *pbCore.Post) (*pbCore.Post, error) {
	entPost := r.convertEnt(g)
	id, _ := r.GetPostExistByName(ctx, *entPost.Name)
	if id > 0 && id != g.GetId() {
		return nil, fmt.Errorf("post name already exists")
	}

	res, err := r.Data.DB(ctx).Post.UpdateOneID(g.GetId()).
		SetName(*entPost.Name).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *postRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Post, error) {
	res, err := r.Data.DB(ctx).Post.Query().Where(post.IDEQ(id)).Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, errors.New("查询数据不存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *postRepo) Delete(ctx context.Context, id uint32) error {
	return r.Data.DB(ctx).Post.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
}

func (r *postRepo) ListByName(ctx context.Context, name string) ([]*pbCore.Post, error) {
	res, err := r.Data.DB(ctx).Post.Query().Where(post.NameContains(name)).All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

func (r *postRepo) CountPosts(ctx context.Context, opts ...biz.ListOption) (int32, error) {
	o := biz.ListOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.Data.DB(ctx).Post.Query().
		Select(post.FieldID).
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *postRepo) ListAll(ctx context.Context) ([]*pbCore.Post, error) {
	res, err := r.Data.DB(ctx).Post.Query().
		Select(post.FieldID, post.FieldName).
		Order(gen.Desc(post.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

func (r *postRepo) ListPosts(ctx context.Context, opts ...biz.ListOption) ([]*pbCore.Post, error) {
	o := biz.ListOptions{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	pos, err := r.Data.DB(ctx).Post.Query().
		Select(post.FieldID, post.FieldName, post.FieldCreatedAt, post.FieldUpdatedAt).
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(pos, r.convertProto), nil
}

func (r *postRepo) ListPage(ctx context.Context, req *pbCore.ListPostsRequest) (*pbCore.ListPostsResponse, error) {
	r.Log.Infof("查询岗位列表分页，page_size=%d page_token=%s", req.GetPageSize(), req.GetPageToken())
	count, err := r.Data.DB(ctx).Post.Query().Select(post.FieldID).Where(post.DeletedAtIsNil()).Count(ctx)
	if err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).Post.Query().
		Select(post.FieldID, post.FieldName, post.FieldCreatedAt, post.FieldUpdatedAt).
		Where().
		Limit(int(req.GetPageSize())).
		Order(gen.Desc(post.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return &pbCore.ListPostsResponse{
		Items: convert.SliceToAny(res, r.convertProto),
		Total: int32(count),
	}, nil
}
