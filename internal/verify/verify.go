// Package verify decides when the results for a unit are good enough.
//
// Owner: Tejas (agreement with majority voting).
// The base version accepts the first result.
// The real version accepts a result when 2 of 3 volunteers agree, and asks for
// 2 more volunteers when no majority exists.
package verify

import "github.com/colmeabhi/sparecycles/internal/tasktable"

// Decision is the outcome of a check.
type Decision struct {
	// Accept is true when Output is the accepted result for the unit.
	Accept bool
	Output []byte
	// ExtraCopies is the number of additional volunteers to ask. Used when no majority exists.
	ExtraCopies int
	// Wrong lists volunteers whose results disagree with the accepted output.
	Wrong []string
}

// Verifier checks the results for one unit.
type Verifier interface {
	// Decide looks at all results for the unit so far.
	// unit.Copies is the number of volunteers that the unit was sent to.
	Decide(unit tasktable.Unit) Decision
}

// FirstResult accepts the first result. It is the base stub.
type FirstResult struct{}

// Decide accepts the first result, if there is one.
func (FirstResult) Decide(unit tasktable.Unit) Decision {
	if len(unit.Results) == 0 {
		return Decision{}
	}
	return Decision{Accept: true, Output: unit.Results[0].Output}
}
