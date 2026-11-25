package election

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
)

// Short settings, so that each test ends within a few seconds.
const (
	testHeartbeat = 20 * time.Millisecond
	testTimeout   = 120 * time.Millisecond
	waitLimit     = 5 * time.Second
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// memNetwork is an in-memory Transport. A node that is down, or a cut link, returns Unavailable.
type memNetwork struct {
	mu    sync.Mutex
	nodes map[string]*Bully
	down  map[string]bool
	cut   map[[2]string]bool // links that drop messages, in both directions
}

func newMemNetwork() *memNetwork {
	return &memNetwork{nodes: map[string]*Bully{}, down: map[string]bool{}, cut: map[[2]string]bool{}}
}

// from returns a Transport that sends as the node at addr, so that cut links apply.
func (n *memNetwork) from(addr string) Transport { return memTransport{net: n, self: addr} }

func (n *memNetwork) target(from, to string) (*Bully, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	node, ok := n.nodes[to]
	if !ok || n.down[to] || n.down[from] || n.cut[[2]string{from, to}] || n.cut[[2]string{to, from}] {
		return nil, status.Error(codes.Unavailable, "unreachable")
	}
	return node, nil
}

func (n *memNetwork) setDown(addr string, down bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.down[addr] = down
}

func (n *memNetwork) cutLink(a, b string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.cut[[2]string{a, b}] = true
}

type memTransport struct {
	net  *memNetwork
	self string
}

func (t memTransport) Election(ctx context.Context, addr string, req *pb.ElectionRequest) (*pb.ElectionResponse, error) {
	node, err := t.net.target(t.self, addr)
	if err != nil {
		return nil, err
	}
	return node.Election(ctx, req)
}

func (t memTransport) Coordinator(ctx context.Context, addr string, req *pb.CoordinatorRequest) (*pb.CoordinatorResponse, error) {
	node, err := t.net.target(t.self, addr)
	if err != nil {
		return nil, err
	}
	return node.Coordinator(ctx, req)
}

func (t memTransport) Heartbeat(ctx context.Context, addr string, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	node, err := t.net.target(t.self, addr)
	if err != nil {
		return nil, err
	}
	return node.Heartbeat(ctx, req)
}

// cluster is a set of Bully nodes on one memNetwork.
type cluster struct {
	t      *testing.T
	net    *memNetwork
	ids    []int
	nodes  map[int]*Bully
	cancel map[int]context.CancelFunc
}

func addr(id int) string { return fmt.Sprintf("node-%d", id) }

func newCluster(t *testing.T, ids ...int) *cluster {
	return &cluster{t: t, net: newMemNetwork(), ids: ids, nodes: map[int]*Bully{}, cancel: map[int]context.CancelFunc{}}
}

// start creates node id (again, with a fresh state) and runs it.
func (c *cluster) start(id int) *Bully {
	c.t.Helper()
	peers := map[int]string{}
	for _, other := range c.ids {
		if other != id {
			peers[other] = addr(other)
		}
	}
	b, err := NewBully(BullyConfig{
		Self: id, Peers: peers, Transport: c.net.from(addr(id)),
		Heartbeat: testHeartbeat, Timeout: testTimeout, Log: quiet,
	})
	if err != nil {
		c.t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.t.Cleanup(cancel)
	c.net.mu.Lock()
	c.net.nodes[addr(id)] = b
	c.net.down[addr(id)] = false
	c.net.mu.Unlock()
	c.nodes[id], c.cancel[id] = b, cancel
	if err := b.Start(ctx); err != nil {
		c.t.Fatal(err)
	}
	return b
}

// stop crashes node id: it stops and nobody can reach it.
func (c *cluster) stop(id int) {
	c.net.setDown(addr(id), true)
	c.cancel[id]()
}

// leaders returns the IDs of the running nodes that say they lead now.
func (c *cluster) leaders() []int {
	var out []int
	for _, id := range c.ids {
		c.net.mu.Lock()
		down := c.net.down[addr(id)]
		c.net.mu.Unlock()
		if b, ok := c.nodes[id]; ok && !down && b.IsLeader() {
			out = append(out, id)
		}
	}
	return out
}

// waitLeader waits until exactly node want leads, and all running nodes know it.
func (c *cluster) waitLeader(want int) uint64 {
	c.t.Helper()
	deadline := time.Now().Add(waitLimit)
	for time.Now().Before(deadline) {
		if l := c.leaders(); len(l) == 1 && l[0] == want && c.allFollow(want) {
			_, term := c.nodes[want].Leader()
			return term
		}
		time.Sleep(testHeartbeat / 2)
	}
	c.t.Fatalf("node %d did not become the only leader; leaders now: %v", want, c.leaders())
	return 0
}

func (c *cluster) allFollow(leader int) bool {
	for _, id := range c.ids {
		c.net.mu.Lock()
		down := c.net.down[addr(id)]
		c.net.mu.Unlock()
		if b, ok := c.nodes[id]; ok && !down {
			if l, _ := b.Leader(); l != leader {
				return false
			}
		}
	}
	return true
}

func TestSingleNodeLeadsAtOnce(t *testing.T) {
	b, err := NewBully(BullyConfig{Self: 1, Heartbeat: testHeartbeat, Timeout: testTimeout, Log: quiet})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if !b.IsLeader() {
		t.Fatal("a node with no peers must lead right after Start")
	}
	if id, term := b.Leader(); id != 1 || term != 1 {
		t.Fatalf("Leader() = %d, %d; want 1, 1", id, term)
	}
}

func TestHighestAliveIDWins(t *testing.T) {
	c := newCluster(t, 1, 2, 3)
	for _, id := range c.ids {
		c.start(id)
	}
	c.waitLeader(3)
}

func TestHighestAliveIDWinsWhenTopNodeIsDown(t *testing.T) {
	c := newCluster(t, 1, 2, 3)
	c.start(1)
	c.start(2)
	c.waitLeader(2)
}

func TestNoLeaderWithoutMajority(t *testing.T) {
	c := newCluster(t, 1, 2, 3)
	c.start(3)
	time.Sleep(5 * testTimeout)
	if l := c.leaders(); len(l) != 0 {
		t.Fatalf("1 of 3 nodes must not lead; leaders: %v", l)
	}
}

func TestOlderTermIsRejected(t *testing.T) {
	b, err := NewBully(BullyConfig{Self: 2, Peers: map[int]string{1: "a", 3: "b"}, Transport: newMemNetwork().from("x"),
		Heartbeat: testHeartbeat, Timeout: testTimeout, Log: quiet})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := b.Coordinator(ctx, &pb.CoordinatorRequest{Term: 5, LeaderId: 3}); err != nil {
		t.Fatal(err)
	}

	hb, err := b.Heartbeat(ctx, &pb.HeartbeatRequest{Term: 4, LeaderId: 3})
	if err != nil || hb.GetTerm() != 5 {
		t.Fatalf("old heartbeat: term %d, err %v; want term 5 to show the rejection", hb.GetTerm(), err)
	}
	co, err := b.Coordinator(ctx, &pb.CoordinatorRequest{Term: 4, LeaderId: 1})
	if err != nil || co.GetTerm() != 5 {
		t.Fatalf("old Coordinator: term %d, err %v; want term 5", co.GetTerm(), err)
	}
	el, err := b.Election(ctx, &pb.ElectionRequest{Term: 4, CandidateId: 1})
	if err != nil || el.GetOk() || el.GetTerm() != 5 {
		t.Fatalf("old Election: ok %v, term %d, err %v; want not ok and term 5", el.GetOk(), el.GetTerm(), err)
	}
	if id, term := b.Leader(); id != 3 || term != 5 {
		t.Fatalf("Leader() = %d, %d; want 3, 5", id, term)
	}
}

func TestOneLeaderPerTerm(t *testing.T) {
	b, err := NewBully(BullyConfig{Self: 1, Peers: map[int]string{2: "a", 3: "b"}, Transport: newMemNetwork().from("x"),
		Heartbeat: testHeartbeat, Timeout: testTimeout, Log: quiet})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := b.Coordinator(ctx, &pb.CoordinatorRequest{Term: 7, LeaderId: 2}); err != nil {
		t.Fatal(err)
	}
	_, err = b.Heartbeat(ctx, &pb.HeartbeatRequest{Term: 7, LeaderId: 3})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("a second leader in term 7: err = %v, want FailedPrecondition", err)
	}
}

func TestLeaderWithoutMajorityStepsDown(t *testing.T) {
	c := newCluster(t, 1, 2, 3)
	for _, id := range c.ids {
		c.start(id)
	}
	oldTerm := c.waitLeader(3)

	// Node 3 can no longer reach anyone, but it keeps running.
	c.net.cutLink(addr(3), addr(1))
	c.net.cutLink(addr(3), addr(2))

	deadline := time.Now().Add(waitLimit)
	for c.nodes[3].IsLeader() {
		if time.Now().After(deadline) {
			t.Fatal("node 3 still leads without a majority")
		}
		time.Sleep(testHeartbeat / 2)
	}
	newTerm := waitOnly(t, c, 2)
	if newTerm <= oldTerm {
		t.Fatalf("new term %d must be higher than old term %d", newTerm, oldTerm)
	}
}

// waitOnly waits until node want is the only node that leads (other nodes may not know it yet).
func waitOnly(t *testing.T, c *cluster, want int) uint64 {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for time.Now().Before(deadline) {
		if l := c.leaders(); len(l) == 1 && l[0] == want {
			_, term := c.nodes[want].Leader()
			return term
		}
		time.Sleep(testHeartbeat / 2)
	}
	t.Fatalf("node %d did not lead; leaders: %v", want, c.leaders())
	return 0
}

func TestHigherNodeThatComesBackTakesOver(t *testing.T) {
	c := newCluster(t, 1, 2, 3)
	c.start(1)
	c.start(2)
	oldTerm := c.waitLeader(2)

	c.start(3) // a fresh node 3, with term 0
	newTerm := c.waitLeader(3)
	if newTerm <= oldTerm {
		t.Fatalf("node 3 leads in term %d, want a term higher than %d", newTerm, oldTerm)
	}
}

func TestLeaderCrashGivesNewLeader(t *testing.T) {
	c := newCluster(t, 1, 2, 3)
	for _, id := range c.ids {
		c.start(id)
	}
	oldTerm := c.waitLeader(3)
	c.stop(3)
	if newTerm := c.waitLeader(2); newTerm <= oldTerm {
		t.Fatalf("new term %d must be higher than %d", newTerm, oldTerm)
	}
}

func TestBeforeLeadRunsBeforeLeading(t *testing.T) {
	var calls sync.Map
	b, err := NewBully(BullyConfig{
		Self: 1, Heartbeat: testHeartbeat, Timeout: testTimeout, Log: quiet,
		BeforeLead: func(_ context.Context, term uint64) error {
			calls.Store(term, true)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := calls.Load(uint64(1)); !ok || !b.IsLeader() {
		t.Fatal("BeforeLead must run for term 1 before the node leads")
	}
}

func TestFailedBeforeLeadMeansNoLeading(t *testing.T) {
	b, err := NewBully(BullyConfig{
		Self: 1, Heartbeat: testHeartbeat, Timeout: testTimeout, Log: quiet,
		BeforeLead: func(context.Context, uint64) error { return errors.New("no table") },
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if b.IsLeader() {
		t.Fatal("a node whose BeforeLead failed must not lead")
	}
}

func TestBadSettings(t *testing.T) {
	cases := map[string]BullyConfig{
		"no heartbeat":     {Self: 1, Timeout: time.Second},
		"short timeout":    {Self: 1, Heartbeat: time.Second, Timeout: 2 * time.Second},
		"peers, no client": {Self: 1, Peers: map[int]string{2: "x"}, Heartbeat: time.Millisecond, Timeout: time.Second},
	}
	for name, cfg := range cases {
		if _, err := NewBully(cfg); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
