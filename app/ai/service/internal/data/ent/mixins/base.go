package mixins

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/mixin"

	pkgMixin "backend-service/pkg/entgo/mixin"
)

var _ ent.Mixin = (*BaseMixin)(nil)

// BaseMixin 是 ai 服务实体的公共字段集合：ID + 审计时间 + 状态 + 租户。
//
// 它同时转发租户隐私策略，因此任何嵌入 BaseMixin 的实体自动具备租户隔离；
// 反过来说，不嵌入就没有隔离，选择必须显式。
type BaseMixin struct{ mixin.Schema }

func (BaseMixin) Fields() []ent.Field {
	var fields []ent.Field
	fields = append(fields, pkgMixin.ID{}.Fields()...)
	fields = append(fields, pkgMixin.CreatedAt{}.Fields()...)
	fields = append(fields, pkgMixin.UpdatedAt{}.Fields()...)
	fields = append(fields, pkgMixin.Status{}.Fields()...)
	fields = append(fields, TenantID{}.Fields()...)
	return fields
}

func (BaseMixin) Indexes() []ent.Index {
	return TenantID{}.Indexes()
}
