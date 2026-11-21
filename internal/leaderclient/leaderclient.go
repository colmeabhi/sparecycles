// Package leaderclient sends RPCs to the current leader.
//
// A Pool holds one connection to each coordinator. It sends each call to the
// coordinator that answered last. If that coordinator is down or is not the
// leader, the Pool tries the next one. This also handles a leader failover.
package leaderclient

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// Pool is a set of coordinator connections.
type Pool struct {
	conns []*grpc.ClientConn
	mu    sync.Mutex
	cur   int
}

// Dial connects to each coordinator address. The connections are lazy.
func Dial(addrs []string) (*Pool, error) {
	if len(addrs) == 0 {
		return nil, errors.New("no coordinator addresses")
	}
	conns := make([]*grpc.ClientConn, 0, len(addrs))
	for _, a := range addrs {
		c, err := grpc.NewClient(a, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			for _, open := range conns {
				_ = open.Close()
			}
			return nil, fmt.Errorf("dial %s: %w", a, err)
		}
		conns = append(conns, c)
	}
	return &Pool{conns: conns}, nil
}

// FromConns returns a Pool that uses existing connections. Tests use it.
func FromConns(conns ...*grpc.ClientConn) *Pool {
	return &Pool{conns: conns}
}

// Close closes all connections.
func (p *Pool) Close() error {
	var errs []error
	for _, c := range p.conns {
		errs = append(errs, c.Close())
	}
	return errors.Join(errs...)
}

// Do calls fn with a connection to the leader.
// It tries each coordinator once, starting with the last one that worked.
func (p *Pool) Do(ctx context.Context, fn func(conn *grpc.ClientConn) error) error {
	p.mu.Lock()
	start := p.cur
	p.mu.Unlock()

	var last error
	for i := 0; i < len(p.conns); i++ {
		idx := (start + i) % len(p.conns)
		err := fn(p.conns[idx])
		if !retryOnNext(err) {
			if err == nil {
				p.mu.Lock()
				p.cur = idx
				p.mu.Unlock()
			}
			return err
		}
		last = err
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return fmt.Errorf("no leader reachable: %w", last)
}

// retryOnNext reports whether the error means "try a different coordinator".
func retryOnNext(err error) bool {
	switch status.Code(err) {
	case codes.Unavailable, codes.FailedPrecondition:
		return true
	}
	return false
}
