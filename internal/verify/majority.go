package verify

import "github.com/colmeabhi/sparecycles/internal/tasktable"

// ExtraCopiesPerRound is how many more volunteers a unit gets when no result
// has a quorum (design decision: "If no 2 of 3 results agree, the leader sends
// the unit to 2 more volunteers").
const ExtraCopiesPerRound = 2

// Majority accepts a unit when Quorum results have the same hash. It is the
// BOINC-style replication and voting: a volunteer with a fault, or a malicious
// one, cannot change the result alone.
//
// Quorum must be 1 or more, and at most the number of copies of a unit.
type Majority struct {
	Quorum int
}

// Decide accepts the output that Quorum results agree on, and names the
// volunteers that disagree. If every copy is back and no output has a quorum,
// it asks for ExtraCopiesPerRound more copies. Otherwise it waits.
//
// The decision depends only on the unit, so calling Decide again on the same
// unit gives the same decision. After more copies are asked for, the unit has
// fewer results than copies, so the same round never asks twice.
func (m Majority) Decide(unit tasktable.Unit) Decision {
	if hash, ok := m.agreedHash(unit.Results); ok {
		return accept(unit.Results, hash)
	}
	if allCopiesBack(unit) {
		// The correct output is not known yet, so no volunteer is named as wrong.
		return Decision{ExtraCopies: ExtraCopiesPerRound}
	}
	return Decision{}
}

// agreedHash returns the hash with the most results, if it has at least Quorum.
// On a tie, the hash that reached that count first wins. That is the hash the
// leader would have accepted at that moment, because Decide runs after each
// result. The choice never depends on map order.
func (m Majority) agreedHash(results []tasktable.Result) (string, bool) {
	counts := map[string]int{}
	best := ""
	for _, r := range results {
		counts[r.Hash]++
		if counts[r.Hash] > counts[best] {
			best = r.Hash
		}
	}
	if best == "" || counts[best] < m.Quorum {
		return "", false
	}
	return best, true
}

// accept builds the decision for an agreed hash. Wrong lists the other
// volunteers in the order of their results.
func accept(results []tasktable.Result, hash string) Decision {
	d := Decision{Accept: true}
	for _, r := range results {
		switch {
		case r.Hash != hash:
			d.Wrong = append(d.Wrong, r.VolunteerID)
		case d.Output == nil:
			d.Output = r.Output
		}
	}
	return d
}

// allCopiesBack reports whether every volunteer that the unit was sent to has answered.
func allCopiesBack(unit tasktable.Unit) bool {
	return len(unit.Results) >= unit.Copies && len(unit.Assignments) == 0
}
