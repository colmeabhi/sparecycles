package coordinator

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

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

// newTrustServer builds a coordinator with 2-of-3 voting and the trust-based Matcher.
func newTrustServer() (*Server, *tasktable.Table, *metrics.Coordinator) {
	table := tasktable.New()
	m := metrics.NewCoordinator()
	matcher := scheduler.NewMatcher(scheduler.MatcherConfig{
		MinTrust: 0.3, LastCopyTrust: 0.6, SlowSpeed: 0.5, ActiveWindow: time.Minute,
		Metrics: m.Scheduler, Log: quiet,
	})
	srv := New(Deps{
		Table:        table,
		Elector:      election.NewStatic(1, nil),
		Replicator:   replication.Noop{},
		Membership:   membership.NewStatic(),
		Leases:       lease.NoExpiry{Length: time.Minute},
		Verifier:     verify.Majority{Quorum: 2},
		Scheduler:    matcher,
		Auth:         security.SharedToken{Token: testToken},
		Signer:       security.NoSign{},
		Workloads:    workload.Default(),
		Metrics:      m,
		VoteReplicas: 3,
		Log:          quiet,
	})
	return srv, table, m
}

// A faulty volunteer is outvoted on every unit of the first job, so its trust
// falls and it is blocked. In the second job it gets no unit at all, and the
// honest volunteers finish the job alone.
func TestFaultyVolunteerLosesTrustAndGetsNoWork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	srv, table, m := newTrustServer()
	conn := serveNode(t, ctx, srv)
	client := pb.NewClientServiceClient(conn)
	pool := leaderclient.FromConns(conn)

	// Job 1: the faulty volunteer runs every unit first, so it is in every vote.
	first, _ := submitVotingJob(t, ctx, client)
	startFaultyVolunteers(ctx, pool, 1)
	waitForResults(t, ctx, table, first, 1)
	startVolunteers(ctx, pool, 3)
	requireHonestResult(t, waitForJob(t, ctx, client, first))

	faulty := wrongVolunteers(t, table, first)
	if len(faulty) != 1 {
		t.Fatalf("volunteers with wrong results in job 1 = %v, want exactly the faulty one", faulty)
	}
	if got := testutil.ToFloat64(m.Scheduler.Blocked); got != 1 {
		t.Fatalf("sc_volunteers_blocked_total = %v, want 1", got)
	}

	// Job 2: the blocked volunteer keeps asking, but gets nothing.
	second, _ := submitVotingJob(t, ctx, client)
	requireHonestResult(t, waitForJob(t, ctx, client, second))
	job, _ := table.Job(second)
	for _, id := range job.UnitIDs {
		u, _ := table.Unit(id)
		if u.HasVolunteer(faulty[0]) {
			t.Fatalf("the blocked volunteer %s ran unit %s of job 2", faulty[0], id)
		}
	}
	if got := testutil.ToFloat64(m.Scheduler.Refusals.WithLabelValues("low_trust")); got < 1 {
		t.Fatalf("low trust refusals = %v, want at least 1", got)
	}
}

// wrongVolunteers returns the volunteers whose result differs from the accepted output, in any unit of the job.
func wrongVolunteers(t *testing.T, table *tasktable.Table, jobID string) []string {
	t.Helper()
	job, _ := table.Job(jobID)
	seen := map[string]bool{}
	var ids []string
	for _, id := range job.UnitIDs {
		u, _ := table.Unit(id)
		accepted := tasktable.Hash(u.Output)
		for _, r := range u.Results {
			if r.Hash != accepted && !seen[r.VolunteerID] {
				seen[r.VolunteerID] = true
				ids = append(ids, r.VolunteerID)
			}
		}
	}
	return ids
}

// When gossip declares a fast volunteer dead, the leader tells the scheduler,
// so a slow volunteer can take the last copy at once instead of waiting.
func TestDeadFastVolunteerIsForgotten(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	members := &fakeMembership{Static: membership.NewStatic(), dead: map[string]bool{}}
	m := metrics.NewCoordinator()
	matcher := scheduler.NewMatcher(scheduler.MatcherConfig{SlowSpeed: 0.5, ActiveWindow: time.Hour, Metrics: m.Scheduler, Log: quiet})
	srv := New(Deps{
		Table: tasktable.New(), Elector: election.NewStatic(1, nil), Replicator: replication.Noop{},
		Membership: members, Leases: lease.NoExpiry{Length: time.Minute}, Verifier: verify.FirstResult{},
		Scheduler: matcher, Auth: security.SharedToken{Token: testToken}, Signer: security.NoSign{},
		Workloads: workload.Default(), Metrics: m, VoteReplicas: 1, Log: quiet, TickInterval: 20 * time.Millisecond,
	})
	vc := pb.NewVolunteerServiceClient(serveNode(t, ctx, srv))

	register := func(speed float64) session {
		r, err := vc.Register(ctx, &pb.RegisterRequest{Token: testToken, Resources: &pb.Resources{SpeedScore: speed}})
		if err != nil {
			t.Fatal(err)
		}
		return newSession(r)
	}
	requestWork := func(v session) *pb.WorkUnit {
		resp, err := vc.RequestWork(ctx, &pb.RequestWorkRequest{VolunteerId: v.id, SessionSecret: []byte(v.secret)})
		if err != nil {
			t.Fatal(err)
		}
		return resp.GetUnit()
	}
	fast, slow := register(2), register(0.3)
	requestWork(fast) // no job yet: the fast volunteer is only seen as active
	if _, err := srv.SubmitJob(ctx, &pb.SubmitJobRequest{
		Workload: "fractal", Params: []byte(`{"width":16,"height":16,"tile_size":16,"max_iter":10}`),
	}); err != nil {
		t.Fatal(err)
	}
	if got := requestWork(slow); got != nil {
		t.Fatal("slow got the only unit while the fast volunteer is active, want nothing")
	}

	members.kill(fast.id)
	for {
		if requestWork(slow) != nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("the slow volunteer never got the unit after the fast one died")
		case <-time.After(20 * time.Millisecond):
		}
	}
}
