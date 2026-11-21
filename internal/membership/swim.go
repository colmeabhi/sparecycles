package membership

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/metrics"
)

const (
	// maxPiggyback is the maximum number of member updates on one message, after the sender's own record.
	maxPiggyback = 8
	// retransmitFactor is lambda in SWIM: each update is sent about lambda * log2(n) times.
	retransmitFactor = 3
)

// SWIMConfig holds the settings for a SWIM member.
type SWIMConfig struct {
	// Self is this node. A volunteer starts with an empty ID and calls SetSelf after it registers.
	Self Member
	// Seeds are members known at start, for example the other coordinators.
	Seeds          []Member
	Transport      Transport
	Interval       time.Duration
	PingTimeout    time.Duration
	SuspectTimeout time.Duration
	IndirectProbes int
	Metrics        *metrics.Gossip  // nil means metrics that are not exported
	Log            *slog.Logger     // nil means slog.Default()
	Now            func() time.Time // nil means time.Now; tests use a fake clock
	Rand           *rand.Rand       // nil means a random source; tests use a fixed seed
}

// node is what this member knows about another member.
type node struct {
	Member
	state        pb.MemberState
	incarnation  uint64
	suspectSince time.Time
	lastSeen     time.Time // last direct contact; used for the detection time metric
}

// SWIM is a member of a SWIM group (Das, Gupta and Motivala, 2002).
//
// Each protocol period it pings one member, in a shuffled round-robin order.
// If the member does not answer, it asks IndirectProbes other members to ping it.
// If they also get no answer, the member becomes suspect, and after
// SuspectTimeout it becomes dead. Every message carries recent updates
// (piggybacking), so news spreads without extra messages. A member that hears
// that it is suspected raises its incarnation number and says it is alive.
type SWIM struct {
	pb.UnimplementedGossipServiceServer

	cfg     SWIMConfig
	log     *slog.Logger
	metrics *metrics.Gossip

	mu      sync.Mutex
	self    Member
	selfInc uint64
	nodes   map[string]*node
	queue   map[string]int // member ID -> sends left for its latest update
	order   []string       // probe order
	next    int            // position in order
	rng     *rand.Rand
}

// NewSWIM returns a SWIM member. Call Start to begin probing.
func NewSWIM(cfg SWIMConfig) *SWIM {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Rand == nil {
		cfg.Rand = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}
	if cfg.Metrics == nil {
		cfg.Metrics = metrics.NewGossip(prometheus.NewRegistry())
	}
	log := cfg.Log
	if log == nil {
		log = slog.Default()
	}
	s := &SWIM{
		cfg:     cfg,
		log:     log,
		metrics: cfg.Metrics,
		self:    cfg.Self,
		// A time-based start lets a restarted node overrule old "dead" news about itself.
		selfInc: uint64(cfg.Now().Unix()),
		nodes:   map[string]*node{},
		queue:   map[string]int{},
		rng:     cfg.Rand,
	}
	for _, m := range cfg.Seeds {
		s.addLocked(m)
	}
	s.updateGaugesLocked()
	return s
}

// Start probes members in the background until ctx ends.
func (s *SWIM) Start(ctx context.Context) error {
	go func() {
		ticker := time.NewTicker(s.cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.probeOnce(ctx)
				s.checkSuspects()
			}
		}
	}()
	return nil
}

// SetSelf sets this node's member ID. A volunteer calls it after each registration.
func (s *SWIM) SetSelf(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == s.self.ID {
		return
	}
	s.log.Info("gossip identity set", "member", id, "previous", s.self.ID)
	s.self.ID = id
	delete(s.nodes, id)
	delete(s.queue, id)
	s.updateGaugesLocked()
}

// Join adds a member, for example a volunteer that just registered.
func (s *SWIM) Join(m Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.ID == "" || m.ID == s.self.ID {
		return
	}
	if n, ok := s.nodes[m.ID]; ok {
		if m.Address != "" {
			n.Address = m.Address
		}
		return
	}
	s.addLocked(m)
	s.log.Info("member joined", "member", m.ID, "address", m.Address)
	s.updateGaugesLocked()
}

// Alive reports whether a member is alive or suspect. This node is always alive.
func (s *SWIM) Alive(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == s.self.ID {
		return true
	}
	n, ok := s.nodes[id]
	return ok && n.state != pb.MemberState_MEMBER_STATE_DEAD
}

