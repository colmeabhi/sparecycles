package main

import (
	"testing"
	"time"
)

func secs(xs ...int) []time.Duration {
	out := make([]time.Duration, len(xs))
	for i, x := range xs {
		out[i] = time.Duration(x) * time.Second
	}
	return out
}

func TestPercentileNearestRank(t *testing.T) {
	d := secs(5, 1, 4, 2, 3, 10, 9, 8, 7, 6) // 1..10, not sorted
	cases := map[float64]int{0.5: 5, 0.9: 9, 0.95: 10, 1.0: 10, 0.0: 1}
	for p, want := range cases {
		if got := percentile(d, p); got != time.Duration(want)*time.Second {
			t.Errorf("p%.0f = %v, want %ds", p*100, got, want)
		}
	}
	if d[0] != 5*time.Second {
		t.Fatal("percentile must not reorder its input")
	}
}

func TestSummarize(t *testing.T) {
	s := summarize(secs(2, 4, 6), 1, 120, 30*time.Second)
	if s.Jobs != 4 || s.Failed != 1 || s.Units != 120 {
		t.Fatalf("counts: %+v", s)
	}
	if s.Mean != 4*time.Second || s.P95 != 6*time.Second {
		t.Fatalf("mean=%v p95=%v, want 4s and 6s", s.Mean, s.P95)
	}
	if s.JobsPerMin != 6 || s.UnitsPerS != 4 { // 3 jobs in 0.5 min; 120 units in 30 s
		t.Fatalf("rates: %.2f jobs/min, %.2f units/s", s.JobsPerMin, s.UnitsPerS)
	}
}

func TestSummarizeAllFailed(t *testing.T) {
	s := summarize(nil, 2, 0, time.Second)
	if s.Jobs != 2 || s.Mean != 0 || s.JobsPerMin != 0 {
		t.Fatalf("unexpected summary: %+v", s)
	}
}
