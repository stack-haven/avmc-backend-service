// Package biz · vocab_source.go
// 通用「词汇来源」接口契约（与具体外部系统解耦）。
//
// 设计原则（用户反馈）：
//   1. 工具不复用 qua-specific 类型（QuaUser/QuaDept），避免外部 API 变化冲击核心
//   2. 任何外部系统（qua/飞书/LDAP/CSV...）通过实现 VocabularySource 接入
//   3. Adapter 返回 opaque RawEntity；字段语义由 Normalizer + 配置规则解释
//   4. VocabularyBuilder 只接收 NormalizedEntry，不知道来源
//
// 数据流：
//   HTTP fetcher（data 层，opaque map）
//     → VocabularySource.Fetch(ctx) → []RawEntity
//       → Normalizer.Normalize(raw) → *NormalizedEntry
//         → VocabularyBuilder.Add(entry)
package biz

import "context"

// VocabularySource 通用词汇来源接口。
//
// 实现方职责：
//   - 从任意外部系统拉取数据（HTTP / gRPC / SQL / 文件...）
//   - 把原始数据转成 RawEntity 列表（不解释字段语义）
//   - 实现方知道自己的 entity_type 命名空间（如 "user" / "department"）
//
// Normalizer 职责：
//   - 按配置的规则把 RawEntity 转成 NormalizedEntry
//   - 不在 adapter 内部做语义解析（保证 adapter 可复用）
type VocabularySource interface {
	// Name 来源标识，全局唯一（如 "qua"、"feishu"、"ldap"）。
	Name() string

	// Fetch 拉取全部原始实体；size 由调用方控制（全量/分页）。
	Fetch(ctx context.Context) ([]RawEntity, error)
}

// RawEntity 来自外部系统的原始实体（adapter 层产出，业务无关）。
//
// 字段说明：
//   - SourceID: 外部系统唯一标识（如 qua 的 user.id）
//   - EntityType: 实体类型（如 "user" / "department" / "product"）
//   - Source: 数据来源名（与 VocabularySource.Name() 一致）
//   - Data: 不透明 payload，由 Normalizer 按路径访问
type RawEntity struct {
	SourceID   string         `json:"source_id"`
	EntityType string         `json:"entity_type"`
	Source     string         `json:"source"`
	Data       map[string]any `json:"data"`
}

// 编译期断言：确保 VocabularySource 是接口（不能被意外用作 struct）。
var _ VocabularySource = (*nullSource)(nil)

// nullSource 仅用于编译期断言，正常代码不实例化。
type nullSource struct{}

func (nullSource) Name() string                                       { return "" }
func (nullSource) Fetch(context.Context) ([]RawEntity, error)          { return nil, nil }