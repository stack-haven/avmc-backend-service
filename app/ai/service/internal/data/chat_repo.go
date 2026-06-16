package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/ai/service/v1"
	"backend-service/api/common/enum"
	"backend-service/app/ai/service/internal/biz"
	"backend-service/app/ai/service/internal/data/ent/gen"
	"backend-service/app/ai/service/internal/data/ent/gen/chat"
	"backend-service/pkg/utils/convert"

	"github.com/go-kratos/aip-go/ents"
)

var _ biz.ChatRepo = (*chatRepo)(nil)

// chatRepo 结构体
// 包含数据访问层实例和日志记录器
type chatRepo struct {
	data *Data
	log  *log.Helper
}

// NewChatRepo 创建新的对话仓库实例
// 参数：data 数据访问层实例，logger 日志记录器
// 返回值：对话仓库实例指针
func NewChatRepo(data *Data, logger log.Logger) biz.ChatRepo {
	return &chatRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// convertProto 转换gen.Chat为pb.Chat
func (r *chatRepo) convertProto(res *gen.Chat) *pb.Chat {
	return &pb.Chat{
		Id:        res.ID,
		Name:      res.Name,
		Status:    (*enum.Status)(res.Status),
		CreatedAt: convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt: convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

// convertEnt 转换pb.Chat为gen.Chat
func (r *chatRepo) convertEnt(g *pb.Chat) *gen.Chat {
	return &gen.Chat{
		ID:     g.GetId(),
		Name:   g.Name,
		Status: (*int32)(g.Status),
	}
}

// ExistByName 获取对话名是否存在
// 参数：ctx 上下文，name 对话名
// 返回值：对话ID，错误信息
func (r *chatRepo) ExistByName(ctx context.Context, name string) (uint32, error) {
	r.log.Infof("获取对话名是否存在，对话名：%v", name)
	entChat, err := r.data.DB(ctx).Chat.Query().Where(chat.Name(name)).Select(chat.FieldID).First(ctx)
	if err != nil {
		r.log.Errorf("获取对话名是否存在失败，对话名：%v，错误：%v", name, err)
		return 0, err
	}
	return entChat.ID, nil
}

// Save 保存对话信息
// 参数：ctx 上下文，g 对话信息
// 返回值：对话信息，错误信息
func (r *chatRepo) Save(ctx context.Context, g *pb.Chat) (*pb.Chat, error) {
	r.log.Infof("保存对话，对话名称：%s", g.GetName())
	entChat := r.convertEnt(g)
	builder := r.data.DB(ctx).Chat.Create()

	res, err := builder.SetName(*entChat.Name).
		SetNillableStatus(entChat.Status).
		Save(ctx)
	if err != nil {
		r.log.Errorf("保存对话失败，对话名称：%s，错误：%v", g.GetName(), err)
		return nil, err
	}
	return r.convertProto(res), nil
}

// Update 更新对话信息
// 参数：ctx 上下文，g 对话信息
// 返回值：对话信息，错误信息
func (r *chatRepo) Update(ctx context.Context, g *pb.Chat) (*pb.Chat, error) {
	r.log.Infof("更新对话，对话ID：%d", g.GetId())
	entChat := r.convertEnt(g)
	builder := r.data.DB(ctx).Chat.UpdateOneID(g.GetId())
	if g.Name != nil {
		id, _ := r.ExistByName(ctx, *entChat.Name)
		if id > 0 && id != g.GetId() {
			r.log.Errorf("对话名已存在，对话ID：%d", g.GetId())
			return nil, fmt.Errorf("chat name already exists")
		}
		builder = builder.SetName(*entChat.Name)
	}

	res, err := builder.
		SetNillableStatus(entChat.Status).
		Save(ctx)
	if err != nil {
		r.log.Errorf("更新对话失败，对话ID：%d，错误：%v", g.GetId(), err)
		return nil, err
	}
	return r.convertProto(res), nil
}

// FindByID 通过ID查询对话信息
// 参数：ctx 上下文，id 对话ID
// 返回值：对话信息，错误信息
func (r *chatRepo) FindByID(ctx context.Context, id uint32) (*pb.Chat, error) {
	r.log.Infof("通过ID查询对话，ID：%d", id)
	res, err := r.data.DB(ctx).Chat.Query().
		// Select(chat.FieldID, chat.FieldName, chat.FieldEmail, chat.FieldNickname, chat.FieldRealname, chat.FieldGender, chat.FieldAvatar, chat.FieldDescription, chat.FieldPhone, chat.FieldStatus, chat.FieldBirthday, chat.FieldCreatedAt, chat.FieldUpdatedAt).
		Where(chat.IDEQ(id)).Only(ctx)
	if err != nil {
		r.log.Errorf("通过ID查询对话失败，ID：%d，错误：%v", id, err)
		if gen.IsNotFound(err) {
			return nil, errors.New("查询数据不存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

// Count 统计对话数量
// 参数：ctx 上下文
// 返回值：对话数量，错误信息
func (r *chatRepo) Count(ctx context.Context, condition []string) (int64, error) {
	r.log.Infof("统计对话数量")
	entQuery := r.data.DB(ctx).Chat.Query()
	if len(condition) > 0 {

		// entQuery.Where(sql.Column(chat.FieldName).)
	}
	count, err := entQuery.Count(ctx)
	if err != nil {
		r.log.Errorf("统计对话数量失败，错误：%v", err)
		return 0, err
	}
	return int64(count), nil
}

// ListByName 通过对话名查询对话列表
// 参数：ctx 上下文，name 对话名
// 返回值：对话列表，错误信息
func (r *chatRepo) ListByName(ctx context.Context, name string) ([]*pb.Chat, error) {
	r.log.Infof("通过对话名查询对话，对话名：%s", name)
	res, err := r.data.DB(ctx).Chat.Query().Where(chat.NameContains(name)).All(ctx)
	if err != nil {
		r.log.Errorf("通过对话名查询对话失败，对话名：%s，错误：%v", name, err)
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

// ListAll 查询所有对话列表
// 参数：ctx 上下文
// 返回值：对话列表，错误信息
func (r *chatRepo) ListAll(ctx context.Context) ([]*pb.Chat, error) {
	r.log.Infof("查询所有对话列表")
	res, err := r.data.DB(ctx).Chat.Query().Select(chat.FieldID, chat.FieldName).Order(gen.Desc(chat.FieldID)).All(ctx)
	if err != nil {
		r.log.Errorf("查询所有对话列表失败，错误：%v", err)
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

// ListPageSimple 查询对话简单列表分页
// 参数：ctx 上下文，pagination 分页请求
// 返回值：对话列表响应，错误信息
func (r *chatRepo) ListPageSimple(ctx context.Context, opts ...biz.ListOption) ([]*pb.Chat, error) {
	o := biz.ListOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	r.log.Infof("查询对话简单列表分页，limit=%d offset=%d", o.Limit, o.Offset)
	res, err := r.data.DB(ctx).Chat.Query().
		Select(chat.FieldID, chat.FieldName).
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).
		Limit(o.Limit).
		Order(gen.Desc(chat.FieldID)).
		All(ctx)
	if err != nil {
		r.log.Errorf("查询对话简单列表分页失败，分页请求：%v，错误：%v", o, err)
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

// Delete 删除对话
// 参数：ctx 上下文，id 对话ID
// 返回值：错误信息
func (r *chatRepo) Delete(ctx context.Context, id uint32) error {
	r.log.Infof("删除对话，对话ID：%d", id)
	err := r.data.DB(ctx).Chat.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if err != nil {
		r.log.Errorf("删除对话失败，对话ID：%d，错误：%v", id, err)
		return err
	}
	return nil
}

// CountChats 查询对话数量
// 参数：ctx 上下文，filter 过滤条件
// 返回值：对话数量，错误信息
func (r *chatRepo) CountChats(ctx context.Context, opts ...biz.ListOption) (int32, error) {
	o := biz.ListOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	r.log.Infof("查询对话数量")
	count, err := r.data.db.Chat.Query().
		Select(chat.FieldID).
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		r.log.Errorf("查询所有对话列表失败，错误：%v", err)
		return 0, err
	}
	return int32(count), nil
}

// ListChats 查询对话列表
// 参数：ctx 上下文，opts 分页选项
// 返回值：对话列表，错误信息
func (r *chatRepo) ListChats(ctx context.Context, opts ...biz.ListOption) ([]*pb.Chat, error) {
	o := biz.ListOptions{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	r.log.Infof("查询对话列表，limit=%d offset=%d", o.Limit, o.Offset)
	pos, err := r.data.db.Chat.Query().
		Select(
			chat.FieldID,
			chat.FieldName,
			chat.FieldStatus,
			chat.FieldCreatedAt,
			chat.FieldUpdatedAt,
		).
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).
		Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(pos, r.convertProto), nil
}
