package biz

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	pb "backend-service/api/edai/service/v1"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── fakes ────────────────────────────────────────────────────────────────

type fakeFileRepo struct {
	mu      sync.Mutex
	records map[string]*pb.FileRecord // raw_path → record
	missing int32
}

func newFakeFileRepo() *fakeFileRepo {
	return &fakeFileRepo{records: map[string]*pb.FileRecord{}}
}

// UpsertScanned 要并发安全：扫描现在由 worker 池写入（见 file_scan.go）。
func (f *fakeFileRepo) UpsertScanned(_ context.Context, rec *pb.FileRecord) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, exists := f.records[rec.GetRawPath()]
	f.records[rec.GetRawPath()] = rec
	return !exists, nil
}
func (f *fakeFileRepo) GetByID(context.Context, uint32) (*pb.FileRecord, error) { return nil, nil }
func (f *fakeFileRepo) GetByPath(_ context.Context, _ uint32, p string) (*pb.FileRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.records[p], nil
}
func (f *fakeFileRepo) List(context.Context, ...FileListOption) ([]*pb.FileRecord, error) {
	return nil, nil
}
func (f *fakeFileRepo) Count(context.Context, ...FileListOption) (int32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return int32(len(f.records)), nil
}
func (f *fakeFileRepo) Stats(context.Context, uint32) (*FileStats, error) { return &FileStats{}, nil }

// MarkMissingExcept 语义与 data 层一致：本轮没出现的路径标为 missing。
func (f *fakeFileRepo) MarkMissingExcept(_ context.Context, _ uint32, keepPaths []string) (int32, error) {
	keep := make(map[string]struct{}, len(keepPaths))
	for _, p := range keepPaths {
		keep[p] = struct{}{}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int32
	for p := range f.records {
		if _, ok := keep[p]; !ok {
			n++
		}
	}
	f.missing = n
	return n, nil
}
func (f *fakeFileRepo) RecomputeDuplicates(context.Context, uint32) (int32, error) { return 0, nil }
func (f *fakeFileRepo) len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.records)
}

func newFileUsecaseForTest(repo FileRepo) *FileUsecase {
	return NewFileUsecase(repo, nil, log.NewStdLogger(discard{}))
}

// ── helpers ──────────────────────────────────────────────────────────────

// buildTree 构造测试目录：2 个业务目录 + 1 个缓存目录 + 1 个符号链接。
func buildTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "A-客户甲"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "B-客户乙"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".cache"), 0o750))

	for i := 0; i < 3; i++ {
		require.NoError(t, os.WriteFile(
			filepath.Join(root, "A-客户甲", "poster_"+string(rune('a'+i))+".txt"),
			[]byte("design"), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "B-客户乙", "banner.jpg"), []byte("img"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".cache", "tmp.tmp"), []byte("x"), 0o600))
	// 符号链接必须被跳过（防越界）
	_ = os.Symlink("/etc/hosts", filepath.Join(root, "link"))
	return root
}

// ── tests ────────────────────────────────────────────────────────────────

func TestScan_RegistersRegularFilesOnly(t *testing.T) {
	root := buildTree(t)
	repo := newFakeFileRepo()
	uc := newFileUsecaseForTest(repo)

	res, err := uc.Scan(context.Background(), 1, ScanOptions{RootPath: root})
	require.NoError(t, err)

	// 3 txt + 1 jpg + 1 tmp = 5 个常规文件；符号链接被跳过。
	// 未配置 exclude 时 .cache 也会被扫到——排除必须显式配置，不做隐式猜测
	// （系统设计基线 §6.1「不能依赖单一固定目录模板」）。
	assert.Equal(t, 5, res.Discovered)
	assert.Equal(t, 5, res.Created)
	assert.Equal(t, 0, res.Failed)
	assert.Equal(t, 1, res.Skipped, "符号链接应被跳过")
	assert.Equal(t, 5, repo.len())
}

