// Command volunteer runs one SpareCycles volunteer.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"google.golang.org/grpc"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/config"
	"github.com/colmeabhi/sparecycles/internal/leaderclient"
	"github.com/colmeabhi/sparecycles/internal/logging"
	"github.com/colmeabhi/sparecycles/internal/membership"
	"github.com/colmeabhi/sparecycles/internal/security"
	"github.com/colmeabhi/sparecycles/internal/volunteer"
	"github.com/colmeabhi/sparecycles/internal/workload"
)

func main() {
	config.LoadDotEnv()
	log := logging.New("volunteer")

	cfg, err := config.LoadVolunteer()
	if err != nil {
		log.Error("bad configuration", "err", err)
		os.Exit(2)
	}
	log = log.With("name", cfg.Name)

	if err := run(cfg, log); err != nil {
		log.Error("volunteer stopped", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Volunteer, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := leaderclient.Dial(cfg.Coordinators)
	if err != nil {
		return fmt.Errorf("connect to coordinators: %w", err)
	}
	defer pool.Close()

	vcfg := volunteer.Config{
		Name: cfg.Name, Token: cfg.Token,
		CPUCores: cfg.CPUCores, MemoryMB: cfg.MemoryMB, SpeedScore: cfg.SpeedScore,
	}
	if cfg.GossipAddr != config.GossipOff {
		gossip, stopGossip, err := startGossip(ctx, cfg, log)
		if err != nil {
			return err
		}
		defer stopGossip()
		vcfg.Address = gossip.addr
		vcfg.OnRegister = gossip.swim.SetSelf
	} else {
		log.Info("gossip off: this volunteer is found dead only by lease expiry")
	}

	verifier, err := newVerifier(cfg, log)
	if err != nil {
		return err
	}
	v := volunteer.New(vcfg, pool, workload.Default(), verifier, log)
	log.Info("volunteer started", "coordinators", cfg.Coordinators, "gossip", vcfg.Address)
	return v.Run(ctx)
}

// newVerifier returns the task signature check. With no public key, tasks are not checked.
func newVerifier(cfg config.Volunteer, log *slog.Logger) (security.Verifier, error) {
	if cfg.SigningPublicKey == "" {
		log.Warn("SC_SIGNING_PUBLIC_KEY is empty: task signatures are not checked")
		return security.NoSign{}, nil
	}
	v, err := security.NewEd25519Verifier(cfg.SigningPublicKey)
	if err != nil {
		return nil, fmt.Errorf("SC_SIGNING_PUBLIC_KEY: %w", err)
	}
	log.Info("task signatures are checked with Ed25519")
	return v, nil
}

type gossipNode struct {
	swim *membership.SWIM
	addr string // advertised address
}

// startGossip starts the SWIM listener. The volunteer joins the group when the
// leader registers it; the leader's pings then teach it the other members.
func startGossip(ctx context.Context, cfg config.Volunteer, log *slog.Logger) (gossipNode, func(), error) {
	lis, err := net.Listen("tcp", cfg.GossipAddr)
	if err != nil {
		return gossipNode{}, nil, fmt.Errorf("gossip listen on %s: %w", cfg.GossipAddr, err)
	}
	port := lis.Addr().(*net.TCPAddr).Port
	addr := net.JoinHostPort(cfg.GossipHost, strconv.Itoa(port))

	transport := membership.NewGRPCTransport()
	swim := membership.NewSWIM(membership.SWIMConfig{
		Self:           membership.Member{Address: addr},
		Transport:      transport,
		Interval:       cfg.Gossip.Interval,
		PingTimeout:    cfg.Gossip.PingTimeout,
		SuspectTimeout: cfg.Gossip.SuspectTimeout,
		IndirectProbes: cfg.Gossip.IndirectProbes,
		Log:            log,
	})
	srv := grpc.NewServer()
	pb.RegisterGossipServiceServer(srv, swim)
	go func() { _ = srv.Serve(lis) }() // Serve returns when Stop is called
	if err := swim.Start(ctx); err != nil {
		srv.Stop()
		return gossipNode{}, nil, err
	}
	stopAll := func() {
		srv.Stop()
		_ = transport.Close()
	}
	return gossipNode{swim: swim, addr: addr}, stopAll, nil
}
