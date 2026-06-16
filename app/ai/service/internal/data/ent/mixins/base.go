package mixins

import (
	pkgMixin "backend-service/pkg/entgo/mixin"

	"entgo.io/ent/schema/mixin"

	"entgo.io/ent"
)

var _ ent.Mixin = (*BaseMixin)(nil)

type BaseMixin struct{ mixin.Schema }

func (BaseMixin) Fields() []ent.Field {
	var fields []ent.Field
	fields = append(fields, pkgMixin.ID{}.Fields()...)
	fields = append(fields, pkgMixin.CreatedAt{}.Fields()...)
	fields = append(fields, pkgMixin.UpdatedAt{}.Fields()...)
	fields = append(fields, pkgMixin.Status{}.Fields()...)
	fields = append(fields, pkgMixin.DomainID{}.Fields()...)
	return fields
}
