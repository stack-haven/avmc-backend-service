package entc

import "entgo.io/ent/schema"

// GlobalScope 显式声明一个实体**不受租户隔离约束**。
//
// 存在的唯一理由是消解"有 tenant_id 字段却没有隐私策略"这种不一致：生成期会
// 强制要求两者同时存在或同时缺席，因此确实需要保留列但暂不隔离的实体必须在这里
// 留下可审查的声明，而不是默默跳过。
//
// 使用约束：
//   - 只能标注在**没有**隐私策略的实体上（声明与策略并存会自相矛盾）；
//   - 标注之后该实体不会进入生成的租户分派表，查询不会附带任何租户谓词，
//     即访问控制完全依赖上层（例如平台操作面鉴权）；
//   - 一旦实体接入隐私体系，必须删除本注解。
//
// 典型场景：平台级全局表（菜单、套餐模板），以及暂缓多租户化的内部表。
type GlobalScope struct{}

var (
	_ schema.Annotation = GlobalScope{}
	_ schema.Annotation = (*GlobalScope)(nil)
)

// Name 返回注解序列化时使用的键名（schema 图里据此识别）。
// 使用值接收者，便于在 schema 中以 entgo.GlobalScope{} 字面量书写。
func (GlobalScope) Name() string {
	return "GlobalScope"
}