func TestScan_ExcludePatterns(t *testing.T) {
	root := buildTree(t)
	repo := newFakeFileRepo()
	uc := newFileUsecaseForTest(repo)

	res, err := uc.Scan(context.Background(), 1, ScanOptions{
		RootPath: root,
		Exclude:  []string{".cache", "*.tmp"},
	})
	require.NoError(t, err)
	assert.Equal(t, 4, res.Discovered, "排除 .cache 后剩 3 txt + 1 jpg")

	// .cache 目录被整棵跳过：tmp.tmp 不应登记
	_, exists := repo.records[filepath.Join(root, ".cache", "tmp.tmp")]
	assert.False(t, exists, "排除目录内的文件不应被登记")
}

func TestScan_IncludeOnlyMatching(t *testing.T) {
	root := buildTree(t)
	repo := newFakeFileRepo()
	uc := newFileUsecaseForTest(repo)

	res, err := uc.Scan(context.Background(), 1, ScanOptions{
		RootPath: root,
		Include:  []string{"*.jpg"},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Discovered)
	assert.Equal(t, 1, repo.len())
}

func TestScan_PreservesRawPathAndTimes(t *testing.T) {
	root := buildTree(t)
	repo := newFakeFileRepo()
	uc := newFileUsecaseForTest(repo)

	_, err := uc.Scan(context.Background(), 1, ScanOptions{RootPath: root})
	require.NoError(t, err)

	for path, rec := range repo.records {
		assert.Equal(t, path, rec.GetRawPath(), "原始路径必须原样保留")
		assert.Equal(t, filepath.Dir(path), rec.GetRawParentPath())
		assert.NotEmpty(t, rec.GetQuickFingerprint())
		assert.Equal(t, "present", rec.GetFileStatus())
		// 文件系统时间必须保留原始语义（不等于扫描时间）
		assert.NotEmpty(t, rec.GetFsModifiedAt())
	}
}

func TestScan_HashIsOptIn(t *testing.T) {
	root := buildTree(t)
	repo := newFakeFileRepo()
	uc := newFileUsecaseForTest(repo)

	_, err := uc.Scan(context.Background(), 1, ScanOptions{RootPath: root})
	require.NoError(t, err)
	for _, rec := range repo.records {
		assert.Empty(t, rec.GetContentHash(), "默认不计算内容哈希（大文件成本高）")
	}

	repo2 := newFakeFileRepo()
	uc2 := newFileUsecaseForTest(repo2)
	_, err = uc2.Scan(context.Background(), 1, ScanOptions{RootPath: root, ComputeHash: true})
	require.NoError(t, err)
	for _, rec := range repo2.records {
		assert.Len(t, rec.GetContentHash(), 64, "SHA256 十六进制长度")
	}
}

func TestScan_IncrementalMarksUpdated(t *testing.T) {
	root := buildTree(t)
	repo := newFakeFileRepo()
	uc := newFileUsecaseForTest(repo)

	first, err := uc.Scan(context.Background(), 1, ScanOptions{RootPath: root})
	require.NoError(t, err)
	assert.Equal(t, first.Discovered, first.Created)

	second, err := uc.Scan(context.Background(), 1, ScanOptions{RootPath: root})
	require.NoError(t, err)
	assert.Equal(t, 0, second.Created, "路径未变不应重复登记")
	assert.Equal(t, second.Discovered, second.Updated)
}

func TestScan_RenamedFileKeepsOldRecord(t *testing.T) {
	root := buildTree(t)
	repo := newFakeFileRepo()
	uc := newFileUsecaseForTest(repo)

	_, err := uc.Scan(context.Background(), 1, ScanOptions{RootPath: root})
	require.NoError(t, err)
	before := repo.len()

	oldPath := filepath.Join(root, "A-客户甲", "poster_a.txt")
	newPath := filepath.Join(root, "A-客户甲", "poster_a_v2.txt")
	require.NoError(t, os.Rename(oldPath, newPath))

	res, err := uc.Scan(context.Background(), 1, ScanOptions{RootPath: root})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Created, "新路径产生新记录（路径不是文件身份）")
	assert.Equal(t, before+1, repo.len(), "旧记录不删除，由 MarkMissingExcept 标记")

	n, err := repo.MarkMissingExcept(context.Background(), 1, res.ScannedPaths)
	require.NoError(t, err)
	assert.Equal(t, int32(1), n, "旧路径应被标记 missing")
}

