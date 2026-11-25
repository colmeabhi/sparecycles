package replication

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/tasktable"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// fakeElector reports a fixed leader and term that a test can change.
type fakeElector struct {
	mu     sync.Mutex
	leader int
	term   uint64
}

func (f *fakeElector) Leader() (int, uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.leader, f.term
}

func (f *fakeElector) set(leader int, term uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.leader, f.term = leader, term
}

// memNetwork is an in-memory Transport. A node that is down returns Unavailable.
type memNetwork struct {
	mu    sync.Mutex
	nodes map[string]*Quorum
	down  map[string]bool
}

func (n *memNetwork) target(addr string) (*Quorum, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	q, ok := n.nodes[addr]
	if !ok || n.down[addr] {
		return nil, status.Error(codes.Unavailable, "unreachable")
	}
	return q, nil
}

func (n *memNetwork) setDown(addr string, down bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.down[addr] = down
}

func (n *memNetwork) Replicate(ctx context.Context, addr string, req *pb.ReplicateRequest) (*pb.ReplicateResponse, error) {
	q, err := n.target(addr)
	if err != nil {
		return nil, err
	}
	return q.Replicate(ctx, req)
}

func (n *memNetwork) Snapshot(ctx context.Context, addr string, req *pb.SnapshotRequest) (*pb.SnapshotResponse, error) {
	q, err := n.target(addr)
	if err != nil {
		return nil, err
	}
	return q.Snapshot(ctx, req)
}

// node is one coordinator in a test cluster.
type node struct {
	q       *Quorum
	table   *tasktable.Table
	elector *fakeElector
}

// newCluster makes nodes 1 to n on one network. Every node thinks that leader leads term 1.
func newCluster(t *testing.T, n, leader int) (map[int]*node, *memNetwork) {
	t.Helper()
	network := &memNetwork{nodes: map[string]*Quorum{}, down: map[string]bool{}}
	nodes := map[int]*node{}
	for id := 1; id <= n; id++ {
		peers := map[int]string{}
		for other := 1; other <= n; other++ {
			if other != id {
				peers[other] = addr(other)
			}
		}
		nd := &node{table: tasktable.New(), elector: &fakeElector{leader: leader, term: 1}}
		q, err := NewQuorum(QuorumConfig{
			Self: id, Peers: peers, Table: nd.table, Elector: nd.elector,
			Transport: network, Timeout: 200 * time.Millisecond, Log: quiet,
		})
		if err != nil {
			t.Fatal(err)
		}
		nd.q = q
		nodes[id] = nd
		network.nodes[addr(id)] = q
	}
	return nodes, network
}

func addr(id int) string { return fmt.Sprintf("node-%d", id) }

