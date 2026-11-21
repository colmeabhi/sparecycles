package membership

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
)

var errUnreachable = errors.New("unreachable")

// memNet is an in-memory network of SWIM nodes. Tests can take nodes down and cut links.
type memNet struct {
	mu      sync.Mutex
	nodes   map[string]*SWIM // by address
	down    map[string]bool
	cut     map[[2]string]bool // [from, to] address pairs that drop messages
	pings   map[string]int     // direct pings sent, by sender address
	pingReq map[string]int     // indirect requests sent, by sender address
}

func newMemNet() *memNet {
	return &memNet{nodes: map[string]*SWIM{}, down: map[string]bool{}, cut: map[[2]string]bool{}, pings: map[string]int{}, pingReq: map[string]int{}}
}

func (n *memNet) target(from, to string) (*SWIM, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.down[to] || n.down[from] || n.cut[[2]string{from, to}] {
		return nil, errUnreachable
	}
	s, ok := n.nodes[to]
	if !ok {
		return nil, errUnreachable
	}
	return s, nil
}

func (n *memNet) setCut(a, b string, cut bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.cut[[2]string{a, b}] = cut
}

func (n *memNet) setDown(addr string, down bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.down[addr] = down
}

func (n *memNet) count(m map[string]int, from string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	m[from]++
}

type memTransport struct {
	net  *memNet
	from string
}

func (t memTransport) Ping(ctx context.Context, addr string, req *pb.PingRequest) (*pb.Ack, error) {
	t.net.count(t.net.pings, t.from)
	s, err := t.net.target(t.from, addr)
	if err != nil {
		return nil, err
	}
	return s.Ping(ctx, req)
}

func (t memTransport) PingReq(ctx context.Context, addr string, req *pb.PingReqRequest) (*pb.Ack, error) {
	t.net.count(t.net.pingReq, t.from)
	s, err := t.net.target(t.from, addr)
	if err != nil {
		return nil, err
	}
	return s.PingReq(ctx, req)
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

const testSuspectTimeout = 5 * time.Second

// newCluster builds n nodes with IDs n1..nN at addresses a1..aN. Each node knows all others.
func newCluster(t *testing.T, n int) (*memNet, *fakeClock, []*SWIM) {
	t.Helper()
	net := newMemNet()
	clock := &fakeClock{t: time.Unix(1_000_000, 0)}
	members := make([]Member, n)
	for i := range members {
		members[i] = Member{ID: fmt.Sprintf("n%d", i+1), Address: fmt.Sprintf("a%d", i+1)}
	}
	nodes := make([]*SWIM, n)
	for i, self := range members {
		var seeds []Member
		for j, m := range members {
			if j != i {
				seeds = append(seeds, m)
			}
		}
		nodes[i] = NewSWIM(SWIMConfig{
			Self: self, Seeds: seeds,
			Transport:      memTransport{net: net, from: self.Address},
			Interval:       time.Second,
			PingTimeout:    100 * time.Millisecond,
			SuspectTimeout: testSuspectTimeout,
			IndirectProbes: 2,
			Log:            discard,
			Now:            clock.now,
			Rand:           rand.New(rand.NewPCG(uint64(i+1), 7)),
		})
		net.nodes[self.Address] = nodes[i]
	}
	return net, clock, nodes
}

// probeRound runs one full round, so the node pings every member it knows once.
func probeRound(s *SWIM) {
	s.mu.Lock()
	rounds := len(s.nodes)
	s.mu.Unlock()
	for i := 0; i < rounds; i++ {
		s.probeOnce(context.Background())
	}
}

func stateOf(s *SWIM, id string) pb.MemberState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.nodes[id]; ok {
		return n.state
	}
	return pb.MemberState_MEMBER_STATE_UNSPECIFIED
}

func TestHealthyClusterStaysAlive(t *testing.T) {
	_, _, nodes := newCluster(t, 4)
	for _, s := range nodes {
		probeRound(s)
	}
	for _, s := range nodes {
		for _, other := range nodes {
			if other != s && stateOf(s, other.self.ID) != pb.MemberState_MEMBER_STATE_ALIVE {
				t.Fatalf("%s sees %s as %v", s.self.ID, other.self.ID, stateOf(s, other.self.ID))
			}
		}
	}
}

