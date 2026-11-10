package biz

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	pb "backend-service/api/edai/service/v1"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
)

// FileRepo 文件记录仓库。
type FileRepo interface {
	UpsertScanned(ctx context.Context, rec *pb.FileRecord) (created bool, err error)
	GetByID(ctx context.Context, id uint32) (*pb.FileRecord, error)
	GetByPath(ctx context.Context, sourceID uint32, rawPath string) (*pb.FileRecord, error)
	List(ctx context.Context, opts ...FileListOption) ([]*pb.FileRecord, error)
	Count(ctx context.Context, opts ...FileListOption) (int32, error)
	Stats(ctx context.Context, sourceID uint32) (*FileStats, error)
	// MarkMissingExcept 把本轮扫描没再看到的文件标记为 missing。
	//
	// keepPaths 是本轮实际登记成功的路径集合。用路径集合而不是时间戳：
	// last_scanned_at 只有秒级精度，同一秒内两次扫描无法区分先后。
	MarkMissingExcept(ctx context.Context, sourceID uint32, keepPaths []string) (int32, error)
	// RecomputeDuplicates 重算指定数据源的内容去重标记，返回被标记为重复的记录数。
	RecomputeDuplicates(ctx context.Context, sourceID uint32) (int32, error)
}

// FileStats 文件统计（数据层聚合结果）。
type FileStats struct {
	Total         int32
	Present       int32
	Missing       int32
	Unreachable   int32
	Duplicate     int32
	TotalSize     int64
	TopExtensions []ExtensionCount
}

// ExtensionCount 扩展名计数。
type ExtensionCount struct {
	Extension string
	Count     int32
}

// FileListOption 列表查询选项。
type FileListOption func(*FileListQuery)

// FileListQuery 列表查询参数。
type FileListQuery struct {
	// Filter 是已解析并做过字段声明的 AIP-160 过滤器；
	// 未声明字段在解析阶段就会被拒绝，不会静默忽略。
	Filter   filtering.Filter
	OrderBy  ordering.OrderBy
	Offset   int
	Limit    int
	SourceID uint32
}

// FileUsecase 文件记录业务逻辑。
//
// 只读 + 登记：EDAI 不修改、不移动、不重命名原始文件（系统设计基线 §1.3 原则 2）。
type FileUsecase struct {
	repo   FileRepo
	source *SourceUsecase
	log    *log.Helper
}

// NewFileUsecase 创建文件记录 usecase。
func NewFileUsecase(repo FileRepo, source *SourceUsecase, logger log.Logger) *FileUsecase {
	return &FileUsecase{repo: repo, source: source, log: log.NewHelper(log.With(logger, "module", "edai/biz/file"))}
}

// Get 查询文件记录。
func (uc *FileUsecase) Get(ctx context.Context, id uint32) (*pb.FileRecord, error) {
	if id == 0 {
		return nil, pb.ErrorBadRequest("文件记录ID不能为空")
	}
	rec, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, pb.ErrorAssetNotFound("文件记录不存在")
	}
	return rec, nil
}

