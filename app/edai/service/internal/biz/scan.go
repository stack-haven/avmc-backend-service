package biz

import (
	"context"
	"sync"
	"time"

	pb "backend-service/api/edai/service/v1"

	"github.com/go-kratos/kratos/v2/log"
	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
)

// 任务状态（与系统设计基线 §14.2 对齐，取 V1.0 实际使用的最小子集）。
const (
	ScanStatusPending         = "pending"
	ScanStatusRunning         = "running"
	ScanStatusSucceeded       = "succeeded"
	ScanStatusFailed          = "failed"
	ScanStatusCancelled       = "cancelled"
	ScanStatusCancelRequested = "cancel_requested"
)

// 合法状态迁移表：只允许这些边，其它一律拒绝。
var scanTransitions = map[string]map[string]bool{
	ScanStatusPending: {
		ScanStatusRunning:   true,
		ScanStatusCancelled: true,
	},
	ScanStatusRunning: {
		ScanStatusSucceeded:       true,
		ScanStatusFailed:          true,
		ScanStatusCancelRequested: true,
	},
	ScanStatusCancelRequested: {
		ScanStatusCancelled: true,
	},
	// 终态不可再迁移
	ScanStatusSucceeded: {},
	ScanStatusFailed:    {},
	ScanStatusCancelled: {},
}

// ScanTaskRepo 扫描任务仓库。
type ScanTaskRepo interface {
	Create(ctx context.Context, task *pb.ScanTask) (*pb.ScanTask, error)
	Get(ctx context.Context, id uint32) (*pb.ScanTask, error)
	List(ctx context.Context, opts ...ScanTaskListOption) ([]*pb.ScanTask, error)
	Count(ctx context.Context, opts ...ScanTaskListOption) (int32, error)
	// TransitionStatus 以 CAS 语义更新状态：仅当当前状态允许迁移时才成功。
	TransitionStatus(ctx context.Context, id uint32, from, to string) (bool, error)
	UpdateProgress(ctx context.Context, id uint32, progress ScanProgress) error
	Finish(ctx context.Context, id uint32, status string, progress ScanProgress, errMsg string) error
	// RecoverInterrupted 把服务重启后遗留的 running 任务标记为失败（无法续跑的部分）。
	RecoverInterrupted(ctx context.Context) (int32, error)
}

// ScanProgress 扫描进度。
type ScanProgress struct {
	Discovered    int32
	Created       int32
	Updated       int32
	MarkedMissing int32
	Skipped       int32
	Failed        int32
}

// ScanTaskListOption 列表查询选项。
type ScanTaskListOption func(*ScanTaskListQuery)

// ScanTaskListQuery 列表查询参数。
type ScanTaskListQuery struct {
	// Filter 是已解析并做过字段声明的 AIP-160 过滤器。
	Filter   filtering.Filter
	OrderBy  ordering.OrderBy
	Offset   int
	Limit    int
	SourceID uint32
}

// ScanLimits 扫描并发与限速上限。
//
// 系统设计基线 §7 Discovery 要求「必须有边界、取消机制、并发限制、资源限制」，
// 而 §19 把「大规模扫描资源占用」列为风险项：并发数与限速必须可配，
// 不能硬编码——不同客户环境（本机 SSD / 远端 SMB）差异极大。
type ScanLimits struct {
	// Workers 并发处理文件的工作协程数；<=0 用 DefaultScanWorkers。
	Workers int
	// FilesPerSecond 每秒处理文件数上限；<0 表示不限速，0 用默认值。
	FilesPerSecond int
}

// ScanUsecase 扫描任务编排。
//
// 任务引擎自建（4-2 ADR-013 决策 5）：
//   - 任务状态以持久化数据为准，内存只做「当前是否有 worker 在跑」的取消信号；
//   - 取消是协作式的：置 cancel_requested，worker 在安全点停止；
//   - 服务重启后，遗留的 running 任务标记失败（不做假装续跑）。
type ScanUsecase struct {
	repo   ScanTaskRepo
	files  *FileUsecase
	source *SourceUsecase
	log    *log.Helper
	limits ScanLimits

	mu      sync.Mutex
	running map[uint32]context.CancelFunc // taskID → cancel
}

