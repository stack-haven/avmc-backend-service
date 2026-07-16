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
}
