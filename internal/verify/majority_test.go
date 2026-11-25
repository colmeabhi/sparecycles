package verify

import (
	"bytes"
	"reflect"
	"slices"
	"testing"

	"github.com/colmeabhi/sparecycles/internal/tasktable"
)

// result returns the result of one volunteer, with the hash the table would store.
func result(volunteerID, output string) tasktable.Result {
	return tasktable.Result{VolunteerID: volunteerID, Output: []byte(output), Hash: tasktable.Hash([]byte(output))}
}

// unit returns an open unit with these results and leases still out.
func unit(copies int, leased []string, results ...tasktable.Result) tasktable.Unit {
	u := tasktable.Unit{ID: "u-0", State: tasktable.UnitOpen, Copies: copies, Results: results, Assignments: map[string]tasktable.Assignment{}}
	for _, id := range leased {
		u.Assignments[id] = tasktable.Assignment{VolunteerID: id}
	}
	return u
}

var twoOfThree = Majority{Quorum: 2}

func TestMajorityAcceptsTwoOfThree(t *testing.T) {
	d := twoOfThree.Decide(unit(3, nil, result("a", "good"), result("b", "bad"), result("c", "good")))
	if !d.Accept || !bytes.Equal(d.Output, []byte("good")) {
		t.Fatalf("decision = %+v, want accept with output good", d)
	}
	if !slices.Equal(d.Wrong, []string{"b"}) {
		t.Fatalf("wrong = %v, want [b]", d.Wrong)
	}
	if d.ExtraCopies != 0 {
		t.Fatalf("extra copies = %d, want 0", d.ExtraCopies)
	}
}

// The first 2 results agree: accept at once, without waiting for the third copy.
func TestMajorityAcceptsBeforeLastCopy(t *testing.T) {
	d := twoOfThree.Decide(unit(3, []string{"c"}, result("a", "good"), result("b", "good")))
	if !d.Accept || !bytes.Equal(d.Output, []byte("good")) || len(d.Wrong) != 0 {
		t.Fatalf("decision = %+v, want accept with output good and nobody wrong", d)
	}
}

func TestMajorityAsksForMoreCopiesWhenAllDiffer(t *testing.T) {
	d := twoOfThree.Decide(unit(3, nil, result("a", "x"), result("b", "y"), result("c", "z")))
	if d.Accept || d.ExtraCopies != ExtraCopiesPerRound {
		t.Fatalf("decision = %+v, want %d extra copies and no accept", d, ExtraCopiesPerRound)
	}
	if len(d.Wrong) != 0 {
		t.Fatalf("wrong = %v, want nobody: the correct output is not known yet", d.Wrong)
	}
}

func TestMajorityWaits(t *testing.T) {
	cases := map[string]tasktable.Unit{
		"1 of 3 back":                  unit(3, []string{"b", "c"}, result("a", "x")),
		"2 of 3 back and they differ":  unit(3, []string{"c"}, result("a", "x"), result("b", "y")),
		"2 differ, third lease gone":   unit(3, nil, result("a", "x"), result("b", "y")),
		"no result yet":                unit(3, []string{"a", "b", "c"}),
		"copies raised to 5, 3 differ": unit(5, nil, result("a", "x"), result("b", "y"), result("c", "z")),
	}
	for name, u := range cases {
		t.Run(name, func(t *testing.T) {
			if d := twoOfThree.Decide(u); !reflect.DeepEqual(d, Decision{}) {
				t.Fatalf("decision = %+v, want to wait", d)
			}
		})
	}
}

// After copies were raised to 5, the round ends when 2 results agree.
func TestMajorityAcceptsAfterCopiesRaised(t *testing.T) {
	u := unit(5, []string{"e"}, result("a", "x"), result("b", "y"), result("c", "z"), result("d", "y"))
	d := twoOfThree.Decide(u)
	if !d.Accept || !bytes.Equal(d.Output, []byte("y")) {
		t.Fatalf("decision = %+v, want accept with output y", d)
	}
	if !slices.Equal(d.Wrong, []string{"a", "c"}) {
		t.Fatalf("wrong = %v, want [a c]", d.Wrong)
	}
}

// 5 copies all back with 3 different outputs and no pair: ask for 2 more again.
func TestMajorityAsksAgainInTheNextRound(t *testing.T) {
	u := unit(5, nil, result("a", "x"), result("b", "y"), result("c", "z"), result("d", "w"), result("e", "v"))
	if d := twoOfThree.Decide(u); d.ExtraCopies != ExtraCopiesPerRound || d.Accept {
		t.Fatalf("decision = %+v, want %d extra copies", d, ExtraCopiesPerRound)
	}
}

func TestMajoritySameStateSameDecision(t *testing.T) {
	units := []tasktable.Unit{
		unit(3, nil, result("a", "good"), result("b", "bad"), result("c", "good")),
		unit(3, nil, result("a", "x"), result("b", "y"), result("c", "z")),
		unit(3, []string{"b"}, result("a", "x")),
	}
	for _, u := range units {
		first := twoOfThree.Decide(u)
		for i := 0; i < 10; i++ {
			if again := twoOfThree.Decide(u); !reflect.DeepEqual(first, again) {
				t.Fatalf("call %d: %+v, first call: %+v", i, again, first)
			}
		}
	}
}

// With more copies than 2 quorums, two outputs can both reach the quorum.
// The one with more results wins. On a tie, the one that reached that count first.
func TestMajorityPicksTheLargestGroupThenTheFirst(t *testing.T) {
	larger := unit(7, nil, result("a", "x"), result("b", "x"), result("c", "y"), result("d", "y"), result("e", "y"))
	if d := twoOfThree.Decide(larger); !bytes.Equal(d.Output, []byte("y")) {
		t.Fatalf("output = %q, want y (3 votes against 2)", d.Output)
	}
	// y has 2 votes after c, x only after d.
	tie := unit(4, nil, result("a", "x"), result("b", "y"), result("c", "y"), result("d", "x"))
	if d := twoOfThree.Decide(tie); !bytes.Equal(d.Output, []byte("y")) {
		t.Fatalf("output = %q, want y (first to reach 2 votes)", d.Output)
	}
}

// Quorum 1 is the baseline mode: the first result is accepted.
func TestMajorityQuorumOne(t *testing.T) {
	d := Majority{Quorum: 1}.Decide(unit(1, nil, result("a", "x")))
	if !d.Accept || !bytes.Equal(d.Output, []byte("x")) {
		t.Fatalf("decision = %+v, want accept with output x", d)
	}
}

// An empty output is a valid output, and empty outputs can agree.
func TestMajorityEmptyOutputs(t *testing.T) {
	d := twoOfThree.Decide(unit(3, []string{"c"}, result("a", ""), result("b", "")))
	if !d.Accept || len(d.Output) != 0 {
		t.Fatalf("decision = %+v, want accept with an empty output", d)
	}
}
