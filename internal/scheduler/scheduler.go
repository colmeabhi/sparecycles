// Package scheduler chooses which unit a volunteer gets.
//
// Owner: Tejas (resource discovery).
// The base version gives the oldest open unit to any volunteer (FIFO).
// The real version matches units to volunteer CPU, memory and speed.
package scheduler

import "github.com/colmeabhi/sparecycles/internal/tasktable"

// Resources describes what a volunteer can run.
type Resources struct {
	CPUCores   int
	MemoryMB   int
	SpeedScore float64 // 1.0 is the reference speed
}

// Volunteer is a registered volunteer.
type Volunteer struct {
	ID        string
	Name      string
	Address   string
	Resources Resources
}

// Scheduler picks one unit for a volunteer.
type Scheduler interface {
	// Pick returns the unit that the volunteer must run, or nil if no candidate fits.
	// Candidates are open units that need a volunteer and that this volunteer does not have yet.
	Pick(candidates []tasktable.Unit, v Volunteer) *tasktable.Unit
}

// FIFO gives the oldest candidate to any volunteer. It is the base stub.
type FIFO struct{}

// Pick returns the first candidate.
func (FIFO) Pick(candidates []tasktable.Unit, _ Volunteer) *tasktable.Unit {
	if len(candidates) == 0 {
		return nil
	}
	return &candidates[0]
}
