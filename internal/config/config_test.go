package config

import (
	"testing"
	"time"
)

func TestLoadCoordinatorDefaults(t *testing.T) {
	t.Setenv("SC_NODE_ID", "")
	t.Setenv("SC_PEERS", "")
	t.Setenv("SC_LEASE_DURATION", "")
	t.Setenv("SC_VOTE_REPLICAS", "")

	c, err := LoadCoordinator()
	if err != nil {
		t.Fatal(err)
	}
	if c.NodeID != 1 || c.LeaseDuration != 3*time.Minute || c.VoteReplicas != 3 || len(c.Peers) != 0 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadCoordinatorPeers(t *testing.T) {
	t.Setenv("SC_PEERS", "2=c2:7400, 3=c3:7400")

	c, err := LoadCoordinator()
	if err != nil {
		t.Fatal(err)
	}
	if c.Peers[2] != "c2:7400" || c.Peers[3] != "c3:7400" {
		t.Fatalf("unexpected peers: %v", c.Peers)
	}
}

func TestGossipDefaults(t *testing.T) {
	c, err := LoadCoordinator()
	if err != nil {
		t.Fatal(err)
	}
	g := c.Gossip
	if g.Interval != time.Second || g.PingTimeout != 400*time.Millisecond || g.SuspectTimeout != 5*time.Second || g.IndirectProbes != 3 {
		t.Fatalf("unexpected gossip defaults: %+v", g)
	}
	if c.AdvertiseAddr != "127.0.0.1:7400" {
		t.Fatalf("advertise = %q, want 127.0.0.1:7400", c.AdvertiseAddr)
	}

	v, err := LoadVolunteer()
	if err != nil {
		t.Fatal(err)
	}
	if v.GossipAddr != ":0" || v.GossipHost != "127.0.0.1" {
		t.Fatalf("volunteer gossip = %q on host %q, want :0 on 127.0.0.1", v.GossipAddr, v.GossipHost)
	}
}

func TestElectionDefaults(t *testing.T) {
	c, err := LoadCoordinator()
	if err != nil {
		t.Fatal(err)
	}
	if c.Election.Heartbeat != 500*time.Millisecond || c.Election.Timeout != 2*time.Second {
		t.Fatalf("unexpected election defaults: %+v", c.Election)
	}
}

func TestClientDefaultReachesComposeCoordinators(t *testing.T) {
	c := LoadClient()
	if len(c.Coordinators) != 4 || c.Coordinators[0] != "localhost:7400" || c.Coordinators[3] != "localhost:7403" {
		t.Fatalf("client coordinators = %v, want localhost:7400 to 7403", c.Coordinators)
	}
}

func TestLoadCoordinatorBadValues(t *testing.T) {
	cases := map[string]string{
		"SC_NODE_ID":                "one",
		"SC_PEERS":                  "c2:7400",
		"SC_LEASE_DURATION":         "3",
		"SC_VOTE_REPLICAS":          "0",
		"SC_GOSSIP_INTERVAL":        "0s",
		"SC_GOSSIP_PING_TIMEOUT":    "2s", // not less than the 1s interval
		"SC_GOSSIP_SUSPECT_TIMEOUT": "-1s",
		"SC_GOSSIP_INDIRECT":        "-1",
		"SC_ELECTION_HEARTBEAT":     "1s", // the 2s timeout is not more than 2 heartbeats
		"SC_ELECTION_TIMEOUT":       "soon",
	}
	for key, value := range cases {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, value)
			if _, err := LoadCoordinator(); err == nil {
				t.Fatalf("%s=%q: expected an error", key, value)
			}
		})
	}
}
