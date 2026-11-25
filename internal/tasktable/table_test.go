package tasktable

import (
	"testing"
	"time"
)

func mustApply(t *testing.T, tb *Table, c Change) {
	t.Helper()
	if err := tb.Apply(c); err != nil {
		t.Fatal(err)
	}
}

func TestUnitLifecycle(t *testing.T) {
	tb := New()
	mustApply(t, tb, Change{Kind: JobAdded, JobID: "j1", Workload: "w", Units: [][]byte{[]byte("a"), []byte("b")}, Copies: 2})

	if got := len(tb.OpenUnits()); got != 2 {
		t.Fatalf("open units = %d, want 2", got)
	}
	u0 := UnitID("j1", 0)
	expiry := time.Unix(100, 0).UTC()
	mustApply(t, tb, Change{Kind: UnitAssigned, UnitID: u0, VolunteerID: "v1", LeaseExpiry: expiry})

	// The same volunteer must not get the same unit twice.
	if err := tb.Apply(Change{Kind: UnitAssigned, UnitID: u0, VolunteerID: "v1"}); err == nil {
		t.Fatal("expected an error for a second lease to the same volunteer")
	}

	mustApply(t, tb, Change{Kind: ResultRecorded, UnitID: u0, VolunteerID: "v1", Output: []byte("out")})
	u, _ := tb.Unit(u0)
	if len(u.Assignments) != 0 || len(u.Results) != 1 || u.Results[0].Hash != Hash([]byte("out")) {
		t.Fatalf("unexpected unit after result: %+v", u)
	}
	if !u.NeedsVolunteer() {
		t.Fatal("unit with 1 of 2 copies must need a volunteer")
	}

	mustApply(t, tb, Change{Kind: UnitAccepted, UnitID: u0, Output: []byte("out")})
	job, _ := tb.Job("j1")
	if job.Done != 1 {
		t.Fatalf("done = %d, want 1", job.Done)
	}
	if _, ok := tb.Outputs("j1"); ok {
		t.Fatal("outputs must not be ready while a unit is open")
	}
	if err := tb.Apply(Change{Kind: ResultRecorded, UnitID: u0, VolunteerID: "v2"}); err == nil {
		t.Fatal("expected an error for a result on a done unit")
	}
}

func TestLeaseReleased(t *testing.T) {
	tb := New()
	mustApply(t, tb, Change{Kind: JobAdded, JobID: "j1", Units: [][]byte{nil}, Copies: 1})
	u0 := UnitID("j1", 0)
	mustApply(t, tb, Change{Kind: UnitAssigned, UnitID: u0, VolunteerID: "v1"})
	if u, _ := tb.Unit(u0); u.NeedsVolunteer() {
		t.Fatal("unit with an active lease must not need a volunteer")
	}
	mustApply(t, tb, Change{Kind: LeaseReleased, UnitID: u0, VolunteerID: "v1"})
	if u, _ := tb.Unit(u0); !u.NeedsVolunteer() {
		t.Fatal("unit must need a volunteer after its lease is released")
	}
}

func TestFailedChangeDoesNotMoveIndex(t *testing.T) {
	tb := New()
	if err := tb.Apply(Change{Kind: "bad"}); err == nil {
		t.Fatal("expected an error")
	}
	if tb.Index() != 0 {
		t.Fatalf("index = %d, want 0", tb.Index())
	}
}

// Two tables that apply the same changes must hold the same state.
// Replication depends on this.
func TestReplayAndSnapshot(t *testing.T) {
	changes := []Change{
		{Kind: JobAdded, JobID: "j1", Units: [][]byte{[]byte("a")}, Copies: 1},
		{Kind: UnitAssigned, UnitID: UnitID("j1", 0), VolunteerID: "v1"},
		{Kind: ResultRecorded, UnitID: UnitID("j1", 0), VolunteerID: "v1", Output: []byte("x")},
		{Kind: UnitAccepted, UnitID: UnitID("j1", 0), Output: []byte("x")},
		{Kind: JobFinished, JobID: "j1", Result: []byte("final")},
	}
	a, b := New(), New()
	for _, c := range changes {
		raw, err := c.Encode()
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeChange(raw)
		if err != nil {
			t.Fatal(err)
		}
		mustApply(t, a, c)
		mustApply(t, b, decoded)
	}
	sa, _ := a.Snapshot()
	sb, _ := b.Snapshot()
	if string(sa) != string(sb) {
		t.Fatal("tables differ after the same changes")
	}

	c := New()
	if err := c.Restore(sa); err != nil {
		t.Fatal(err)
	}
	if c.Index() != uint64(len(changes)) {
		t.Fatalf("restored index = %d", c.Index())
	}
	job, _ := c.Job("j1")
	if job.State != JobDone || string(job.Result) != "final" {
		t.Fatalf("unexpected restored job: %+v", job)
	}
}

func TestRunningJobs(t *testing.T) {
	tbl := New()
	for _, id := range []string{"job-b", "job-a"} {
		if err := tbl.Apply(Change{Kind: JobAdded, JobID: id, Units: [][]byte{[]byte("p")}, Copies: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tbl.Apply(Change{Kind: JobFinished, JobID: "job-b", Error: "stopped"}); err != nil {
		t.Fatal(err)
	}
	if got := tbl.RunningJobs(); len(got) != 1 || got[0] != "job-a" {
		t.Fatalf("running jobs = %v, want [job-a]", got)
	}
}
