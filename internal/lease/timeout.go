package lease

import (
	"cmp"
	"slices"
	"time"

	"github.com/colmeabhi/sparecycles/internal/scheduler"
	"github.com/colmeabhi/sparecycles/internal/tasktable"
)

// Timeout gives every volunteer a lease of the same length and releases a lease
// once its expiry time has passed. A volunteer that crashes, hangs or leaves
// without a word then loses its unit, and the next volunteer that asks gets it.
//
// Length must be positive: with 0, every lease expires on the next tick and no
// unit can finish. The coordinator reads it from SC_LEASE_DURATION, which config checks.
type Timeout struct {
	Length time.Duration
}

// Duration returns the fixed length.
func (p Timeout) Duration(scheduler.Volunteer) time.Duration { return p.Length }

// Expired returns each lease in an open unit whose expiry is before now.
//
// A lease is still valid at the exact expiry time, and expires right after it.
// The result is sorted by unit ID, then volunteer ID, so that logs and tests
// are the same on every run (map order in Go is random).
func (Timeout) Expired(units []tasktable.Unit, now time.Time) []Expired {
	var out []Expired
	for _, u := range units {
		if u.State != tasktable.UnitOpen {
			continue
		}
		for volunteerID, a := range u.Assignments {
			if a.LeaseExpiry.Before(now) {
				out = append(out, Expired{UnitID: u.ID, VolunteerID: volunteerID})
			}
		}
	}
	slices.SortFunc(out, compareExpired)
	return out
}

func compareExpired(a, b Expired) int {
	return cmp.Or(cmp.Compare(a.UnitID, b.UnitID), cmp.Compare(a.VolunteerID, b.VolunteerID))
}
