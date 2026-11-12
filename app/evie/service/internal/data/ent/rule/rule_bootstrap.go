//go:build ent_bootstrap
// +build ent_bootstrap

// 本文件是 rule 包的「引导期替身」，仅在 internal/data/ent/gen 尚不存在时参与编译。
//
// 原因：schema 包在个别实体上直接引用了 rule（例如字典项的外键租户校验钩子），
// 而 rule 的真实实现依赖生成物（*gen.XxxMutation 等）。此时 schema 无法编译，
// 生成也无法启动，形成先有鸡还是先有蛋。
//
// pkg/entgo/entc 在检测到目标目录缺失时，会用 -tags ent_bootstrap 先加载一次
// schema 完成引导生成，然后立刻用真实标签重新生成。最终产物不包含本文件的行为。
package rule

import (
	"context"

	"entgo.io/ent"
	"entgo.io/ent/privacy"
)

type bootstrapRule struct{}

func (bootstrapRule) EvalQuery(context.Context, ent.Query) error { return privacy.Skip }

func (bootstrapRule) EvalMutation(context.Context, ent.Mutation) error { return privacy.Skip }

// FilterTenantRule 引导期不施加隔离（真实实现在 !ent_bootstrap 文件中）。
func FilterTenantRule() privacy.QueryMutationRule { return bootstrapRule{} }
