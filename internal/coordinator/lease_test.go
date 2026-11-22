package coordinator

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/election"
	"github.com/colmeabhi/sparecycles/internal/leaderclient"
	"github.com/colmeabhi/sparecycles/internal/lease"
	"github.com/colmeabhi/sparecycles/internal/membership"
	"github.com/colmeabhi/sparecycles/internal/metrics"
	"github.com/colmeabhi/sparecycles/internal/replication"
	"github.com/colmeabhi/sparecycles/internal/scheduler"
	"github.com/colmeabhi/sparecycles/internal/security"
	"github.com/colmeabhi/sparecycles/internal/tasktable"
	"github.com/colmeabhi/sparecycles/internal/verify"
	"github.com/colmeabhi/sparecycles/internal/workload"
)

// testLease is short, so that a lost unit moves on within the test deadline.
const testLease = 300 * time.Millisecond

// startLeaseNode starts a coordinator with real lease expiry and 1 copy per unit.
// With 1 copy, only a released lease lets a second volunteer run a lost unit,
// so the test cannot pass by accident through voting.
// It returns the table and the metrics, so that the test can check who ran each unit.
// now is the leader clock; nil means the real time.
func startLeaseNode(t *testing.T, ctx context.Context, now func() time.Time) (*grpc.ClientConn, *tasktable.Table, *metrics.Coordinator) {
	t.Helper()
	table := tasktable.New()
	m := metrics.NewCoordinator()
	srv := New(Deps{
		Table:        table,
		Elector:      election.NewStatic(1, nil),
		Replicator:   replication.Noop{},
		Membership:   membership.NewStatic(), // never reports a volunteer dead: only the lease can free a unit
		Leases:       lease.Timeout{Length: testLease},
		Verifier:     verify.FirstResult{},
		Scheduler:    scheduler.FIFO{},
		Auth:         security.SharedToken{Token: testToken},
		Signer:       security.NoSign{},
		Workloads:    workload.Default(),
		Metrics:      m,
		VoteReplicas: 1,
		Log:          quiet,
		Now:          now,
		TickInterval: 20 * time.Millisecond,
	})
	lis := bufconn.Listen(1 << 20)
	g := grpc.NewServer()
	srv.RegisterServices(g)
	go func() { _ = g.Serve(lis) }()
	go func() { _ = srv.Run(ctx) }()
	t.Cleanup(g.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, table, m
}

// A volunteer takes a unit and goes silent. Its lease expires, another volunteer
// runs the unit, the job finishes, and a late result from the silent volunteer is refused.
func TestExpiredLeaseMovesUnitToAnotherVolunteer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, table, m := startLeaseNode(t, ctx, nil)
	vc := pb.NewVolunteerServiceClient(conn)
	client := pb.NewClientServiceClient(conn)

	silent, err := vc.Register(ctx, &pb.RegisterRequest{Token: testToken, Name: "silent"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := client.SubmitJob(ctx, &pb.SubmitJobRequest{
		Workload: "fractal", Params: []byte(`{"width":64,"height":64,"tile_size":32,"max_iter":50}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	taken, err := vc.RequestWork(ctx, &pb.RequestWorkRequest{VolunteerId: silent.GetVolunteerId()})
	if err != nil || taken.GetUnit() == nil {
		t.Fatalf("the silent volunteer got no unit: %v", err)
	}
	lost := taken.GetUnit()

	// The silent volunteer never sends a result. Only real volunteers do.
	startVolunteers(ctx, leaderclient.FromConns(conn), 2)

	done := waitForJob(t, ctx, client, job.GetJobId())
	if done.GetState() != pb.JobState_JOB_STATE_DONE {
		t.Fatalf("job state = %v, error = %q", done.GetState(), done.GetError())
	}

	u, ok := table.Unit(lost.GetUnitId())
	if !ok || len(u.Results) != 1 {
		t.Fatalf("lost unit %s: found = %v, results = %d, want 1 result", lost.GetUnitId(), ok, len(u.Results))
	}
	if u.Results[0].VolunteerID == silent.GetVolunteerId() {
		t.Fatalf("lost unit %s was run by the silent volunteer, want another volunteer", lost.GetUnitId())
	}
	if got := testutil.ToFloat64(m.LeasesExpired); got < 1 {
		t.Fatalf("sc_leases_expired_total = %v, want at least 1", got)
	}

	late, err := vc.SubmitResult(ctx, &pb.SubmitResultRequest{
		VolunteerId: silent.GetVolunteerId(), UnitId: lost.GetUnitId(), Output: []byte("too late"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if late.GetRecorded() {
		t.Fatal("a result after the lease expired was recorded, want it refused")
	}
}

// fakeClock is a leader clock that moves only when the test moves it.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// The leader keeps a lease at its exact expiry time and releases it right after.
//
// The clock is fake, so a slow machine cannot change the result. A second
// "sentinel" lease expires 1 ms earlier: once it is released, a tick has seen
// the clock at the exact expiry of the first lease. The first lease is on unit 0,
// which the policy lists first, so a wrong release would already be visible.
func TestLeaseIsReleasedOnlyAfterItsExpiry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: start}
	conn, table, m := startLeaseNode(t, ctx, clock.Now)
	vc := pb.NewVolunteerServiceClient(conn)

	if _, err := pb.NewClientServiceClient(conn).SubmitJob(ctx, &pb.SubmitJobRequest{
		Workload: "fractal", Params: []byte(`{"width":32,"height":16,"tile_size":16,"max_iter":10}`),
	}); err != nil {
		t.Fatal(err)
	}
	watched := takeUnit(t, ctx, vc) // unit 0, expires at start + testLease
	clock.set(start.Add(-time.Millisecond))
	sentinel := takeUnit(t, ctx, vc) // unit 1, expires 1 ms earlier

	clock.set(start.Add(testLease))
	waitUntilReleased(t, ctx, table, sentinel)
	if !holdsLease(table, watched) {
		t.Fatal("the lease was released at its exact expiry time, want it kept until after")
	}
	if got := testutil.ToFloat64(m.LeasesExpired); got != 1 {
		t.Fatalf("sc_leases_expired_total = %v, want 1 (only the sentinel)", got)
	}

	clock.set(start.Add(testLease + time.Millisecond))
	waitUntilReleased(t, ctx, table, watched)
	if got := testutil.ToFloat64(m.LeasesExpired); got != 2 {
		t.Fatalf("sc_leases_expired_total = %v, want 2", got)
	}

	// The unit is still open, but the volunteer lost its lease, so its result must be refused.
	late, err := vc.SubmitResult(ctx, &pb.SubmitResultRequest{
		VolunteerId: watched.volunteerID, UnitId: watched.unitID, Output: []byte("too late"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if late.GetRecorded() {
		t.Fatal("a result after the lease expired was recorded, want it refused")
	}
	if u, _ := table.Unit(watched.unitID); u.State != tasktable.UnitOpen || len(u.Results) != 0 {
		t.Fatalf("unit state = %v with %d results, want open with no result", u.State, len(u.Results))
	}
}

// heldLease names one lease in the table.
type heldLease struct{ unitID, volunteerID string }

// takeUnit registers a new volunteer and gives it one unit.
func takeUnit(t *testing.T, ctx context.Context, vc pb.VolunteerServiceClient) heldLease {
	t.Helper()
	reg, err := vc.Register(ctx, &pb.RegisterRequest{Token: testToken})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := vc.RequestWork(ctx, &pb.RequestWorkRequest{VolunteerId: reg.GetVolunteerId()})
	if err != nil || resp.GetUnit() == nil {
		t.Fatalf("got no unit: %v", err)
	}
	return heldLease{unitID: resp.GetUnit().GetUnitId(), volunteerID: reg.GetVolunteerId()}
}

func holdsLease(table *tasktable.Table, l heldLease) bool {
	u, _ := table.Unit(l.unitID)
	_, ok := u.Assignments[l.volunteerID]
	return ok
}

func waitUntilReleased(t *testing.T, ctx context.Context, table *tasktable.Table, l heldLease) {
	t.Helper()
	for holdsLease(table, l) {
		select {
		case <-ctx.Done():
			t.Fatalf("the lease of %s on %s was never released", l.volunteerID, l.unitID)
		case <-time.After(5 * time.Millisecond):
		}
	}
}
