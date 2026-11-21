package coordinator

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
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

// startSignedNode starts a coordinator that checks a token list and signs tasks.
func startSignedNode(t *testing.T, ctx context.Context, signer security.Signer) (*grpc.ClientConn, *metrics.Coordinator) {
	t.Helper()
	tokens, err := security.NewTokenList([]string{"alice=token-a", "bob=token-b"})
	if err != nil {
		t.Fatal(err)
	}
	m := metrics.NewCoordinator()
	srv := New(Deps{
		Table:        tasktable.New(),
		Elector:      election.NewStatic(1, nil),
		Replicator:   replication.Noop{},
		Membership:   membership.NewStatic(),
		Leases:       lease.NoExpiry{Length: time.Minute},
		Verifier:     verify.FirstResult{},
		Scheduler:    scheduler.FIFO{},
		Auth:         tokens,
		Signer:       signer,
		Workloads:    workload.Default(),
		Metrics:      m,
		VoteReplicas: 1,
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
	return conn, m
}

func keyPair(t *testing.T) (*security.Ed25519Signer, *security.Ed25519Verifier) {
	t.Helper()
	seed, pub, err := security.GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	s, err := security.NewEd25519Signer(seed)
	if err != nil {
		t.Fatal(err)
	}
	v, err := security.NewEd25519Verifier(pub)
	if err != nil {
		t.Fatal(err)
	}
	return s, v
}

func TestVolunteerWithWrongKeyStopsAndSendsNothing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	signer, _ := keyPair(t)
	_, wrongVerifier := keyPair(t)
	conn, m := startSignedNode(t, ctx, signer)

	if _, err := pb.NewClientServiceClient(conn).SubmitJob(ctx, &pb.SubmitJobRequest{
		Workload: "fractal", Params: []byte(`{"width":32,"height":32,"tile_size":16,"max_iter":10}`),
	}); err != nil {
		t.Fatal(err)
	}
	v := volunteer.New(volunteer.Config{Name: "mallory", Token: "token-a"},
		leaderclient.FromConns(conn), workload.Default(), wrongVerifier, quiet)

	err := v.Run(ctx)
	if !errors.Is(err, security.ErrBadSignature) {
		t.Fatalf("Run returned %v, want a bad signature error", err)
	}
	if got := testutil.ToFloat64(m.ResultsReceived); got != 0 {
		t.Fatalf("results received = %v, want 0", got)
	}
}

func TestVolunteerWithRightKeyFinishesJob(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	signer, verifier := keyPair(t)
	conn, _ := startSignedNode(t, ctx, signer)

	resp, err := pb.NewClientServiceClient(conn).SubmitJob(ctx, &pb.SubmitJobRequest{
		Workload: "fractal", Params: []byte(`{"width":32,"height":32,"tile_size":16,"max_iter":10}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	v := volunteer.New(volunteer.Config{Name: "bob", Token: "token-b"},
		leaderclient.FromConns(conn), workload.Default(), verifier, quiet)
	go func() { _ = v.Run(ctx) }()

	job := waitForJob(t, ctx, pb.NewClientServiceClient(conn), resp.GetJobId())
	if job.GetState() != pb.JobState_JOB_STATE_DONE {
		t.Fatalf("job state = %v, error = %q", job.GetState(), job.GetError())
	}
}

func TestUnknownTokenIsRejectedAndCounted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	signer, _ := keyPair(t)
	conn, m := startSignedNode(t, ctx, signer)

	_, err := pb.NewVolunteerServiceClient(conn).Register(ctx, &pb.RegisterRequest{Token: "stolen"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, want Unauthenticated", status.Code(err))
	}
	if got := testutil.ToFloat64(m.RegistrationsRejected); got != 1 {
		t.Fatalf("sc_registrations_rejected_total = %v, want 1", got)
	}
}
