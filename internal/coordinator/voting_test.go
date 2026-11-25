package coordinator

import (
	"bytes"
	"context"
	"net"
	"strings"
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
	"github.com/colmeabhi/sparecycles/internal/volunteer"
	"github.com/colmeabhi/sparecycles/internal/workload"
)

// votingJob is small, so that the tests are fast: 4 tiles of 16×16.
const votingJob = `{"width":32,"height":32,"tile_size":16,"max_iter":50}`

// newVotingServer builds a coordinator with quorum 2 and this many copies per unit.
// Leases never expire, so every result in the tests comes from the volunteer that took the unit.
func newVotingServer(copies int) (*Server, *tasktable.Table, *metrics.Coordinator) {
	table := tasktable.New()
	m := metrics.NewCoordinator()
	srv := New(Deps{
		Table:        table,
		Elector:      election.NewStatic(1, nil),
		Replicator:   replication.Noop{},
		Membership:   membership.NewStatic(),
		Leases:       lease.NoExpiry{Length: time.Minute},
		Verifier:     verify.Majority{Quorum: 2},
		Scheduler:    scheduler.FIFO{},
		Auth:         security.SharedToken{Token: testToken},
		Signer:       security.NoSign{},
		Workloads:    workload.Default(),
		Metrics:      m,
		VoteReplicas: copies,
		Log:          quiet,
	})
	return srv, table, m
}

// startVotingNode serves a 2-of-3 voting coordinator on an in-memory listener.
func startVotingNode(t *testing.T, ctx context.Context) (*grpc.ClientConn, *tasktable.Table, *metrics.Coordinator) {
	t.Helper()
	srv, table, m := newVotingServer(3)
	return serveNode(t, ctx, srv), table, m
}

// serveNode runs srv on an in-memory listener until the test ends, and returns a connection to it.
func serveNode(t *testing.T, ctx context.Context, srv *Server) *grpc.ClientConn {
	t.Helper()
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
	return conn
}

func startFaultyVolunteers(ctx context.Context, pool *leaderclient.Pool, n int) {
	for i := 0; i < n; i++ {
		v := volunteer.New(volunteer.Config{Name: "faulty", Token: testToken, CPUCores: 1, SpeedScore: 1, FaultyRate: 1},
			pool, workload.Default(), security.NoSign{}, quiet)
		go func() { _ = v.Run(ctx) }()
	}
}

