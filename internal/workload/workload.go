// Package workload defines the jobs that SpareCycles can run.
//
// A workload splits a job into independent units, runs one unit, and merges the
// unit outputs into the final result. Run must be deterministic: the same payload
// must give the same bytes on every volunteer, so that results can be compared.
package workload

import (
	"context"
	"fmt"
	"sort"
)

// Workload is one type of job.
type Workload interface {
	// Name is the name that clients use, for example "fractal".
	Name() string
	// Split cuts a job into unit payloads. params is JSON.
	Split(params []byte) ([][]byte, error)
	// Run computes one unit.
	Run(ctx context.Context, payload []byte) ([]byte, error)
	// Merge joins the unit outputs, given in unit order, into the final result.
	Merge(params []byte, outputs [][]byte) ([]byte, error)
}

// Registry maps workload names to workloads.
type Registry struct {
	byName map[string]Workload
}

// NewRegistry returns a registry with the given workloads.
func NewRegistry(ws ...Workload) *Registry {
	r := &Registry{byName: map[string]Workload{}}
	for _, w := range ws {
		r.byName[w.Name()] = w
	}
	return r
}

// Default returns a registry with all built-in workloads.
func Default() *Registry {
	return NewRegistry(Fractal{}, MonteCarlo{})
}

// Get returns the workload with this name.
func (r *Registry) Get(name string) (Workload, error) {
	w, ok := r.byName[name]
	if !ok {
		return nil, fmt.Errorf("unknown workload %q (known: %v)", name, r.Names())
	}
	return w, nil
}

// Names returns the registered names, sorted.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.byName))
	for n := range r.byName {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
