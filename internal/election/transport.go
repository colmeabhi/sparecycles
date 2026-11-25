package election

import (
	"context"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/peerconn"
)

// Transport sends election messages to other coordinators by address.
// Tests replace it with an in-memory network.
type Transport interface {
	Election(ctx context.Context, addr string, req *pb.ElectionRequest) (*pb.ElectionResponse, error)
	Coordinator(ctx context.Context, addr string, req *pb.CoordinatorRequest) (*pb.CoordinatorResponse, error)
	Heartbeat(ctx context.Context, addr string, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error)
}

// GRPCTransport sends election messages over gRPC, on the shared coordinator connections.
type GRPCTransport struct {
	pool *peerconn.Pool
}

// NewGRPCTransport returns a GRPCTransport that uses the connections in pool.
func NewGRPCTransport(pool *peerconn.Pool) *GRPCTransport {
	return &GRPCTransport{pool: pool}
}

// Election asks the node at addr if it is alive.
func (t *GRPCTransport) Election(ctx context.Context, addr string, req *pb.ElectionRequest) (*pb.ElectionResponse, error) {
	conn, err := t.pool.Conn(addr)
	if err != nil {
		return nil, err
	}
	return pb.NewElectionServiceClient(conn).Election(ctx, req)
}

// Coordinator tells the node at addr that the sender won the election.
func (t *GRPCTransport) Coordinator(ctx context.Context, addr string, req *pb.CoordinatorRequest) (*pb.CoordinatorResponse, error) {
	conn, err := t.pool.Conn(addr)
	if err != nil {
		return nil, err
	}
	return pb.NewElectionServiceClient(conn).Coordinator(ctx, req)
}

// Heartbeat tells the node at addr that the leader is alive.
func (t *GRPCTransport) Heartbeat(ctx context.Context, addr string, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	conn, err := t.pool.Conn(addr)
	if err != nil {
		return nil, err
	}
	return pb.NewElectionServiceClient(conn).Heartbeat(ctx, req)
}
