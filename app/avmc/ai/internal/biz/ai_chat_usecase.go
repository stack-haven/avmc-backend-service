package biz

import (
	pbEnum "backend-service/api/common/enum"
	"context"

	pb "backend-service/api/avmc/ai/v1"

	"github.com/go-kratos/kratos/v2/log"
)

// ChatRepo is a Chat repo.
type ChatRepo interface {
	Save(context.Context, *pb.Chat) (*pb.Chat, error)
	Update(context.Context, *pb.Chat) (*pb.Chat, error)
	FindByID(context.Context, uint32) (*pb.Chat, error)
	ListChats(context.Context, ...ListOption) ([]*pb.Chat, error)
	CountChats(context.Context, ...ListOption) (int32, error)
	ListAll(context.Context) ([]*pb.Chat, error)
	Delete(context.Context, uint32) error
}

// ChatUsecase is a Chat usecase.
// 包含用户仓库和日志记录器
type ChatUsecase struct {
	repo ChatRepo
	log  *log.Helper
}

// NewChatUsecase new a Chat usecase.
func NewChatUsecase(repo ChatRepo, logger log.Logger) *ChatUsecase {
	return &ChatUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Create 处理创建用户请求
// 参数：ctx 上下文，g 用户信息
// 返回值：创建用户响应，错误信息
func (uc *ChatUsecase) Create(ctx context.Context, g *pb.Chat) (*pb.Chat, error) {
	uc.log.WithContext(ctx).Infof("CreateChat: %v", g.Name)
	return uc.repo.Save(ctx, g)
}

// Get 处理获取用户详情请求
// 参数：ctx 上下文，id 用户ID
// 返回值：用户详情响应，错误信息
func (uc *ChatUsecase) Get(ctx context.Context, id uint32) (*pb.Chat, error) {
	uc.log.WithContext(ctx).Infof("GetChat: %v", id)
	return uc.repo.FindByID(ctx, id)
}

// Update 处理更新用户请求
// 参数：ctx 上下文，g 用户信息
// 返回值：更新用户响应，错误信息
func (uc *ChatUsecase) Update(ctx context.Context, g *pb.Chat) (*pb.Chat, error) {
	uc.log.WithContext(ctx).Infof("UpdateChat: %v", g.GetId())
	return uc.repo.Update(ctx, g)
}

// ListPageSimple 处理分页用户简单列表请求
// 参数：ctx 上下文，opts 分页选项
// 返回值：用户列表响应，错误信息
func (uc *ChatUsecase) ListPageSimple(ctx context.Context, opts ...ListOption) ([]*pb.Chat, error) {
	resp, err := uc.repo.ListChats(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Delete 处理删除用户请求
// 参数：ctx 上下文，id 用户ID
// 返回值：错误信息
func (uc *ChatUsecase) Delete(ctx context.Context, id uint32) error {
	return uc.repo.Delete(ctx, id)
}

// UpdateStatus 处理更新用户状态请求
// 参数：ctx 上下文，id 用户ID，status 用户状态
// 返回值：更新后的用户信息，错误信息
func (uc *ChatUsecase) UpdateStatus(ctx context.Context, id uint32, status pbEnum.Status) (*pb.Chat, error) {
	uc.log.WithContext(ctx).Infof("UpdateStatus：%v %v", id, status)
	g, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	g.Status = &status
	return uc.repo.Update(ctx, g)
}

// ListChats 处理分页用户列表请求
// 参数：ctx 上下文，opts 分页选项
// 返回值：用户列表响应，错误信息
func (uc *ChatUsecase) ListChats(ctx context.Context, opts ...ListOption) ([]*pb.Chat, error) {
	return uc.repo.ListChats(ctx, opts...)
}

// CountChats 处理用户条件查询聚合请求
// 参数：ctx 上下文，opts 分页选项
// 返回值：用户数量，错误信息
func (uc *ChatUsecase) CountChats(ctx context.Context, opts ...ListOption) (int32, error) {
	resp, err := uc.repo.CountChats(ctx, opts...)
	if err != nil {
		return 0, err
	}
	return resp, nil
}
