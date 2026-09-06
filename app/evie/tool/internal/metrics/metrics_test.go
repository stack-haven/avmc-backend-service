package metrics

import (
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCounterVec_IncAndAdd(t *testing.T) {
	c := NewCounterVec("test_counter", "help", "a", "b")
	c.Inc("v1", "v2")
	c.Inc("v1", "v2")
	c.Add(3, "v1", "v2")
	c.Inc("v2", "v3")

	var got uint64
	c.store.Range(func(k, v any) bool {
		if k.(string) == "v1\x00v2" {
			got = atomic.LoadUint64(v.(*uint64))
		}
		return true
	})
	if got != 5 {
		t.Errorf("counter v1/v2 = %d, want 5", got)
	}
}

func TestHistogramVec_Observe(t *testing.T) {
	h := NewHistogramVec("test_hist", "help", []float64{0.1, 0.5, 1.0}, "label")
	h.Observe(0.05, "a")
	h.Observe(0.3, "a")
	h.Observe(2.0, "a")
	h.Observe(0.6, "a")

	state, _ := h.store.Load("a")
	s := state.(*histState)
	if s.count != 4 {
		t.Errorf("count = %d, want 4", s.count)
	}
	if s.sum < 2.95 || s.sum > 2.96 {
		t.Errorf("sum = %f, want ~2.95", s.sum)
	}
}

func TestRegistry_Handler_Format(t *testing.T) {
	r := New()
	c := NewCounterVec("evie_demo_total", "demo", "kind")
	c.Inc("ok")
	c.Inc("error")
	r.RegisterCounter(c)

	h := NewHistogramVec("evie_demo_seconds", "demo", []float64{0.1, 1, 10}, "kind")
	h.Observe(0.2, "ok")
	h.Observe(2, "ok")
	r.RegisterHistogram(h)

	rr := httptest.NewRecorder()
	r.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/metrics", nil))

	body := rr.Body.String()
	if !strings.Contains(body, "evie_demo_total") {
		t.Errorf("missing counter name in output: %s", body)
	}
	if !strings.Contains(body, "evie_demo_seconds_bucket") {
		t.Errorf("missing histogram bucket line: %s", body)
	}
	if !strings.Contains(body, `kind="ok"`) {
		t.Errorf("missing label kind=ok: %s", body)
	}
}

func TestObserveSince(t *testing.T) {
	h := NewHistogramVec("evie_since_seconds", "h", []float64{0.1, 1}, "k")
	ObserveSince(h, time.Now().Add(-50*time.Millisecond), "x")
	_, ok := h.store.Load("x")
	if !ok {
		t.Fatal("expected state to be recorded")
	}
}