// NewScanUsecase 创建扫描任务 usecase。
func NewScanUsecase(repo ScanTaskRepo, files *FileUsecase, source *SourceUsecase, limits ScanLimits, logger log.Logger) *ScanUsecase {
	return &ScanUsecase{
		repo:    repo,
		files:   files,
		source:  source,
		log:     log.NewHelper(log.With(logger, "module", "edai/biz/scan")),
		limits:  limits,
		running: make(map[uint32]context.CancelFunc),
	}
}

// Start 创建任务并异步执行。
func (uc *ScanUsecase) Start(ctx context.Context, req *pb.StartScanRequest) (*pb.ScanTask, error) {
	// 断言必须在派生后台协程**之前**：run() 用 WithoutCancel 脱离请求生命周期，
	// 放到协程里就等于绕过鉴权。
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	src, err := uc.source.Get(ctx, req.GetSourceId())
	if err != nil {
		return nil, err
	}

	task, err := uc.repo.Create(ctx, &pb.ScanTask{
		SourceId:    src.GetId(),
		Status:      ScanStatusPending,
		RootPath:    src.GetRootPath(),
		SubPath:     req.SubPath,
		FullScan:    req.GetFullScan(),
		ComputeHash: req.GetComputeHash(),
		StartedAt:   time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return nil, err
	}

	// 用 Background：扫描不能因为 HTTP 请求结束而中止（系统设计基线 §4.3
	// 「关闭浏览器不停止后台任务」）。
	go uc.run(context.WithoutCancel(ctx), task.GetId(), src.GetRootPath(), src.GetIncludePatterns(), src.GetExcludePatterns(), req)

	return task, nil
}

// run 在后台执行扫描。
func (uc *ScanUsecase) run(ctx context.Context, taskID uint32, rootPath string, include, exclude []string, req *pb.StartScanRequest) {
	runCtx, cancel := context.WithCancel(ctx)
	uc.mu.Lock()
	uc.running[taskID] = cancel
	uc.mu.Unlock()
	defer func() {
		uc.mu.Lock()
		delete(uc.running, taskID)
		uc.mu.Unlock()
		cancel()
	}()

	ok, err := uc.repo.TransitionStatus(ctx, taskID, ScanStatusPending, ScanStatusRunning)
	if err != nil || !ok {
		uc.log.Warnf("task %d: failed to enter running (ok=%v err=%v)", taskID, ok, err)
		return
	}

	result, scanErr := uc.files.Scan(runCtx, mustSourceID(req), ScanOptions{
		RootPath:          rootPath,
		SubPath:           req.GetSubPath(),
		Include:           include,
		Exclude:           exclude,
		ComputeHash:       req.GetComputeHash(),
		MaxWorkers:        uc.limits.Workers,
		MaxFilesPerSecond: uc.limits.FilesPerSecond,
		ShouldStop:        func() bool { return runCtx.Err() != nil },
		Progress: func(discovered int) {
			// 每 200 个文件落一次进度，避免高频写库
			if discovered%200 == 0 {
				_ = uc.repo.UpdateProgress(ctx, taskID, ScanProgress{Discovered: int32(discovered)})
			}
		},
	})

	progress := ScanProgress{}
	var status, errMsg string
	if result != nil {
		progress = ScanProgress{
			Discovered: int32(result.Discovered),
			Created:    int32(result.Created),
			Updated:    int32(result.Updated),
			Skipped:    int32(result.Skipped),
			Failed:     int32(result.Failed),
		}
	}
	// 注意：这里必须判 scanErr，不能判上面的 err（那是状态迁移的错误，已处理过）。
	switch {
	case scanErr != nil && result != nil && result.Stopped:
		status = ScanStatusCancelled
	case scanErr != nil:
		status = ScanStatusFailed
		errMsg = scanErr.Error()
	default:
		status = ScanStatusSucceeded
	}

	// 标记本轮未再出现的文件为 missing。
	// 判定条件见 allowsMissingDetection：必须是「全量 + 整棵根目录」。
	if status == ScanStatusSucceeded && result != nil && allowsMissingDetection(req) {
		n, merr := uc.files.repo.MarkMissingExcept(ctx, req.GetSourceId(), result.ScannedPaths)
		if merr != nil {
			uc.log.Warnf("task %d: mark missing failed: %v", taskID, merr)
		} else {
			progress.MarkedMissing = n
		}
	}

	// 内容去重：只有真正算过哈希才有意义（未算哈希 = 不知道是否重复，
	// 不能当成「不重复」，所以也不在这时清标记）。
	if status == ScanStatusSucceeded && result != nil && req.GetComputeHash() {
		if n, derr := uc.files.repo.RecomputeDuplicates(ctx, req.GetSourceId()); derr != nil {
			uc.log.Warnf("task %d: recompute duplicates failed: %v", taskID, derr)
		} else {
			uc.log.Infof("task %d: %d duplicate record(s) marked", taskID, n)
		}
	}

	if ferr := uc.repo.Finish(ctx, taskID, status, progress, errMsg); ferr != nil {
		uc.log.Errorf("task %d: finish failed: %v", taskID, ferr)
	}
	uc.log.Infof("task %d finished: status=%s discovered=%d created=%d updated=%d failed=%d",
		taskID, status, progress.Discovered, progress.Created, progress.Updated, progress.Failed)
}

// Get 查询任务。
func (uc *ScanUsecase) Get(ctx context.Context, id uint32) (*pb.ScanTask, error) {
	if id == 0 {
		return nil, pb.ErrorBadRequest("任务ID不能为空")
	}
	task, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, pb.ErrorTaskNotFound("任务不存在")
	}
	return task, nil
}

