package biz

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
)

// quickFingerprint 生成「大小 + 修改时间」指纹，用于增量扫描判断是否变化。
//
// 不用内容哈希：大文件全量哈希成本高（系统设计基线 §7.3 要求按策略决定时机）。
func quickFingerprint(size int64, modTime time.Time) string {
	return fmt.Sprintf("%d-%d", size, modTime.UnixNano())
}

// hashFile 计算文件内容的 SHA256。
//
// 只读打开；文件不可读时返回错误，由调用方记 failed，不影响其它文件。
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
