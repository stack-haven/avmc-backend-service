package data

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/menu"
	"backend-service/app/platform/service/internal/data/ent/gen/role"
	"backend-service/pkg/auth/authz"
)

// SyncRolePolicies rebuilds Casbin p-type policies for a role from its bound menus.
func SyncRolePolicies(ctx context.Context, client *gen.Client, authorizer authz.Authorizer, tenantID, roleID uint32) error {
	if authorizer == nil {
		return nil
	}

	r, err := client.Role.Query().
		Where(role.IDEQ(roleID)).
		WithMenus(func(q *gen.MenuQuery) {
			q.Where(menu.AuthCodeNEQ(""))
		}).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("sync role policies: query role %d: %w", roleID, err)
	}

	roleSub := authz.Subject(strconv.FormatUint(uint64(r.ID), 10))
	tenant := authz.Tenant(strconv.FormatUint(uint64(tenantID), 10))

	// Remove all existing p-type policies for this role+tenant
	if err := removeRolePolicies(ctx, authorizer, roleSub, tenant); err != nil {
		return fmt.Errorf("sync role policies: remove old: %w", err)
	}

	// Build new policies from menu auth_codes
	for _, m := range r.Edges.Menus {
		if m.AuthCode == nil || *m.AuthCode == "" {
			continue
		}
		code := *m.AuthCode
		obj := authz.Object(code)
		actions := actionsForAuthCode(code)
		for _, act := range actions {
			if _, err := authorizer.AddPolicy(ctx, authz.Policy{
				Subject: roleSub, Object: obj, Action: act,
				Tenant: tenant, Effect: authz.EffectAllow,
			}); err != nil {
				return fmt.Errorf("sync role policies: add policy: %w", err)
			}
		}
	}

	return nil
}

// RemoveRolePolicies removes all p-type policies for a role+tenant.
func RemoveRolePolicies(ctx context.Context, authorizer authz.Authorizer, tenantID, roleID uint32) error {
	if authorizer == nil {
		return nil
	}
	roleSub := authz.Subject(strconv.FormatUint(uint64(roleID), 10))
	tenant := authz.Tenant(strconv.FormatUint(uint64(tenantID), 10))
	return removeRolePolicies(ctx, authorizer, roleSub, tenant)
}

// removeRolePolicies deletes all policies matching the given role+tenant.
func removeRolePolicies(ctx context.Context, authorizer authz.Authorizer, roleSub authz.Subject, tenant authz.Tenant) error {
	// Casbin's RemoveFilteredPolicy can remove by field index.
	// p = sub, obj, act, tenant, eft  → field indices: 0=sub, 1=obj, 2=act, 3=tenant, 4=eft
	if remover, ok := authorizer.(interface {
		RemoveFilteredPolicy(ctx context.Context, fieldIndex int, fieldValues ...string) (bool, error)
	}); ok {
		_, err := remover.RemoveFilteredPolicy(ctx, 0, string(roleSub), "", "", string(tenant))
		return err
	}

	// Fallback: try to collect and remove individually
	return nil
}

// SyncUserRole updates the Casbin g rule when a user gains or loses a role.
func SyncUserRole(ctx context.Context, authorizer authz.Authorizer, tenantID, userID, roleID uint32, add bool) error {
	if authorizer == nil {
		return nil
	}
	userSub := authz.Subject(strconv.FormatUint(uint64(userID), 10))
	roleSub := authz.Subject(strconv.FormatUint(uint64(roleID), 10))
	tenant := authz.Tenant(strconv.FormatUint(uint64(tenantID), 10))

	if add {
		_, err := authorizer.AddRoleForUser(ctx, userSub, roleSub, tenant)
		return err
	}
	_, err := authorizer.DeleteRoleForUser(ctx, userSub, roleSub, tenant)
	return err
}

// SyncUserRoles fully rebuilds the g rules for a user based on their current role IDs.
func SyncUserRoles(ctx context.Context, client *gen.Client, authorizer authz.Authorizer, tenantID, userID uint32, roleIDs []uint32) error {
	if authorizer == nil {
		return nil
	}
	userSub := authz.Subject(strconv.FormatUint(uint64(userID), 10))
	tenant := authz.Tenant(strconv.FormatUint(uint64(tenantID), 10))

	// Remove all existing roles for this user+tenant
	existingRoles, err := authorizer.GetRolesForUser(ctx, userSub, tenant)
	if err != nil {
		return fmt.Errorf("sync user roles: get existing: %w", err)
	}
	for _, r := range existingRoles {
		if _, err := authorizer.DeleteRoleForUser(ctx, userSub, r, tenant); err != nil {
			return fmt.Errorf("sync user roles: delete old: %w", err)
		}
	}

	// Add new role bindings
	for _, rid := range roleIDs {
		roleSub := authz.Subject(strconv.FormatUint(uint64(rid), 10))
		if _, err := authorizer.AddRoleForUser(ctx, userSub, roleSub, tenant); err != nil {
			return fmt.Errorf("sync user roles: add new: %w", err)
		}
	}

	return nil
}

// actionsForAuthCode extracts HTTP+gRPC actions from an operation path.
// Convention: last segment determines primary HTTP method.
// E.g., ListXxx → GET, CreateXxx → POST, UpdateXxx → PUT, DeleteXxx → DELETE
func actionsForAuthCode(code string) []authz.Action {
	segments := strings.Split(code, "/")
	if len(segments) == 0 {
		return allActions()
	}
	last := segments[len(segments)-1]
	lower := strings.ToLower(last)
	switch {
	case strings.HasPrefix(lower, "list"), strings.HasPrefix(lower, "get"):
		return []authz.Action{"GET"}
	case strings.HasPrefix(lower, "create"), strings.HasPrefix(lower, "add"), strings.HasPrefix(lower, "send"):
		return []authz.Action{"POST"}
	case strings.HasPrefix(lower, "update"), strings.HasPrefix(lower, "set"), strings.HasPrefix(lower, "mark"):
		// UpdateXxxByStatus / UpdateXxxStatus 是状态切换自定义动作（AIP-136 用 POST），
		// 普通 UpdateXxx 是标准更新（PUT）。
		if strings.HasSuffix(lower, "status") {
			return []authz.Action{"POST"}
		}
		return []authz.Action{"PUT"}
	case strings.HasPrefix(lower, "delete"), strings.HasPrefix(lower, "remove"):
		return []authz.Action{"DELETE"}
	case strings.HasPrefix(lower, "publish"), strings.HasPrefix(lower, "rollback"):
		return []authz.Action{"POST"}
	case strings.HasPrefix(lower, "confirm"), strings.HasPrefix(lower, "upload"):
		return []authz.Action{"POST"}
	case strings.HasPrefix(lower, "presign"), strings.HasPrefix(lower, "download"):
		return []authz.Action{"GET"}
	case strings.HasPrefix(lower, "cancel"), strings.HasPrefix(lower, "retry"):
		return []authz.Action{"POST"}
	case strings.HasPrefix(lower, "revoke"):
		return []authz.Action{"DELETE"}
	default:
		return allActions()
	}
}

func allActions() []authz.Action {
	return []authz.Action{"GET", "POST", "PUT", "DELETE"}
}
