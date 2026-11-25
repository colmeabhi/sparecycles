package coordinator

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/colmeabhi/sparecycles/internal/election"
	"github.com/colmeabhi/sparecycles/internal/lease"
	"github.com/colmeabhi/sparecycles/internal/membership"
	"github.com/colmeabhi/sparecycles/internal/metrics"
	"github.com/colmeabhi/sparecycles/internal/peerconn"
	"github.com/colmeabhi/sparecycles/internal/replication"
	"github.com/colmeabhi/sparecycles/internal/scheduler"
	"github.com/colmeabhi/sparecycles/internal/security"
	"github.com/colmeabhi/sparecycles/internal/tasktable"
	"github.com/colmeabhi/sparecycles/internal/verify"
	"github.com/colmeabhi/sparecycles/internal/workload"
)

// clusterNode is a coordinator with a Bully elector on a real localhost port.
type clusterNode struct {
	table   *tasktable.Table
	metrics *metrics.Coordinator
	elector *election.Bully
	conn    *grpc.ClientConn
	stop    func()
}

// clusterOptions choose the parts that differ between the cluster tests.
type clusterOptions struct {
	replicate bool          // Quorum replication with CatchUp; false means Noop
	lease     time.Duration // lease length; expired leases are released
}

// startCluster starts n coordinators that elect a leader with Bully.
func startCluster(t *testing.T, ctx context.Context, n int, opts clusterOptions) []*clusterNode {
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
	nodes := make([]*clusterNode, n)
	for i, lis := range listeners {
		nodes[i] = startClusterNode(t, ctx, i+1, addrs, lis, opts)
	}
	return nodes
}

func startClusterNode(t *testing.T, ctx context.Context, id int, addrs map[int]string, lis net.Listener, opts clusterOptions) *clusterNode {
	t.Helper()
	peers := map[int]string{}
	for other, a := range addrs {
		if other != id {
			peers[other] = a
		}
	}
	pool := peerconn.New()
	t.Cleanup(func() { _ = pool.Close() })
	node := &clusterNode{table: tasktable.New(), metrics: metrics.NewCoordinator()}
	node.elector, node.stop = startServer(t, ctx, lis, node, newClusterParts(t, id, peers, pool, node, opts), opts)

	conn, err := grpc.NewClient(addrs[id], grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	node.conn = conn
	return node
}

// clusterParts are the elector and replicator of one cluster node.
type clusterParts struct {
	elector    *election.Bully
	replicator replication.Replicator
}

func newClusterParts(t *testing.T, id int, peers map[int]string, pool *peerconn.Pool, node *clusterNode, opts clusterOptions) clusterParts {
	t.Helper()
	var quorum *replication.Quorum
	cfg := election.BullyConfig{
		Self: id, Peers: peers, Transport: election.NewGRPCTransport(pool),
		Heartbeat: 50 * time.Millisecond, Timeout: 300 * time.Millisecond, Log: quiet,
	}
	if opts.replicate {
		cfg.BeforeLead = func(ctx context.Context, _ uint64) error { return quorum.CatchUp(ctx) }
	}
	elector, err := election.NewBully(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !opts.replicate {
		return clusterParts{elector: elector, replicator: replication.Noop{}}
	}
	quorum, err = replication.NewQuorum(replication.QuorumConfig{
		Self: id, Peers: peers, Table: node.table, Elector: elector,
		Transport: replication.NewGRPCTransport(pool), Timeout: time.Second,
		Metrics: node.metrics.Replication, Log: quiet,
	})
	if err != nil {
		t.Fatal(err)
	}
	return clusterParts{elector: elector, replicator: quorum}
}

// startServer runs the coordinator and returns a function that crashes it.
func startServer(t *testing.T, ctx context.Context, lis net.Listener, node *clusterNode, p clusterParts, opts clusterOptions) (*election.Bully, func()) {
	t.Helper()
	srv := New(Deps{
		Table:        node.table,
		Elector:      p.elector,
		Replicator:   p.replicator,
		Membership:   membership.NewStatic(),
		Leases:       expiringLeases{length: opts.lease},
		Verifier:     verify.FirstResult{},
		Scheduler:    scheduler.FIFO{},
		Auth:         security.SharedToken{Token: testToken},
		Signer:       security.NoSign{},
		Workloads:    workload.Default(),
		Metrics:      node.metrics,
		VoteReplicas: 1,
		Log:          quiet,
		TickInterval: 50 * time.Millisecond,
	})
	g := grpc.NewServer(peerconn.ServerOptions()...)
	srv.RegisterServices(g)
	go func() { _ = g.Serve(lis) }()
	nodeCtx, cancel := context.WithCancel(ctx)
	go func() { _ = srv.Run(nodeCtx) }()
	stop := func() { cancel(); g.Stop() }
	t.Cleanup(stop)
	return p.elector, stop
}

// expiringLeases releases leases after their expiry time. After a failover,
// volunteers register again under new IDs, so the leases of their old IDs must end.
// The real lease policy is issue #6; this one keeps the test independent of it.
type expiringLeases struct {
	length time.Duration
}

func (e expiringLeases) Duration(scheduler.Volunteer) time.Duration { return e.length }

func (e expiringLeases) Expired(units []tasktable.Unit, now time.Time) []lease.Expired {
	var out []lease.Expired
	for _, u := range units {
		for id, a := range u.Assignments {
			if now.After(a.LeaseExpiry) {
				out = append(out, lease.Expired{UnitID: u.ID, VolunteerID: id})
			}
		}
	}
	return out
}

func waitLeader(t *testing.T, ctx context.Context, node *clusterNode) {
	t.Helper()
	for !node.elector.IsLeader() {
		select {
		case <-ctx.Done():
			t.Fatal("no leader before the deadline")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