func TestFailedNodeBecomesSuspectThenDead(t *testing.T) {
	net, clock, nodes := newCluster(t, 3)
	n1, n2 := nodes[0], nodes[1]
	net.setDown("a3", true)

	probeRound(n1)
	if got := stateOf(n1, "n3"); got != pb.MemberState_MEMBER_STATE_SUSPECT {
		t.Fatalf("after probes: n3 is %v, want suspect", got)
	}
	if n1.Dead("n3") {
		t.Fatal("a suspect member must not be dead yet")
	}

	clock.advance(testSuspectTimeout - time.Millisecond)
	n1.checkSuspects()
	if n1.Dead("n3") {
		t.Fatal("dead before the suspect timeout")
	}
	clock.advance(time.Millisecond)
	n1.checkSuspects()
	if !n1.Dead("n3") {
		t.Fatal("n3 must be dead after the suspect timeout")
	}

	// The news spreads to n2 on n1's next message.
	probeRound(n1)
	if !n2.Dead("n3") {
		t.Fatalf("n2 sees n3 as %v, want dead", stateOf(n2, "n3"))
	}
}

func TestIndirectPingPreventsFalseSuspicion(t *testing.T) {
	net, _, nodes := newCluster(t, 3)
	n1 := nodes[0]
	// Only the link from n1 to n3 is broken. n2 can still reach n3.
	net.setCut("a1", "a3", true)

	probeRound(n1)
	if got := stateOf(n1, "n3"); got != pb.MemberState_MEMBER_STATE_ALIVE {
		t.Fatalf("n3 is %v, want alive (n2 reached it)", got)
	}
	if net.pingReq["a1"] == 0 {
		t.Fatal("expected an indirect ping request")
	}
}

func TestSuspectedNodeRefutes(t *testing.T) {
	net, _, nodes := newCluster(t, 3)
	n1, n3 := nodes[0], nodes[2]
	for _, s := range nodes {
		probeRound(s) // healthy round: every node learns the real incarnations
	}
	net.setCut("a1", "a3", true)
	net.setCut("a2", "a3", true)
	probeRound(n1)
	if got := stateOf(n1, "n3"); got != pb.MemberState_MEMBER_STATE_SUSPECT {
		t.Fatalf("setup: n3 is %v, want suspect", got)
	}
	incBefore := n3.selfInc

	// The links heal. n1's next ping carries the suspicion to n3, and n3 answers alive with a higher incarnation.
	net.setCut("a1", "a3", false)
	net.setCut("a2", "a3", false)
	probeRound(n1)
	if got := stateOf(n1, "n3"); got != pb.MemberState_MEMBER_STATE_ALIVE {
		t.Fatalf("n3 is %v, want alive after it refuted", got)
	}
	if n3.selfInc <= incBefore {
		t.Fatalf("n3 incarnation = %d, want more than %d", n3.selfInc, incBefore)
	}
}

func TestPrecedenceRules(t *testing.T) {
	_, _, nodes := newCluster(t, 2)
	s := nodes[0]
	alive, suspect, dead := pb.MemberState_MEMBER_STATE_ALIVE, pb.MemberState_MEMBER_STATE_SUSPECT, pb.MemberState_MEMBER_STATE_DEAD
	s.applyAll([]*pb.Member{{Id: "x", Address: "ax", State: alive, Incarnation: 5}})

	steps := []struct {
		state pb.MemberState
		inc   uint64
		want  pb.MemberState
	}{
		{suspect, 4, alive},   // older suspicion is ignored
		{suspect, 5, suspect}, // suspect(i) overrides alive(i)
		{alive, 5, suspect},   // alive(i) does not override suspect(i)
		{alive, 6, alive},     // alive(i+1) overrides suspect(i)
		{dead, 5, alive},      // older death is ignored
		{dead, 6, dead},       // dead(i) overrides alive(i)
		{alive, 6, dead},      // alive(i) does not override dead(i)
		{suspect, 9, dead},    // suspect never overrides dead
		{alive, 7, alive},     // a higher incarnation rejoins
	}
	for i, st := range steps {
		s.applyAll([]*pb.Member{{Id: "x", State: st.state, Incarnation: st.inc}})
		if got := stateOf(s, "x"); got != st.want {
			t.Fatalf("step %d: after %v(%d) state is %v, want %v", i, st.state, st.inc, got, st.want)
		}
	}
}