// List 查询列表。
func (uc *FileUsecase) List(ctx context.Context, q FileListQuery) ([]*pb.FileRecord, int32, error) {
	opt := func(o *FileListQuery) { *o = q }
	items, err := uc.repo.List(ctx, opt)
	if err != nil {
		return nil, 0, err
	}
	total, err := uc.repo.Count(ctx, opt)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Stats 返回统计。
func (uc *FileUsecase) Stats(ctx context.Context, sourceID uint32) (*pb.GetFileStatsResponse, error) {
	stats, err := uc.repo.Stats(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	resp := &pb.GetFileStatsResponse{
		Total:            stats.Total,
		PresentCount:     stats.Present,
		MissingCount:     stats.Missing,
		UnreachableCount: stats.Unreachable,
		DuplicateCount:   stats.Duplicate,
		TotalSize:        stats.TotalSize,
	}
	for _, e := range stats.TopExtensions {
		resp.TopExtensions = append(resp.TopExtensions, &pb.ExtensionCount{
			Extension: e.Extension,
			Count:     e.Count,
		})
	}
	return resp, nil
}

// ScanOptions 扫描参数。
type ScanOptions struct {
	RootPath    string
	SubPath     string
	Exclude     []string
	Include     []string
	ComputeHash bool
	// MaxWorkers 并发处理文件的工作协程数；<=0 时用 DefaultScanWorkers。
	MaxWorkers int
	// MaxFilesPerSecond 全局限速（每秒处理文件数）；<=0 时用 DefaultScanFilesPerSecond，
	// 显式传 -1 表示不限速。
	MaxFilesPerSecond int
	// Progress 在每个文件处理后回调，用于取消检查与进度上报。
	Progress func(discovered int)
	// ShouldStop 返回 true 时在安全点停止扫描（取消）。
	ShouldStop func() bool
}

// 扫描并发与限速默认值。
//
// 系统设计基线 §7 Discovery 要求「必须有边界、取消机制、并发限制、资源限制」：
// 早期实现是单 goroutine 串行遍历 + 逐文件同步写库，几万文件时既慢又占满一个核。
// 现在改为「单协程遍历 + worker 池处理」，并对文件处理速率限流，避免把
// 远端 SMB 或机械盘打到不可用。
const (
	// DefaultScanWorkers 默认工作协程数。
	DefaultScanWorkers = 4
	// DefaultScanFilesPerSecond 默认每秒处理文件数上限。
	DefaultScanFilesPerSecond = 500
)

// ScanResult 扫描结果统计。
type ScanResult struct {
	Discovered   int
	Created      int
	Updated      int
	Skipped      int
	Failed       int
	Stopped      bool
	ScannedPaths []string
}

// ErrScanCancelled 表示扫描在安全点被取消。
var ErrScanCancelled = errors.New("scan cancelled")

// Scan 遍历目录并把文件登记为 FileRecord。
//
// 行为约定（系统设计基线 §7.2 / §7.3）：
//   - 只读：不创建、不修改、不移动、不重命名任何原始文件；
//   - 有边界：遵循 include/exclude 规则，跳过符号链接与目录循环；
//   - 可取消：每处理一个文件检查一次 ShouldStop，停在安全点；
//   - 有资源上限：worker 池 + 全局限速（见 ScanOptions）；
//   - 失败隔离：单个文件读取失败只记 failed，不中断整体扫描；
//   - 不跟随符号链接：避免越界与循环。
func (uc *FileUsecase) Scan(ctx context.Context, sourceID uint32, opts ScanOptions) (*ScanResult, error) {
	root := opts.RootPath
	if root == "" {
		return nil, pb.ErrorBadRequest("扫描根路径不能为空")
	}
	start := root
	if opts.SubPath != "" {
		start = filepath.Join(root, filepath.Clean(opts.SubPath))
		// 越界防护：子路径不得跳出根路径
		if rel, err := filepath.Rel(root, start); err != nil || strings.HasPrefix(rel, "..") {
			return nil, pb.ErrorBadRequest("子路径越界")
		}
	}
	info, err := os.Stat(start)
	if err != nil {
		return &ScanResult{}, pb.ErrorSourcePathUnreachable("%s", describePathError(start, err))
	}
	if !info.IsDir() {
		return &ScanResult{}, pb.ErrorSourcePathUnreachable("%s", "扫描起点不是目录: "+start)
	}

	w := newScanWorker(ctx, uc, sourceID, opts, start)
	defer w.close()

	if werr := w.walk(ctx); werr != nil {
		if errors.Is(werr, ErrScanCancelled) {
			w.res.Stopped = true
			w.wait()
			return w.res, ErrScanCancelled
		}
		w.wait()
		return w.res, werr
	}
	w.wait()
	return w.res, nil
}

// isIncluded 判断文件名是否命中包含规则（空规则表示包含全部）。
func isIncluded(name string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, name); ok {
			return true
		}
	}
	return false
}

// isExcluded 判断路径或目录名是否命中排除规则。
func isExcluded(path, name string, patterns []string) bool {
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, name); ok {
			return true
		}
		if ok, _ := filepath.Match(p, path); ok {
			return true
		}
	}
	return false
}
