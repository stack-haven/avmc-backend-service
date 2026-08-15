package data

import (
	"testing"

	"backend-service/pkg/auth/authz"
)

func TestActionsForAuthCodeStatusUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		code     string
		want     []authz.Action
	}{
		{
			name: "status-update 自定义动作返回 POST",
			code: "/platform.admin.v1.UserService/UpdateUserByStatus",
			want: []authz.Action{"POST"},
		},
		{
			name: "普通 update 返回 PUT",
			code: "/platform.admin.v1.UserService/UpdateUser",
			want: []authz.Action{"PUT"},
		},
		{
			name: "菜单权限组状态（UpdateXxxStatus 后缀）返回 POST",
			code: "/platform.admin.v1.TenantMenuPermissionGroupService/UpdateTenantMenuPermissionGroupStatus",
			want: []authz.Action{"POST"},
		},
		{
			name: "菜单状态更新返回 POST",
			code: "/platform.admin.v1.MenuService/UpdateMenuByStatus",
			want: []authz.Action{"POST"},
		},
		{
			name: "普通项目更新返回 PUT",
			code: "/platform.admin.v1.ProjectService/UpdateProject",
			want: []authz.Action{"PUT"},
		},
		{
			name: "list 前缀返回 GET",
			code: "/platform.admin.v1.UserService/ListUsers",
			want: []authz.Action{"GET"},
		},
		{
			name: "delete 前缀返回 DELETE",
			code: "/platform.admin.v1.UserService/DeleteUser",
			want: []authz.Action{"DELETE"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := actionsForAuthCode(tt.code)
			if len(got) != len(tt.want) {
				t.Fatalf("actionsForAuthCode(%q) = %v, want %v", tt.code, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("actionsForAuthCode(%q) = %v, want %v", tt.code, got, tt.want)
				}
			}
		})
	}
}
