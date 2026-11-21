// Package config reads settings from environment variables.
//
// Each binary first loads a .env file from the current directory, if one exists.
// Variables that are already set in the environment keep their values.
// See .env.example for the full list of variables.
package config

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// LoadDotEnv loads .env from the current directory if the file exists.
func LoadDotEnv() {
	_ = godotenv.Load()
}

// Coordinator holds the settings for one coordinator node.
type Coordinator struct {
	NodeID         int
	ListenAddr     string
	Peers          map[int]string // other coordinators, by node ID
	MetricsAddr    string
	VolunteerToken string
	LeaseDuration  time.Duration
	VoteReplicas   int
}

// LoadCoordinator reads the coordinator settings.
func LoadCoordinator() (Coordinator, error) {
	var c Coordinator
	var err error
	if c.NodeID, err = intVar("SC_NODE_ID", 1); err != nil {
		return c, err
	}
	c.ListenAddr = stringVar("SC_LISTEN_ADDR", ":7400")
	if c.Peers, err = peersVar("SC_PEERS"); err != nil {
		return c, err
	}
	c.MetricsAddr = stringVar("SC_METRICS_ADDR", ":9400")
	c.VolunteerToken = stringVar("SC_VOLUNTEER_TOKEN", "")
	if c.LeaseDuration, err = durationVar("SC_LEASE_DURATION", 3*time.Minute); err != nil {
		return c, err
	}
	if c.VoteReplicas, err = intVar("SC_VOTE_REPLICAS", 3); err != nil {
		return c, err
	}
	if c.VoteReplicas < 1 {
		return c, fmt.Errorf("SC_VOTE_REPLICAS must be 1 or more, got %d", c.VoteReplicas)
	}
	return c, nil
}

// Volunteer holds the settings for one volunteer.
type Volunteer struct {
	Coordinators []string
	Token        string
	Name         string
	CPUCores     int
	MemoryMB     int
	SpeedScore   float64
}

// LoadVolunteer reads the volunteer settings.
func LoadVolunteer() (Volunteer, error) {
	var v Volunteer
	var err error
	v.Coordinators = listVar("SC_COORDINATORS", []string{"localhost:7400"})
	v.Token = stringVar("SC_VOLUNTEER_TOKEN", "")
	host, _ := os.Hostname()
	v.Name = stringVar("SC_VOLUNTEER_NAME", host)
	if v.CPUCores, err = intVar("SC_CPU_CORES", runtime.NumCPU()); err != nil {
		return v, err
	}
	if v.MemoryMB, err = intVar("SC_MEMORY_MB", 0); err != nil {
		return v, err
	}
	if v.SpeedScore, err = floatVar("SC_SPEED_SCORE", 1.0); err != nil {
		return v, err
	}
	return v, nil
}

// Client holds the settings for the job client.
type Client struct {
	Coordinators []string
}

// LoadClient reads the client settings.
func LoadClient() Client {
	return Client{Coordinators: listVar("SC_COORDINATORS", []string{"localhost:7400"})}
}

func stringVar(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func intVar(key string, def int) (int, error) {
	s := stringVar(key, "")
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not an integer", key, s)
	}
	return n, nil
}

func floatVar(key string, def float64) (float64, error) {
	s := stringVar(key, "")
	if s == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a number", key, s)
	}
	return f, nil
}

func durationVar(key string, def time.Duration) (time.Duration, error) {
	s := stringVar(key, "")
	if s == "" {
		return def, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a duration (example: 3m)", key, s)
	}
	return d, nil
}

func listVar(key string, def []string) []string {
	s := stringVar(key, "")
	if s == "" {
		return def
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// peersVar parses "2=host2:7400,3=host3:7400".
func peersVar(key string) (map[int]string, error) {
	peers := map[int]string{}
	for _, item := range listVar(key, nil) {
		idStr, addr, ok := strings.Cut(item, "=")
		if !ok {
			return nil, fmt.Errorf("%s: %q must look like id=host:port", key, item)
		}
		id, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			return nil, fmt.Errorf("%s: %q has an ID that is not an integer", key, item)
		}
		peers[id] = strings.TrimSpace(addr)
	}
	return peers, nil
}
