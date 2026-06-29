package data

import (
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/menu"
	"backend-service/pkg/utils/convert"
	"context"
	"sort"

	"backend-service/api/common/enum"
)

type menuConverter interface {
	convertProto(*gen.Menu) *pbCore.Menu
}

func withMenuAncestors(ctx context.Context, client *gen.Client, menus []*gen.Menu) ([]*gen.Menu, error) {
	menuMap := make(map[uint32]*gen.Menu, len(menus))
	for _, m := range menus {
		if m != nil {
			menuMap[m.ID] = m
		}
	}
	for {
		parentIDs := make([]uint32, 0)
		seen := make(map[uint32]struct{})
		for _, m := range menuMap {
			parentID := convert.ToValue(m.ParentID)
			if parentID == 0 {
				continue
			}
			if _, ok := menuMap[parentID]; ok {
				continue
			}
			if _, ok := seen[parentID]; ok {
				continue
			}
			seen[parentID] = struct{}{}
			parentIDs = append(parentIDs, parentID)
		}
		if len(parentIDs) == 0 {
			break
		}
		parents, err := client.Menu.Query().
			Where(menu.IDIn(parentIDs...), menu.StatusEQ(int32(enum.Status_STATUS_ENABLED))).
			All(ctx)
		if err != nil {
			return nil, err
		}
		if len(parents) == 0 {
			break
		}
		for _, parent := range parents {
			menuMap[parent.ID] = parent
		}
	}

	ids := make([]uint32, 0, len(menuMap))
	for id := range menuMap {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left, right := menuMap[ids[i]], menuMap[ids[j]]
		if convert.ToValue(left.Sort) == convert.ToValue(right.Sort) {
			return left.ID < right.ID
		}
		return convert.ToValue(left.Sort) < convert.ToValue(right.Sort)
	})
	result := make([]*gen.Menu, 0, len(ids))
	for _, id := range ids {
		result = append(result, menuMap[id])
	}
	return result, nil
}

func buildMenuTree(menus []*pbCore.Menu) []*pbCore.Menu {
	nodes := make(map[uint32]*pbCore.Menu, len(menus))
	roots := make([]*pbCore.Menu, 0, len(menus))
	for _, m := range menus {
		if m == nil || m.GetType() == pbCore.MenuType_MENU_TYPE_BUTTON {
			continue
		}
		m.Children = nil
		nodes[m.GetId()] = m
	}
	for _, m := range menus {
		if m == nil || m.GetType() == pbCore.MenuType_MENU_TYPE_BUTTON {
			continue
		}
		parentID := m.GetParentId()
		if parentID == 0 {
			roots = append(roots, m)
			continue
		}
		if parent, ok := nodes[parentID]; ok {
			parent.Children = append(parent.Children, m)
			continue
		}
		roots = append(roots, m)
	}
	return roots
}
