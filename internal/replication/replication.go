// Package replication copies task-table changes from the leader to the backups.
//
// Owner: Abhishek (replication and consistency).
// The base version copies nothing, so a new leader starts with an empty table.
// The real version sends each change to the backups and returns when 1 backup
// has it (2 of 3 copies). Backups apply the changes to their own table.
package replication

import (
	"context"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
)

// Replicator sends changes to the backups.
type Replicator interface {
	// ReplicationServiceServer handles changes that arrive from the leader.
	pb.ReplicationServiceServer
	// Send sends one encoded change with its log index to the backups.
	// It returns nil when enough backups have the change.
	// The leader applies the change to its own table only after nil.
	Send(ctx context.Context, index uint64, change []byte) error
}

// Noop copies nothing. It is the base stub.
type Noop struct {
	pb.UnimplementedReplicationServiceServer
}

// Send returns nil at once.
func (Noop) Send(context.Context, uint64, []byte) error { return nil }
