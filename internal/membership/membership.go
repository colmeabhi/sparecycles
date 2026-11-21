// Package membership tracks which nodes are alive.
//
// Owner: Kushagra (gossip and failure detection, SWIM style).
// The base version thinks that every node it knows is alive.
// The real version pings random members, uses indirect pings through other
// members, and spreads alive, suspect and dead updates by gossip.
package membership

import (
	"context"
	"sync"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
)

// Member is one node in the group.
type Member struct {
	ID          string
	Address     string
	Coordinator bool
}

// Membership tracks the group.
type Membership interface {
	// GossipServiceServer handles pings from other members.
	pb.GossipServiceServer
	// Start runs failure detection in the background until ctx ends.
	Start(ctx context.Context) error
	// Join adds a member, for example a new volunteer.
	Join(m Member)
	// Alive reports whether a member is alive now.
	Alive(id string) bool
	// Members returns all members that are not dead.
	Members() []Member
}

// Static keeps every member alive forever. It is the base stub.
type Static struct {
	pb.UnimplementedGossipServiceServer
	mu      sync.RWMutex
	members map[string]Member
}

// NewStatic returns an empty Static membership.
func NewStatic() *Static {
	return &Static{members: map[string]Member{}}
}

// Start does nothing.
func (s *Static) Start(context.Context) error { return nil }

// Join adds a member.
func (s *Static) Join(m Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members[m.ID] = m
}

// Alive reports whether the member was added.
func (s *Static) Alive(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.members[id]
	return ok
}

// Members returns all added members.
func (s *Static) Members() []Member {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Member, 0, len(s.members))
	for _, m := range s.members {
		out = append(out, m)
	}
	return out
}