func TestUpdateIsSentALimitedNumberOfTimes(t *testing.T) {
	_, _, nodes := newCluster(t, 3) // each node knows 2 others
	s := nodes[0]
	want := retransmitFactor * ceilLog2(2+2)

	sent := 0
	for i := 0; i < 3*want; i++ {
		for _, u := range s.takeUpdates() {
			if u.GetId() == "n2" {
				sent++
			}
		}
	}
	if sent != want {
		t.Fatalf("n2 was sent %d times, want %d", sent, want)
	}
}

func TestOwnRecordIsOnEveryMessage(t *testing.T) {
	_, _, nodes := newCluster(t, 2)
	for i := 0; i < 50; i++ {
		if u := nodes[0].takeUpdates(); len(u) == 0 || u[0].GetId() != "n1" {
			t.Fatalf("message %d does not start with the own record: %v", i, u)
		}
	}
}

func TestNewIdentityAtSameAddressIsNotTheOldMember(t *testing.T) {
	_, _, nodes := newCluster(t, 3)
	n1, n3 := nodes[0], nodes[2]
	n3.SetSelf("n3-new") // for example, a volunteer that registered again

	probeRound(n1)
	if got := stateOf(n1, "n3"); got != pb.MemberState_MEMBER_STATE_SUSPECT {
		t.Fatalf("old identity n3 is %v, want suspect", got)
	}
	if got := stateOf(n1, "n3-new"); got != pb.MemberState_MEMBER_STATE_ALIVE {
		t.Fatalf("new identity is %v, want alive (learned from the ack)", got)
	}
}

// SWIM's main property: one direct ping per node per period, whatever the group size.
func TestLoadPerPeriodDoesNotGrowWithGroupSize(t *testing.T) {
	for _, size := range []int{5, 20, 50} {
		net, _, nodes := newCluster(t, size)
		const periods = 10
		for i := 0; i < periods; i++ {
			nodes[0].probeOnce(context.Background())
		}
		if got := net.pings["a1"]; got != periods {
			t.Fatalf("size %d: %d direct pings in %d periods, want %d", size, got, periods, periods)
		}
		if got := net.pingReq["a1"]; got != 0 {
			t.Fatalf("size %d: %d indirect requests in a healthy group, want 0", size, got)
		}
	}
}

func TestUnknownMemberIsNeitherAliveNorDead(t *testing.T) {
	_, _, nodes := newCluster(t, 2)
	if nodes[0].Alive("nobody") || nodes[0].Dead("nobody") {
		t.Fatal("an unknown member must be neither alive nor dead")
	}
	if !nodes[0].Alive("n1") {
		t.Fatal("a node is always alive to itself")
	}
}

func TestJoinedMemberIsProbed(t *testing.T) {
	net, _, nodes := newCluster(t, 2)
	extra := NewSWIM(SWIMConfig{
		Self:      Member{ID: "v1", Address: "av1"},
		Transport: memTransport{net: net, from: "av1"},
		Interval:  time.Second, PingTimeout: 100 * time.Millisecond, SuspectTimeout: testSuspectTimeout,
		Log: discard,
	})
	net.nodes["av1"] = extra
	nodes[0].Join(Member{ID: "v1", Address: "av1"})

	probeRound(nodes[0])
	if got := stateOf(nodes[0], "v1"); got != pb.MemberState_MEMBER_STATE_ALIVE {
		t.Fatalf("v1 is %v, want alive", got)
	}
	// The joined node learns the prober from the ping it received.
	if got := stateOf(extra, "n1"); got != pb.MemberState_MEMBER_STATE_ALIVE {
		t.Fatalf("v1 sees n1 as %v, want alive", got)
	}
}
