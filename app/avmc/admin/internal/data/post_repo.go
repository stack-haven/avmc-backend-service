package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/post"
	"backend-service/pkg/utils/convert"
)

var _ biz.PostRepo = (*postRepo)(nil)

type postRepo struct {
	data *Data
	log  *log.Helper
}

// NewPostRepo 创建新的岗位仓库实例
// 参数：data 数据访问层实例，logger 日志记录器
// 返回值：岗位仓库实例指针
func NewPostRepo(data *Data, logger log.Logger) biz.PostRepo {
	return &postRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// toProto 转换gen.Post为pbCore.Post
func (r *postRepo) toProto(res *gen.Post) *pbCore.Post {
	return &pbCore.Post{
		Id:        res.ID,
		Name:      res.Name,
		CreatedAt: convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt: convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

// toEnt 转换pbCore.Post为gen.Post
func (r *postRepo) toEnt(g *pbCore.Post) *gen.Post {
	return &gen.Post{
		ID:   g.GetId(),
		Name: g.Name,
	}
}

// Save 保存岗位信息
// 参数：ctx 上下文，g 岗位信息
// 返回值：岗位信息，错误信息
func (r *postRepo) Save(ctx context.Context, g *pbCore.Post) (*pbCore.Post, error) {
	r.log.Infof("保存岗位，岗位信息：%v", g)
	entPost := r.toEnt(g)
	builder := r.data.DB(ctx).Post.Create()

	id, _ := r.GetPostExistByName(ctx, *entPost.Name)
	if id > 0 {
		r.log.Errorf("岗位名称已存在，岗位信息：%v", g)
		return nil, fmt.Errorf("post name already exists")
	}

	res, err := builder.SetName(*entPost.Name).
		Save(ctx)
	if err != nil {
		r.log.Errorf("保存岗位失败，岗位信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.toProto(res), nil
}

// GetPostExistByName 获取岗位名称是否存在
// 参数：ctx 上下文，name 岗位名称
// 返回值：岗位ID，错误信息
func (r *postRepo) GetPostExistByName(ctx context.Context, name string) (uint32, error) {
	r.log.Infof("获取岗位名称是否存在，岗位名称：%v", name)
	entPost, err := r.data.DB(ctx).Post.Query().Where(post.Name(name)).Select(post.FieldID).First(ctx)
	if err != nil {
		r.log.Errorf("获取岗位名称是否存在失败，岗位名称：%v，错误：%v", name, err)
		return 0, err
	}
	return entPost.ID, nil
}

// Update 更新岗位信息
// 参数：ctx 上下文，g 岗位信息
// 返回值：岗位信息，错误信息
func (r *postRepo) Update(ctx context.Context, g *pbCore.Post) (*pbCore.Post, error) {
	r.log.Infof("更新岗位，岗位信息：%v", g)
	entPost := r.toEnt(g)
	builder := r.data.DB(ctx).Post.UpdateOneID(g.GetId())
	id, _ := r.GetPostExistByName(ctx, *entPost.Name)
	if id > 0 && id != g.GetId() {
		r.log.Errorf("岗位名称已存在，岗位信息：%v", g)
		return nil, fmt.Errorf("post name already exists")
	}

	res, err := builder.
		SetName(*entPost.Name).
		Save(ctx)
	if err != nil {
		r.log.Errorf("更新岗位失败，岗位信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.toProto(res), nil
}

// FindByID 通过ID查询岗位信息
// 参数：ctx 上下文，id 岗位ID
// 返回值：岗位信息，错误信息
func (r *postRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Post, error) {
	r.log.Infof("通过ID查询岗位，ID：%d", id)
	res, err := r.data.DB(ctx).Post.Query().
		Where(post.IDEQ(id)).Only(ctx)
	if err != nil {
		r.log.Errorf("通过ID查询岗位失败，ID：%d，错误：%v", id, err)
		if gen.IsNotFound(err) {
			return nil, errors.New("查询数据不存在")
		}
		return nil, err
	}
	return r.toProto(res), nil
}

// Delete 删除岗位
// 参数：ctx 上下文，id 岗位ID
// 返回值：错误信息
func (r *postRepo) Delete(ctx context.Context, id uint32) error {
	r.log.Infof("删除岗位，岗位ID：%d", id)
	err := r.data.DB(ctx).Post.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if err != nil {
		r.log.Errorf("删除岗位失败，岗位ID：%d，错误：%v", id, err)
		return err
	}
	return nil
}

// ListByName 通过岗位名称查询岗位列表
// 参数：ctx 上下文，name 岗位名称
// 返回值：岗位列表，错误信息
func (r *postRepo) ListByName(ctx context.Context, name string) ([]*pbCore.Post, error) {
	r.log.Infof("通过岗位名称查询岗位，岗位名称：%s", name)
	res, err := r.data.DB(ctx).Post.Query().Where(post.NameContains(name)).All(ctx)
	if err != nil {
		r.log.Errorf("通过岗位名称查询岗位失败，岗位名称：%s，错误：%v", name, err)
		return nil, err
	}
	return convert.SliceToAny(res, r.toProto), nil
}

// ListAll 查询所有岗位列表
// 参数：ctx 上下文
// 返回值：岗位列表，错误信息
func (r *postRepo) ListAll(ctx context.Context) ([]*pbCore.Post, error) {
	r.log.Infof("查询所有岗位列表")
	res, err := r.data.DB(ctx).Post.Query().Select(post.FieldID, post.FieldName).Where().Order(gen.Desc(post.FieldID)).All(ctx)
	if err != nil {
		r.log.Errorf("查询所有岗位列表失败，错误：%v", err)
		return nil, err
	}
	return convert.SliceToAny(res, r.toProto), nil
}

// ListPage 查询岗位列表分页
// 参数：ctx 上下文，pagination 分页请求
// 返回值：岗位列表响应，错误信息
func (r *postRepo) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListPostResponse, error) {
	r.log.Infof("查询岗位列表分页，分页请求：%v", pagination)
	count, err := r.data.DB(ctx).Post.Query().Select(post.FieldID).Where(post.DeletedAtIsNil()).Count(ctx)
	if err != nil {
		r.log.Errorf("查询所有岗位列表失败，错误：%v", err)
		return nil, err
	}
	res, err := r.data.DB(ctx).Post.Query().
		Select(
			post.FieldID,
			post.FieldName,
			post.FieldCreatedAt,
			post.FieldUpdatedAt,
		).
		Where().
		Offset(int((pagination.GetPage() - 1) * pagination.GetPageSize())).
		Limit(int(pagination.GetPageSize())).
		Order(gen.Desc(post.FieldID)).
		All(ctx)
	if err != nil {
		r.log.Errorf("查询岗位列表分页失败，分页请求：%v，错误：%v", pagination, err)
		return nil, err
	}
	return &pbCore.ListPostResponse{
		Items: convert.SliceToAny(res, r.toProto),
		Total: int32(count),
	}, nil
}
