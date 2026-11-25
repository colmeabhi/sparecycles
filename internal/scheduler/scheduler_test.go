package scheduler

import (
	"fmt"
	"io"
	"log/slog"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/colmeabhi/sparecycles/internal/metrics"
	"github.com/colmeabhi/sparecycles/internal/tasktable"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// testClock is a clock that moves only when the test moves it.
type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time { return c.now }

func newTestMatcher(clock *testClock) (*Matcher, *metrics.Scheduler) {
	m := metrics.NewScheduler(prometheus.NewRegistry())
	return NewMatcher(MatcherConfig{
		MinTrust: 0.3, LastCopyTrust: 0.6, SlowSpeed: 0.5, MinMemoryMB: 1024,
		ActiveWindow: time.Minute, Metrics: m, Log: quiet, Now: clock.Now,
	}), m
}

func volunteer(id string, speed float64) Volunteer {
	return Volunteer{ID: id, Resources: Resources{CPUCores: 1, SpeedScore: speed}}
}

// openUnit returns a unit with this many copies, leased to some volunteers
// and with results from others.
func openUnit(id string, copies int, leased []string, done ...string) tasktable.Unit {
	u := tasktable.Unit{ID: id, State: tasktable.UnitOpen, Copies: copies, Assignments: map[string]tasktable.Assignment{}}
	for _, v := range leased {
		u.Assignments[v] = tasktable.Assignment{VolunteerID: v}
	}
	for _, v := range done {
		u.Results = append(u.Results, tasktable.Result{VolunteerID: v})
	}
	return u
}

// fresh returns n units that each need all 3 copies, oldest first.
func fresh(n int) []tasktable.Unit {
	units := make([]tasktable.Unit, n)
	for i := range units {
		units[i] = openUnit(fmt.Sprintf("u-%d", i), 3, nil)
	}
	return units
}

func pickedID(u *tasktable.Unit) string {
	if u == nil {
		return "<none>"
	}
	return u.ID
}

func TestTrustScore(t *testing.T) {
	cases := []struct {
		score trustScore
		want  float64
	}{
		{trustScore{}, 0.5},
		{trustScore{correct: 1}, 2.0 / 3},
		{trustScore{wrong: 1}, 1.0 / 3},
		{trustScore{wrong: 2}, 0.25},
		{trustScore{correct: 10, wrong: 1}, 11.0 / 13},
	}
	for _, c := range cases {
		if got := c.score.value(); math.Abs(got-c.want) > 1e-9 {
			t.Fatalf("trust of %+v = %v, want %v", c.score, got, c.want)
		}
	}
}

func TestNewVolunteerGetsOldestUnit(t *testing.T) {
	m, metric := newTestMatcher(&testClock{})
	if got := m.Pick(fresh(3), volunteer("a", 1)); pickedID(got) != "u-0" {
		t.Fatalf("got %s, want u-0", pickedID(got))
	}
	if got := testutil.ToFloat64(metric.Picks.WithLabelValues("fast")); got != 1 {
		t.Fatalf("fast picks = %v, want 1", got)
	}
	if got := m.Pick(nil, volunteer("a", 1)); got != nil {
		t.Fatalf("got %s with no candidates, want none", got.ID)
	}
}

func TestTrustRisesAndFalls(t *testing.T) {
	m, _ := newTestMatcher(&testClock{})
	m.RecordResult("a", true)
	m.RecordResult("a", true)
	if got := m.trustOf("a"); got <= 0.5 {
		t.Fatalf("trust after 2 correct = %v, want above 0.5", got)
	}
	m.RecordResult("b", false)
	if got := m.trustOf("b"); got >= 0.5 {
		t.Fatalf("trust after 1 wrong = %v, want below 0.5", got)
	}
}

// After 2 wrong results the trust is 0.25, below the minimum of 0.3.
func TestLowTrustVolunteerGetsNothing(t *testing.T) {
	m, metric := newTestMatcher(&testClock{})
	m.RecordResult("bad", false)
	if got := m.Pick(fresh(1), volunteer("bad", 1)); got == nil {
		t.Fatal("1 wrong result (trust 0.33) blocked the volunteer, want it still allowed")
	}
	m.RecordResult("bad", false)
	m.RecordResult("bad", false)
	for i := 0; i < 3; i++ {
		if got := m.Pick(fresh(2), volunteer("bad", 1)); got != nil {
			t.Fatalf("a volunteer with trust %v got %s, want nothing", m.trustOf("bad"), got.ID)
		}
	}
	if got := testutil.ToFloat64(metric.Blocked); got != 1 {
		t.Fatalf("sc_volunteers_blocked_total = %v, want 1 (counted once)", got)
	}
	if got := testutil.ToFloat64(metric.Refusals.WithLabelValues(reasonLowTrust)); got != 3 {
		t.Fatalf("low trust refusals = %v, want 3", got)
	}
}

// A volunteer that is not fully trusted leaves the last copy, which often
// decides the vote, to others. If only last copies are open, it takes one.
func TestLowTrustVolunteerAvoidsLastCopy(t *testing.T) {
	m, _ := newTestMatcher(&testClock{})
	m.RecordResult("doubtful", false) // trust 0.33: allowed, but below 0.6
	m.RecordResult("good", true)      // trust 0.67
	lastCopy := openUnit("u-0", 3, []string{"x"}, "y")
	firstCopy := openUnit("u-1", 3, nil)

	if got := m.Pick([]tasktable.Unit{lastCopy, firstCopy}, volunteer("doubtful", 1)); pickedID(got) != "u-1" {
		t.Fatalf("doubtful volunteer got %s, want u-1 (not a last copy)", pickedID(got))
	}
	if got := m.Pick([]tasktable.Unit{lastCopy}, volunteer("doubtful", 1)); pickedID(got) != "u-0" {
		t.Fatalf("doubtful volunteer got %s, want u-0 when it is the only unit", pickedID(got))
	}
	if got := m.Pick([]tasktable.Unit{lastCopy, firstCopy}, volunteer("good", 1)); pickedID(got) != "u-0" {
		t.Fatalf("trusted volunteer got %s, want the oldest unit u-0", pickedID(got))
	}
}

func TestSlowVolunteerSkipsOldestWhileFastIsActive(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)}
	m, metric := newTestMatcher(clock)

	if got := m.Pick(fresh(5), volunteer("slow", 0.3)); pickedID(got) != "u-0" {
		t.Fatalf("with no fast volunteer, slow got %s, want the oldest u-0", pickedID(got))
	}
	m.Pick(fresh(5), volunteer("fast", 2)) // the fast volunteer is now active
	if got := m.Pick(fresh(5), volunteer("slow", 0.3)); pickedID(got) != "u-4" {
		t.Fatalf("with a fast volunteer active, slow got %s, want the newest u-4", pickedID(got))
	}
	clock.now = clock.now.Add(2 * time.Minute) // the fast volunteer is no longer active
	if got := m.Pick(fresh(5), volunteer("slow", 0.3)); pickedID(got) != "u-0" {
		t.Fatalf("after the fast volunteer left, slow got %s, want u-0", pickedID(got))
	}
	if got := testutil.ToFloat64(metric.Picks.WithLabelValues("slow")); got != 3 {
		t.Fatalf("slow picks = %v, want 3", got)
	}
}

