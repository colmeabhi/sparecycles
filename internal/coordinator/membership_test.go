package coordinator

import (
	"context"
	"net"
	"sync"
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

// fakeMembership lets a test declare members dead.
type fakeMembership struct {
	*membership.Static
	mu   sync.Mutex
	dead map[string]bool
}

func (f *fakeMembership) Dead(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dead[id]
}

func (f *fakeMembership) kill(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dead[id] = true
}

func TestDeadVolunteerLosesLeaseBeforeExpiry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	members := &fakeMembership{Static: membership.NewStatic(), dead: map[string]bool{}}
	m := metrics.NewCoordinator()
	srv := New(Deps{
		Table:        tasktable.New(),
		Elector:      election.NewStatic(1, nil),
		Replicator:   replication.Noop{},
		Membership:   members,
		Leases:       lease.NoExpiry{Length: time.Hour}, // the lease alone would never free the unit
		Verifier:     verify.FirstResult{},
		Scheduler:    scheduler.FIFO{},
		Auth:         security.SharedToken{},
		Signer:       security.NoSign{},
		Workloads:    workload.Default(),
		Metrics:      m,
		VoteReplicas: 1,
		Log:          quiet,
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

	vc := pb.NewVolunteerServiceClient(conn)
	a, err := vc.Register(ctx, &pb.RegisterRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pb.NewClientServiceClient(conn).SubmitJob(ctx, &pb.SubmitJobRequest{
		Workload: "fractal", Params: []byte(`{"width":16,"height":16,"tile_size":16,"max_iter":10}`),
	}); err != nil {
		t.Fatal(err)
	}
	first, err := vc.RequestWork(ctx, &pb.RequestWorkRequest{VolunteerId: a.GetVolunteerId(), SessionSecret: a.GetSessionSecret()})
	if err != nil || first.GetUnit() == nil {
		t.Fatalf("volunteer A got no unit: %v", err)
	}

	// Gossip declares A dead. The leader must free A's unit and forget A.
	members.kill(a.GetVolunteerId())

	b, err := vc.Register(ctx, &pb.RegisterRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for {
		resp, err := vc.RequestWork(ctx, &pb.RequestWorkRequest{VolunteerId: b.GetVolunteerId(), SessionSecret: b.GetSessionSecret()})
		if err != nil {
			t.Fatal(err)
		}
		if resp.GetUnit() != nil {
			if resp.GetUnit().GetUnitId() != first.GetUnit().GetUnitId() {
				t.Fatalf("B got unit %s, want A's unit %s", resp.GetUnit().GetUnitId(), first.GetUnit().GetUnitId())
			}
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("A's unit was never released")
		case <-time.After(20 * time.Millisecond):
		}
	}

	if got := testutil.ToFloat64(m.LeasesDeadVolunteer); got != 1 {
		t.Fatalf("sc_leases_dead_volunteer_total = %v, want 1", got)
	}
	_, err = vc.RequestWork(ctx, &pb.RequestWorkRequest{VolunteerId: a.GetVolunteerId(), SessionSecret: a.GetSessionSecret()})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("dead volunteer A: code = %v, want NotFound so that it registers again", status.Code(err))
	}
}
