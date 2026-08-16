package data

import (
	"context"

	pbEnum "backend-service/api/common/enum"
	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/dept"
	"backend-service/app/platform/service/internal/data/ent/gen/role"
	"backend-service/app/platform/service/internal/data/ent/gen/user"
	"backend-service/pkg/auth/authn"
)

type dataScopeUsers struct {
	all     bool
	userIDs []uint32
}

func (r BaseRepo) resolveDataScopeUsers(ctx context.Context) (*dataScopeUsers, error) {
	actorID := authn.GetAuthUserID(ctx)
	if actorID == 0 {
		return &dataScopeUsers{all: true}, nil
	}
	actor, err := r.Data.DB(ctx).User.Query().
		Where(user.IDEQ(actorID)).
		WithRoles(func(q *gen.RoleQuery) {
			q.Where(role.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED))).
				WithDataScopeDepts(func(dq *gen.DeptQuery) {
					dq.Select(dept.FieldID)
				})
		}).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return &dataScopeUsers{userIDs: []uint32{actorID}}, nil
		}
		return nil, err
	}

	deptIDs := make(map[uint32]struct{})
	all := false
	includeDescendants := false
	for _, item := range actor.Edges.Roles {
		scope := int32(2)
		if item.DataScope != nil {
			scope = *item.DataScope
		}
		if item.IsTenantAdmin || scope == 1 {
			all = true
			break
		}
		switch scope {
		case 3:
			if actor.DeptID != nil {
				deptIDs[*actor.DeptID] = struct{}{}
			}
		case 4:
			if actor.DeptID != nil {
				deptIDs[*actor.DeptID] = struct{}{}
				includeDescendants = true
			}
		case 5:
			for _, dataDept := range item.Edges.DataScopeDepts {
				deptIDs[dataDept.ID] = struct{}{}
			}
		}
	}
	if all {
		return &dataScopeUsers{all: true}, nil
	}
	if includeDescendants && actor.DeptID != nil {
		depts, queryErr := r.Data.DB(ctx).Dept.Query().
			Select(dept.FieldID, dept.FieldAncestors).
			All(ctx)
		if queryErr != nil {
			return nil, queryErr
		}
		for _, item := range depts {
			for _, ancestor := range item.Ancestors {
				if uint32(ancestor) == *actor.DeptID {
					deptIDs[item.ID] = struct{}{}
					break
				}
			}
		}
	}
	if len(deptIDs) == 0 {
		return &dataScopeUsers{userIDs: []uint32{actorID}}, nil
	}
	ids := make([]uint32, 0, len(deptIDs))
	for id := range deptIDs {
		ids = append(ids, id)
	}
	users, err := r.Data.DB(ctx).User.Query().
		Where(user.Or(user.IDEQ(actorID), user.DeptIDIn(ids...))).
		Select(user.FieldID).
		All(ctx)
	if err != nil {
		return nil, err
	}
	userIDs := make([]uint32, 0, len(users))
	for _, item := range users {
		userIDs = append(userIDs, item.ID)
	}
	if len(userIDs) == 0 {
		userIDs = append(userIDs, actorID)
	}
	return &dataScopeUsers{userIDs: userIDs}, nil
}
