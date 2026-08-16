package data

import (
	"context"
	"testing"
)

func TestPlatformHealthCheckerDetailsExposeAuthorizationCacheStats(t *testing.T) {
	data := &Data{}
	data.authorizationStats.hits.Add(3)
	data.authorizationStats.misses.Add(1)
	data.authorizationStats.sets.Add(4)
	data.authorizationStats.bypasses.Add(2)
	data.authorizationStats.expired.Add(1)
	data.authorizationStats.clears.Add(1)
	data.authorizationStats.invalidations.Add(1)
	data.resourceQuotaStats.consumes.Add(4)
	data.resourceQuotaStats.releases.Add(1)
	data.resourceQuotaStats.quotaExceeded.Add(2)
	data.resourceQuotaStats.idempotencyConflicts.Add(1)

	checker := &platformHealthChecker{data: data}
	details := checker.Details(context.Background())
	payload, ok := details["authorization_cache"].(map[string]any)
	if !ok {
		t.Fatalf("authorization_cache details missing: %+v", details)
	}
	if payload["hits"] != uint64(3) ||
		payload["misses"] != uint64(1) ||
		payload["sets"] != uint64(4) ||
		payload["bypasses"] != uint64(2) ||
		payload["expired"] != uint64(1) ||
		payload["clears"] != uint64(1) ||
		payload["invalidations"] != uint64(1) {
		t.Fatalf("authorization cache counters = %+v", payload)
	}
	if payload["hit_rate"] != 0.6 {
		t.Fatalf("hit_rate = %v, want 0.6", payload["hit_rate"])
	}
	if payload["bypass_rate"] != float64(2)/float64(7) {
		t.Fatalf("bypass_rate = %v, want %v", payload["bypass_rate"], float64(2)/float64(7))
	}
	quotaPayload, ok := details["resource_quota"].(map[string]any)
	if !ok {
		t.Fatalf("resource_quota details missing: %+v", details)
	}
	if quotaPayload["consumes"] != uint64(4) ||
		quotaPayload["releases"] != uint64(1) ||
		quotaPayload["quota_exceeded"] != uint64(2) ||
		quotaPayload["idempotency_conflicts"] != uint64(1) ||
		quotaPayload["mutations"] != uint64(5) {
		t.Fatalf("resource quota counters = %+v", quotaPayload)
	}
	if quotaPayload["exceeded_rate"] != 0.5 {
		t.Fatalf("exceeded_rate = %v, want 0.5", quotaPayload["exceeded_rate"])
	}
	if quotaPayload["conflict_rate"] != 0.2 {
		t.Fatalf("conflict_rate = %v, want 0.2", quotaPayload["conflict_rate"])
	}
}
