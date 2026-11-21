// Package election chooses the leader among the coordinators.
//
// Owner: Abhishek (leader election, Bully algorithm with terms).
// The base version uses a fixed leader: the coordinator with the highest node ID.
// It never changes the leader, so a leader crash stops the system.
// The real version runs the Bully algorithm, uses term numbers, and needs 2 of 3
// coordinators to act, so that two leaders cannot act at the same time.
package election

import (
	"context"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
)

// Elector tracks who the leader is.
type Elector interface {
	// ElectionServiceServer handles election messages from other coordinators.
	pb.ElectionServiceServer
	// Start runs the election in the background until ctx ends.
	Start(ctx context.Context) error
	// IsLeader reports whether this node is the leader now.
	IsLeader() bool
	// Leader returns the current leader ID and term.
	Leader() (id int, term uint64)
}

// Static always picks the highest node ID as the leader. It is the base stub.
type Static struct {
	pb.UnimplementedElectionServiceServer
	self   int
	leader int
}

// NewStatic returns a Static elector for node self with the given peer IDs.
func NewStatic(self int, peers map[int]string) *Static {
	leader := self
	for id := range peers {
		if id > leader {
			leader = id
		}
	}
	return &Static{self: self, leader: leader}
}

// Start does nothing.
func (s *Static) Start(context.Context) error { return nil }

// IsLeader reports whether this node has the highest ID.
func (s *Static) IsLeader() bool { return s.self == s.leader }

// Leader returns the highest ID and term 1.
func (s *Static) Leader() (int, uint64) { return s.leader, 1 }
