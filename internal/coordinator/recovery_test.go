package coordinator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/election"
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

// failingReplicator fails the next Send of a chosen change kind once, as real
// replication does when the backups cannot be reached.
type failingReplicator struct {
	replication.Noop
	mu       sync.Mutex
	failNext map[tasktable.ChangeKind]bool
}

func (f *failingReplicator) failOnce(kind tasktable.ChangeKind) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failNext[kind] = true
}

func (f *failingReplicator) Send(_ context.Context, _ uint64, change []byte) error {
	c, err := tasktable.DecodeChange(change)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext[c.Kind] {
		delete(f.failNext, c.Kind)
		return errors.New("no backup answered")
	}
	return nil
}

// startRecoveryServer runs a 2-of-3 voting leader with a fast tick and a replicator that can fail.
func startRecoveryServer(t *testing.T, ctx context.Context) (*Server, *tasktable.Table, *metrics.Coordinator, *failingReplicator) {
	t.Helper()
	table := tasktable.New()
	m := metrics.NewCoordinator()
	rep := &failingReplicator{failNext: map[tasktable.ChangeKind]bool{}}
	srv := New(Deps{
		Table: table, Elector: election.NewStatic(1, nil), Replicator: rep,
		Membership: membership.NewStatic(), Leases: lease.NoExpiry{Length: time.Minute},
		Verifier: verify.Majority{Quorum: 2}, Scheduler: scheduler.FIFO{},
		Auth: security.SharedToken{Token: testToken}, Signer: security.NoSign{},
		Workloads: workload.Default(), Metrics: m, VoteReplicas: 3, Log: quiet,
		TickInterval: 20 * time.Millisecond,
	})
	go func() { _ = srv.Run(ctx) }()
	return srv, table, m, rep
}

// submitOneUnitJob submits a 1-unit fractal job, gives its 3 copies to 3 new
// volunteers, and returns the job ID, the volunteers and the right tile.
func submitOneUnitJob(t *testing.T, ctx context.Context, srv *Server) (string, []session, string, []byte) {
	t.Helper()
	params := []byte(`{"width":16,"height":16,"tile_size":16,"max_iter":10}`)
	job, err := srv.SubmitJob(ctx, &pb.SubmitJobRequest{Workload: "fractal", Params: params})
	if err != nil {
		t.Fatal(err)
	}
	var volunteers []session
	var unitID string
	for i := 0; i < 3; i++ {
		var v session
		v, unitID = takeOneUnit(t, ctx, srv)
		volunteers = append(volunteers, v)
	}
	return job.GetJobId(), volunteers, unitID, honestTile(t, params)
}

func send(ctx context.Context, srv *Server, v session, unitID string, output []byte) error {
	_, err := srv.SubmitResult(ctx, &pb.SubmitResultRequest{
		VolunteerId: v.id, SessionSecret: []byte(v.secret), UnitId: unitID, Output: output,
	})
	return err
}

func waitForJobState(t *testing.T, ctx context.Context, table *tasktable.Table, jobID string) tasktable.Job {
	t.Helper()
	for {
		if job, _ := table.Job(jobID); job.State != tasktable.JobRunning {
			return job
		}
		select {
		case <-ctx.Done():
			t.Fatalf("job %s is still running", jobID)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// The last result is recorded, but the commit that accepts the unit fails. No
// new result will come, so the leader must vote again on a later tick, and count
// the wrong result only once.
func TestFailedAcceptIsRetried(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv, table, m, rep := startRecoveryServer(t, ctx)

	jobID, v, unitID, tile := submitOneUnitJob(t, ctx, srv)
	if err := send(ctx, srv, v[0], unitID, tile); err != nil {
		t.Fatal(err)
	}
	if err := send(ctx, srv, v[1], unitID, []byte("wrong")); err != nil {
		t.Fatal(err)
	}
	rep.failOnce(tasktable.UnitAccepted)
	if err := send(ctx, srv, v[2], unitID, tile); status.Code(err) != codes.Unavailable {
		t.Fatalf("last result: %v, want Unavailable from the failed accept", err)
	}

	if job := waitForJobState(t, ctx, table, jobID); job.State != tasktable.JobDone {
		t.Fatalf("job state = %v, error = %q, want done", job.State, job.Error)
	}
	if got := testutil.ToFloat64(m.UnitsAccepted); got != 1 {
		t.Fatalf("sc_units_accepted_total = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.WrongResults); got != 1 {
		t.Fatalf("sc_wrong_results_total = %v, want 1 (not counted again on the retry)", got)
	}
}

// The unit is accepted, but the commit that finishes the job fails. The leader
// must finish the job on a later tick.
func TestFailedJobFinishIsRetried(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv, table, m, rep := startRecoveryServer(t, ctx)

	jobID, v, unitID, tile := submitOneUnitJob(t, ctx, srv)
	if err := send(ctx, srv, v[0], unitID, tile); err != nil {
		t.Fatal(err)
	}
	rep.failOnce(tasktable.JobFinished)
	if err := send(ctx, srv, v[1], unitID, tile); status.Code(err) != codes.Unavailable {
		t.Fatalf("second result: %v, want Unavailable from the failed finish", err)
	}

	if job := waitForJobState(t, ctx, table, jobID); job.State != tasktable.JobDone {
		t.Fatalf("job state = %v, error = %q, want done", job.State, job.Error)
	}
	if got := testutil.ToFloat64(m.JobsFinished.WithLabelValues("done")); got != 1 {
		t.Fatalf("jobs finished = %v, want 1", got)
	}
}

// With 2 volunteers that disagree, a 2-of-3 vote needs a third volunteer that
// does not exist. The leader reports the unit; a new volunteer clears it.
func TestUnitWithoutVolunteerIsReported(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv, _, m, _ := startRecoveryServer(t, ctx)

	params := []byte(`{"width":16,"height":16,"tile_size":16,"max_iter":10}`)
	if _, err := srv.SubmitJob(ctx, &pb.SubmitJobRequest{Workload: "fractal", Params: params}); err != nil {
		t.Fatal(err)
	}
	a, unitID := takeOneUnit(t, ctx, srv)
	b, _ := takeOneUnit(t, ctx, srv)
	if err := send(ctx, srv, a, unitID, honestTile(t, params)); err != nil {
		t.Fatal(err)
	}
	if err := send(ctx, srv, b, unitID, []byte("wrong")); err != nil {
		t.Fatal(err)
	}
	waitForGauge(t, ctx, m, 1)

	if _, err := srv.Register(ctx, &pb.RegisterRequest{Token: testToken}); err != nil {
		t.Fatal(err)
	}
	waitForGauge(t, ctx, m, 0)
}

func waitForGauge(t *testing.T, ctx context.Context, m *metrics.Coordinator, want float64) {
	t.Helper()
	for testutil.ToFloat64(m.UnitsWithoutVolunteer) != want {
		select {
		case <-ctx.Done():
			t.Fatalf("sc_units_without_volunteer = %v, want %v", testutil.ToFloat64(m.UnitsWithoutVolunteer), want)
		case <-time.After(10 * time.Millisecond):
		}
	}
}
