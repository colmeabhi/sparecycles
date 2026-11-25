package replication

import (
	"context"
	"sync"
	"time"
)

// peer is what the leader knows about one backup.
type peer struct {
	id   int
	addr string
	turn chan struct{} // one call at a time, so the backup gets the changes in order

	mu        sync.Mutex
	downUntil time.Time // skip the backup until this time after a failed call
	dirty     uint64    // index of a dropped change the backup may hold; 0 if none
}

func newPeer(id int, addr string) *peer {
	return &peer{id: id, addr: addr, turn: make(chan struct{}, 1)}
}

// lock waits for this backup's turn. It returns false if ctx ends first.
func (p *peer) lock(ctx context.Context) bool {
	select {
	case p.turn <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (p *peer) unlock() { <-p.turn }

// skip reports whether the backup failed recently. After downUntil, one call tries it again.
func (p *peer) skip(now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return now.Before(p.downUntil)
}

func (p *peer) markDown(until time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.downUntil = until
}

func (p *peer) markUp() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.downUntil = time.Time{}
}

func (p *peer) dirtyIndex() uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dirty
}

func (p *peer) setDirty(index uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dirty = index
}

func (p *peer) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.downUntil, p.dirty = time.Time{}, 0
}