// Dead reports whether a member was declared dead. An unknown member is not dead.
func (s *SWIM) Dead(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[id]
	return ok && n.state == pb.MemberState_MEMBER_STATE_DEAD
}

// Members returns this node and all members that are not dead.
func (s *SWIM) Members() []Member {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Member{s.self}
	for _, n := range s.nodes {
		if n.state != pb.MemberState_MEMBER_STATE_DEAD {
			out = append(out, n.Member)
		}
	}
	return out
}

// Ping answers a direct ping.
func (s *SWIM) Ping(_ context.Context, req *pb.PingRequest) (*pb.Ack, error) {
	s.applyAll(req.GetUpdates())
	s.seen(req.GetFromId())
	return &pb.Ack{FromId: s.selfID(), Ok: true, Updates: s.takeUpdates()}, nil
}

// PingReq pings a target for another member and reports the result.
func (s *SWIM) PingReq(ctx context.Context, req *pb.PingReqRequest) (*pb.Ack, error) {
	s.applyAll(req.GetUpdates())
	s.seen(req.GetFromId())
	ok := s.ping(ctx, req.GetTargetId(), req.GetTargetAddress())
	return &pb.Ack{FromId: s.selfID(), Ok: ok, Updates: s.takeUpdates()}, nil
}

// probeOnce runs one protocol period: direct ping, then indirect pings, then suspect.
func (s *SWIM) probeOnce(ctx context.Context) {
	target, ok := s.nextTarget()
	if !ok {
		return
	}
	s.metrics.PingsSent.Inc()
	if s.ping(ctx, target.ID, target.Address) {
		return
	}
	s.metrics.PingFailures.Inc()
	if s.indirectPing(ctx, target) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.nodes[target.ID]; ok && n.state == pb.MemberState_MEMBER_STATE_ALIVE {
		s.markSuspectLocked(n, n.incarnation)
	}
}

// ping sends one direct ping and reports whether the expected member answered.
func (s *SWIM) ping(ctx context.Context, id, addr string) bool {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.PingTimeout)
	defer cancel()
	ack, err := s.cfg.Transport.Ping(ctx, addr, &pb.PingRequest{FromId: s.selfID(), Updates: s.takeUpdates()})
	if err != nil {
		return false
	}
	s.applyAll(ack.GetUpdates())
	// A different ID at the same address is a new identity, for example a volunteer
	// that registered again. The old identity did not answer.
	if !ack.GetOk() || ack.GetFromId() != id {
		return false
	}
	s.seen(id)
	return true
}

// indirectPing asks helper members to ping the target. It returns true if any helper reached it.
func (s *SWIM) indirectPing(ctx context.Context, target Member) bool {
	helpers := s.pickHelpers(target.ID)
	if len(helpers) == 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 2*s.cfg.PingTimeout)
	defer cancel()
	results := make(chan bool, len(helpers))
	for _, h := range helpers {
		s.metrics.IndirectProbes.Inc()
		req := &pb.PingReqRequest{FromId: s.selfID(), TargetId: target.ID, TargetAddress: target.Address, Updates: s.takeUpdates()}
		go func(addr string) {
			ack, err := s.cfg.Transport.PingReq(ctx, addr, req)
			if err != nil {
				results <- false
				return
			}
			s.applyAll(ack.GetUpdates())
			results <- ack.GetOk()
		}(h.Address)
	}
	for range helpers {
		if <-results {
			s.seen(target.ID)
			return true
		}
	}
	return false
}

// checkSuspects declares dead every member that stayed suspect for SuspectTimeout.
func (s *SWIM) checkSuspects() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.cfg.Now()
	for _, n := range s.nodes {
		if n.state == pb.MemberState_MEMBER_STATE_SUSPECT && now.Sub(n.suspectSince) >= s.cfg.SuspectTimeout {
			s.markDeadLocked(n, n.incarnation)
		}
	}
}

