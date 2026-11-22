package workload

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"
)

// runAll splits params and runs every unit in this process.
func runAll(t *testing.T, params string) [][]byte {
	t.Helper()
	units, err := MonteCarlo{}.Split([]byte(params))
	if err != nil {
		t.Fatal(err)
	}
	outputs := make([][]byte, len(units))
	for i, u := range units {
		if outputs[i], err = (MonteCarlo{}).Run(context.Background(), u); err != nil {
			t.Fatal(err)
		}
	}
	return outputs
}

func mergeResult(t *testing.T, params string, outputs [][]byte) MonteCarloResult {
	t.Helper()
	raw, err := MonteCarlo{}.Merge([]byte(params), outputs)
	if err != nil {
		t.Fatal(err)
	}
	var r MonteCarloResult
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("result %s is not JSON: %v", raw, err)
	}
	return r
}

func TestMonteCarloSplit(t *testing.T) {
	units, err := MonteCarlo{}.Split([]byte(`{"units":5,"samples_per_unit":10,"seed":100}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 5 {
		t.Fatalf("units = %d, want 5", len(units))
	}
	for i, raw := range units {
		var u monteCarloUnit
		if err := json.Unmarshal(raw, &u); err != nil {
			t.Fatal(err)
		}
		if u.Seed != uint64(100+i) || u.Samples != 10 {
			t.Fatalf("unit %d = %+v, want seed %d and 10 samples", i, u, 100+i)
		}
	}
}

func TestMonteCarloDefaults(t *testing.T) {
	for _, params := range []string{``, `{}`, `{"units":0,"samples_per_unit":0,"seed":0}`} {
		p, err := parseMonteCarloParams([]byte(params))
		if err != nil {
			t.Fatal(err)
		}
		if p != (MonteCarloParams{Units: 16, SamplesPerUnit: 50_000_000, Seed: 1}) {
			t.Fatalf("params %q gave %+v, want the defaults", params, p)
		}
	}
}

// Voting compares hashes, so the same unit must give the same bytes every time.
func TestMonteCarloRunIsDeterministic(t *testing.T) {
	a := runAll(t, `{"units":2,"samples_per_unit":100000,"seed":9}`)
	b := runAll(t, `{"units":2,"samples_per_unit":100000,"seed":9}`)
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			t.Fatalf("unit %d: two runs differ: %x and %x", i, a[i], b[i])
		}
	}
	if bytes.Equal(a[0], a[1]) {
		t.Fatal("units 0 and 1 gave the same output, want different seeds to give different points")
	}
}

// The exact counts for a fixed seed. They are the same on arm64 and amd64,
// because Run uses only integer math. CI runs on amd64 and laptops on arm64,
// so this test fails if a change makes the output depend on the CPU, or if a
// new Go version changes the PCG numbers.
func TestMonteCarloGoldenCounts(t *testing.T) {
	want := []uint64{786162, 785489, 785008}
	for i, out := range runAll(t, `{"units":3,"samples_per_unit":1000000,"seed":42}`) {
		inside, total, err := decodeMonteCarloOutput(out)
		if err != nil {
			t.Fatal(err)
		}
		if inside != want[i] || total != 1_000_000 {
			t.Fatalf("unit %d: inside %d of %d, want %d of 1000000", i, inside, total, want[i])
		}
	}
}

func TestMonteCarloEstimatesPi(t *testing.T) {
	params := `{"units":10,"samples_per_unit":1000000}` // 10 million samples
	r := mergeResult(t, params, runAll(t, params))
	if r.Total != 10_000_000 || r.Units != 10 {
		t.Fatalf("result = %+v, want 10 units and 10000000 samples", r)
	}
	if math.Abs(r.Estimate-math.Pi) > 0.01 {
		t.Fatalf("estimate = %v, want within 0.01 of pi", r.Estimate)
	}
	if want := 4 * float64(r.Inside) / float64(r.Total); r.Estimate != want {
		t.Fatalf("estimate = %v, want 4 × inside / total = %v", r.Estimate, want)
	}
}

func TestMonteCarloBadParams(t *testing.T) {
	for _, p := range []string{
		`not json`,
		`{"units":-1}`,
		`{"units":10001}`,
		`{"samples_per_unit":-5}`,
		`{"samples_per_unit":20000000001}`,
		`{"seed":-1}`,
		`{"sample_per_unit":1000}`, // typo: unknown field
		`{"units":2} {"units":3}`,  // data after the object
	} {
		if _, err := (MonteCarlo{}).Split([]byte(p)); err == nil {
			t.Fatalf("params %s: expected an error", p)
		}
	}
}

func TestMonteCarloBadPayload(t *testing.T) {
	for _, p := range []string{`not json`, `{"seed":1,"samples":0}`, `{"seed":1,"samples":-3}`} {
		if _, err := (MonteCarlo{}).Run(context.Background(), []byte(p)); err == nil {
			t.Fatalf("payload %s: expected an error", p)
		}
	}
}

func TestMonteCarloBadOutputs(t *testing.T) {
	params := `{"units":2,"samples_per_unit":1000}`
	good := runAll(t, params)
	output := func(inside, total uint64) []byte {
		b := make([]byte, 16)
		binary.BigEndian.PutUint64(b[0:8], inside)
		binary.BigEndian.PutUint64(b[8:16], total)
		return b
	}
	cases := map[string][][]byte{
		"too few outputs":        good[:1],
		"short output":           {good[0], good[1][:8]},
		"inside more than total": {good[0], output(2000, 1000)},
		"wrong sample count":     {good[0], output(10, 999)},
	}
	for name, outputs := range cases {
		if _, err := (MonteCarlo{}).Merge([]byte(params), outputs); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
}

// The volunteer cancels the context when its lease ends, and Run must stop
// soon, also in the middle of the loop.
func TestMonteCarloStopsWhenCancelled(t *testing.T) {
	units, err := MonteCarlo{}.Split([]byte(`{"units":1,"samples_per_unit":20000000000}`)) // about a minute of work
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (MonteCarlo{}).Run(ctx, units[0]); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run with a cancelled context returned %v, want context.Canceled", err)
	}

	// A deadline that passes while the loop runs, as when a lease ends.
	ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := MonteCarlo{}.Run(ctx, units[0])
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Run returned %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop within 5 s after its deadline")
	}
}

func TestDefaultRegistryHasMonteCarlo(t *testing.T) {
	w, err := Default().Get("montecarlo")
	if err != nil || w.Name() != "montecarlo" {
		t.Fatalf("Get(montecarlo) = %v, %v", w, err)
	}
}
