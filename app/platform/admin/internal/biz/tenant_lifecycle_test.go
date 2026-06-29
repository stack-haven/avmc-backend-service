package biz

import (
	pbCore "backend-service/api/core/service/v1"
	"testing"
)

func TestTenantLifecycleTransitionAllowed(t *testing.T) {
	pending := pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_PENDING
	active := pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE
	suspended := pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED
	expired := pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_EXPIRED
	cancelled := pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED

	tests := []struct {
		name    string
		current pbCore.TenantLifecycleStatus
		target  pbCore.TenantLifecycleStatus
		want    bool
	}{
		{name: "pending activates", current: pending, target: active, want: true},
		{name: "active suspends", current: active, target: suspended, want: true},
		{name: "suspended resumes", current: suspended, target: active, want: true},
		{name: "expired renews", current: expired, target: active, want: true},
		{name: "active cancels", current: active, target: cancelled, want: true},
		{name: "cancelled is terminal", current: cancelled, target: active, want: false},
		{name: "active cannot return pending", current: active, target: pending, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tenantLifecycleTransitionAllowed(tt.current, tt.target); got != tt.want {
				t.Fatalf("tenantLifecycleTransitionAllowed(%v, %v) = %v, want %v", tt.current, tt.target, got, tt.want)
			}
		})
	}
}
