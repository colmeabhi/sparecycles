package membership

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
)

// Transport sends gossip messages to other members by address.
// Tests replace it with an in-memory network.
type Transport interface {
	Ping(ctx context.Context, addr string, req *pb.PingRequest) (*pb.Ack, error)
	PingReq(ctx context.Context, addr string, req *pb.PingReqRequest) (*pb.Ack, error)
}

// GRPCTransport sends gossip messages over gRPC. It keeps one connection per address.
type GRPCTransport struct {
	mu    sync.Mutex
	conns map[string]*grpc.ClientConn
}

// NewGRPCTransport returns an empty GRPCTransport.
func NewGRPCTransport() *GRPCTransport {
	return &GRPCTransport{conns: map[string]*grpc.ClientConn{}}
}

// Ping sends a direct ping.
func (t *GRPCTransport) Ping(ctx context.Context, addr string, req *pb.PingRequest) (*pb.Ack, error) {
	client, err := t.client(addr)
	if err != nil {
		return nil, err
	}
	return client.Ping(ctx, req)
}

// PingReq asks the member at addr to ping a target for us.
func (t *GRPCTransport) PingReq(ctx context.Context, addr string, req *pb.PingReqRequest) (*pb.Ack, error) {
	client, err := t.client(addr)
	if err != nil {
		return nil, err
	}
	return client.PingReq(ctx, req)
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

func (t *GRPCTransport) client(addr string) (pb.GossipServiceClient, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	conn, ok := t.conns[addr]
	if !ok {
		var err error
		conn, err = grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			// Retry connections quickly, so a member that comes back is reachable again within about a second.
			grpc.WithConnectParams(grpc.ConnectParams{
				Backoff:           backoff.Config{BaseDelay: 100 * time.Millisecond, Multiplier: 1.6, Jitter: 0.2, MaxDelay: time.Second},
				MinConnectTimeout: 500 * time.Millisecond,
			}))
		if err != nil {
			return nil, err
		}
		t.conns[addr] = conn
	}
	return pb.NewGossipServiceClient(conn), nil
}
