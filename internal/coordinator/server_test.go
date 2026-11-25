package coordinator

import (
	"bytes"
	"context"
	"image/png"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
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

const testToken = "test-token"

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// startNode starts one coordinator on an in-memory listener and returns a connection to it.
func startNode(t *testing.T, ctx context.Context, nodeID int, peers map[int]string, replicas int) *grpc.ClientConn {
	t.Helper()
	srv := New(Deps{
		Table:        tasktable.New(),
		Elector:      election.NewStatic(nodeID, peers),
		Replicator:   replication.Noop{},
		Membership:   membership.NewStatic(),
		Leases:       lease.NoExpiry{Length: time.Minute},
		Verifier:     verify.FirstResult{},
		Scheduler:    scheduler.FIFO{},
		Auth:         security.SharedToken{Token: testToken},
		Signer:       security.NoSign{},
		Workloads:    workload.Default(),
		Metrics:      metrics.NewCoordinator(),
		VoteReplicas: replicas,
		Log:          quiet,
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
	return conn
}

func startVolunteers(ctx context.Context, pool *leaderclient.Pool, n int) {
	for i := 0; i < n; i++ {
		v := volunteer.New(volunteer.Config{Name: "v", Token: testToken, CPUCores: 1, SpeedScore: 1},
			pool, workload.Default(), security.NoSign{}, quiet)
		go func() { _ = v.Run(ctx) }()
	}
}

// waitForJob polls until the job is done or failed.
func waitForJob(t *testing.T, ctx context.Context, client pb.ClientServiceClient, jobID string) *pb.GetJobResponse {
	t.Helper()
	for {
		job, err := client.GetJob(ctx, &pb.GetJobRequest{JobId: jobID})
		if err != nil {
			t.Fatal(err)
		}
		if job.GetState() != pb.JobState_JOB_STATE_RUNNING {
			return job
		}
		select {
		case <-ctx.Done():
			t.Fatalf("job did not finish: %d/%d units done", job.GetUnitsDone(), job.GetUnitsTotal())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// A full job runs from submission to the merged image.
func TestEndToEndFractal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn := startNode(t, ctx, 1, nil, 3)
	startVolunteers(ctx, leaderclient.FromConns(conn), 3)

	client := pb.NewClientServiceClient(conn)
	resp, err := client.SubmitJob(ctx, &pb.SubmitJobRequest{
		Workload: "fractal", Params: []byte(`{"width":128,"height":96,"tile_size":32,"max_iter":100}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetUnits() != 12 {
		t.Fatalf("units = %d, want 12", resp.GetUnits())
	}

	job := waitForJob(t, ctx, client, resp.GetJobId())
	if job.GetState() != pb.JobState_JOB_STATE_DONE {
		t.Fatalf("job state = %v, error = %q", job.GetState(), job.GetError())
	}
	img, err := png.Decode(bytes.NewReader(job.GetResult()))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 128 || b.Dy() != 96 {
		t.Fatalf("image size = %v", b)
	}
}

// With 3 copies per unit, one volunteer must not get the same unit twice.
// Otherwise one volunteer could win the vote alone.
func TestVolunteerNeverGetsSameUnitTwice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn := startNode(t, ctx, 1, nil, 3)
	vc := pb.NewVolunteerServiceClient(conn)
	reg, err := vc.Register(ctx, &pb.RegisterRequest{Token: testToken})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pb.NewClientServiceClient(conn).SubmitJob(ctx, &pb.SubmitJobRequest{
		Workload: "fractal", Params: []byte(`{"width":32,"height":32,"tile_size":32,"max_iter":10}`),
	}); err != nil {
		t.Fatal(err)
	}

	first, err := vc.RequestWork(ctx, &pb.RequestWorkRequest{VolunteerId: reg.GetVolunteerId(), SessionSecret: reg.GetSessionSecret()})
	if err != nil || first.GetUnit() == nil {
		t.Fatalf("first request: unit=%v err=%v", first.GetUnit(), err)
	}
	second, err := vc.RequestWork(ctx, &pb.RequestWorkRequest{VolunteerId: reg.GetVolunteerId(), SessionSecret: reg.GetSessionSecret()})
	if err != nil {
		t.Fatal(err)
	}
	if second.GetUnit() != nil {
		t.Fatalf("volunteer got unit %s twice", second.GetUnit().GetUnitId())
	}
	if second.GetRetryAfterMs() == 0 {
		t.Fatal("expected a retry delay when no work is available")
	}
}

func TestWrongTokenIsRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn := startNode(t, ctx, 1, nil, 1)
	_, err := pb.NewVolunteerServiceClient(conn).Register(ctx, &pb.RegisterRequest{Token: "wrong"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestUnknownWorkloadIsRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn := startNode(t, ctx, 1, nil, 1)
	_, err := pb.NewClientServiceClient(conn).SubmitJob(ctx, &pb.SubmitJobRequest{Workload: "nope"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
}

// A follower rejects work RPCs, and the pool moves on to the leader.
func TestPoolFindsLeader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Node 2 is the leader because it has the highest ID.
	follower := startNode(t, ctx, 1, map[int]string{2: "n2"}, 1)
	leader := startNode(t, ctx, 2, map[int]string{1: "n1"}, 1)

	_, err := pb.NewClientServiceClient(follower).SubmitJob(ctx, &pb.SubmitJobRequest{Workload: "fractal"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("follower: code = %v, want FailedPrecondition", status.Code(err))
	}

	pool := leaderclient.FromConns(follower, leader)
	startVolunteers(ctx, pool, 2)

	var jobID string
	err = pool.Do(ctx, func(c *grpc.ClientConn) error {
		resp, err := pb.NewClientServiceClient(c).SubmitJob(ctx, &pb.SubmitJobRequest{
			Workload: "fractal", Params: []byte(`{"width":64,"height":64,"tile_size":32,"max_iter":50}`),
		})
		if err == nil {
			jobID = resp.GetJobId()
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, ctx, pb.NewClientServiceClient(leader), jobID)
	if job.GetState() != pb.JobState_JOB_STATE_DONE {
		t.Fatalf("job state = %v", job.GetState())
	}
}
