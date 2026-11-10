package data

import (
	"testing"

	pb "backend-service/api/edai/service/v1"

	"github.com/stretchr/testify/require"
)

// TestFileFilter_IsDuplicateTriState 覆盖 is_duplicate 的三态语义。
//
// 背景（浏览器实测缺陷）：表里 is_duplicate 是 bool，未计算内容哈希时恒为
// false。用户在界面上筛「非重复」时，实际混进了大量「没算过哈希」的记录，
// 看起来像是确认过没有副本——UI 上这些行显示的是「-」。
//
// 修复后：两个方向都要求 content_hash 非空，只返回有结论的记录。
func TestFileFilter_IsDuplicateTriState(t *testing.T) {
	d := openTestData(t)
	repo := NewFileRepo(d)

	hashA := "hash-a"
	hashB := "hash-b"
	cases := []struct {
		path string
		hash *string
	}{
		{"/d/1/original.txt", &hashA}, // 正本
		{"/d/2/copy.txt", &hashA},     // 重复
		{"/d/3/unique.txt", &hashB},   // 唯一
		{"/d/4/nohash.txt", nil},      // 未算哈希 → 不知道
	}
	for _, c := range cases {
		_, err := repo.UpsertScanned(t.Context(), &pb.FileRecord{
			SourceId:         1,
			RawPath:          c.path,
			RawParentPath:    "/d",
			FileName:         "f.txt",
			FileSize:         1,
			FileStatus:       "present",
			QuickFingerprint: "fp",
			ContentHash:      c.hash,
			FirstSeenAt:      "2026-01-01T00:00:00Z",
			LastScannedAt:    "2026-01-01T00:00:00Z",
		})
		require.NoError(t, err)
	}
	_, err := repo.RecomputeDuplicates(t.Context(), 1)
	require.NoError(t, err)

	t.Run("仅重复", func(t *testing.T) {
		items := listWithFilter(t, repo, `is_duplicate=true`)
		require.Len(t, items, 1)
		require.Equal(t, "/d/2/copy.txt", items[0].GetRawPath())
	})

	t.Run("非重复只返回有结论的", func(t *testing.T) {
		items := listWithFilter(t, repo, `is_duplicate=false`)
		paths := make([]string, 0, len(items))
		for _, it := range items {
			paths = append(paths, it.GetRawPath())
		}
		require.ElementsMatch(t, []string{"/d/1/original.txt", "/d/3/unique.txt"}, paths,
			"未计算哈希的记录必须排除：「没算过」不等于「不重复」")
	})

	// 未算哈希的记录仍应能被普通条件检索到
	t.Run("未算哈希仍可检索", func(t *testing.T) {
		items := listWithFilter(t, repo, `file_name:"f.txt"`)
		require.Len(t, items, len(cases))
	})
}
