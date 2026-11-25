package main

import "testing"

// A Monte Carlo result is JSON, so it must not go to result.png by default.
func TestDefaultOut(t *testing.T) {
	cases := map[string]string{"fractal": "result.png", "montecarlo": "result.json", "other": "result.json"}
	for workload, want := range cases {
		if got := defaultOut(workload); got != want {
			t.Fatalf("defaultOut(%q) = %q, want %q", workload, got, want)
		}
	}
}
