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

// Replication holds the task-table replication settings.
type Replication struct {
	Timeout time.Duration // time the leader waits for the backups for one table change
}

func loadReplication() (Replication, error) {
	var r Replication
	var err error
	if r.Timeout, err = durationVar("SC_REPLICATION_TIMEOUT", time.Second); err != nil {
		return r, err
	}
	if r.Timeout <= 0 {
		return r, fmt.Errorf("SC_REPLICATION_TIMEOUT must be positive, got %s", r.Timeout)
	}
	return r, nil
}
