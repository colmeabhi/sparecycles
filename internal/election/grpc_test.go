package election

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/peerconn"
)

// grpcNode is one Bully node behind a real gRPC server on localhost.
type grpcNode struct {
	id     int
	bully  *Bully
	server *grpc.Server
	cancel context.CancelFunc
}

// startGRPCCluster starts one node per listener; listeners decide the addresses.
func startGRPCCluster(t *testing.T, n int) []*grpcNode {
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
	nodes := make([]*grpcNode, n)
	for i, lis := range listeners {
		nodes[i] = startGRPCNode(t, i+1, addrs, lis)
	}
	return nodes
}

func startGRPCNode(t *testing.T, id int, addrs map[int]string, lis net.Listener) *grpcNode {
	t.Helper()
	peers := map[int]string{}
	for other, a := range addrs {
		if other != id {
			peers[other] = a
		}
	}
	pool := peerconn.New()
	t.Cleanup(func() { _ = pool.Close() })
	transport := NewGRPCTransport(pool)
	b, err := NewBully(BullyConfig{
		Self: id, Peers: peers, Transport: transport,
		Heartbeat: 50 * time.Millisecond, Timeout: 300 * time.Millisecond, Log: quiet,
	})
	if err != nil {
		t.Fatal(err)
	}
	g := grpc.NewServer()
	pb.RegisterElectionServiceServer(g, b)
	go func() { _ = g.Serve(lis) }()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); g.Stop() })
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	return &grpcNode{id: id, bully: b, server: g, cancel: cancel}
}

// safetyWatch checks, in a loop, that two nodes never lead at the same time or in the same term.
type safetyWatch struct {
	mu         sync.Mutex
	leaderOf   map[uint64]int
	violations []string
}

func (w *safetyWatch) run(ctx context.Context, nodes []*grpcNode, isStopped func(id int) bool) {
	for ctx.Err() == nil {
		var now []int
		for _, n := range nodes {
			if isStopped(n.id) || !n.bully.IsLeader() {
				continue
			}
			now = append(now, n.id)
			_, term := n.bully.Leader()
			w.record(term, n.id)
		}
		if len(now) > 1 {
			w.fail("two leaders at the same time: %v", now)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (w *safetyWatch) record(term uint64, id int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if prev, ok := w.leaderOf[term]; ok && prev != id {
		w.violations = append(w.violations, fmt.Sprintf("nodes %d and %d both led term %d", prev, id, term))
	}
	w.leaderOf[term] = id
}

func (w *safetyWatch) fail(format string, args ...any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.violations = append(w.violations, fmt.Sprintf(format, args...))
}

func TestGRPCLeaderFailover(t *testing.T) {
	nodes := startGRPCCluster(t, 3)
	var stoppedMu sync.Mutex
	stopped := map[int]bool{}
	isStopped := func(id int) bool {
		stoppedMu.Lock()
		defer stoppedMu.Unlock()
		return stopped[id]
	}

	watch := &safetyWatch{leaderOf: map[uint64]int{}}
	watchCtx, stopWatch := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { watch.run(watchCtx, nodes, isStopped); close(done) }()

	oldTerm := waitGRPCLeader(t, nodes, 3, isStopped)

	stoppedMu.Lock()
	stopped[3] = true
	stoppedMu.Unlock()
	nodes[2].cancel()
	nodes[2].server.Stop()

	newTerm := waitGRPCLeader(t, nodes, 2, isStopped)
	if newTerm <= oldTerm {
		t.Fatalf("new term %d must be higher than %d", newTerm, oldTerm)
	}
	stopWatch()
	<-done
	if len(watch.violations) > 0 {
		t.Fatalf("safety broken: %v", watch.violations)
	}
}

// waitGRPCLeader waits until want is the only running node that leads.
func waitGRPCLeader(t *testing.T, nodes []*grpcNode, want int, isStopped func(int) bool) uint64 {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var leaders []int
		for _, n := range nodes {
			if !isStopped(n.id) && n.bully.IsLeader() {
				leaders = append(leaders, n.id)
			}
		}
		if len(leaders) == 1 && leaders[0] == want {
			_, term := nodes[want-1].bully.Leader()
			return term
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("node %d did not become the only leader", want)
	return 0
}
