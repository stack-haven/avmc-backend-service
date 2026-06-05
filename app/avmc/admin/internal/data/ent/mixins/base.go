package mixins

import (
	pkgMixin "backend-service/pkg/entgo/mixin"

	"entgo.io/ent"
	"entgo.io/ent/schema/mixin"
)

var _ ent.Mixin = (*BaseMixin)(nil)

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
