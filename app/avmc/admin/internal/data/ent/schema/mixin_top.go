package schema

import (
	// "backend-service/app/avmc/admin/internal/data/mixins"

	pkgMixin "backend-service/pkg/entgo/mixin"

	"entgo.io/ent/schema/mixin"

	"entgo.io/ent"
)

var _ ent.Mixin = (*MixinTop)(nil)

type MixinTop struct{ mixin.Schema }

func (MixinTop) Fields() []ent.Field {
	var fields []ent.Field
	fields = append(fields, pkgMixin.ID{}.Fields()...)
	// fields = append(fields, pkgMixin.TimeAt{}.Fields()...)
	fields = append(fields, pkgMixin.CreatedAt{}.Fields()...)
	fields = append(fields, pkgMixin.UpdatedAt{}.Fields()...)
	fields = append(fields, SoftDeleteMixin{}.Fields()...)
	fields = append(fields, pkgMixin.Status{}.Fields()...)
	fields = append(fields, pkgMixin.DomainID{}.Fields()...)
	return fields
}
