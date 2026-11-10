package data

import (
	"fmt"
	"testing"

	pb "backend-service/api/edai/service/v1"

	"github.com/stretchr/testify/require"
)

// TestMarkMissingExcept_PaginationDoesNotSkip 覆盖「边改状态边分页」的坑。
//
// 背景（实测缺陷）：早期实现用 Offset 分页读取 present 记录，同时把过期行
// 改成 missing。被改的行随即离开 present 集合，Offset 就会跳过这些空位之后
// 的记录 —— 10000 个文件删 300 个，只标出 287 个（漏 13 个）。
// 修复改用 keyset（id > cursor）翻页。
func TestMarkMissingExcept_PaginationDoesNotSkip(t *testing.T) {
	d := openTestData(t)
	repo := NewFileRepo(d)

	// 数量要跨过 missingPageSize(2000) 才能触发多页；
	// 每页都删一部分，最大程度暴露 Offset 漂移。
	const total = 4500
	var keep []string
	for i := range total {
		path := fmt.Sprintf("/k/f%05d.dat", i)
		_, err := repo.UpsertScanned(t.Context(), &pb.FileRecord{
			SourceId:         1,
			RawPath:          path,
			RawParentPath:    "/k",
			FileName:         fmt.Sprintf("f%05d.dat", i),
			FileSize:         int64(i),
			FileStatus:       "present",
			QuickFingerprint: "fp",
			FirstSeenAt:      "2026-01-01T00:00:00Z",
			LastScannedAt:    "2026-01-01T00:00:00Z",
		})
		require.NoError(t, err)
		// 每 3 个保留 2 个 → 期望标 missing 的正好是 total/3
		if i%3 != 0 {
			keep = append(keep, path)
		}
	}

	n, err := repo.MarkMissingExcept(t.Context(), 1, keep)
	require.NoError(t, err)
	require.Equal(t, int32(total/3), n, "所有未保留的记录都必须被标记，不能因分页漂移漏标")
}

// TestMarkMissingExcept_EmptyKeepIsNoop 空 keepPaths 不得清空整个数据源。
func TestMarkMissingExcept_EmptyKeepIsNoop(t *testing.T) {
	d := openTestData(t)
	repo := NewFileRepo(d)
	_, err := repo.UpsertScanned(t.Context(), &pb.FileRecord{
		SourceId:         1,
		RawPath:          "/k/a.dat",
		RawParentPath:    "/k",
		FileName:         "a.dat",
		FileSize:         1,
		FileStatus:       "present",
		QuickFingerprint: "fp",
		FirstSeenAt:      "2026-01-01T00:00:00Z",
		LastScannedAt:    "2026-01-01T00:00:00Z",
	})
	require.NoError(t, err)

	n, err := repo.MarkMissingExcept(t.Context(), 1, nil)
	require.NoError(t, err)
	require.Zero(t, n, "空的保留集通常意味着扫描起点不可读，不能据此清空标记")

	rec, err := repo.GetByPath(t.Context(), 1, "/k/a.dat")
	require.NoError(t, err)
	require.Equal(t, "present", rec.GetFileStatus())
}
