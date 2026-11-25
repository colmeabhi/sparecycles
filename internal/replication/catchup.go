package replication

import (
	"context"
	"sync"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
)

// CatchUp gives a new leader the newest table before it acts.
//
// Bully picks the highest ID, not the node with the newest table. A confirmed
// change is on a majority, and the new leader reaches a majority, so the newest
// table among the reachable coordinators holds every confirmed change.
// The election calls CatchUp through its BeforeLead hook.
func (q *Quorum) CatchUp(ctx context.Context) error {
	q.backupMu.Lock()
	defer q.backupMu.Unlock()
	q.resetPeers()
	_, term := q.cfg.Elector.Leader()
	q.syncedTerm = term

	best, from := q.newestSnapshot(ctx, term)
	local := q.cfg.Table.Index()
	if best != nil && best.GetIndex() > local {
		if err := q.restore(best, from); err != nil {
			return err
		}
	}
	q.cfg.Log.Info("leader continues from table index", "index", q.cfg.Table.Index(), "term", term)
	return nil
}

// newestSnapshot asks each peer for its table and returns the one with the highest index.
func (q *Quorum) newestSnapshot(ctx context.Context, term uint64) (*pb.SnapshotResponse, int) {
	ctx, cancel := context.WithTimeout(ctx, q.cfg.Timeout)
	defer cancel()
	var mu sync.Mutex
	var best *pb.SnapshotResponse
	from := 0
	var wg sync.WaitGroup
	for _, p := range q.peers {
		wg.Go(func() {
			resp, err := q.cfg.Transport.Snapshot(ctx, p.addr, &pb.SnapshotRequest{Term: term})
			if err != nil {
				q.cfg.Log.Debug("catch-up: peer did not answer", "peer", p.id, "err", err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if best == nil || resp.GetIndex() > best.GetIndex() {
				best, from = resp, p.id
			}
		})
	}
	wg.Wait()
	return best, from
}
