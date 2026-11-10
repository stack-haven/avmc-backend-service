package biz

import (
	"context"
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	pb "backend-service/api/edai/service/v1"

	"golang.org/x/time/rate"
)

// scanWorker 是扫描的并发执行体。
//
// 结构是「单协程遍历 + worker 池处理」：
//   - filepath.WalkDir 只在主协程跑，遍历顺序天然串行，不会并发读同一个目录；
//   - 文件的实际处理（stat / 哈希 / 写库）投递给固定数量的 worker，
//     用全局限速器约束每秒处理量（系统设计基线 §7 Discovery 的资源限制要求）。
//
// 所有对 res 的写入都在 mu 保护下，避免把统计写成数据竞争。
type scanWorker struct {
	uc       *FileUsecase
	sourceID uint32
	opts     ScanOptions
	start    string
	limiter  *rate.Limiter

	mu      sync.Mutex
	res     *ScanResult
	stopped bool // ShouldStop 命中，用于让 worker 尽快退出
	closed  bool

	// 进度上报独立加锁 + 单调过滤：多个 worker 并发回调时，
	// 落后的 discovered 值不能覆盖已经上报的更大值，否则前端进度会回退。
	progressMu   sync.Mutex
	lastReported int

	ctx  context.Context
	jobs chan scanJob
	wg   sync.WaitGroup
}

// scanJob 一个待处理的常规文件。
type scanJob struct {
	path string
	info fs.FileInfo
}

func newScanWorker(
	ctx context.Context,
	uc *FileUsecase,
	sourceID uint32,
	opts ScanOptions,
	start string,
) *scanWorker {
	workers := opts.MaxWorkers
	if workers <= 0 {
		workers = DefaultScanWorkers
	}

	var limiter *rate.Limiter
	// MaxFilesPerSecond < 0 表示显式不限速；0 用默认值。
	if opts.MaxFilesPerSecond >= 0 {
		limit := opts.MaxFilesPerSecond
		if limit == 0 {
			limit = DefaultScanFilesPerSecond
		}
		// burst 取 limit 与 workers 的较大值：突发量太小会让小目录扫描无谓等待。
		burst := max(limit, workers)
		limiter = rate.NewLimiter(rate.Limit(limit), burst)
	}

	w := &scanWorker{
		uc:       uc,
		sourceID: sourceID,
		opts:     opts,
		start:    start,
		limiter:  limiter,
		res:      &ScanResult{},
		ctx:      ctx,
		jobs:     make(chan scanJob, workers*4),
	}
	w.wg.Add(workers)
	for range workers {
		go w.work()
	}
	return w
}

// work 处理投递过来的文件，直到 jobs 关闭。
func (w *scanWorker) work() {
	defer w.wg.Done()
	for job := range w.jobs {
		w.process(job)
	}
}

// process 处理单个文件：建记录 → 按需哈希 → 写库。
//
// 失败只累加 failed 并记录日志，绝不中断整体扫描（系统设计基线 §7 失败隔离）。
func (w *scanWorker) process(job scanJob) {
	if w.isStopped() {
		return
	}
	// 限速在真正干活前等待；ctx 取消立即返回。
	if w.limiter != nil {
		if err := w.limiter.Wait(w.ctx); err != nil {
			return
		}
	}

	now := time.Now()
	rec := &pb.FileRecord{
		SourceId:         w.sourceID,
		RawPath:          job.path,
		RawParentPath:    filepath.Dir(job.path),
		FileName:         filepath.Base(job.path),
		FileSize:         job.info.Size(),
		FileStatus:       "present",
		FirstSeenAt:      now.Format(time.RFC3339),
		LastScannedAt:    now.Format(time.RFC3339),
		QuickFingerprint: quickFingerprint(job.info.Size(), job.info.ModTime()),
	}
	if ext := filepath.Ext(job.path); ext != "" {
		rec.Extension = &ext
	}
	// 文件系统时间：保留原始语义，不解读为设计/项目时间
	if !job.info.ModTime().IsZero() {
		mt := job.info.ModTime().Format(time.RFC3339)
		rec.FsModifiedAt = &mt
	}
	if ct, ok := createdTime(job.info); ok {
		s := ct.Format(time.RFC3339)
		rec.FsCreatedAt = &s
	}

	// 内容哈希按需计算：大文件成本高，由调用方按策略决定（§7.3）
	if w.opts.ComputeHash {
		h, herr := hashFile(job.path)
		if herr != nil {
			w.fail()
			w.uc.log.Warnf("hash %s: %v", job.path, herr)
			// 哈希失败不登记：留下 content_hash 为空的记录会让「已计算」语义失真。
			return
		}
		rec.ContentHash = &h
	}

	// 写库用不随 ctx 取消的上下文：已经读到的文件应该落库。
	// 否则取消瞬间在途的写操作会返回 "context canceled"，被计成 failed——
	// 实测取消一次扫描会冒出 8 个假失败，用户以为有文件读不出来。
	writeCtx := context.WithoutCancel(w.ctx)
	created, uerr := w.uc.repo.UpsertScanned(writeCtx, rec)
	if uerr != nil {
		w.fail()
		w.uc.log.Warnf("upsert %s: %v", job.path, uerr)
		return
	}

	w.mu.Lock()
	if created {
		w.res.Created++
	} else {
		w.res.Updated++
	}
	w.res.Discovered++
	w.res.ScannedPaths = append(w.res.ScannedPaths, job.path)
	discovered := w.res.Discovered
	w.mu.Unlock()

	w.reportProgress(discovered)
}

