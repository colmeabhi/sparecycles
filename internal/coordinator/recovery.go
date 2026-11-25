package coordinator

import (
	"context"

	"github.com/colmeabhi/sparecycles/internal/tasktable"
)

// retryStuckVotes runs the vote again for each open unit that has every copy
// back and no lease out, and finishes each running job whose units are all done.
//
// A vote runs only when a result arrives. If its commit fails (for example with
// Unavailable while the backups cannot be reached), no new result comes for that
// unit, and a retry of the last result is refused because its lease is gone.
// The same holds for a job whose last JobFinished commit failed, and for a vote
// that an old leader did not commit before a failover. Without this, such a
// unit or job would wait forever.
func (s *Server) retryStuckVotes(ctx context.Context) {
	s.decideMu.Lock()
	defer s.decideMu.Unlock()
	for _, u := range s.d.Table.OpenUnits() {
		if len(u.Assignments) > 0 || len(u.Results) < u.Copies {
			continue
		}
		if err := s.decide(ctx, u.ID); err != nil {
			s.d.Log.Warn("vote again", "unit", u.ID, "err", err)
		}
	}
	for _, jobID := range s.d.Table.RunningJobs() {
		if err := s.finishJobIfDone(ctx, jobID); err != nil {
			s.d.Log.Warn("finish job again", "job", jobID, "err", err)
		}
	}
}

// reportUnitsWithoutVolunteer finds open units whose vote has started and that
// need another volunteer, but every registered volunteer has already run them.
// Such a unit waits until a new volunteer joins, for example with 2 volunteers of
// which 1 is faulty: 2 of 3 can never agree. It logs each unit once and sets
// sc_units_without_volunteer.
func (s *Server) reportUnitsWithoutVolunteer() {
	units := s.d.Table.OpenUnits()
	s.mu.Lock()
	defer s.mu.Unlock()
	waiting := map[string]bool{}
	for _, u := range units {
		if len(u.Results) == 0 || !u.NeedsVolunteer() || s.someVolunteerCanTake(u) {
			continue
		}
		waiting[u.ID] = true
		if !s.waiting[u.ID] {
			s.d.Log.Warn("unit stuck: every volunteer already ran it", "unit", u.ID,
				"copies", u.Copies, "results", len(u.Results), "volunteers", len(s.volunteers))
		}
	}
	s.waiting = waiting
	s.d.Metrics.UnitsWithoutVolunteer.Set(float64(len(waiting)))
}

// someVolunteerCanTake reports whether a registered volunteer has not run the unit yet.
// The caller holds s.mu.
func (s *Server) someVolunteerCanTake(u tasktable.Unit) bool {
	for id := range s.volunteers {
		if !u.HasVolunteer(id) {
			return true
		}
	}
	return false
}