// At the end of a job, fast volunteers take the last copies.
func TestSlowVolunteerWaitsAtTheEnd(t *testing.T) {
	m, metric := newTestMatcher(&testClock{})
	m.Pick(nil, volunteer("fast-1", 2))
	m.Pick(nil, volunteer("fast-2", 2))
	end := []tasktable.Unit{openUnit("u-0", 3, []string{"x"}, "y"), openUnit("u-1", 3, []string{"x"}, "y")}

	if got := m.Pick(end, volunteer("slow", 0.3)); got != nil {
		t.Fatalf("slow got %s with 2 open copies and 2 fast volunteers, want nothing", got.ID)
	}
	if got := testutil.ToFloat64(metric.Refusals.WithLabelValues(reasonSlowEndgame)); got != 1 {
		t.Fatalf("slow endgame refusals = %v, want 1", got)
	}
	more := append(end, openUnit("u-2", 3, nil))
	if got := m.Pick(more, volunteer("slow", 0.3)); pickedID(got) != "u-2" {
		t.Fatalf("slow got %s with more copies than fast volunteers, want the newest u-2", pickedID(got))
	}
}

// After a vote with no majority, a unit can need copies that the fast
// volunteers already ran. The slow volunteer must take them, or the unit waits forever.
func TestSlowVolunteerTakesCopiesFastOnesCannot(t *testing.T) {
	m, _ := newTestMatcher(&testClock{})
	m.Pick(nil, volunteer("fast-1", 2))
	m.Pick(nil, volunteer("fast-2", 2))
	raised := openUnit("u-0", 5, nil, "fast-1", "fast-2", "other")

	if got := m.Pick([]tasktable.Unit{raised}, volunteer("slow", 0.3)); pickedID(got) != "u-0" {
		t.Fatalf("slow got %s, want u-0: the fast volunteers already ran it", pickedID(got))
	}
}

