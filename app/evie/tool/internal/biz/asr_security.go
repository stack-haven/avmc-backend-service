package biz

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// safeIDPattern 限制 session_id / tenant_id 用于文件路径的合法字符集。
//
// 约束：
//   - 仅字母数字 + `-` `_`
//   - 长度 1-64
//   - 防止路径穿越字符（`..`、分隔符、绝对路径前缀等）
var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// SanitizeSessionID 校验/返回客户端传入的 session id。
//
// 空字符串视为 “未提供”，调用方应自行生成。
// 不合法返回错误，调用方应直接拒绝请求。
func SanitizeSessionID(id string) (string, error) {
	if id == "" {
		return "", nil
	}
	if !safeIDPattern.MatchString(id) {
		return "", fmt.Errorf("biz.asr: invalid session_id")
	}
	return id, nil
}

// SanitizeTenantID 校验租户 ID，用于文件路径拼接。
//
// tenant ID 不允许为空，也不允许包含路径字符。
func SanitizeTenantID(id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("biz.asr: empty tenant_id")
	}
	if !safeIDPattern.MatchString(id) {
		return "", fmt.Errorf("biz.asr: invalid tenant_id")
	}
	return id, nil
}

// IsPathWithin 判断 target 是否在 base 目录之下（解析为绝对路径后比较）。
//
// 防止 `..` 穿越或绝对路径逃逸出安全根目录。
func IsPathWithin(base, target string) bool {
	if base == "" || target == "" {
		return false
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if strings.HasPrefix(rel, "..") || strings.Contains(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}
