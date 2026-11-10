package mixins

import (
	pkgMixin "backend-service/pkg/entgo/mixin"

	"entgo.io/ent"
	"entgo.io/ent/schema/mixin"
)

var _ ent.Mixin = (*BaseMixin)(nil)

// BaseMixin 组合 EDAI 所有实体共用的字段：ID / 时间戳 / 状态 / tenant_id。
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

func (BaseMixin) Policy() ent.Policy {
	return TenantID{}.Policy()
}

var _ ent.Mixin = (*BaseMixinNoStatus)(nil)

// BaseMixinNoStatus 与 BaseMixin 相同，但**不含通用 status 字段**。
//
// 用于自带业务状态机的实体（如 ScanTask 用字符串状态
// pending/running/succeeded）：通用 Status 是 int32，两者会冲突。
type BaseMixinNoStatus struct{ mixin.Schema }

func (BaseMixinNoStatus) Fields() []ent.Field {
	var fields []ent.Field
	fields = append(fields, pkgMixin.ID{}.Fields()...)
	fields = append(fields, pkgMixin.CreatedAt{}.Fields()...)
	fields = append(fields, pkgMixin.UpdatedAt{}.Fields()...)
	fields = append(fields, TenantID{}.Fields()...)
	return fields
}

func (BaseMixinNoStatus) Indexes() []ent.Index {
	return TenantID{}.Indexes()
}

func (BaseMixinNoStatus) Policy() ent.Policy {
	return TenantID{}.Policy()
}
