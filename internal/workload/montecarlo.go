package workload

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand/v2"
)

// MonteCarlo estimates pi by sampling random points in a square and counting
// the share that falls inside a quarter circle. Each unit draws its own points
// from its own seed, so units are independent.
type MonteCarlo struct{}

// MonteCarloParams are the job parameters. Zero values take the defaults.
type MonteCarloParams struct {
	Units          int    `json:"units"`            // default 16
	SamplesPerUnit int64  `json:"samples_per_unit"` // default 50,000,000
	Seed           uint64 `json:"seed"`             // default 1 (0 also means 1); unit i uses Seed + i
}

// MonteCarloResult is the merged result, returned as JSON.
type MonteCarloResult struct {
	Estimate float64 `json:"estimate"` // 4 × inside / total
	Inside   uint64  `json:"inside"`
	Total    uint64  `json:"total"`
	Units    int     `json:"units"`
}

type monteCarloUnit struct {
	Seed    uint64 `json:"seed"`
	Samples int64  `json:"samples"`
}

const (
	maxMonteCarloUnits = 10_000
	// maxSamplesPerUnit is about 60 seconds on a laptop (3 ns per sample). The
	// design is 60-second units with 3-minute leases: a unit that takes longer
	// than its lease is given out again and again, and never finishes.
	maxSamplesPerUnit    = 20_000_000_000
	monteCarloOutputSize = 16 // inside and total, 2 big-endian uint64

	// pcgStream is the second PCG seed, the same for every unit. Only the first
	// seed changes per unit, so each unit gets its own stream of numbers.
	pcgStream = 0x5ca1ab1e_c0ffee00
	// checkEvery is how many samples run between two checks of the context.
	checkEvery = 1 << 20
	// The point (x, y) has 31-bit integer coordinates, so the square has side
	// 2^31 and the quarter circle has radius 2^31. x² + y² < 2^62 fits in a uint64.
	coordinateBits = 31
	radiusSquared  = uint64(1) << (2 * coordinateBits)
)

// Name returns "montecarlo".
func (MonteCarlo) Name() string { return "montecarlo" }

func parseMonteCarloParams(raw []byte) (MonteCarloParams, error) {
	var p MonteCarloParams
	if len(bytes.TrimSpace(raw)) > 0 {
		// Unknown fields are an error, so that a typo such as "sample_per_unit"
		// does not silently run the default size.
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&p); err != nil {
			return p, fmt.Errorf("montecarlo params: %w", err)
		}
		if dec.More() {
			return p, fmt.Errorf("montecarlo params: unexpected data after the JSON object")
		}
	}
	if p.Units == 0 {
		p.Units = 16
	}
	if p.SamplesPerUnit == 0 {
		p.SamplesPerUnit = 50_000_000
	}
	if p.Seed == 0 {
		p.Seed = 1
	}
	if p.Units < 1 || p.Units > maxMonteCarloUnits {
		return p, fmt.Errorf("montecarlo params: units must be from 1 to %d, got %d", maxMonteCarloUnits, p.Units)
	}
	if p.SamplesPerUnit < 1 || p.SamplesPerUnit > maxSamplesPerUnit {
		return p, fmt.Errorf("montecarlo params: samples_per_unit must be from 1 to %d, got %d", int64(maxSamplesPerUnit), p.SamplesPerUnit)
	}
	return p, nil
}

// Split returns one payload per unit. Unit i uses the seed Seed + i.
func (MonteCarlo) Split(params []byte) ([][]byte, error) {
	p, err := parseMonteCarloParams(params)
	if err != nil {
		return nil, err
	}
	units := make([][]byte, p.Units)
	for i := range units {
		b, err := json.Marshal(monteCarloUnit{Seed: p.Seed + uint64(i), Samples: p.SamplesPerUnit})
		if err != nil {
			return nil, err
		}
		units[i] = b
	}
	return units, nil
}

// Run counts the samples inside the quarter circle. It returns the count and
// the number of samples as 2 big-endian uint64 values.
//
// It uses only integer math, so every CPU gives the same bytes. Floating point
// could differ in the last bit between CPUs, for example when one fuses a
// multiply and an add, and then the vote would see different hashes.
func (MonteCarlo) Run(ctx context.Context, payload []byte) ([]byte, error) {
	var u monteCarloUnit
	if err := json.Unmarshal(payload, &u); err != nil {
		return nil, fmt.Errorf("montecarlo unit: %w", err)
	}
	if u.Samples < 1 || u.Samples > maxSamplesPerUnit {
		return nil, fmt.Errorf("montecarlo unit: samples must be from 1 to %d, got %d", int64(maxSamplesPerUnit), u.Samples)
	}
	inside, err := countInside(ctx, rand.NewPCG(u.Seed, pcgStream), u.Samples)
	if err != nil {
		return nil, err
	}
	out := make([]byte, monteCarloOutputSize)
	binary.BigEndian.PutUint64(out[0:8], inside)
	binary.BigEndian.PutUint64(out[8:16], uint64(u.Samples))
	return out, nil
}

// countInside draws samples points and counts those with x² + y² < r².
// Each point uses one 64-bit number: x is bits 33 to 63, y is bits 2 to 32.
func countInside(ctx context.Context, src *rand.PCG, samples int64) (uint64, error) {
	const mask = uint64(1)<<coordinateBits - 1
	var inside uint64
	for i := int64(0); i < samples; i++ {
		if i%checkEvery == 0 {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
		}
		r := src.Uint64()
		x := r >> (64 - coordinateBits)
		y := (r >> 2) & mask
		if x*x+y*y < radiusSquared {
			inside++
		}
	}
	return inside, nil
}

// Merge adds the counts of all units and returns a MonteCarloResult as JSON.
// It checks that every unit ran the sample count of the job.
func (MonteCarlo) Merge(params []byte, outputs [][]byte) ([]byte, error) {
	p, err := parseMonteCarloParams(params)
	if err != nil {
		return nil, err
	}
	if len(outputs) != p.Units {
		return nil, fmt.Errorf("montecarlo merge: %d outputs, want %d", len(outputs), p.Units)
	}
	r := MonteCarloResult{Units: p.Units}
	for i, out := range outputs {
		inside, total, err := decodeMonteCarloOutput(out)
		if err != nil {
			return nil, fmt.Errorf("montecarlo merge: unit %d: %w", i, err)
		}
		if total != uint64(p.SamplesPerUnit) {
			return nil, fmt.Errorf("montecarlo merge: unit %d has %d samples, want %d", i, total, p.SamplesPerUnit)
		}
		r.Inside += inside
		r.Total += total
	}
	r.Estimate = 4 * float64(r.Inside) / float64(r.Total)
	return json.Marshal(r)
}

func decodeMonteCarloOutput(out []byte) (inside, total uint64, err error) {
	if len(out) != monteCarloOutputSize {
		return 0, 0, fmt.Errorf("output has %d bytes, want %d", len(out), monteCarloOutputSize)
	}
	inside = binary.BigEndian.Uint64(out[0:8])
	total = binary.BigEndian.Uint64(out[8:16])
	if inside > total {
		return 0, 0, fmt.Errorf("inside %d is more than total %d", inside, total)
	}
	return inside, total, nil
}
