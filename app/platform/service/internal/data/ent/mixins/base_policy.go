//go:build !ent_bootstrap
// +build !ent_bootstrap

package mixins

import (
	"entgo.io/ent"
)

func (BaseMixin) Policy() ent.Policy {
	return TenantID{}.Policy()
}
