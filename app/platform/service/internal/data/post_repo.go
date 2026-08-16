package data

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/post"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
)

var _ biz.PostRepo = (*postRepo)(nil)

type postRepo struct {
	BaseRepo
}

func NewPostRepo(data *Data, logger log.Logger) biz.PostRepo {
	return &postRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func (r *postRepo) convertProto(res *gen.Post) *pb.Post {
	status := enum.Status(0)
	if res.Status != nil {
		status = enum.Status(*res.Status)
	}
	return &pb.Post{
		Id:        res.ID,
		Name:      res.Name,
		Sort:      res.Sort,
		Status:    convert.EmptyToNil(status),
		Remark:    res.Remark,
		CreatedAt: convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt: convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

func (r *postRepo) convertEnt(g *pb.Post) *gen.Post {
	return &gen.Post{
		ID:     g.GetId(),
		Name:   g.Name,
		Sort:   g.Sort,
		Status: convert.ToPointer(int32(g.GetStatus())),
		Remark: g.Remark,
	}
}

func (r *postRepo) Save(ctx context.Context, g *pb.Post) (*pb.Post, error) {
	if g == nil || g.GetName() == "" {
		return nil, pb.ErrorPostNameCannotBeEmpty("岗位名称不能为空")
	}
	r.Log.Infof("保存岗位: %s", g.GetName())
	entPost := r.convertEnt(g)
	id, err := r.GetPostExistByName(ctx, *entPost.Name)
	if err != nil {
		return nil, fmt.Errorf("checking post name uniqueness: %w", err)
	}
	if id > 0 {
		return nil, pb.ErrorPostAlreadyExists("岗位名称已存在")
	}

	res, err := r.Data.DB(ctx).Post.Create().
		SetName(*entPost.Name).
		SetNillableSort(entPost.Sort).
		SetNillableStatus(entPost.Status).
		SetNillableRemark(entPost.Remark).
		Save(ctx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorPostAlreadyExists("岗位名称已存在")
		}
		if gen.IsNotFound(err) {
			return nil, pb.ErrorPostNotFound("岗位不存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *postRepo) GetPostExistByName(ctx context.Context, name string) (uint32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	entPost, err := r.Data.DB(ctx).Post.Query().Where(post.Name(name)).Select(post.FieldID).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return entPost.ID, nil
}

func (r *postRepo) Update(ctx context.Context, g *pb.Post) (*pb.Post, error) {
	if g == nil || g.GetId() == 0 || g.GetName() == "" {
		return nil, pb.ErrorPostInvalidId("岗位ID和名称不能为空")
	}
	entPost := r.convertEnt(g)
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	id, err := r.GetPostExistByName(ctx, *entPost.Name)
	if err != nil {
		return nil, fmt.Errorf("checking post name uniqueness: %w", err)
	}
	if id > 0 && id != g.GetId() {
		return nil, pb.ErrorPostAlreadyExists("岗位名称已存在")
	}

	res, err := r.Data.DB(ctx).Post.UpdateOneID(g.GetId()).
		SetName(*entPost.Name).
		SetNillableSort(entPost.Sort).
		SetNillableStatus(entPost.Status).
		SetNillableRemark(entPost.Remark).
		Save(ctx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorPostAlreadyExists("岗位名称已存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *postRepo) FindByID(ctx context.Context, id uint32) (*pb.Post, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).Post.Query().Where(post.IDEQ(id)).Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorPostNotFound("岗位不存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *postRepo) Delete(ctx context.Context, id uint32) error {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return err
	}
	err := r.Data.DB(ctx).Post.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if gen.IsNotFound(err) {
		return pb.ErrorPostNotFound("岗位不存在")
	}
	return err
}

func (r *postRepo) ListByName(ctx context.Context, name string) ([]*pb.Post, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).Post.Query().Where(post.NameContains(name)).All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

func (r *postRepo) CountPosts(ctx context.Context, opts ...listing.Option) (int32, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return 0, err
	}
	o := listing.Options{}
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

func (r *postRepo) ListAll(ctx context.Context) ([]*pb.Post, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).Post.Query().
		Select(post.FieldID, post.FieldName).
		Order(gen.Desc(post.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

func (r *postRepo) ListPosts(ctx context.Context, opts ...listing.Option) ([]*pb.Post, error) {
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	pos, err := r.Data.DB(ctx).Post.Query().
		Select(post.FieldID, post.FieldName, post.FieldSort, post.FieldStatus, post.FieldRemark, post.FieldCreatedAt, post.FieldUpdatedAt).
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(pos, r.convertProto), nil
}

func (r *postRepo) ListPage(ctx context.Context, req *pb.ListPostsRequest) (*pb.ListPostsResponse, error) {
	r.Log.Infof("查询岗位列表分页，page_size=%d page_token=%s", req.GetPageSize(), req.GetPageToken())
	if _, err := r.RequireTenantID(ctx); err != nil {
		return nil, err
	}
	count, err := r.Data.DB(ctx).Post.Query().Select(post.FieldID).Where(post.DeletedAtIsNil()).Count(ctx)
	if err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).Post.Query().
		Select(post.FieldID, post.FieldName, post.FieldSort, post.FieldStatus, post.FieldRemark, post.FieldCreatedAt, post.FieldUpdatedAt).
		Limit(int(req.GetPageSize())).
		Order(gen.Desc(post.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ListPostsResponse{
		Items: convert.SliceToAny(res, r.convertProto),
		Total: int32(count),
	}, nil
}