// honestImage computes the job in this process, with no coordinator and no volunteers.
func honestImage(t *testing.T, params string) []byte {
	t.Helper()
	w := workload.Fractal{}
	payloads, err := w.Split([]byte(params))
	if err != nil {
		t.Fatal(err)
	}
	outputs := make([][]byte, len(payloads))
	for i, p := range payloads {
		if outputs[i], err = w.Run(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	img, err := w.Merge([]byte(params), outputs)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// waitForResults waits until every unit of the job has at least n results.
func waitForResults(t *testing.T, ctx context.Context, table *tasktable.Table, jobID string, n int) {
	t.Helper()
	job, _ := table.Job(jobID)
	for _, id := range job.UnitIDs {
		for {
			u, _ := table.Unit(id)
			if len(u.Results) >= n {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatalf("unit %s has %d results, want %d", id, len(u.Results), n)
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
}

// submitVotingJob submits votingJob and returns its ID and its number of units.
func submitVotingJob(t *testing.T, ctx context.Context, client pb.ClientServiceClient) (string, int) {
	t.Helper()
	resp, err := client.SubmitJob(ctx, &pb.SubmitJobRequest{Workload: "fractal", Params: []byte(votingJob)})
	if err != nil {
		t.Fatal(err)
	}
	return resp.GetJobId(), int(resp.GetUnits())
}

func requireHonestResult(t *testing.T, job *pb.GetJobResponse) {
	t.Helper()
	if job.GetState() != pb.JobState_JOB_STATE_DONE {
		t.Fatalf("job state = %v, error = %q", job.GetState(), job.GetError())
	}
	if !bytes.Equal(job.GetResult(), honestImage(t, votingJob)) {
		t.Fatal("the image differs from the image of honest volunteers only")
	}
}

// A faulty volunteer runs every unit first, so its wrong result is in every vote.
// The 3 honest volunteers then outvote it 2 to 1 on every unit.
func TestFaultyVolunteerIsOutvoted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, table, m := startVotingNode(t, ctx)
	client := pb.NewClientServiceClient(conn)
	pool := leaderclient.FromConns(conn)

	jobID, units := submitVotingJob(t, ctx, client)
	startFaultyVolunteers(ctx, pool, 1)
	waitForResults(t, ctx, table, jobID, 1)
	startVolunteers(ctx, pool, 3)

	requireHonestResult(t, waitForJob(t, ctx, client, jobID))
	if got := testutil.ToFloat64(m.WrongResults); got != float64(units) {
		t.Fatalf("sc_wrong_results_total = %v, want %d (one per unit)", got, units)
	}
	if got := testutil.ToFloat64(m.NoMajority); got != 0 {
		t.Fatalf("sc_votes_no_majority_total = %v, want 0", got)
	}
}

// 2 faulty volunteers send 2 different wrong outputs, and 1 honest volunteer
// makes 3 different results: no majority. The unit goes to 2 more volunteers,
// and the next honest result makes the majority.
func TestNoMajorityAsksForTwoMoreCopies(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, table, m := startVotingNode(t, ctx)
	client := pb.NewClientServiceClient(conn)
	pool := leaderclient.FromConns(conn)

	jobID, units := submitVotingJob(t, ctx, client)
	startFaultyVolunteers(ctx, pool, 2)
	waitForResults(t, ctx, table, jobID, 2)
	startVolunteers(ctx, pool, 3)

	requireHonestResult(t, waitForJob(t, ctx, client, jobID))
	if got := testutil.ToFloat64(m.NoMajority); got != float64(units) {
		t.Fatalf("sc_votes_no_majority_total = %v, want %d (one per unit)", got, units)
	}
	if got := testutil.ToFloat64(m.WrongResults); got != float64(2*units) {
		t.Fatalf("sc_wrong_results_total = %v, want %d (2 per unit)", got, 2*units)
	}
	job, _ := table.Job(jobID)
	for _, id := range job.UnitIDs {
		if u, _ := table.Unit(id); u.Copies != 3+verify.ExtraCopiesPerRound {
			t.Fatalf("unit %s has %d copies, want %d", id, u.Copies, 3+verify.ExtraCopiesPerRound)
		}
	}
}

// Results that arrive at the same time must give one decision per unit: one
// accept, no error to any volunteer, and each wrong result counted once, whether
// it is in the vote or comes back as a late copy. Each round sends 9 copies at
// once (8 right, 1 wrong), so that several results reach the vote together.
func TestConcurrentResultsGiveOneDecision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const copies = 9
	srv, _, m := newVotingServer(copies)

	const rounds = 20
	params := []byte(`{"width":16,"height":16,"tile_size":16,"max_iter":10}`) // 1 unit
	tile := honestTile(t, params)
	for round := 0; round < rounds; round++ {
		if _, err := srv.SubmitJob(ctx, &pb.SubmitJobRequest{Workload: "fractal", Params: params}); err != nil {
			t.Fatal(err)
		}
		// All copies are out before any result comes back.
		results := map[session][]byte{}
		var unitID string
		for i := 0; i < copies; i++ {
			output := tile
			if i == 0 {
				output = []byte("bad")
			}
			var v session
			v, unitID = takeOneUnit(t, ctx, srv)
			results[v] = output
		}
		var wg sync.WaitGroup
		errs := make(chan error, len(results))
		for v, output := range results {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := srv.SubmitResult(ctx, &pb.SubmitResultRequest{
					VolunteerId: v.id, SessionSecret: []byte(v.secret), UnitId: unitID, Output: output,
				})
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("round %d: SubmitResult returned %v, want nil for every result", round, err)
			}
		}
	}
	if got := testutil.ToFloat64(m.UnitsAccepted); got != rounds {
		t.Fatalf("sc_units_accepted_total = %v, want %d", got, rounds)
	}
	if got := testutil.ToFloat64(m.WrongResults); got != rounds {
		t.Fatalf("sc_wrong_results_total = %v, want %d (one per round)", got, rounds)
	}
	if got := testutil.ToFloat64(m.JobsFinished.WithLabelValues("done")); got != rounds {
		t.Fatalf("jobs finished = %v, want %d", got, rounds)
	}
}

// honestTile computes the only unit of a 1-unit fractal job.
func honestTile(t *testing.T, params []byte) []byte {
	t.Helper()
	payloads, err := workload.Fractal{}.Split(params)
	if err != nil || len(payloads) != 1 {
		t.Fatalf("split: %d units, err = %v, want 1 unit", len(payloads), err)
	}
	tile, err := workload.Fractal{}.Run(context.Background(), payloads[0])
	if err != nil {
		t.Fatal(err)
	}
	return tile
}

// session is a registered volunteer: its ID and the secret it must send with each request.
type session struct {
	id     string
	secret string // a string, so that a session can be a map key
}

func newSession(r *pb.RegisterResponse) session {
	return session{id: r.GetVolunteerId(), secret: string(r.GetSessionSecret())}
}

// takeOneUnit registers a new volunteer and gives it the oldest unit that needs one.
func takeOneUnit(t *testing.T, ctx context.Context, srv *Server) (v session, unitID string) {
	t.Helper()
	reg, err := srv.Register(ctx, &pb.RegisterRequest{Token: testToken})
	if err != nil {
		t.Fatal(err)
	}
	v = newSession(reg)
	work, err := srv.RequestWork(ctx, &pb.RequestWorkRequest{VolunteerId: v.id, SessionSecret: []byte(v.secret)})
	if err != nil || work.GetUnit() == nil {
		t.Fatalf("got no unit: %v", err)
	}
	return v, work.GetUnit().GetUnitId()
}

// The 2 honest copies win before the third copy comes back. The late copy is
// still checked: a wrong one is counted, a correct one is not, and a volunteer
// that never had a copy cannot make anyone look wrong.
func TestLateCopyIsChecked(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, m := startVotingNode(t, ctx)
	vc := pb.NewVolunteerServiceClient(conn)
	register := func() session {
		r, err := vc.Register(ctx, &pb.RegisterRequest{Token: testToken})
		if err != nil {
			t.Fatal(err)
		}
		return newSession(r)
	}
	honest1, honest2, last, outsider := register(), register(), register(), register()

	params := `{"width":32,"height":16,"tile_size":16,"max_iter":50}` // 2 units
	if _, err := pb.NewClientServiceClient(conn).SubmitJob(ctx, &pb.SubmitJobRequest{Workload: "fractal", Params: []byte(params)}); err != nil {
		t.Fatal(err)
	}
	tiles := map[string][]byte{}
	for range 2 { // each volunteer takes unit 0, then unit 1
		for _, v := range []session{honest1, honest2, last} {
			resp, err := vc.RequestWork(ctx, &pb.RequestWorkRequest{VolunteerId: v.id, SessionSecret: []byte(v.secret)})
			if err != nil || resp.GetUnit() == nil {
				t.Fatalf("volunteer %s got no unit: %v", v.id, err)
			}
			if tiles[resp.GetUnit().GetUnitId()], err = (workload.Fractal{}).Run(ctx, resp.GetUnit().GetPayload()); err != nil {
				t.Fatal(err)
			}
		}
	}
	submit := func(v session, unitID string, output []byte) bool {
		r, err := vc.SubmitResult(ctx, &pb.SubmitResultRequest{
			VolunteerId: v.id, SessionSecret: []byte(v.secret), UnitId: unitID, Output: output,
		})
		if err != nil {
			t.Fatal(err)
		}
		return r.GetRecorded()
	}
	for unitID, tile := range tiles {
		submit(honest1, unitID, tile)
		submit(honest2, unitID, tile) // the unit is accepted here
	}
	unit0, unit1 := tasktable.UnitID(jobIDOf(tiles), 0), tasktable.UnitID(jobIDOf(tiles), 1)

	if submit(outsider, unit0, []byte("garbage")) || submit(last, unit0, []byte("wrong")) {
		t.Fatal("a result for an accepted unit was recorded")
	}
	submit(last, unit0, []byte("wrong again")) // each copy is checked once
	submit(last, unit1, tiles[unit1])          // late but correct
	if got := testutil.ToFloat64(m.WrongResults); got != 1 {
		t.Fatalf("sc_wrong_results_total = %v, want 1 (only the late wrong copy)", got)
	}
}

// jobIDOf returns the job ID of the unit IDs in tiles ("job-…-0" → "job-…").
func jobIDOf(tiles map[string][]byte) string {
	for unitID := range tiles {
		return unitID[:strings.LastIndex(unitID, "-")]
	}
	return ""
}

func TestLateCopiesExpireAndAreTakenOnce(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	l := lateCopies{}
	l.add("u-0", map[string]tasktable.Assignment{
		"a": {VolunteerID: "a", LeaseExpiry: now.Add(time.Minute)},
		"b": {VolunteerID: "b", LeaseExpiry: now.Add(time.Second)},
	}, now)

	if !l.take("u-0", "a", now) {
		t.Fatal("a had a running copy, want it taken")
	}
	if l.take("u-0", "a", now) {
		t.Fatal("a was taken twice")
	}
	if l.take("u-0", "b", now.Add(2*time.Second)) {
		t.Fatal("b's lease had ended, want it not taken")
	}
	if l.take("u-0", "c", now) || l.take("u-9", "a", now) {
		t.Fatal("an unknown volunteer or unit was taken")
	}

	l.add("u-1", map[string]tasktable.Assignment{"d": {VolunteerID: "d", LeaseExpiry: now.Add(time.Second)}}, now)
	l.add("u-2", nil, now.Add(time.Hour)) // prunes d, whose lease has ended
	if len(l) != 0 {
		t.Fatalf("late copies = %v, want none after prune", l)
	}
}
