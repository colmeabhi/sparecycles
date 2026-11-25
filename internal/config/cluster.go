package config

import (
	"fmt"
	"time"
)

// Election holds the leader election settings.
type Election struct {
	Heartbeat time.Duration // time between two leader heartbeats
	Timeout   time.Duration // a coordinator that hears no leader for this long starts an election
}

func loadElection() (Election, error) {
	var e Election
	var err error
	if e.Heartbeat, err = durationVar("SC_ELECTION_HEARTBEAT", 500*time.Millisecond); err != nil {
		return e, err
	}
	if e.Timeout, err = durationVar("SC_ELECTION_TIMEOUT", 2*time.Second); err != nil {
		return e, err
	}
	if e.Heartbeat <= 0 || e.Timeout <= 2*e.Heartbeat {
		return e, fmt.Errorf("SC_ELECTION_TIMEOUT (%s) must be more than 2 times SC_ELECTION_HEARTBEAT (%s), and both positive", e.Timeout, e.Heartbeat)
	}
	return e, nil
}
