package schema

import (
	pkgMixin "backend-service/pkg/entgo/mixin"

	"entgo.io/ent/schema/mixin"

	"entgo.io/ent"
)

var _ ent.Mixin = (*MixinTop)(nil)

type MixinTop struct{ mixin.Schema }

func (MixinTop) Fields() []ent.Field {
	var fields []ent.Field
	fields = append(fields, pkgMixin.ID{}.Fields()...)
	fields = append(fields, pkgMixin.TimeAt{}.Fields()...)
	fields = append(fields, pkgMixin.Status{}.Fields()...)
	fields = append(fields, pkgMixin.DomainID{}.Fields()...)
	return fields
}