// reportProgress 单调上报进度。
func (w *scanWorker) reportProgress(discovered int) {
	if w.opts.Progress == nil {
		return
	}
	w.progressMu.Lock()
	defer w.progressMu.Unlock()
	if discovered <= w.lastReported {
		return
	}
	w.lastReported = discovered
	w.opts.Progress(discovered)
}

func (w *scanWorker) fail() {
	w.mu.Lock()
	w.res.Failed++
	w.mu.Unlock()
}

func (w *scanWorker) skip(n int) {
	w.mu.Lock()
	w.res.Skipped += n
	w.mu.Unlock()
}

func (w *scanWorker) isStopped() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stopped
}

// stop 标记停止，后续 process 立即返回。
func (w *scanWorker) stop() {
	w.mu.Lock()
	w.stopped = true
	w.mu.Unlock()
}

// walk 遍历目录并把常规文件投递给 worker。
//
// ctx 取消与 ShouldStop 都视为协作式取消：主协程停止遍历，worker 处理完
// 已入队的文件后退出（不丢已处理的登记，也不假装没发生）。
func (w *scanWorker) walk(ctx context.Context) error {
	err := filepath.WalkDir(w.start, func(path string, d fs.DirEntry, err error) error {
		if w.shouldStop(ctx) {
			return ErrScanCancelled
		}
		if err != nil {
			w.fail()
			w.uc.log.Warnf("scan entry error %s: %v", path, err)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			// 不跟随符号链接目录（防越界与循环）
			if d.Type()&fs.ModeSymlink != 0 {
				w.skip(1)
				return fs.SkipDir
			}
			if path != w.start && isExcluded(path, d.Name(), w.opts.Exclude) {
				w.skip(1)
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 || !d.Type().IsRegular() {
			w.skip(1)
			return nil
		}
		if !isIncluded(d.Name(), w.opts.Include) || isExcluded(path, d.Name(), w.opts.Exclude) {
			w.skip(1)
			return nil
		}

		fi, statErr := d.Info()
		if statErr != nil {
			w.fail()
			w.uc.log.Warnf("stat %s: %v", path, statErr)
			return nil
		}

		select {
		case w.jobs <- scanJob{path: path, info: fi}:
			return nil
		case <-ctx.Done():
			return ErrScanCancelled
		}
	})
	if err != nil {
		// 任何遍历错误都让 worker 尽快收尾，避免继续处理半截结果。
		w.stop()
	}
	return err
}

func (w *scanWorker) shouldStop(ctx context.Context) bool {
	if ctx.Err() != nil {
		return true
	}
	if w.opts.ShouldStop != nil && w.opts.ShouldStop() {
		return true
	}
	return w.isStopped()
}

// wait 等待所有 worker 处理完已入队文件，并关闭 jobs（幂等）。
func (w *scanWorker) wait() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	w.mu.Unlock()
	close(w.jobs)
	w.wg.Wait()
}

// close 是 wait 的兜底别名，配合 defer 使用。
func (w *scanWorker) close() { w.wait() }
