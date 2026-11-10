package biz

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	pb "backend-service/api/edai/service/v1"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
)

// DataSourceRepo 数据源仓库。
type DataSourceRepo interface {
	List(ctx context.Context, opts ...DataSourceListOption) ([]*pb.DataSource, error)
	Count(ctx context.Context, opts ...DataSourceListOption) (int32, error)
	Get(ctx context.Context, id uint32) (*pb.DataSource, error)
	Create(ctx context.Context, src *pb.DataSource) (*pb.DataSource, error)
	Update(ctx context.Context, id uint32, src *pb.DataSource) (*pb.DataSource, error)
	Delete(ctx context.Context, id uint32) error
	MarkChecked(ctx context.Context, id uint32, status, message string, checkedAt time.Time) error
}

// DataSourceListOption 列表查询选项（与 pkg/aip/listing 对齐的简化版）。
type DataSourceListOption func(*DataSourceListQuery)

// DataSourceListQuery 列表查询参数。
type DataSourceListQuery struct {
	// Filter 是已解析并做过字段声明的 AIP-160 过滤器。
	Filter  filtering.Filter
	OrderBy ordering.OrderBy
	Offset  int
	Limit   int
}

// SourceUsecase 数据源业务逻辑。
type SourceUsecase struct {
	repo DataSourceRepo
	log  *log.Helper
}

// NewSourceUsecase 创建数据源 usecase。
func NewSourceUsecase(repo DataSourceRepo, logger log.Logger) *SourceUsecase {
	return &SourceUsecase{repo: repo, log: log.NewHelper(log.With(logger, "module", "edai/biz/source"))}
}

// 允许的数据源类型。SMB 走同一路径语义（UNC），V1.0 不做凭据管理。
var allowedSourceTypes = map[string]bool{"local": true, "smb": true}

// Create 校验并创建数据源。
func (uc *SourceUsecase) Create(ctx context.Context, src *pb.DataSource) (*pb.DataSource, error) {
	if err := RequireRole(ctx, RoleAdmin, RoleOperator); err != nil {
		return nil, err
	}
	if err := validateSource(src); err != nil {
		return nil, err
	}
	return uc.repo.Create(ctx, src)
}

// Update 校验并更新数据源。
func (uc *SourceUsecase) Update(ctx context.Context, id uint32, src *pb.DataSource) (*pb.DataSource, error) {
	if err := RequireRole(ctx, RoleAdmin, RoleOperator); err != nil {
		return nil, err
	}
	if id == 0 {
		return nil, pb.ErrorBadRequest("数据源ID不能为空")
	}
	if err := validateSource(src); err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, id, src)
}

// Get 查询单个数据源。
func (uc *SourceUsecase) Get(ctx context.Context, id uint32) (*pb.DataSource, error) {
	if id == 0 {
		return nil, pb.ErrorBadRequest("数据源ID不能为空")
	}
	src, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, pb.ErrorResourceNotFound("数据源不存在")
	}
	return src, nil
}

// List 查询列表。
func (uc *SourceUsecase) List(ctx context.Context, q DataSourceListQuery) ([]*pb.DataSource, int32, error) {
	opts := []DataSourceListOption{
		func(o *DataSourceListQuery) {
			o.Filter, o.OrderBy, o.Offset, o.Limit = q.Filter, q.OrderBy, q.Offset, q.Limit
		},
	}
	items, err := uc.repo.List(ctx, opts...)
	if err != nil {
		return nil, 0, err
	}
	total, err := uc.repo.Count(ctx, opts...)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Delete 删除数据源配置（不触碰原始文件与已登记记录）。
func (uc *SourceUsecase) Delete(ctx context.Context, id uint32) error {
	if err := RequireAdmin(ctx); err != nil {
		return err
	}
	if id == 0 {
		return pb.ErrorBadRequest("数据源ID不能为空")
	}
	return uc.repo.Delete(ctx, id)
}

// TestConnection 校验路径可达与读取权限，并统计顶层条目数。
//
// 对文件系统只读：不创建、不修改、不移动任何文件（系统设计基线 §1.3 原则 2）；
// 但会把探测结果写进 connection_status，属于配置面变更，因此同样要求管理员。
func (uc *SourceUsecase) TestConnection(ctx context.Context, id uint32) (*pb.TestSourceConnectionResponse, error) {
	if err := RequireRole(ctx, RoleAdmin, RoleOperator); err != nil {
		return nil, err
	}
	src, err := uc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	root := src.GetRootPath()

	info, statErr := os.Stat(root)
	checkedAt := time.Now()
	if statErr != nil {
		msg := describePathError(root, statErr)
		if merr := uc.repo.MarkChecked(ctx, id, "unreachable", msg, checkedAt); merr != nil {
			uc.log.Warnf("mark source %d checked failed: %v", id, merr)
		}
		return &pb.TestSourceConnectionResponse{Reachable: false, Message: msg}, nil
	}
	if !info.IsDir() {
		msg := fmt.Sprintf("路径不是目录: %s", root)
		_ = uc.repo.MarkChecked(ctx, id, "unreachable", msg, checkedAt)
		return &pb.TestSourceConnectionResponse{Reachable: false, Message: msg}, nil
	}

	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		msg := describePathError(root, readErr)
		_ = uc.repo.MarkChecked(ctx, id, "unreachable", msg, checkedAt)
		return &pb.TestSourceConnectionResponse{Reachable: false, Message: msg}, nil
	}

	var dirs, files int
	for _, e := range entries {
		if e.IsDir() {
			dirs++
		} else {
			files++
		}
	}
	msg := fmt.Sprintf("可访问：%d 个目录，%d 个文件", dirs, files)
	if merr := uc.repo.MarkChecked(ctx, id, "reachable", msg, checkedAt); merr != nil {
		uc.log.Warnf("mark source %d checked failed: %v", id, merr)
	}
	return &pb.TestSourceConnectionResponse{
		Reachable:      true,
		Message:        msg,
		DirectoryCount: int32(dirs),
		FileCount:      int32(files),
	}, nil
}

func validateSource(src *pb.DataSource) error {
	if src == nil {
		return pb.ErrorBadRequest("数据源不能为空")
	}
	if src.GetName() == "" {
		return pb.ErrorBadRequest("数据源名称不能为空")
	}
	typ := src.GetType()
	if typ == "" {
		typ = "local"
		src.Type = typ
	}
	if !allowedSourceTypes[typ] {
		return pb.ErrorBadRequest("%s", "不支持的数据源类型: "+typ)
	}
	root := src.GetRootPath()
	if root == "" {
		return pb.ErrorBadRequest("根路径不能为空")
	}
	if !filepath.IsAbs(root) && typ == "local" {
		return pb.ErrorBadRequest("本地数据源必须使用绝对路径")
	}
	return nil
}

// describePathError 把文件系统错误转成可读说明，不泄露内部路径细节之外的敏感信息。
func describePathError(path string, err error) string {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return fmt.Sprintf("路径不存在: %s", path)
	case errors.Is(err, os.ErrPermission):
		return fmt.Sprintf("没有读取权限: %s（检查运行服务的账号）", path)
	default:
		return fmt.Sprintf("无法访问 %s: %v", path, err)
	}
}
