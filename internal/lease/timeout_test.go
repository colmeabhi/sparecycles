package lease

import (
	"slices"
	"testing"
	"time"

	"github.com/colmeabhi/sparecycles/internal/scheduler"
	"github.com/colmeabhi/sparecycles/internal/tasktable"
)

var now = time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

// unit returns an open unit with one lease per volunteer, each with its own expiry.
func unit(id string, leases map[string]time.Time) tasktable.Unit {
	u := tasktable.Unit{ID: id, State: tasktable.UnitOpen, Copies: len(leases), Assignments: map[string]tasktable.Assignment{}}
	for volunteerID, expiry := range leases {
		u.Assignments[volunteerID] = tasktable.Assignment{VolunteerID: volunteerID, LeaseExpiry: expiry}
	}
	return u
}

func TestTimeoutDuration(t *testing.T) {
	p := Timeout{Length: 3 * time.Minute}
	if got := p.Duration(scheduler.Volunteer{ID: "vol-1"}); got != 3*time.Minute {
		t.Fatalf("Duration = %v, want 3m", got)
	}
}

func TestTimeoutExpired(t *testing.T) {
	cases := []struct {
		name   string
		expiry time.Time
		want   bool
	}{
		{"expired", now.Add(-time.Second), true},
		{"just expired", now.Add(-time.Nanosecond), true},
		{"exactly at expiry", now, false},
		{"not yet expired", now.Add(time.Second), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Timeout{}.Expired([]tasktable.Unit{unit("u-0", map[string]time.Time{"vol-a": c.expiry})}, now)
			if c.want && !slices.Equal(got, []Expired{{UnitID: "u-0", VolunteerID: "vol-a"}}) {
				t.Fatalf("got %v, want the lease of vol-a", got)
			}
			if !c.want && len(got) != 0 {
				t.Fatalf("got %v, want no expired lease", got)
			}
		})
	}
}

// A done unit has no use for leases. The table clears them on accept, but the
// policy must not depend on that.
func TestTimeoutSkipsDoneUnits(t *testing.T) {
	done := unit("u-0", map[string]time.Time{"vol-a": now.Add(-time.Minute)})
	done.State = tasktable.UnitDone
	if got := (Timeout{}).Expired([]tasktable.Unit{done}, now); len(got) != 0 {
		t.Fatalf("got %v, want nothing for a done unit", got)
	}
}

func TestTimeoutSeveralLeasesOnOneUnit(t *testing.T) {
	u := unit("u-0", map[string]time.Time{
		"vol-c": now.Add(-time.Second),
		"vol-a": now.Add(-time.Minute),
		"vol-b": now.Add(time.Minute), // still valid
	})
	got := Timeout{}.Expired([]tasktable.Unit{u}, now)
	want := []Expired{{UnitID: "u-0", VolunteerID: "vol-a"}, {UnitID: "u-0", VolunteerID: "vol-c"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// Units and leases come from maps and slices in any order. The result must not.
func TestTimeoutStableOrder(t *testing.T) {
	old := now.Add(-time.Second)
	units := []tasktable.Unit{
		unit("job-1-2", map[string]time.Time{"vol-b": old, "vol-a": old}),
		unit("job-1-10", map[string]time.Time{"vol-z": old}),
		unit("job-1-1", map[string]time.Time{"vol-y": old}),
	}
	want := []Expired{
		{UnitID: "job-1-1", VolunteerID: "vol-y"},
		{UnitID: "job-1-10", VolunteerID: "vol-z"},
		{UnitID: "job-1-2", VolunteerID: "vol-a"},
		{UnitID: "job-1-2", VolunteerID: "vol-b"},
	}
	for i := 0; i < 20; i++ {
		if got := (Timeout{}).Expired(units, now); !slices.Equal(got, want) {
			t.Fatalf("run %d: got %v, want %v", i, got, want)
		}
	}
}

func TestTimeoutNoUnits(t *testing.T) {
	if got := (Timeout{}).Expired(nil, now); len(got) != 0 {
		t.Fatalf("got %v, want nothing", got)
	}
}
