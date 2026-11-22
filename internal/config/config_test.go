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

func TestVoteQuorumDefaultsToMajority(t *testing.T) {
	cases := map[string]int{"1": 1, "2": 2, "3": 2, "4": 3, "5": 3}
	for replicas, want := range cases {
		t.Run("replicas="+replicas, func(t *testing.T) {
			t.Setenv("SC_VOTE_REPLICAS", replicas)
			t.Setenv("SC_VOTE_QUORUM", "")
			c, err := LoadCoordinator()
			if err != nil {
				t.Fatal(err)
			}
			if c.VoteQuorum != want {
				t.Fatalf("quorum = %d, want %d", c.VoteQuorum, want)
			}
		})
	}
}

func TestVoteQuorumBadValues(t *testing.T) {
	for _, quorum := range []string{"0", "-1", "4", "two"} {
		t.Run(quorum, func(t *testing.T) {
			t.Setenv("SC_VOTE_REPLICAS", "3")
			t.Setenv("SC_VOTE_QUORUM", quorum)
			if _, err := LoadCoordinator(); err == nil {
				t.Fatalf("SC_VOTE_QUORUM=%q with 3 replicas: expected an error", quorum)
			}
		})
	}
}

func TestFaultyRate(t *testing.T) {
	t.Setenv("SC_FAULTY_RATE", "")
	v, err := LoadVolunteer()
	if err != nil {
		t.Fatal(err)
	}
	if v.FaultyRate != 0 {
		t.Fatalf("default faulty rate = %v, want 0", v.FaultyRate)
	}
	t.Setenv("SC_FAULTY_RATE", "0.25")
	if v, err = LoadVolunteer(); err != nil || v.FaultyRate != 0.25 {
		t.Fatalf("faulty rate = %v, err = %v, want 0.25", v.FaultyRate, err)
	}
	for _, bad := range []string{"-0.1", "1.5", "half", "NaN"} {
		t.Setenv("SC_FAULTY_RATE", bad)
		if _, err := LoadVolunteer(); err == nil {
			t.Fatalf("SC_FAULTY_RATE=%q: expected an error", bad)
		}
	}
}
