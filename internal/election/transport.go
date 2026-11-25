package election

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
)

// Transport sends election messages to other coordinators by address.
// Tests replace it with an in-memory network.
type Transport interface {
	Election(ctx context.Context, addr string, req *pb.ElectionRequest) (*pb.ElectionResponse, error)
	Coordinator(ctx context.Context, addr string, req *pb.CoordinatorRequest) (*pb.CoordinatorResponse, error)
	Heartbeat(ctx context.Context, addr string, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error)
}

// GRPCTransport sends election messages over gRPC. It keeps one connection per address.
type GRPCTransport struct {
	mu    sync.Mutex
	conns map[string]*grpc.ClientConn
}

// NewGRPCTransport returns an empty GRPCTransport.
func NewGRPCTransport() *GRPCTransport {
	return &GRPCTransport{conns: map[string]*grpc.ClientConn{}}
}

// Election asks the node at addr if it is alive.
func (t *GRPCTransport) Election(ctx context.Context, addr string, req *pb.ElectionRequest) (*pb.ElectionResponse, error) {
	client, err := t.client(addr)
	if err != nil {
		return nil, err
	}
	return client.Election(ctx, req)
}

// Coordinator tells the node at addr that the sender won the election.
func (t *GRPCTransport) Coordinator(ctx context.Context, addr string, req *pb.CoordinatorRequest) (*pb.CoordinatorResponse, error) {
	client, err := t.client(addr)
	if err != nil {
		return nil, err
	}
	return client.Coordinator(ctx, req)
}

// Heartbeat tells the node at addr that the leader is alive.
func (t *GRPCTransport) Heartbeat(ctx context.Context, addr string, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	client, err := t.client(addr)
	if err != nil {
		return nil, err
	}
	return client.Heartbeat(ctx, req)
}

// Close closes all connections.
func (t *GRPCTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for addr, c := range t.conns {
		_ = c.Close() // best effort on shutdown
		delete(t.conns, addr)
	}
	return nil
}

func (t *GRPCTransport) client(addr string) (pb.ElectionServiceClient, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	conn, ok := t.conns[addr]
	if !ok {
		var err error
		conn, err = grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			// Retry connections quickly, so a coordinator that comes back joins the election within about a second.
			grpc.WithConnectParams(grpc.ConnectParams{
				Backoff:           backoff.Config{BaseDelay: 100 * time.Millisecond, Multiplier: 1.6, Jitter: 0.2, MaxDelay: time.Second},
				MinConnectTimeout: 500 * time.Millisecond,
			}))
		if err != nil {
			return nil, err
		}
		t.conns[addr] = conn
	}
	return pb.NewElectionServiceClient(conn), nil
}
