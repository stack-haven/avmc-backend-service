//go:build !darwin && !freebsd && !netbsd && !openbsd

package biz

import (
	"os"
	"time"
)

// createdTime 在非 BSD 平台（含 Linux）无可靠的创建时间来源，返回 false。
//
// FileRecord.fs_created_at 因此允许为空——系统设计基线要求「无法确认的字段允许为空，
// 不猜测」，不以 mtime 冒充创建时间。
func createdTime(_ os.FileInfo) (time.Time, bool) {
	return time.Time{}, false
}