func TestMemoryLimit(t *testing.T) {
	m, metric := newTestMatcher(&testClock{})
	small := Volunteer{ID: "small", Resources: Resources{SpeedScore: 1, MemoryMB: 512}}
	unknown := Volunteer{ID: "unknown", Resources: Resources{SpeedScore: 1}}
	large := Volunteer{ID: "large", Resources: Resources{SpeedScore: 1, MemoryMB: 2048}}

	if got := m.Pick(fresh(1), small); got != nil {
		t.Fatalf("a 512 MB volunteer got %s with a 1024 MB limit, want nothing", got.ID)
	}
	if got := m.Pick(fresh(1), unknown); got == nil {
		t.Fatal("a volunteer that reports no memory got nothing, want it allowed")
	}
	if got := m.Pick(fresh(1), large); got == nil {
		t.Fatal("a 2048 MB volunteer got nothing")
	}
	if got := testutil.ToFloat64(metric.Refusals.WithLabelValues(reasonLowMemory)); got != 1 {
		t.Fatalf("low memory refusals = %v, want 1", got)
	}
}

// Zero values turn the rules off: Matcher then works like FIFO.
func TestZeroConfigIsFIFO(t *testing.T) {
	m := NewMatcher(MatcherConfig{Metrics: metrics.NewScheduler(prometheus.NewRegistry()), Log: quiet})
	m.RecordResult("a", false)
	m.RecordResult("a", false)
	if got := m.Pick(fresh(3), volunteer("a", 0.1)); pickedID(got) != "u-0" {
		t.Fatalf("got %s, want u-0", pickedID(got))
	}
}

// Pick runs in many goroutines at the same time as votes update trust.
func TestConcurrentPickAndRecordResult(t *testing.T) {
	m, _ := newTestMatcher(&testClock{now: time.Now()})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("v-%d", i)
		speed := 2.0
		if i%2 == 0 {
			speed = 0.3
		}
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				m.Pick(fresh(4), volunteer(id, speed))
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				m.RecordResult(id, j%3 != 0)
			}
		}()
	}
	wg.Wait()
}

// A dead fast volunteer must not make slow volunteers wait at the end of a job.
func TestForgetDropsAVolunteer(t *testing.T) {
	m, _ := newTestMatcher(&testClock{})
	m.Pick(nil, volunteer("fast", 2))
	m.RecordResult("fast", true)
	end := []tasktable.Unit{openUnit("u-0", 3, []string{"x"}, "y")}
	if got := m.Pick(end, volunteer("slow", 0.3)); got != nil {
		t.Fatalf("slow got %s while the fast volunteer is active, want nothing", got.ID)
	}

	m.Forget("fast")
	if got := m.Pick(end, volunteer("slow", 0.3)); pickedID(got) != "u-0" {
		t.Fatalf("slow got %s after the fast volunteer was forgotten, want u-0", pickedID(got))
	}
	if got := m.trustOf("fast"); got != 0.5 {
		t.Fatalf("trust of a forgotten volunteer = %v, want the neutral 0.5", got)
	}
}

func TestNilMetricsGetADefault(t *testing.T) {
	m := NewMatcher(MatcherConfig{MinTrust: 0.3, Log: quiet})
	m.RecordResult("a", false)
	m.RecordResult("a", false)
	m.Pick(fresh(1), volunteer("a", 1))
	if got := m.Pick(fresh(1), volunteer("b", 1)); got == nil {
		t.Fatal("got nothing, want a unit")
	}
}

// Once blocked, a volunteer stays blocked, even if older correct results of it
// are accepted later and its trust rises above the minimum again.
func TestBlockHolds(t *testing.T) {
	m, metric := newTestMatcher(&testClock{})
	m.RecordResult("v", false)
	m.RecordResult("v", false) // trust 0.25: blocked
	for i := 0; i < 3; i++ {
		m.RecordResult("v", true) // trust 4/7 = 0.57
	}
	if got := m.trustOf("v"); got < 0.3 {
		t.Fatalf("trust = %v, want it above the minimum again for this test", got)
	}
	if got := m.Pick(fresh(1), volunteer("v", 1)); got != nil {
		t.Fatalf("a blocked volunteer got %s after its trust rose, want nothing", got.ID)
	}
	m.RecordResult("v", false)
	m.RecordResult("v", false)
	m.RecordResult("v", false)
	if got := testutil.ToFloat64(metric.Blocked); got != 1 {
		t.Fatalf("sc_volunteers_blocked_total = %v, want 1 (one block, not one per fall)", got)
	}
}
