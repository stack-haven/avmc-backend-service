//go:build !ent_bootstrap
// +build !ent_bootstrap

package mixins

import (
	"entgo.io/ent"

	"backend-service/app/platform/service/internal/data/ent/rule"
)

func (TenantID) Policy() ent.Policy {
	return rule.FilterTenantRule()
}
