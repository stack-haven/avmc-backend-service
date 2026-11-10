package data

import (
	"testing"

	pb "backend-service/api/edai/service/v1"
	"backend-service/app/edai/service/internal/biz"

	"github.com/stretchr/testify/require"
	"go.einride.tech/aip/filtering"
)

// parseForTest 模拟服务层的解析：声明字段 → 解析表达式。
// 用的是与 service 层完全相同的声明集合，避免测试与真实契约脱节。
func parseForTest(t *testing.T, expr string) filtering.Filter {
	t.Helper()
	decls, err := filtering.NewDeclarations(append(
		[]filtering.DeclarationOption{filtering.DeclareStandardFunctions()},
		fileFilterDeclarationsForTest...,
	)...)
	require.NoError(t, err)
	f, err := filtering.ParseFilterString(expr, decls)
	require.NoError(t, err)
	return f
}

// fileFilterDeclarationsForTest 与 service 层声明保持一致（含 true/false 字面量）。
var fileFilterDeclarationsForTest = []filtering.DeclarationOption{
	filtering.DeclareIdent("true", filtering.TypeBool),
	filtering.DeclareIdent("false", filtering.TypeBool),
	filtering.DeclareIdent("file_name", filtering.TypeString),
	filtering.DeclareIdent("raw_path", filtering.TypeString),
	filtering.DeclareIdent("extension", filtering.TypeString),
	filtering.DeclareIdent("file_status", filtering.TypeString),
	filtering.DeclareIdent("source_id", filtering.TypeInt),
	filtering.DeclareIdent("file_size", filtering.TypeInt),
	filtering.DeclareIdent("is_duplicate", filtering.TypeBool),
	filtering.DeclareIdent("fs_modified_at", filtering.TypeTimestamp),
}

// seedFiles 写入一组可用于过滤的样本。
func seedFiles(t *testing.T, repo *fileRepo) {
	t.Helper()
	cases := []struct {
		path, name, ext, status string
		size                    int64
	}{
		{"/c/a/poster_01.psd", "poster_01.psd", ".psd", "present", 100},
		{"/c/a/poster_02.png", "poster_02.png", ".png", "present", 200},
		{"/c/b/banner.png", "banner.png", ".png", "missing", 300},
	}
	for _, c := range cases {
		ext := c.ext
		_, err := repo.UpsertScanned(t.Context(), &pb.FileRecord{
			SourceId:         1,
			RawPath:          c.path,
			RawParentPath:    "/c",
			FileName:         c.name,
			Extension:        &ext,
			FileSize:         c.size,
			FileStatus:       c.status,
			QuickFingerprint: "fp",
			FirstSeenAt:      "2026-01-01T00:00:00Z",
			LastScannedAt:    "2026-01-01T00:00:00Z",
		})
		require.NoError(t, err)
	}
}

func listWithFilter(t *testing.T, repo *fileRepo, expr string) []*pb.FileRecord {
	t.Helper()
	items, err := repo.List(t.Context(), func(q *biz.FileListQuery) {
		q.Limit = 50
		q.Filter = parseForTest(t, expr)
	})
	require.NoError(t, err)
	return items
}

// TestFileFilter_AIP160 覆盖 AIP-160 过滤的常见形态。
func TestFileFilter_AIP160(t *testing.T) {
	d := openTestData(t)
	repo := NewFileRepo(d)
	seedFiles(t, repo)

	t.Run("包含匹配", func(t *testing.T) {
		items := listWithFilter(t, repo, `file_name:"poster"`)
		require.Len(t, items, 2)
	})

	t.Run("等值匹配", func(t *testing.T) {
		items := listWithFilter(t, repo, `file_status="missing"`)
		require.Len(t, items, 1)
		require.Equal(t, "banner.png", items[0].GetFileName())
	})

	t.Run("AND 组合", func(t *testing.T) {
		items := listWithFilter(t, repo, `file_name:"poster" AND extension=".png"`)
		require.Len(t, items, 1)
		require.Equal(t, "poster_02.png", items[0].GetFileName())
	})

	t.Run("OR 组合", func(t *testing.T) {
		items := listWithFilter(t, repo, `file_status="missing" OR extension=".psd"`)
		require.Len(t, items, 2)
	})

	t.Run("NOT 取反", func(t *testing.T) {
		items := listWithFilter(t, repo, `NOT file_status="present"`)
		require.Len(t, items, 1)
	})

	t.Run("数值范围", func(t *testing.T) {
		items := listWithFilter(t, repo, `file_size > 150`)
		require.Len(t, items, 2)
	})

	t.Run("路径包含", func(t *testing.T) {
		items := listWithFilter(t, repo, `raw_path:"/c/b"`)
		require.Len(t, items, 1)
	})
}