// List 查询任务列表。
func (uc *ScanUsecase) List(ctx context.Context, q ScanTaskListQuery) ([]*pb.ScanTask, int32, error) {
	opt := func(o *ScanTaskListQuery) { *o = q }
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

// Cancel 请求取消任务。
//
// 两步：先把状态置 cancel_requested（持久化事实），再触发内存 cancel 信号。
// 若任务尚未开始或已结束，返回状态错误而不是假装成功。
func (uc *ScanUsecase) Cancel(ctx context.Context, id uint32) (*pb.ScanTask, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	task, err := uc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	switch task.GetStatus() {
	case ScanStatusPending:
		if _, err := uc.repo.TransitionStatus(ctx, id, ScanStatusPending, ScanStatusCancelled); err != nil {
			return nil, err
		}
	case ScanStatusRunning:
		ok, err := uc.repo.TransitionStatus(ctx, id, ScanStatusRunning, ScanStatusCancelRequested)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, pb.ErrorTaskInvalidState("任务状态已变化，请刷新后重试")
		}
		uc.mu.Lock()
		if cancel, ok := uc.running[id]; ok {
			cancel()
		}
		uc.mu.Unlock()
	case ScanStatusCancelRequested:
		// 幂等：已经在取消中
	default:
		return nil, pb.ErrorTaskInvalidState("任务已结束，无法取消")
	}
	return uc.Get(ctx, id)
}

// RecoverInterrupted 服务启动时调用：把遗留的 running / cancel_requested 任务标记失败。
//
// 不做「假装续跑」：EDAI 的扫描是幂等的（按路径 upsert），但中断点未知，
// 直接标记失败让用户重跑，比静默续跑更可解释。
func (uc *ScanUsecase) RecoverInterrupted(ctx context.Context) (int32, error) {
	n, err := uc.repo.RecoverInterrupted(ctx)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		uc.log.Warnf("recovered %d interrupted scan task(s) as failed", n)
	}
	return n, nil
}

func mustSourceID(req *pb.StartScanRequest) uint32 { return req.GetSourceId() }

// allowsMissingDetection 判断这次扫描有没有资格做「消失」判定。
//
// 消失 = 「本轮没再看到」，这个结论只有在看全了整棵根目录时才成立：
// 带 sub_path 的扫描即使标了 fullScan，观察范围也只是子树，
// 子树之外的记录本来就看不到，据此标 missing 会把整个数据源误判成消失
// （实测：只扫 A-客户甲 会让 B-客户乙 的文件全部变 missing）。
//
// 因此：全量 **且** 未限定子路径，才做消失判定。
func allowsMissingDetection(req *pb.StartScanRequest) bool {
	return req.GetFullScan() && req.GetSubPath() == ""
}
