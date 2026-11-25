package replication

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/tasktable"
)

// Replicate applies one change from the leader. Changes apply in index order.
//
//   - A change from an older term is rejected.
//   - The first change from a new leader makes this node load the leader's table first,
//     because the old leader may have left changes here that the new leader does not have.
//   - A gap, or a different change at the last index, also makes this node load the leader's table.
func (q *Quorum) Replicate(ctx context.Context, req *pb.ReplicateRequest) (*pb.ReplicateResponse, error) {
	_, term := q.cfg.Elector.Leader()
	if req.GetTerm() < term {
		return q.answer(false, term), nil
	}
	q.backupMu.Lock()
	defer q.backupMu.Unlock()
	if req.GetTerm() > q.syncedTerm {
		if err := q.resync(ctx, req); err != nil {
			return q.answer(false, term), nil
		}
	}
	ok, err := q.applyEntry(req)
	if err != nil {
		return nil, err
	}
	if !ok {
		// One repair, then try again: the snapshot may already hold this change.
		if err := q.resync(ctx, req); err != nil {
			return q.answer(false, term), nil
		}
		if ok, err = q.applyEntry(req); err != nil {
			return nil, err
		}
	}
	return q.answer(ok, term), nil
}

// applyEntry applies the change if it is the next one. It returns false if the
// table must load a snapshot first.
func (q *Quorum) applyEntry(req *pb.ReplicateRequest) (bool, error) {
	local := q.cfg.Table.Index()
	hash := tasktable.Hash(req.GetChange())
	switch {
	case req.GetIndex() == local+1:
		c, err := tasktable.DecodeChange(req.GetChange())
		if err != nil {
			return false, status.Errorf(codes.InvalidArgument, "decode change %d: %v", req.GetIndex(), err)
		}
		if err := q.cfg.Table.Apply(c); err != nil {
			q.cfg.Log.Warn("backup cannot apply change", "index", req.GetIndex(), "err", err)
			return false, nil
		}
		q.lastHash = hash
		return true, nil
	case req.GetIndex() < local, req.GetIndex() == local && (q.lastHash == "" || q.lastHash == hash):
		return true, nil // a retry of a change this node has
	case req.GetIndex() == local:
		q.cfg.Log.Info("backup has a different change", "index", local)
		return false, nil
	default:
		q.cfg.Log.Info("backup behind", "leader_index", req.GetIndex()-1, "backup_index", local)
		return false, nil
	}
}

// resync loads the table of the leader that sent req.
func (q *Quorum) resync(ctx context.Context, req *pb.ReplicateRequest) error {
	addr, ok := q.cfg.Peers[int(req.GetLeaderId())]
	if !ok {
		return fmt.Errorf("unknown leader %d", req.GetLeaderId())
	}
	resp, err := q.cfg.Transport.Snapshot(ctx, addr, &pb.SnapshotRequest{Term: req.GetTerm()})
	if err != nil {
		q.cfg.Log.Warn("snapshot from leader failed", "leader", req.GetLeaderId(), "err", err)
		return err
	}
	if err := q.restore(resp, int(req.GetLeaderId())); err != nil {
		return err
	}
	q.syncedTerm = req.GetTerm()
	return nil
}

// restore loads a snapshot into the table.
func (q *Quorum) restore(resp *pb.SnapshotResponse, from int) error {
	if err := q.cfg.Table.Restore(resp.GetTable()); err != nil {
		q.cfg.Log.Warn("snapshot restore failed", "from", from, "err", err)
		return fmt.Errorf("restore snapshot from node %d: %w", from, err)
	}
	q.lastHash = ""
	q.cfg.Metrics.SnapshotsRestored.Inc()
	q.cfg.Log.Info("snapshot restored", "from", from, "index", q.cfg.Table.Index())
	return nil
}

func (q *Quorum) answer(ok bool, term uint64) *pb.ReplicateResponse {
	return &pb.ReplicateResponse{Ok: ok, Term: term, LastIndex: q.cfg.Table.Index()}
}

// Snapshot returns the full table of this node.
func (q *Quorum) Snapshot(_ context.Context, _ *pb.SnapshotRequest) (*pb.SnapshotResponse, error) {
	_, term := q.cfg.Elector.Leader()
	// Read the index first: the table can only grow, so the answer never claims more than the bytes hold.
	index := q.cfg.Table.Index()
	table, err := q.cfg.Table.Snapshot()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "snapshot: %v", err)
	}
	q.cfg.Metrics.SnapshotsServed.Inc()
	return &pb.SnapshotResponse{Term: term, Index: index, Table: table}, nil
}
