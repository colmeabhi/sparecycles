package coordinator

import (
	"time"

	"github.com/colmeabhi/sparecycles/internal/tasktable"
)

// lateCopies remembers the copies that were still running when their unit was
// accepted. Voting accepts a unit as soon as a quorum agrees, so the last copy
// often comes back after that. Its result is no longer needed, but it is still
// checked against the accepted output, as BOINC does, so that a faulty volunteer
// that is always last is still caught.
//
// Only volunteers that held a lease at the moment of the accept are listed, and
// only until that lease would have expired. Session secrets stop another
// volunteer from sending a wrong result under their ID to make them look faulty.
//
// It is leader-local: after a failover, late copies are not checked. It is not
// safe for concurrent use; SubmitResult holds decideMu while it uses it.
type lateCopies map[string]map[string]time.Time // unit ID → volunteer ID → lease expiry

// add records the running copies of a unit that was just accepted, and forgets
// copies whose lease has ended, so that the map does not grow.
func (l lateCopies) add(unitID string, leases map[string]tasktable.Assignment, now time.Time) {
	l.prune(now)
	if len(leases) == 0 {
		return
	}
	copies := make(map[string]time.Time, len(leases))
	for volunteerID, a := range leases {
		copies[volunteerID] = a.LeaseExpiry
	}
	l[unitID] = copies
}

// take reports whether the volunteer had a running copy of the unit whose lease
// has not ended, and forgets it, so that each copy is checked once.
func (l lateCopies) take(unitID, volunteerID string, now time.Time) bool {
	expiry, ok := l[unitID][volunteerID]
	if !ok {
		return false
	}
	delete(l[unitID], volunteerID)
	if len(l[unitID]) == 0 {
		delete(l, unitID)
	}
	return !now.After(expiry)
}

func (l lateCopies) prune(now time.Time) {
	for unitID, copies := range l {
		for volunteerID, expiry := range copies {
			if now.After(expiry) {
				delete(copies, volunteerID)
			}
		}
		if len(copies) == 0 {
			delete(l, unitID)
		}
	}
}

// checkLateResult counts a late result as wrong if it differs from the accepted
// output. It ignores results from volunteers that had no running copy.
// The caller holds decideMu.
func (s *Server) checkLateResult(unitID, volunteerID string, output []byte) {
	if !s.lateCopies.take(unitID, volunteerID, s.d.Now()) {
		return
	}
	u, ok := s.d.Table.Unit(unitID)
	if !ok || u.State != tasktable.UnitDone {
		return
	}
	correct := tasktable.Hash(output) == tasktable.Hash(u.Output)
	s.recordTrust(volunteerID, correct)
	if correct {
		s.d.Log.Debug("late result agrees", "unit", unitID, "volunteer", volunteerID)
		return
	}
	s.d.Metrics.WrongResults.Inc()
	s.d.Log.Warn("wrong result", "unit", unitID, "volunteer", volunteerID, "late", true)
}
