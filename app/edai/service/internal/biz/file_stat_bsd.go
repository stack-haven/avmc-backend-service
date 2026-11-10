//go:build darwin || freebsd || netbsd || openbsd

package biz

import (
	"os"
	"syscall"
	"time"
)

// createdTime 从 Stat_t.Birthtimespec 读取文件创建时间（BSD/macOS 支持）。
func createdTime(fi os.FileInfo) (time.Time, bool) {
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, false
	}
	birth := stat.Birthtimespec
	if birth.Sec == 0 && birth.Nsec == 0 {
		return time.Time{}, false
	}
	return time.Unix(birth.Sec, birth.Nsec), true
}
