package coordinator

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

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

// electedNode is a coordinator with a Bully elector on a real localhost port.
type electedNode struct {
	elector *election.Bully
	conn    *grpc.ClientConn
	stop    func()
}

// startElectedCluster starts n coordinators that elect a leader with Bully.
func startElectedCluster(t *testing.T, ctx context.Context, n int) []*electedNode {
	t.Helper()
	listeners := make([]net.Listener, n)
	addrs := map[int]string{}
	for i := range listeners {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		listeners[i], addrs[i+1] = lis, lis.Addr().String()
	}
	nodes := make([]*electedNode, n)
	for i, lis := range listeners {
		nodes[i] = startElectedNode(t, ctx, i+1, addrs, lis)
	}
	return nodes
}

func startElectedNode(t *testing.T, ctx context.Context, id int, addrs map[int]string, lis net.Listener) *electedNode {
	t.Helper()
	peers := map[int]string{}
	for other, a := range addrs {
		if other != id {
			peers[other] = a
		}
	}
	transport := election.NewGRPCTransport()
	t.Cleanup(func() { _ = transport.Close() })
	elector, err := election.NewBully(election.BullyConfig{
		Self: id, Peers: peers, Transport: transport,
		Heartbeat: 50 * time.Millisecond, Timeout: 300 * time.Millisecond, Log: quiet,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := New(Deps{
		Table:        tasktable.New(),
		Elector:      elector,
		Replicator:   replication.Noop{},
		Membership:   membership.NewStatic(),
		Leases:       lease.NoExpiry{Length: time.Minute},
		Verifier:     verify.FirstResult{},
		Scheduler:    scheduler.FIFO{},
		Auth:         security.SharedToken{Token: testToken},
		Signer:       security.NoSign{},
		Workloads:    workload.Default(),
		Metrics:      metrics.NewCoordinator(),
		VoteReplicas: 1,
		Log:          quiet,
		TickInterval: 50 * time.Millisecond,
	})
	g := grpc.NewServer()
	srv.RegisterServices(g)
	go func() { _ = g.Serve(lis) }()
	nodeCtx, cancel := context.WithCancel(ctx)
	go func() { _ = srv.Run(nodeCtx) }()
	stop := func() { cancel(); g.Stop() }
	t.Cleanup(stop)

	conn, err := grpc.NewClient(addrs[id], grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &electedNode{elector: elector, conn: conn, stop: stop}
}

func waitElected(t *testing.T, ctx context.Context, node *electedNode) {
	t.Helper()
	for !node.elector.IsLeader() {
		select {
		case <-ctx.Done():
			t.Fatal("no leader before the deadline")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// After the leader crashes, a new leader is elected and a new job finishes.
// The job in progress is lost until replication (issue #14) exists, so the test submits a new one.
func TestLeaderCrashNewJobFinishes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	nodes := startElectedCluster(t, ctx, 3)
	waitElected(t, ctx, nodes[2])

	pool := leaderclient.FromConns(nodes[0].conn, nodes[1].conn, nodes[2].conn)
	startVolunteers(ctx, pool, 2)

	nodes[2].stop()
	waitElected(t, ctx, nodes[1])

	var resp *pb.SubmitJobResponse
	err := pool.Do(ctx, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pb.NewClientServiceClient(conn).SubmitJob(ctx, &pb.SubmitJobRequest{
			Workload: "fractal", Params: []byte(`{"width":64,"height":64,"tile_size":32,"max_iter":50}`),
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	job := waitForJob(t, ctx, pb.NewClientServiceClient(nodes[1].conn), resp.GetJobId())
	if job.GetState() != pb.JobState_JOB_STATE_DONE {
		t.Fatalf("job state = %v, error = %q", job.GetState(), job.GetError())
	}
}
