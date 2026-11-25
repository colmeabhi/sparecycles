package election

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/metrics"
)

// role is the part a node plays in the current term.
type role int

const (
	follower role = iota
	candidate
	leader
)

// BullyConfig holds the settings of a Bully elector.
type BullyConfig struct {
	Self      int
	Peers     map[int]string // other coordinators, by node ID
	Transport Transport
	Heartbeat time.Duration // time between two leader heartbeats; also the timeout of each election RPC
	Timeout   time.Duration // a node that hears no leader for this long starts an election
	// BeforeLead runs after a node wins and before IsLeader returns true.
	// Replication uses it to load the newest task table. If it fails, the node does not lead.
	BeforeLead func(ctx context.Context, term uint64) error
	Metrics    *metrics.Election // nil means metrics that nobody reads
	Log        *slog.Logger
	Now        func() time.Time // for tests; nil means time.Now
}

// Bully elects the alive coordinator with the highest ID, with terms and a leader lease.
//
// Plain Bully can give two leaders when the network splits. These rules prevent that:
//   - Each election uses a new term. A node rejects messages from an older term.
//   - A node accepts one leader per term. A winner needs a majority to accept it.
//   - The leader acts only while a majority answered a heartbeat within the lease.
//   - A node does not accept a new leader within one lease of answering another leader,
//     and a winner waits one lease after it last answered another leader.
//     So the lease of the old leader has ended before the new leader acts.
type Bully struct {
	pb.UnimplementedElectionServiceServer
	cfg      BullyConfig
	lease    time.Duration
	majority int
	wake     chan struct{} // asks the loop to start an election now

	mu        sync.Mutex
	term      uint64
	leaderID  int // leader accepted in this term; self while a candidate or leader; 0 if none
	role      role
	leading   bool              // the leader finished BeforeLead and may act
	deadline  time.Time         // a follower or candidate starts an election after this time
	lastHeard time.Time         // last heartbeat or Coordinator accepted from another leader
	ackedID   int               // leader of the last accepted heartbeat or Coordinator
	acks      map[int]time.Time // leader only: send time of the last heartbeat each peer accepted
	lostAt    time.Time         // last contact with a leader that is gone; zero if none
}

// NewBully returns a Bully elector. Call Start to run it.
func NewBully(cfg BullyConfig) (*Bully, error) {
	switch {
	case cfg.Heartbeat <= 0:
		return nil, errors.New("election heartbeat must be positive")
	case cfg.Timeout <= 2*cfg.Heartbeat:
		return nil, fmt.Errorf("election timeout (%s) must be more than 2 heartbeats (%s)", cfg.Timeout, cfg.Heartbeat)
	case len(cfg.Peers) > 0 && cfg.Transport == nil:
		return nil, errors.New("election transport is needed when there are peers")
	}
	if cfg.Metrics == nil {
		cfg.Metrics = metrics.NewElection(prometheus.NewRegistry())
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Bully{
		cfg:      cfg,
		lease:    cfg.Timeout - cfg.Heartbeat,
		majority: (len(cfg.Peers)+1)/2 + 1,
		wake:     make(chan struct{}, 1),
		acks:     map[int]time.Time{},
	}, nil
}

// Start runs the election in the background until ctx ends.
// With no peers, the node becomes the leader before Start returns.
func (b *Bully) Start(ctx context.Context) error {
	b.mu.Lock()
	b.resetTimerLocked(b.cfg.Now())
	b.mu.Unlock()
	if len(b.cfg.Peers) == 0 {
		if term, won := b.elect(ctx); won {
			b.promote(ctx, term)
		}
	}
	go b.loop(ctx)
	return nil
}

// IsLeader reports whether this node is the leader now and may act.
func (b *Bully) IsLeader() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.role == leader && b.leading && b.hasLeaseLocked(b.cfg.Now())
}

// Leader returns the current leader ID (0 if unknown) and the current term.
func (b *Bully) Leader() (int, uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.role == candidate {
		return 0, b.term
	}
	return b.leaderID, b.term
}

// ---------------------------------------------------------------------------
// Background loop
// ---------------------------------------------------------------------------

