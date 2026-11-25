package replication

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/metrics"
	"github.com/colmeabhi/sparecycles/internal/tasktable"
)

// Leadership tells who leads in which term. election.Elector implements it.
type Leadership interface {
	Leader() (id int, term uint64)
}

// QuorumConfig holds the settings of a Quorum replicator.
type QuorumConfig struct {
	Self      int
	Peers     map[int]string // other coordinators, by node ID
	Table     *tasktable.Table
	Elector   Leadership
	Transport Transport
	Timeout   time.Duration        // time a leader waits for the backups for one change
	Metrics   *metrics.Replication // nil means metrics that nobody reads
	Log       *slog.Logger
	Now       func() time.Time // for tests; nil means time.Now
}

// Quorum copies each change to the backups. The leader confirms a change when
// a majority of all coordinators has it (with 3 coordinators: the leader and 1 backup).
//
// The same type is the backup side: it applies the changes that arrive from the
// leader, in index order, and loads a snapshot from the leader when it is behind.
type Quorum struct {
	pb.UnimplementedReplicationServiceServer
	cfg   QuorumConfig
	peers []*peer
	need  int // backups that must have a change before the leader confirms it

	mu   sync.Mutex
	last entry // leader: the last confirmed change, to repair a backup that may hold a change the leader dropped

	backupMu   sync.Mutex // backup: one change at a time, in index order
	syncedTerm uint64     // backup: term of the leader whose table this node copied
	lastHash   string     // backup: hash of the last applied change; empty after a snapshot
}

// entry is one change in the log.
type entry struct {
	term   uint64
	index  uint64
	change []byte
}

// NewQuorum returns a Quorum replicator.
func NewQuorum(cfg QuorumConfig) (*Quorum, error) {
	switch {
	case cfg.Table == nil || cfg.Elector == nil:
		return nil, errors.New("replication needs a table and an elector")
	case cfg.Timeout <= 0:
		return nil, errors.New("replication timeout must be positive")
	case len(cfg.Peers) > 0 && cfg.Transport == nil:
		return nil, errors.New("replication transport is needed when there are peers")
	}
	if cfg.Metrics == nil {
		cfg.Metrics = metrics.NewReplication(prometheus.NewRegistry())
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	q := &Quorum{cfg: cfg, need: (len(cfg.Peers) + 1) / 2}
	for id, addr := range cfg.Peers {
		q.peers = append(q.peers, newPeer(id, addr))
	}
	return q, nil
}

// outcome is what one backup did with one change.
type outcome int

const (
	applied    outcome = iota // the backup has the change
	notApplied                // the backup said no, or the leader did not send it
	unknown                   // no answer: the backup may or may not have the change
)

// Send copies one change to the backups. It returns nil as soon as enough
// backups have it. Slower backups keep receiving it in the background.
func (q *Quorum) Send(ctx context.Context, index uint64, change []byte) error {
	if len(q.peers) == 0 {
		return nil
	}
	leaderID, term := q.cfg.Elector.Leader()
	if leaderID != q.cfg.Self {
		return fmt.Errorf("node %d does not lead term %d", q.cfg.Self, term)
	}
	start := q.cfg.Now()
	// The calls may outlive Send, so they keep their own deadline and ignore ctx cancellation.
	callCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), q.cfg.Timeout)
	e := entry{term: term, index: index, change: change}
	// Read the previous change now: a fast backup can confirm e, and replace q.last,
	// before the call to a slow backup starts.
	prev := q.lastEntry()
	results := make(chan peerResult, len(q.peers))
	var wg sync.WaitGroup
	for _, p := range q.peers {
		wg.Go(func() { results <- peerResult{p, q.sendTo(callCtx, p, prev, e)} })
	}
	go func() { wg.Wait(); cancel() }()

	var got []peerResult
	acks := 0
	for range q.peers {
		r := <-results
		got = append(got, r)
		if r.outcome == applied {
			acks++
		}
		if acks >= q.need {
			q.confirm(e, start)
			return nil
		}
	}
	q.markDirty(index, got)
	return fmt.Errorf("index %d: %d of %d backups have it, need %d", index, acks, len(q.peers), q.need)
}

type peerResult struct {
	peer    *peer
	outcome outcome
}

func (q *Quorum) confirm(e entry, start time.Time) {
	q.mu.Lock()
	q.last = e
	q.mu.Unlock()
	q.cfg.Metrics.Seconds.Observe(q.cfg.Now().Sub(start).Seconds())
}

// markDirty remembers the backups that may hold a change that the leader now drops.
// The leader repairs them before it sends them the next index.
func (q *Quorum) markDirty(index uint64, results []peerResult) {
	for _, r := range results {
		if r.outcome != notApplied {
			r.peer.setDirty(index)
		}
	}
}

// sendTo sends e to one backup, after any repair the backup needs.
// prev is the confirmed change just before e.
func (q *Quorum) sendTo(ctx context.Context, p *peer, prev, e entry) outcome {
	if !p.lock(ctx) {
		return notApplied // the earlier calls to this backup took the whole timeout
	}
	defer p.unlock()
	if p.skip(q.cfg.Now()) {
		return notApplied // the backup did not answer recently; it loads a snapshot when it is back
	}
	if d := p.dirtyIndex(); d != 0 && d+1 == e.index && prev.index == d {
		// The backup may hold a dropped change at d. Send the confirmed change at d
		// first: the backup sees that it differs and loads a snapshot.
		if out := q.call(ctx, p, entry{term: e.term, index: prev.index, change: prev.change}); out != applied {
			return notApplied
		}
	}
	return q.call(ctx, p, e)
}

func (q *Quorum) lastEntry() entry {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.last
}

// call sends one Replicate and reads the answer.
func (q *Quorum) call(ctx context.Context, p *peer, e entry) outcome {
	resp, err := q.cfg.Transport.Replicate(ctx, p.addr, &pb.ReplicateRequest{
		Term: e.term, LeaderId: int32(q.cfg.Self), Index: e.index, Change: e.change,
	})
	if err != nil {
		p.markDown(q.cfg.Now().Add(q.cfg.Timeout))
		q.cfg.Log.Debug("replicate failed", "peer", p.id, "index", e.index, "err", err)
		return unknown
	}
	p.markUp()
	if resp.GetTerm() > e.term {
		return notApplied // a newer leader exists; the elector learns it from heartbeats
	}
	lag := e.index - min(resp.GetLastIndex(), e.index)
	q.cfg.Metrics.BackupLag.WithLabelValues(strconv.Itoa(p.id)).Set(float64(lag))
	if !resp.GetOk() {
		q.cfg.Log.Info("backup behind", "peer", p.id, "leader_index", e.index-1, "backup_index", resp.GetLastIndex())
		return notApplied
	}
	p.setDirty(0)
	return applied
}

// resetPeers forgets what the leader knew about each backup. A new leader calls it.
func (q *Quorum) resetPeers() {
	for _, p := range q.peers {
		p.reset()
	}
	q.mu.Lock()
	q.last = entry{}
	q.mu.Unlock()
}