// TestFileCount_MatchesFilter 计数必须与过滤后的列表一致，
// 否则分页总数会骗人（翻页翻到空页）。
func TestFileCount_MatchesFilter(t *testing.T) {
	d := openTestData(t)
	repo := NewFileRepo(d)
	seedFiles(t, repo)

	filter := parseForTest(t, `file_name:"poster"`)
	opt := func(q *biz.FileListQuery) { q.Filter = filter }

	items, err := repo.List(t.Context(), opt)
	require.NoError(t, err)
	total, err := repo.Count(t.Context(), opt)
	require.NoError(t, err)
	require.Equal(t, int32(len(items)), total)
}

// TestFileFilter_UnknownFieldRejected 未声明字段必须在解析阶段被拒绝。
//
// 这条是防止「静默忽略」回归：早期白名单实现里写错的字段名会被跳过，
// 用户看到的却是全量结果，误以为筛选生效。
func TestFileFilter_UnknownFieldRejected(t *testing.T) {
	decls, err := filtering.NewDeclarations(append(
		[]filtering.DeclarationOption{filtering.DeclareStandardFunctions()},
		fileFilterDeclarationsForTest...,
	)...)
	require.NoError(t, err)
	_, err = filtering.ParseFilterString(`nonsense_field="x"`, decls)
	require.Error(t, err, "未声明字段必须解析失败")
}

// TestRecomputeDuplicates 覆盖内容去重标记。
func TestRecomputeDuplicates(t *testing.T) {
	d := openTestData(t)
	repo := NewFileRepo(d)

	sameHash := "aaaa"
	otherHash := "bbbb"
	records := []struct {
		path string
		hash *string
	}{
		{"/c/1/a.txt", &sameHash},
		{"/c/2/a-copy.txt", &sameHash}, // 与第一条同哈希 → 重复
		{"/c/3/b.txt", &otherHash},
		{"/c/4/nohash.txt", nil}, // 未计算哈希，不参与去重
	}
	for _, r := range records {
		_, err := repo.UpsertScanned(t.Context(), &pb.FileRecord{
			SourceId:         1,
			RawPath:          r.path,
			RawParentPath:    "/c",
			FileName:         "f.txt",
			FileSize:         1,
			FileStatus:       "present",
			QuickFingerprint: "fp",
			ContentHash:      r.hash,
			FirstSeenAt:      "2026-01-01T00:00:00Z",
			LastScannedAt:    "2026-01-01T00:00:00Z",
		})
		require.NoError(t, err)
	}

	n, err := repo.RecomputeDuplicates(t.Context(), 1)
	require.NoError(t, err)
	require.Equal(t, int32(1), n, "同哈希的第二条被标记重复")

	// 保留 id 最小的一条为正本
	first, err := repo.GetByPath(t.Context(), 1, "/c/1/a.txt")
	require.NoError(t, err)
	require.False(t, first.GetIsDuplicate(), "id 最小的记录保留为正本")

	second, err := repo.GetByPath(t.Context(), 1, "/c/2/a-copy.txt")
	require.NoError(t, err)
	require.True(t, second.GetIsDuplicate())

	// 未计算哈希的不参与
	noHash, err := repo.GetByPath(t.Context(), 1, "/c/4/nohash.txt")
	require.NoError(t, err)
	require.False(t, noHash.GetIsDuplicate())

	// 幂等：再算一次仍然只有 1 条
	n, err = repo.RecomputeDuplicates(t.Context(), 1)
	require.NoError(t, err)
	require.Equal(t, int32(1), n)
}

// TestRecomputeDuplicates_ScopedBySource 不同数据源之间不去重。
func TestRecomputeDuplicates_ScopedBySource(t *testing.T) {
	d := openTestData(t)
	repo := NewFileRepo(d)

	hash := "cccc"
	for i, sourceID := range []uint32{1, 2} {
		_, err := repo.UpsertScanned(t.Context(), &pb.FileRecord{
			SourceId:         sourceID,
			RawPath:          "/c/same.txt",
			RawParentPath:    "/c",
			FileName:         "same.txt",
			FileSize:         int64(i + 1),
			FileStatus:       "present",
			QuickFingerprint: "fp",
			ContentHash:      &hash,
			FirstSeenAt:      "2026-01-01T00:00:00Z",
			LastScannedAt:    "2026-01-01T00:00:00Z",
		})
		require.NoError(t, err)
	}

	n, err := repo.RecomputeDuplicates(t.Context(), 1)
	require.NoError(t, err)
	require.Zero(t, n, "跨数据源不算重复")
}