func (b *Bully) loop(ctx context.Context) {
	tick := time.NewTicker(b.cfg.Heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			b.step(ctx)
		case <-b.wake:
			if b.is(follower) {
				b.campaign(ctx)
			}
		}
	}
}

// step sends heartbeats as the leader, or starts an election when the leader is silent.
func (b *Bully) step(ctx context.Context) {
	switch {
	case b.is(leader):
		b.sendHeartbeats(ctx)
		b.checkLease()
	case b.timedOut():
		b.campaign(ctx)
	}
}

func (b *Bully) campaign(ctx context.Context) {
	if term, won := b.elect(ctx); won {
		go b.promote(ctx, term)
	}
}

// elect runs one Bully round and reports whether this node won the term.
func (b *Bully) elect(ctx context.Context) (uint64, bool) {
	term := b.becomeCandidate()
	if b.higherAlive(ctx, term) {
		return term, false // a higher node takes over; wait for its Coordinator message
	}
	accepted, sent := b.announce(ctx, term)
	return term, b.win(term, accepted, sent)
}

func (b *Bully) becomeCandidate() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.cfg.Now()
	if b.role == follower && b.leaderID != 0 && b.lostAt.IsZero() {
		b.lostAt = b.lastHeard
	}
	b.term++
	b.role, b.leaderID, b.leading = candidate, b.cfg.Self, false
	b.resetTimerLocked(now)
	b.cfg.Metrics.Started.Inc()
	b.cfg.Metrics.Term.Set(float64(b.term))
	b.cfg.Log.Info("election started", "term", b.term)
	return b.term
}

// higherAlive sends Election to every node with a higher ID. It reports true if
// one of them is alive, or if a newer term ended this election.
func (b *Bully) higherAlive(ctx context.Context, term uint64) bool {
	req := &pb.ElectionRequest{Term: term, CandidateId: int32(b.cfg.Self)}
	var mu sync.Mutex
	alive := false
	b.eachPeer(ctx, b.higherPeers(), func(ctx context.Context, _ int, addr string) {
		resp, err := b.cfg.Transport.Election(ctx, addr, req)
		if err != nil {
			return // the node is down or unreachable
		}
		b.observe(resp.GetTerm())
		if resp.GetOk() {
			mu.Lock()
			alive = true
			mu.Unlock()
		}
	})
	return alive || !b.inTerm(candidate, term)
}

// announce sends Coordinator to every peer and returns the peers that accepted.
func (b *Bully) announce(ctx context.Context, term uint64) ([]int, time.Time) {
	req := &pb.CoordinatorRequest{Term: term, LeaderId: int32(b.cfg.Self)}
	sent := b.cfg.Now()
	var mu sync.Mutex
	var accepted []int
	b.eachPeer(ctx, b.cfg.Peers, func(ctx context.Context, id int, addr string) {
		resp, err := b.cfg.Transport.Coordinator(ctx, addr, req)
		if err != nil {
			return // rejected, down or unreachable
		}
		if resp.GetTerm() != term {
			b.observe(resp.GetTerm())
			return
		}
		mu.Lock()
		accepted = append(accepted, id)
		mu.Unlock()
	})
	return accepted, sent
}

// win makes this node the leader if a majority accepted it and the term is still current.
func (b *Bully) win(term uint64, accepted []int, sent time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(accepted)+1 < b.majority || b.role != candidate || b.term != term {
		return false
	}
	b.role = leader
	clear(b.acks)
	for _, id := range accepted {
		b.acks[id] = sent
	}
	return true
}