func TestScan_SubPathTraversalRejected(t *testing.T) {
	root := buildTree(t)
	uc := newFileUsecaseForTest(newFakeFileRepo())

	_, err := uc.Scan(context.Background(), 1, ScanOptions{RootPath: root, SubPath: "../../etc"})
	require.Error(t, err, "越界子路径必须拒绝")
	assert.Contains(t, err.Error(), "越界")
}

func TestScan_SubPathScoped(t *testing.T) {
	root := buildTree(t)
	repo := newFakeFileRepo()
	uc := newFileUsecaseForTest(repo)

	res, err := uc.Scan(context.Background(), 1, ScanOptions{RootPath: root, SubPath: "B-客户乙"})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Discovered, "只扫指定子目录")
}

func TestScan_CancelAtSafePoint(t *testing.T) {
	root := buildTree(t)
	repo := newFakeFileRepo()
	uc := newFileUsecaseForTest(repo)

	calls := 0
	res, err := uc.Scan(context.Background(), 1, ScanOptions{
		RootPath: root,
		ShouldStop: func() bool {
			calls++
			return calls > 1 // 第二个文件后停止
		},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrScanCancelled)
	assert.True(t, res.Stopped)
	assert.Less(t, res.Discovered, 5, "取消后不应继续扫完全部文件")
}

func TestScan_EmptyRootRejected(t *testing.T) {
	uc := newFileUsecaseForTest(newFakeFileRepo())
	_, err := uc.Scan(context.Background(), 1, ScanOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "根路径")
}

func TestScan_MissingRootReportsUnreachable(t *testing.T) {
	uc := newFileUsecaseForTest(newFakeFileRepo())
	_, err := uc.Scan(context.Background(), 1, ScanOptions{RootPath: "/nonexistent/path/edai"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "路径不存在")
}

// ── 状态机 ───────────────────────────────────────────────────────────────

func TestScanStatusTransitions(t *testing.T) {
	allowed := [][2]string{
		{ScanStatusPending, ScanStatusRunning},
		{ScanStatusPending, ScanStatusCancelled},
		{ScanStatusRunning, ScanStatusSucceeded},
		{ScanStatusRunning, ScanStatusFailed},
		{ScanStatusRunning, ScanStatusCancelRequested},
		{ScanStatusCancelRequested, ScanStatusCancelled},
	}
	for _, p := range allowed {
		assert.True(t, scanTransitions[p[0]][p[1]], "%s → %s 应允许", p[0], p[1])
	}

	denied := [][2]string{
		{ScanStatusPending, ScanStatusSucceeded},
		{ScanStatusSucceeded, ScanStatusRunning},
		{ScanStatusFailed, ScanStatusRunning},
		{ScanStatusCancelled, ScanStatusRunning},
		{ScanStatusCancelRequested, ScanStatusSucceeded},
	}
	for _, p := range denied {
		assert.False(t, scanTransitions[p[0]][p[1]], "%s → %s 应拒绝", p[0], p[1])
	}
}

// TestAllowsMissingDetection 覆盖「只有看全了整棵根目录才有资格判消失」。
//
// 实测缺陷：带 sub_path 的扫描即使标了 fullScan，观察范围也只有子树，
// 之前会据此把子树外的记录全部标 missing（扫 A-客户甲 → B-客户乙 全消失）。
func TestAllowsMissingDetection(t *testing.T) {
	cases := []struct {
		name     string
		fullScan bool
		subPath  string
		want     bool
	}{
		{name: "整棵全量", fullScan: true, want: true},
		{name: "子路径全量", fullScan: true, subPath: "A-客户甲", want: false},
		{name: "增量", fullScan: false, want: false},
		{name: "子路径增量", fullScan: false, subPath: "A-客户甲", want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := &pb.StartScanRequest{FullScan: c.fullScan}
			if c.subPath != "" {
				req.SubPath = &c.subPath
			}
			assert.Equal(t, c.want, allowsMissingDetection(req))
		})
	}
}
