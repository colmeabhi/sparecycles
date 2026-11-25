package replication

import (
	"context"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/peerconn"
)

// Transport sends replication messages to other coordinators by address.
// Tests replace it with an in-memory network.
type Transport interface {
	Replicate(ctx context.Context, addr string, req *pb.ReplicateRequest) (*pb.ReplicateResponse, error)
	Snapshot(ctx context.Context, addr string, req *pb.SnapshotRequest) (*pb.SnapshotResponse, error)
}

// GRPCTransport sends replication messages over gRPC, on the shared coordinator connections.
type GRPCTransport struct {
	pool *peerconn.Pool
}

// NewGRPCTransport returns a GRPCTransport that uses the connections in pool.
func NewGRPCTransport(pool *peerconn.Pool) *GRPCTransport {
	return &GRPCTransport{pool: pool}
}

// Replicate sends one change to the backup at addr.
func (t *GRPCTransport) Replicate(ctx context.Context, addr string, req *pb.ReplicateRequest) (*pb.ReplicateResponse, error) {
	conn, err := t.pool.Conn(addr)
	if err != nil {
		return nil, err
	}
	return pb.NewReplicationServiceClient(conn).Replicate(ctx, req)
}

// Snapshot asks the coordinator at addr for its full table.
func (t *GRPCTransport) Snapshot(ctx context.Context, addr string, req *pb.SnapshotRequest) (*pb.SnapshotResponse, error) {
	conn, err := t.pool.Conn(addr)
	if err != nil {
		return nil, err
	}
	return pb.NewReplicationServiceClient(conn).Snapshot(ctx, req)
}
