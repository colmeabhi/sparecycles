package membership

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
)

// TestGRPCClusterDetectsFailure runs 5 real SWIM nodes over gRPC on localhost,
// stops one, and checks that every other node marks it dead.
func TestGRPCClusterDetectsFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("real network test")
	}
	const size = 5
	type running struct {
		swim   *SWIM
		srv    *grpc.Server
		cancel context.CancelFunc
		tr     *GRPCTransport
	}

	listeners := make([]net.Listener, size)
	members := make([]Member, size)
	for i := range listeners {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		listeners[i] = lis
		members[i] = Member{ID: fmt.Sprintf("n%d", i+1), Address: lis.Addr().String()}
	}

	nodes := make([]running, size)
	for i, self := range members {
		var seeds []Member
		for j, m := range members {
			if j != i {
				seeds = append(seeds, m)
			}
		}
		tr := NewGRPCTransport()
		s := NewSWIM(SWIMConfig{
			Self: self, Seeds: seeds, Transport: tr,
			Interval: 100 * time.Millisecond, PingTimeout: 50 * time.Millisecond,
			SuspectTimeout: 600 * time.Millisecond, IndirectProbes: 2, Log: discard,
		})
		srv := grpc.NewServer()
		pb.RegisterGossipServiceServer(srv, s)
		go func(lis net.Listener) { _ = srv.Serve(lis) }(listeners[i])
		ctx, cancel := context.WithCancel(context.Background())
		if err := s.Start(ctx); err != nil {
			t.Fatal(err)
		}
		nodes[i] = running{swim: s, srv: srv, cancel: cancel, tr: tr}
	}
	t.Cleanup(func() {
		for _, n := range nodes {
			n.cancel()
			n.srv.Stop()
			_ = n.tr.Close()
		}
	})

	time.Sleep(500 * time.Millisecond) // let the group settle
	victim := nodes[size-1]
	victim.cancel()
	victim.srv.Stop()

	deadline := time.Now().Add(10 * time.Second)
	for {
		allSee := true
		for _, n := range nodes[:size-1] {
			if !n.swim.Dead(members[size-1].ID) {
				allSee = false
			}
		}
		if allSee {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("not every node marked the stopped node dead within 10s")
		}
		time.Sleep(50 * time.Millisecond)
	}

	for _, n := range nodes[:size-1] {
		for _, m := range members[:size-1] {
			if n.swim.Dead(m.ID) {
				t.Fatalf("%s wrongly marked live node %s dead", n.swim.self.ID, m.ID)
			}
		}
	}
}