// promote runs BeforeLead and waits for the old lease to end, then lets the leader act.
func (b *Bully) promote(ctx context.Context, term uint64) {
	if b.cfg.BeforeLead != nil {
		if err := b.cfg.BeforeLead(ctx, term); err != nil {
			b.stepDown(term, fmt.Sprintf("before-lead step failed: %v", err))
			return
		}
	}
	if !b.waitOldLease(ctx) {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.role != leader || b.term != term {
		return
	}
	b.leading = true
	b.cfg.Metrics.Won.Inc()
	if !b.lostAt.IsZero() {
		b.cfg.Metrics.FailoverSeconds.Observe(b.cfg.Now().Sub(b.lostAt).Seconds())
		b.lostAt = time.Time{}
	}
	b.cfg.Log.Info("became leader", "term", term)
}

// waitOldLease waits until a leader that this node last answered can no longer
// count on that answer. It returns false if ctx ends first.
func (b *Bully) waitOldLease(ctx context.Context) bool {
	b.mu.Lock()
	wait := b.lastHeard.Add(b.lease).Sub(b.cfg.Now())
	b.mu.Unlock()
	if wait <= 0 {
		return true
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (b *Bully) sendHeartbeats(ctx context.Context) {
	b.mu.Lock()
	term := b.term
	b.mu.Unlock()
	req := &pb.HeartbeatRequest{Term: term, LeaderId: int32(b.cfg.Self)}
	sent := b.cfg.Now()
	b.eachPeer(ctx, b.cfg.Peers, func(ctx context.Context, id int, addr string) {
		resp, err := b.cfg.Transport.Heartbeat(ctx, addr, req)
		if err != nil {
			return // the lease check below counts this peer as silent
		}
		if resp.GetTerm() != term {
			b.observe(resp.GetTerm())
			return
		}
		b.recordAck(term, id, sent)
	})
}

func (b *Bully) recordAck(term uint64, id int, sent time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.role == leader && b.term == term && sent.After(b.acks[id]) {
		b.acks[id] = sent
	}
}

// checkLease steps down a leader that a majority has not answered within the lease.
func (b *Bully) checkLease() {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.cfg.Now()
	if b.role == leader && !b.hasLeaseLocked(now) {
		b.stepDownLocked("no majority within the lease", now)
	}
}

func (b *Bully) stepDown(term uint64, reason string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.role == leader && b.term == term {
		b.stepDownLocked(reason, b.cfg.Now())
	}
}

// eachPeer calls fn for each peer in parallel, with one heartbeat as the timeout.
func (b *Bully) eachPeer(ctx context.Context, peers map[int]string, fn func(ctx context.Context, id int, addr string)) {
	ctx, cancel := context.WithTimeout(ctx, b.cfg.Heartbeat)
	defer cancel()
	var wg sync.WaitGroup
	for id, addr := range peers {
		wg.Go(func() { fn(ctx, id, addr) })
	}
	wg.Wait()
}

func (b *Bully) higherPeers() map[int]string {
	higher := map[int]string{}
	for id, addr := range b.cfg.Peers {
		if id > b.cfg.Self {
			higher[id] = addr
		}
	}
	return higher
}

// ---------------------------------------------------------------------------
// ElectionService handlers
// ---------------------------------------------------------------------------

// Election answers a candidate with a lower ID. This node is alive, so it
// answers ok and, unless a leader is alive, starts its own election.
func (b *Bully) Election(_ context.Context, req *pb.ElectionRequest) (*pb.ElectionResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.cfg.Now()
	switch {
	case req.GetTerm() < b.term || int(req.GetCandidateId()) >= b.cfg.Self:
		return &pb.ElectionResponse{Ok: false, Term: b.term}, nil
	case b.role == leader || b.hasLiveLeaderLocked(now):
		// The candidate hears from the leader soon. A new election would only disturb a working leader.
		return &pb.ElectionResponse{Ok: true, Term: b.term}, nil
	}
	b.observeLocked(req.GetTerm(), now)
	if b.role == follower {
		b.requestElection()
	}
	return &pb.ElectionResponse{Ok: true, Term: b.term}, nil
}

// Coordinator accepts the winner of an election as the leader, if the rules allow it.
func (b *Bully) Coordinator(_ context.Context, req *pb.CoordinatorRequest) (*pb.CoordinatorResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.cfg.Now()
	term, from := req.GetTerm(), int(req.GetLeaderId())
	if term < b.term {
		return &pb.CoordinatorResponse{Term: b.term}, nil
	}
	if err := b.canAcceptLocked(term, from, now); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	b.followLocked(term, from, now)
	return &pb.CoordinatorResponse{Term: b.term}, nil
}

// Heartbeat accepts a message from the leader. If the leader has a lower ID,
// this node starts an election to take over, as in Bully.
func (b *Bully) Heartbeat(_ context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	term, from := req.GetTerm(), int(req.GetLeaderId())
	if term < b.term {
		return &pb.HeartbeatResponse{Term: b.term}, nil
	}
	if term == b.term && b.leaderID != 0 && b.leaderID != from {
		return nil, status.Errorf(codes.FailedPrecondition, "term %d already has leader %d", term, b.leaderID)
	}
	b.followLocked(term, from, b.cfg.Now())
	if from < b.cfg.Self {
		b.requestElection()
	}
	return &pb.HeartbeatResponse{Term: b.term}, nil
}

// canAcceptLocked checks if this node may accept node from as the leader of term.
func (b *Bully) canAcceptLocked(term uint64, from int, now time.Time) error {
	switch {
	case from == b.cfg.Self:
		return errors.New("a Coordinator message from this node itself")
	case b.role == leader && from < b.cfg.Self:
		return fmt.Errorf("node %d leads and has a higher ID than %d", b.cfg.Self, from)
	case b.ackedID != 0 && b.ackedID != from && now.Sub(b.lastHeard) < b.lease:
		return fmt.Errorf("node %d answered leader %d less than one lease ago", b.cfg.Self, b.ackedID)
	case term == b.term && b.leaderID != 0 && b.leaderID != from && !(b.role == candidate && from > b.cfg.Self):
		return fmt.Errorf("term %d already has leader %d", term, b.leaderID)
	}
	return nil
}

// ---------------------------------------------------------------------------
// State changes (the caller holds mu, except for the small wrappers)
// ---------------------------------------------------------------------------

// followLocked makes node from the leader of term.
func (b *Bully) followLocked(term uint64, from int, now time.Time) {
	if term > b.term {
		b.term = term
		b.cfg.Metrics.Term.Set(float64(term))
	}
	b.leaveLeadershipLocked(fmt.Sprintf("node %d leads term %d", from, term))
	if b.role != follower || b.leaderID != from {
		b.cfg.Log.Info("new leader", "leader", from, "term", term)
	}
	b.role, b.leaderID = follower, from
	b.lastHeard, b.ackedID = now, from
	b.lostAt = time.Time{}
	b.resetTimerLocked(now)
}

// observe moves to a newer term that another node reported.
func (b *Bully) observe(term uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.observeLocked(term, b.cfg.Now())
}

func (b *Bully) observeLocked(term uint64, now time.Time) {
	if term <= b.term {
		return
	}
	b.term = term
	b.cfg.Metrics.Term.Set(float64(term))
	b.leaveLeadershipLocked(fmt.Sprintf("newer term %d", term))
	b.role, b.leaderID = follower, 0
	b.resetTimerLocked(now)
}

func (b *Bully) stepDownLocked(reason string, now time.Time) {
	b.leaveLeadershipLocked(reason)
	b.role, b.leaderID = follower, 0
	b.lostAt = now
	b.resetTimerLocked(now)
}

// leaveLeadershipLocked stops a leader from acting. The caller sets the new role.
func (b *Bully) leaveLeadershipLocked(reason string) {
	if b.role != leader {
		return
	}
	b.role, b.leading = follower, false
	b.cfg.Metrics.StepDowns.Inc()
	b.cfg.Log.Info("stepped down", "term", b.term, "reason", reason)
}

// resetTimerLocked sets the election deadline, with a random delay so that two nodes rarely start at the same time.
func (b *Bully) resetTimerLocked(now time.Time) {
	b.deadline = now.Add(b.cfg.Timeout + rand.N(b.cfg.Timeout/4+1))
}

func (b *Bully) hasLeaseLocked(now time.Time) bool {
	answered := 1 // this node
	for _, sent := range b.acks {
		if now.Sub(sent) < b.lease {
			answered++
		}
	}
	return answered >= b.majority
}

func (b *Bully) hasLiveLeaderLocked(now time.Time) bool {
	return b.role == follower && b.leaderID != 0 && now.Sub(b.lastHeard) < b.cfg.Timeout
}

func (b *Bully) requestElection() {
	select {
	case b.wake <- struct{}{}:
	default: // an election is already requested
	}
}

func (b *Bully) is(r role) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.role == r
}

func (b *Bully) inTerm(r role, term uint64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.role == r && b.term == term
}

func (b *Bully) timedOut() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.role != leader && b.cfg.Now().After(b.deadline)
}
