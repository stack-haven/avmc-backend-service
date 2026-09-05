// Package data · qua_source.go
// QuaVocabularySource：把 qua opaque map 包成 RawEntity。
//
// 设计要点（Q13）：
//   1. Source Name 固定为 "qua"（与配置规则中的 source key 对齐）
//   2. 把 fetcher 返回的 opaque map 包成 RawEntity；**不解释字段语义**
//   3. EntityType 取固定常量："user" / "department"（与 Normalizer 规则对齐）
//   4. 错误沿用 fetcher 的 kratos error；adapter 不重新包装
package data

import (
	"context"
	"errors"
	"fmt"

	"backend-service/app/evie/tool/internal/biz"
)

// quaEntityType qua 系统对外暴露的 entity_type 命名（与 vocab_rules.yaml 对齐）。
//
// 未来 qua 增加新类型时只需在此追加 + 在 YAML 加规则，零核心代码变更。
const (
	QuaEntityUser       = "user"
	QuaEntityDepartment = "department"
)

// QuaSourceName VocabularySource.Name() 返回值（与 conf.VocabRules.sources key 对齐）。
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
// 仅做"包"动作：不读 Data 的具体字段；语义解析由 Normalizer + YAML 规则处理。
//
// 错误语义（M5 阶段进化）：
//   - user + dept 都成功 → (data, nil)
//   - 只有 user 成功 → (users, err)  // 仍返回 users 给调用方使用
//   - 只有 dept 成功 → (depts, err)
//   - 全失败 → (nil, err)
func (s *quaVocabularySource) Fetch(ctx context.Context) ([]biz.RawEntity, error) {
	if s.fetcher == nil {
		return nil, nil
	}

	out := make([]biz.RawEntity, 0)
	var errs []error

	// 1. 用户（partial-failure 容忍：501 等业务错误仍继续 dept）
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
//
// qua 系统 ID 通常是数字 bigint（JSON 序列化为 string 或 number）；
// 这里统一转 string 便于 VocabularyBuilder 去重。
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
		// JSON 数字默认反序列化为 float64
		return formatFloat(x)
	case int64:
		return itoa(x)
	case int:
		return itoa(int64(x))
	default:
		return fmtSprint(x)
	}
}

// formatFloat / itoa / fmtSprint 提取到独立函数，便于单测。
func formatFloat(f float64) string {
	// JSON 数字通常为整数（qua ID）；用 %v 保持可读性
	// 注意：超过 2^53 精度的数字（如 qua 的 19 位 ID）float64 会丢精度。
	// 为避免误差，仅取整数部分。
	if f == float64(int64(f)) {
		return itoa(int64(f))
	}
	return fmtSprint(f)
}

func itoa(n int64) string {
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

func fmtSprint(v any) string {
	// 避免直接引用 fmt，保持此文件依赖最小
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	case float64:
		return formatFloat(x)
	case int64:
		return itoa(x)
	case int:
		return itoa(int64(x))
	case bool:
		if x {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return ""
	}
}