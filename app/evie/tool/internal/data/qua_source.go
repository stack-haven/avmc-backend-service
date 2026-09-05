// Package data · qua_source.go
// QuaVocabularySource：把 qua opaque 数据包成 biz.RawEntity。
//
// 本文件是 biz.VocabularySource 的具体实现，薄包装 QuaFetcher。
//
// 设计要点（Q13）：
//   1. Source.Name() = "qua"（与 conf.VocabRules.sources key 对齐）
//   2. fetcher 返回的 opaque map 直接打包为 RawEntity；不解释字段语义
//   3. EntityType 取常量 QuaEntityUser / QuaEntityDepartment
//   4. 错误沿用 fetcher 的 kratos error；adapter 不重新包装
package data

import (
	"context"
	"errors"
	"fmt"

	"backend-service/app/evie/tool/internal/biz"
)

// quaEntityType qua 系统对外暴露的 entity_type 命名（与 vocab_rules.yaml 对齐）。
const (
	QuaEntityUser       = "user"
	QuaEntityDepartment = "department"
)

// QuaSourceName VocabularySource.Name() 返回值。
const QuaSourceName = "qua"

// quaVocabularySource 实现 biz.VocabularySource。
type quaVocabularySource struct {
	fetcher QuaFetcher
}

// NewQuaVocabularySource 构造 qua VocabularySource adapter。
func NewQuaVocabularySource(fetcher QuaFetcher) biz.VocabularySource {
	return &quaVocabularySource{fetcher: fetcher}
}

// Name 返回来源标识（"qua"）。
func (s *quaVocabularySource) Name() string { return QuaSourceName }

// Fetch 拉取原始实体并包装为 RawEntity 列表。
//
// partial-failure 容忍：user + dept 任一失败时仍返回已拉到的实体。
func (s *quaVocabularySource) Fetch(ctx context.Context) ([]biz.RawEntity, error) {
	if s.fetcher == nil {
		return nil, nil
	}

	out := make([]biz.RawEntity, 0)
	var errs []error

	// 1. 用户
	users, err := s.fetcher.FetchUsersRaw(ctx)
	if err != nil {
		errs = append(errs, fmt.Errorf("users: %w", err))
	} else {
		for _, u := range users {
			out = append(out, biz.RawEntity{
				SourceID:   extractStringID(u, "id"),
				EntityType: QuaEntityUser,
				Source:     QuaSourceName,
				Data:       u,
			})
		}
	}

	// 2. 部门
	depts, derr := s.fetcher.FetchDeptsRaw(ctx)
	if derr != nil {
		errs = append(errs, fmt.Errorf("depts: %w", derr))
	} else {
		for _, d := range depts {
			out = append(out, biz.RawEntity{
				SourceID:   extractStringID(d, "id"),
				EntityType: QuaEntityDepartment,
				Source:     QuaSourceName,
				Data:       d,
			})
		}
	}

	if len(errs) == 0 {
		return out, nil
	}
	if len(out) == 0 {
		return nil, errors.Join(errs...)
	}
	return out, errors.Join(errs...)
}

// extractStringID 从 opaque map 提取 ID 字符串（兼容数字 ID）。
func extractStringID(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return itoa64(int64(x))
		}
		return fmtSprintAny(x)
	case int64:
		return itoa64(x)
	case int:
		return itoa64(int64(x))
	}
	return fmtSprintAny(v)
}

// itoa64 / fmtSprintAny 简化版（避免每次引用 fmt 包外符号）。
func itoa64(n int64) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func fmtSprintAny(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return itoa64(int64(x))
	case int64:
		return itoa64(x)
	case int:
		return itoa64(int64(x))
	case bool:
		if x {
			return "true"
		}
		return "false"
	}
	return ""
}

// 编译期断言
var _ biz.VocabularySource = (*quaVocabularySource)(nil)
