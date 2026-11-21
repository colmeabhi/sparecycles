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

func TestLoadCoordinatorBadValues(t *testing.T) {
	cases := map[string]string{
		"SC_NODE_ID":        "one",
		"SC_PEERS":          "c2:7400",
		"SC_LEASE_DURATION": "3",
		"SC_VOTE_REPLICAS":  "0",
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