// nextTarget returns the next member to ping in shuffled round-robin order.
func (s *SWIM) nextTarget() (Member, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for tries := 0; tries <= len(s.order); tries++ {
		if s.next >= len(s.order) {
			s.order = s.order[:0]
			for id, n := range s.nodes {
				if probeable(n) {
					s.order = append(s.order, id)
				}
			}
			if len(s.order) == 0 {
				return Member{}, false
			}
			sort.Strings(s.order) // map order is random; sort first so a fixed seed gives a fixed order
			s.rng.Shuffle(len(s.order), func(i, j int) { s.order[i], s.order[j] = s.order[j], s.order[i] })
			s.next = 0
		}
		n := s.nodes[s.order[s.next]]
		s.next++
		if n != nil && probeable(n) {
			return n.Member, true
		}
	}
	return Member{}, false
}

// pickHelpers returns up to IndirectProbes alive members, other than the target, for indirect pings.
func (s *SWIM) pickHelpers(targetID string) []Member {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for id, n := range s.nodes {
		if id != targetID && n.state == pb.MemberState_MEMBER_STATE_ALIVE && n.Address != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	s.rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	if len(ids) > s.cfg.IndirectProbes {
		ids = ids[:s.cfg.IndirectProbes]
	}
	out := make([]Member, len(ids))
	for i, id := range ids {
		out[i] = s.nodes[id].Member
	}
	return out
}

// takeUpdates returns the updates to piggyback on the next message:
// this node's own record first, then the updates with the most sends left.
func (s *SWIM) takeUpdates() []*pb.Member {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*pb.Member{{
		Id: s.self.ID, Address: s.self.Address, Coordinator: s.self.Coordinator,
		State: pb.MemberState_MEMBER_STATE_ALIVE, Incarnation: s.selfInc,
	}}
	ids := make([]string, 0, len(s.queue))
	for id := range s.queue {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if s.queue[ids[i]] != s.queue[ids[j]] {
			return s.queue[ids[i]] > s.queue[ids[j]]
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		if len(out) > maxPiggyback {
			break
		}
		n, ok := s.nodes[id]
		if !ok {
			delete(s.queue, id)
			continue
		}
		out = append(out, &pb.Member{
			Id: n.ID, Address: n.Address, Coordinator: n.Coordinator,
			State: n.state, Incarnation: n.incarnation,
		})
		if s.queue[id]--; s.queue[id] <= 0 {
			delete(s.queue, id)
		}
	}
	return out
}

func (s *SWIM) applyAll(updates []*pb.Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range updates {
		s.applyLocked(u)
	}
}

// applyLocked merges one update with the SWIM precedence rules:
//
//	alive(i)   overrides any state with incarnation < i
//	suspect(i) overrides alive(j) with j <= i, and suspect(j) with j < i
//	dead(i)    overrides alive(j) and suspect(j) with j <= i
func (s *SWIM) applyLocked(u *pb.Member) {
	id := u.GetId()
	if id == "" {
		return
	}
	if id == s.self.ID {
		s.refuteLocked(u)
		return
	}
	n, known := s.nodes[id]
	if !known {
		s.addFromUpdateLocked(u)
		return
	}
	switch u.GetState() {
	case pb.MemberState_MEMBER_STATE_ALIVE:
		if u.GetIncarnation() > n.incarnation {
			wasAlive := n.state == pb.MemberState_MEMBER_STATE_ALIVE
			n.state, n.incarnation = pb.MemberState_MEMBER_STATE_ALIVE, u.GetIncarnation()
			if u.GetAddress() != "" {
				n.Address = u.GetAddress()
			}
			n.Coordinator = n.Coordinator || u.GetCoordinator()
			s.enqueueLocked(id)
			if !wasAlive {
				s.log.Info("member alive again", "member", id, "incarnation", n.incarnation)
				s.updateGaugesLocked()
			}
		}
	case pb.MemberState_MEMBER_STATE_SUSPECT:
		if (n.state == pb.MemberState_MEMBER_STATE_ALIVE && u.GetIncarnation() >= n.incarnation) ||
			(n.state == pb.MemberState_MEMBER_STATE_SUSPECT && u.GetIncarnation() > n.incarnation) {
			s.markSuspectLocked(n, u.GetIncarnation())
		}
	case pb.MemberState_MEMBER_STATE_DEAD:
		if n.state != pb.MemberState_MEMBER_STATE_DEAD && u.GetIncarnation() >= n.incarnation {
			s.markDeadLocked(n, u.GetIncarnation())
		}
	}
}

// refuteLocked answers news about this node: if others think it is suspect or dead, say it is alive.
func (s *SWIM) refuteLocked(u *pb.Member) {
	state := u.GetState()
	if state != pb.MemberState_MEMBER_STATE_SUSPECT && state != pb.MemberState_MEMBER_STATE_DEAD {
		return
	}
	if u.GetIncarnation() < s.selfInc {
		return // old news; the current incarnation already overrules it
	}
	s.selfInc = u.GetIncarnation() + 1
	s.metrics.Refutations.Inc()
	s.log.Info("refuted suspicion", "member", s.self.ID, "incarnation", s.selfInc)
	// The own record goes on every message, so the refutation spreads without a queue entry.
}

func (s *SWIM) addLocked(m Member) {
	now := s.cfg.Now()
	s.nodes[m.ID] = &node{Member: m, state: pb.MemberState_MEMBER_STATE_ALIVE, lastSeen: now}
	s.enqueueLocked(m.ID)
}

func (s *SWIM) addFromUpdateLocked(u *pb.Member) {
	now := s.cfg.Now()
	n := &node{
		Member:      Member{ID: u.GetId(), Address: u.GetAddress(), Coordinator: u.GetCoordinator()},
		state:       u.GetState(),
		incarnation: u.GetIncarnation(),
		lastSeen:    now,
	}
	switch n.state {
	case pb.MemberState_MEMBER_STATE_SUSPECT:
		n.suspectSince = now
	case pb.MemberState_MEMBER_STATE_UNSPECIFIED:
		n.state = pb.MemberState_MEMBER_STATE_ALIVE
	}
	s.nodes[n.ID] = n
	s.enqueueLocked(n.ID)
	s.log.Info("member learned", "member", n.ID, "state", stateName(n.state), "address", n.Address)
	s.updateGaugesLocked()
}

func (s *SWIM) markSuspectLocked(n *node, incarnation uint64) {
	if n.state != pb.MemberState_MEMBER_STATE_SUSPECT {
		n.suspectSince = s.cfg.Now()
	}
	n.state, n.incarnation = pb.MemberState_MEMBER_STATE_SUSPECT, incarnation
	s.enqueueLocked(n.ID)
	s.log.Info("member suspect", "member", n.ID, "incarnation", incarnation)
	s.updateGaugesLocked()
}

func (s *SWIM) markDeadLocked(n *node, incarnation uint64) {
	n.state, n.incarnation = pb.MemberState_MEMBER_STATE_DEAD, incarnation
	s.enqueueLocked(n.ID)
	silent := s.cfg.Now().Sub(n.lastSeen)
	s.metrics.DetectionSeconds.Observe(silent.Seconds())
	s.log.Info("member dead", "member", n.ID, "silent_for", silent.Round(time.Millisecond))
	s.updateGaugesLocked()
}

// enqueueLocked schedules the member's current record for about lambda * log2(n) sends.
func (s *SWIM) enqueueLocked(id string) {
	s.queue[id] = retransmitFactor * ceilLog2(len(s.nodes)+2)
}

func (s *SWIM) seen(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.nodes[id]; ok {
		n.lastSeen = s.cfg.Now()
	}
}

func (s *SWIM) selfID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.self.ID
}

func (s *SWIM) updateGaugesLocked() {
	counts := map[pb.MemberState]int{}
	for _, n := range s.nodes {
		counts[n.state]++
	}
	for _, st := range []pb.MemberState{pb.MemberState_MEMBER_STATE_ALIVE, pb.MemberState_MEMBER_STATE_SUSPECT, pb.MemberState_MEMBER_STATE_DEAD} {
		s.metrics.Members.WithLabelValues(stateName(st)).Set(float64(counts[st]))
	}
}

func probeable(n *node) bool {
	return n.state != pb.MemberState_MEMBER_STATE_DEAD && n.Address != ""
}

func stateName(s pb.MemberState) string {
	switch s {
	case pb.MemberState_MEMBER_STATE_ALIVE:
		return "alive"
	case pb.MemberState_MEMBER_STATE_SUSPECT:
		return "suspect"
	case pb.MemberState_MEMBER_STATE_DEAD:
		return "dead"
	}
	return "unknown"
}

// ceilLog2 returns the smallest k with 2^k >= n, and at least 1.
func ceilLog2(n int) int {
	k := 1
	for 1<<k < n {
		k++
	}
	return k
}
