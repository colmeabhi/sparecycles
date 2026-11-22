package scheduler

import (
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/colmeabhi/sparecycles/internal/metrics"
	"github.com/colmeabhi/sparecycles/internal/tasktable"
)

// Reasons for giving a volunteer no unit. They are the "reason" label of
// sc_scheduler_refusals_total and appear in the debug log.
const (
	reasonLowMemory   = "low_memory"
	reasonLowTrust    = "low_trust"
	reasonSlowEndgame = "slow_endgame"
)

// MatcherConfig holds the Matcher rules. Zero values turn a rule off, except
// Now and Log, which have defaults.
type MatcherConfig struct {
	MinTrust      float64            // below this trust, a volunteer gets no unit
	LastCopyTrust float64            // below this trust, a volunteer avoids the last copy of a unit
	SlowSpeed     float64            // a volunteer with a speed score below this is slow
	MinMemoryMB   int                // a volunteer that reports less memory gets no unit; 0 means no limit
	ActiveWindow  time.Duration      // a fast volunteer counts as active this long after its last request
	Metrics       *metrics.Scheduler // nil means a private registry that nobody serves
	Log           *slog.Logger
	Now           func() time.Time // for tests; nil means time.Now
}

// Matcher gives each volunteer a unit that it can finish in time and that the
// system trusts it with. This is resource discovery based on VECTrust [4].
// Volunteers pull work, so Matcher only decides which candidate a volunteer gets,
// or none. The rules, in order:
//
//  1. Memory: a volunteer that reports less than MinMemoryMB gets nothing.
//     A volunteer that reports no memory (0) is allowed.
//  2. Trust: a volunteer with trust below MinTrust gets nothing. Trust comes
//     from votes (RecordResult) and starts at 0.5.
//  3. Last copy: a volunteer with trust below LastCopyTrust skips units where
//     it would take the last copy, if any other unit is open. The last copy
//     often decides the vote, so it goes to a volunteer with a good history.
//  4. Speed: while a fast volunteer is active, a slow volunteer takes the
//     newest unit, not the oldest, so the oldest jobs finish on fast volunteers.
//     At the end of the work, when the active fast volunteers can take every
//     open copy themselves (no more copies than fast volunteers, and none that
//     they already ran), the slow volunteer gets nothing: the fast ones finish
//     sooner. If a copy is one the fast volunteers cannot take, the slow one
//     takes it, so a unit never waits for nobody.
//  5. Otherwise the volunteer gets the oldest unit (FIFO).
//
// Matcher is safe for concurrent use. Its state is leader-local: after a
// failover, trust starts again at 0.5. Forget drops the state of a dead volunteer.
type Matcher struct {
	cfg MatcherConfig

	mu       sync.Mutex
	trust    map[string]trustScore
	blocked  map[string]bool
	fastSeen map[string]time.Time // fast volunteers that may take work, by their last request
}

// NewMatcher returns a Matcher with these rules.
func NewMatcher(cfg MatcherConfig) *Matcher {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Metrics == nil {
		cfg.Metrics = metrics.NewScheduler(prometheus.NewRegistry())
	}
	return &Matcher{cfg: cfg, trust: map[string]trustScore{}, blocked: map[string]bool{}, fastSeen: map[string]time.Time{}}
}

// Pick returns the unit that the volunteer must run, or nil. See Matcher for the rules.
func (m *Matcher) Pick(candidates []tasktable.Unit, v Volunteer) *tasktable.Unit {
	m.mu.Lock()
	defer m.mu.Unlock()
	if reason := m.excluded(v); reason != "" {
		m.refuse(v, reason)
		return nil
	}
	now := m.cfg.Now()
	slow := m.isSlow(v)
	if !slow {
		m.fastSeen[v.ID] = now
	}
	if len(candidates) == 0 {
		return nil
	}
	pool := m.withoutLastCopies(candidates, v)
	if slow {
		if fast := m.activeFast(now); len(fast) > 0 {
			if fastCanTakeAll(candidates, fast) {
				m.refuse(v, reasonSlowEndgame)
				return nil
			}
			return m.give(pool[len(pool)-1], v, "slow volunteer: newest unit")
		}
	}
	return m.give(pool[0], v, "oldest unit")
}