// jobChange returns a valid change that adds job number i.
func jobChange(t *testing.T, i int) []byte {
	t.Helper()
	b, err := tasktable.Change{Kind: tasktable.JobAdded, JobID: fmt.Sprintf("j%d", i), Units: [][]byte{nil}, Copies: 1}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// commit does what the coordinator does: Send, then Apply on the leader.
func commit(t *testing.T, leader *node, i int) error {
	t.Helper()
	raw := jobChange(t, i)
	if err := leader.q.Send(context.Background(), leader.table.Index()+1, raw); err != nil {
		return err
	}
	c, err := tasktable.DecodeChange(raw)
	if err != nil {
		t.Fatal(err)
	}
	return leader.table.Apply(c)
}

func replicate(t *testing.T, q *Quorum, term uint64, index uint64, change []byte) *pb.ReplicateResponse {
	t.Helper()
	resp, err := q.Replicate(context.Background(), &pb.ReplicateRequest{Term: term, LeaderId: 1, Index: index, Change: change})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestBackupAppliesInOrder(t *testing.T) {
	nodes, _ := newCluster(t, 3, 1)
	backup := nodes[2]
	for i := 1; i <= 3; i++ {
		if resp := replicate(t, backup.q, 1, uint64(i), jobChange(t, i)); !resp.GetOk() || resp.GetLastIndex() != uint64(i) {
			t.Fatalf("change %d: ok %v, last index %d", i, resp.GetOk(), resp.GetLastIndex())
		}
	}
	if backup.table.Index() != 3 {
		t.Fatalf("backup index = %d, want 3", backup.table.Index())
	}
}

func TestRepeatedIndexIsOK(t *testing.T) {
	nodes, _ := newCluster(t, 3, 1)
	backup := nodes[2]
	replicate(t, backup.q, 1, 1, jobChange(t, 1))
	if resp := replicate(t, backup.q, 1, 1, jobChange(t, 1)); !resp.GetOk() || backup.table.Index() != 1 {
		t.Fatalf("retry: ok %v, index %d; want ok and index 1", resp.GetOk(), backup.table.Index())
	}
}

func TestGapWithoutLeaderGivesLastIndex(t *testing.T) {
	nodes, network := newCluster(t, 3, 1)
	backup := nodes[2]
	replicate(t, backup.q, 1, 1, jobChange(t, 1))
	network.setDown(addr(1), true) // the backup cannot load the leader's table
	resp := replicate(t, backup.q, 1, 3, jobChange(t, 3))
	if resp.GetOk() || resp.GetLastIndex() != 1 {
		t.Fatalf("gap: ok %v, last index %d; want not ok and last index 1", resp.GetOk(), resp.GetLastIndex())
	}
}

func TestGapLoadsSnapshotFromLeader(t *testing.T) {
	nodes, _ := newCluster(t, 3, 1)
	leader, backup := nodes[1], nodes[2]
	for i := 1; i <= 4; i++ {
		mustApply(t, leader.table, jobChange(t, i))
	}
	// The backup missed 1 to 4. Change 5 makes it load the leader's table, then apply 5.
	if resp := replicate(t, backup.q, 1, 5, jobChange(t, 5)); !resp.GetOk() || backup.table.Index() != 5 {
		t.Fatalf("ok %v, backup index %d; want ok and index 5", resp.GetOk(), backup.table.Index())
	}
}

func TestOlderTermIsRejected(t *testing.T) {
	nodes, _ := newCluster(t, 3, 1)
	backup := nodes[2]
	backup.elector.set(3, 4)
	resp := replicate(t, backup.q, 3, 1, jobChange(t, 1))
	if resp.GetOk() || resp.GetTerm() != 4 || backup.table.Index() != 0 {
		t.Fatalf("old term: ok %v, term %d, index %d; want not ok, term 4, index 0", resp.GetOk(), resp.GetTerm(), backup.table.Index())
	}
}

func TestSendNeedsOneOfTwoBackups(t *testing.T) {
	nodes, network := newCluster(t, 3, 1)
	network.setDown(addr(3), true)
	if err := commit(t, nodes[1], 1); err != nil {
		t.Fatalf("1 of 2 backups is enough: %v", err)
	}
	if nodes[2].table.Index() != 1 {
		t.Fatalf("backup 2 index = %d, want 1", nodes[2].table.Index())
	}
}

func TestSendFailsWhenBothBackupsAreDown(t *testing.T) {
	nodes, network := newCluster(t, 3, 1)
	network.setDown(addr(2), true)
	network.setDown(addr(3), true)
	if err := commit(t, nodes[1], 1); err == nil {
		t.Fatal("Send must fail with no backup")
	}
	if nodes[1].table.Index() != 0 {
		t.Fatal("the leader must not apply a change that no backup has")
	}
}

func TestSendWithNoPeers(t *testing.T) {
	q, err := NewQuorum(QuorumConfig{Self: 1, Table: tasktable.New(), Elector: &fakeElector{leader: 1, term: 1}, Timeout: time.Second, Log: quiet})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Send(context.Background(), 1, jobChange(t, 1)); err != nil {
		t.Fatalf("one coordinator: %v", err)
	}
}

func TestCatchUpLoadsNewestTable(t *testing.T) {
	nodes, _ := newCluster(t, 3, 1)
	a, b := nodes[1], nodes[2]
	for i := 1; i <= 12; i++ {
		mustApply(t, b.table, jobChange(t, i))
		if i <= 10 {
			mustApply(t, a.table, jobChange(t, i))
		}
	}
	if err := a.q.CatchUp(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a.table.Index() != 12 {
		t.Fatalf("index after catch-up = %d, want 12", a.table.Index())
	}
	snapA, _ := a.table.Snapshot()
	snapB, _ := b.table.Snapshot()
	if !bytes.Equal(snapA, snapB) {
		t.Fatal("after catch-up, A must have the same table as B")
	}
}

// A backup that applied a change the leader dropped gets the leader's table back.
func TestDroppedChangeIsRepaired(t *testing.T) {
	nodes, network := newCluster(t, 3, 1)
	leader, b2, b3 := nodes[1], nodes[2], nodes[3]
	p2 := leader.q.peerByID(2)
	if err := commit(t, leader, 1); err != nil {
		t.Fatal(err)
	}
	// Send returns after the first backup. Let the call to backup 2 finish first.
	waitIndex(t, b2.table, 1)
	waitIdle(p2)

	// Backup 2 gets change X at index 2, but the leader sees no majority and drops X.
	replicate(t, b2.q, 1, 2, jobChange(t, 100))
	leader.q.markDirty(2, []peerResult{{peer: p2, outcome: unknown}})

	// Index 2 becomes Y on the leader and backup 3, while backup 2 is cut off.
	network.setDown(addr(2), true)
	if err := commit(t, leader, 2); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return p2.skip(time.Now()) }) // the failed call to backup 2 ran
	waitIdle(p2)
	network.setDown(addr(2), false)
	p2.markUp()

	if err := commit(t, leader, 3); err != nil {
		t.Fatal(err)
	}
	waitIndex(t, b2.table, 3)
	waitIndex(t, b3.table, 3) // Send returns after the first backup; the other may still be applying
	snapLeader, _ := leader.table.Snapshot()
	snap2, _ := b2.table.Snapshot()
	snap3, _ := b3.table.Snapshot()
	if !bytes.Equal(snapLeader, snap2) || !bytes.Equal(snapLeader, snap3) {
		t.Fatal("all 3 tables must be equal after the repair")
	}
}

// The first change from a new leader replaces the backup's table with the leader's.
func TestNewTermLoadsLeaderTable(t *testing.T) {
	nodes, _ := newCluster(t, 3, 1)
	backup := nodes[2]
	replicate(t, backup.q, 1, 1, jobChange(t, 1))
	replicate(t, backup.q, 1, 2, jobChange(t, 2)) // the new leader never got this one

	newLeader := nodes[3]
	mustApply(t, newLeader.table, jobChange(t, 1))
	for _, n := range nodes {
		n.elector.set(3, 2)
	}
	resp, err := backup.q.Replicate(context.Background(), &pb.ReplicateRequest{Term: 2, LeaderId: 3, Index: 2, Change: jobChange(t, 7)})
	if err != nil || !resp.GetOk() {
		t.Fatalf("ok %v, err %v", resp.GetOk(), err)
	}
	if _, ok := backup.table.Job("j2"); ok {
		t.Fatal("the backup must drop the change that the new leader does not have")
	}
	if _, ok := backup.table.Job("j7"); !ok {
		t.Fatal("the backup must apply the new leader's change")
	}
}

func mustApply(t *testing.T, tb *tasktable.Table, raw []byte) {
	t.Helper()
	c, err := tasktable.DecodeChange(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := tb.Apply(c); err != nil {
		t.Fatal(err)
	}
}

// waitIndex waits until a background call brings the table to index want.
func waitIndex(t *testing.T, tb *tasktable.Table, want uint64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for tb.Index() != want {
		if time.Now().After(deadline) {
			t.Fatalf("index = %d, want %d", tb.Index(), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitFor waits until cond is true.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met before the deadline")
		}
		time.Sleep(time.Millisecond)
	}
}

// waitIdle waits until no call to the backup is running. Use it after the call has started.
func waitIdle(p *peer) {
	p.lock(context.Background())
	p.unlock()
}

func (q *Quorum) peerByID(id int) *peer {
	for _, p := range q.peers {
		if p.id == id {
			return p
		}
	}
	return nil
}
