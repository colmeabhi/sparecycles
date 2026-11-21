// Package lease decides lease lengths and finds expired leases.
//
// Owner: Tejas (resource allocation with leases).
// The base version gives leases that never expire, so lost work is never reassigned.
// The real version releases a lease after its expiry time, so a different volunteer gets the unit.
//
// A Policy keeps no state of its own. Lease expiry times live in the task table,
// so a new leader can continue after a failover.
package lease

import (
	"time"

	"github.com/colmeabhi/sparecycles/internal/scheduler"
	"github.com/colmeabhi/sparecycles/internal/tasktable"
)

// Expired names one lease that the leader must release.
type Expired struct {
	UnitID      string
	VolunteerID string
}

// Policy controls leases.
type Policy interface {
	// Duration returns the lease length for a unit given to this volunteer.
	Duration(v scheduler.Volunteer) time.Duration
	// Expired returns the leases in these units that the leader must release at time now.
	Expired(units []tasktable.Unit, now time.Time) []Expired
}

// NoExpiry gives leases of a fixed length but never releases them. It is the base stub.
type NoExpiry struct {
	Length time.Duration
}

// Duration returns the fixed length.
func (p NoExpiry) Duration(scheduler.Volunteer) time.Duration { return p.Length }

// Expired returns nothing.
func (NoExpiry) Expired([]tasktable.Unit, time.Time) []Expired { return nil }