// RecordResult updates the trust of a volunteer after a vote, and blocks it
// when its trust falls below MinTrust.
func (m *Matcher) RecordResult(volunteerID string, correct bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.trust[volunteerID]
	if correct {
		t.correct++
	} else {
		t.wrong++
	}
	m.trust[volunteerID] = t
	m.cfg.Log.Debug("trust updated", "volunteer", volunteerID, "correct", correct, "trust", t.value())
	if t.value() < m.cfg.MinTrust && !m.blocked[volunteerID] {
		m.blocked[volunteerID] = true
		delete(m.fastSeen, volunteerID)
		m.cfg.Metrics.Blocked.Inc()
		m.cfg.Log.Info("volunteer blocked: low trust", "volunteer", volunteerID,
			"trust", t.value(), "correct", t.correct, "wrong", t.wrong)
	}
}

// Forget drops all state of a volunteer. A dead fast volunteer then no longer
// makes slow volunteers wait at the end of a job.
func (m *Matcher) Forget(volunteerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.trust, volunteerID)
	delete(m.blocked, volunteerID)
	delete(m.fastSeen, volunteerID)
}

// excluded returns why the volunteer may get no unit at all, or "".
func (m *Matcher) excluded(v Volunteer) string {
	if m.cfg.MinMemoryMB > 0 && v.Resources.MemoryMB > 0 && v.Resources.MemoryMB < m.cfg.MinMemoryMB {
		return reasonLowMemory
	}
	if m.trustOf(v.ID) < m.cfg.MinTrust {
		return reasonLowTrust
	}
	return ""
}

func (m *Matcher) trustOf(volunteerID string) float64 {
	return m.trust[volunteerID].value()
}

func (m *Matcher) isSlow(v Volunteer) bool {
	return v.Resources.SpeedScore < m.cfg.SlowSpeed
}

// withoutLastCopies drops the units where a volunteer with low trust would take
// the last copy. If that drops every unit, it keeps them all, so work goes on.
func (m *Matcher) withoutLastCopies(candidates []tasktable.Unit, v Volunteer) []tasktable.Unit {
	if m.trustOf(v.ID) >= m.cfg.LastCopyTrust {
		return candidates
	}
	var pool []tasktable.Unit
	for _, u := range candidates {
		if missingCopies(u) > 1 {
			pool = append(pool, u)
		}
	}
	if len(pool) == 0 {
		return candidates
	}
	if len(pool) < len(candidates) {
		m.cfg.Log.Debug("last copies left for trusted volunteers", "volunteer", v.ID,
			"trust", m.trustOf(v.ID), "skipped", len(candidates)-len(pool))
	}
	return pool
}

// activeFast returns the fast volunteers that asked for work within
// ActiveWindow, and forgets the others.
func (m *Matcher) activeFast(now time.Time) []string {
	var ids []string
	for id, seen := range m.fastSeen {
		if now.Sub(seen) > m.cfg.ActiveWindow {
			delete(m.fastSeen, id)
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// fastCanTakeAll reports whether these fast volunteers can take every open copy
// of the units by themselves: there are no more open copies than fast
// volunteers, and each unit has enough fast volunteers that do not have it yet.
func fastCanTakeAll(units []tasktable.Unit, fast []string) bool {
	if openCopies(units) > len(fast) {
		return false
	}
	for _, u := range units {
		free := 0
		for _, id := range fast {
			if !u.HasVolunteer(id) {
				free++
			}
		}
		if free < missingCopies(u) {
			return false
		}
	}
	return true
}

func (m *Matcher) give(u tasktable.Unit, v Volunteer, reason string) *tasktable.Unit {
	m.cfg.Metrics.Picks.WithLabelValues(m.speedClass(v)).Inc()
	m.cfg.Log.Debug("unit picked", "unit", u.ID, "volunteer", v.ID, "reason", reason, "trust", m.trustOf(v.ID))
	return &u
}

func (m *Matcher) refuse(v Volunteer, reason string) {
	m.cfg.Metrics.Refusals.WithLabelValues(reason).Inc()
	m.cfg.Log.Debug("no unit for volunteer", "volunteer", v.ID, "reason", reason, "trust", m.trustOf(v.ID))
}

// speedClass is "slow" below SlowSpeed, else "fast" (which includes normal speed).
func (m *Matcher) speedClass(v Volunteer) string {
	if m.isSlow(v) {
		return "slow"
	}
	return "fast"
}

// missingCopies is how many more volunteers the unit needs.
func missingCopies(u tasktable.Unit) int {
	return u.Copies - len(u.Assignments) - len(u.Results)
}

// openCopies is how many more volunteers all these units need together.
func openCopies(units []tasktable.Unit) int {
	n := 0
	for _, u := range units {
		n += missingCopies(u)
	}
	return n
}
