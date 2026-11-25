// Package peerconn keeps one gRPC connection to each other coordinator.
//
// Election and replication share one Pool, so that each pair of coordinators uses one connection.
package peerconn

import (
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

// MaxMessageBytes is the largest gRPC message between coordinators and to clients.
// A table snapshot holds every result of every job, and a finished job holds the
// merged image, so both can pass the 4 MB gRPC default.
const MaxMessageBytes = 256 << 20

// ServerOptions returns the gRPC server options that accept messages up to MaxMessageBytes.
func ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.MaxRecvMsgSize(MaxMessageBytes), grpc.MaxSendMsgSize(MaxMessageBytes)}
}

// Pool holds one lazy connection per address. It is safe for concurrent use.
type Pool struct {
	mu    sync.Mutex
	conns map[string]*grpc.ClientConn
}

// New returns an empty Pool.
func New() *Pool {
	return &Pool{conns: map[string]*grpc.ClientConn{}}
}

// Conn returns the connection to addr, and creates it on first use.
func (p *Pool) Conn(addr string) (*grpc.ClientConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if conn, ok := p.conns[addr]; ok {
		return conn, nil
	}
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageBytes), grpc.MaxCallSendMsgSize(MaxMessageBytes)),
		// Retry connections quickly, so a coordinator that comes back is reachable again within about a second.
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoff.Config{BaseDelay: 100 * time.Millisecond, Multiplier: 1.6, Jitter: 0.2, MaxDelay: time.Second},
			MinConnectTimeout: 500 * time.Millisecond,
		}))
	if err != nil {
		return nil, err
	}
	p.conns[addr] = conn
	return conn, nil
}

// Close closes all connections.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for addr, c := range p.conns {
		_ = c.Close() // best effort on shutdown
		delete(p.conns, addr)
	}
	return nil
}
