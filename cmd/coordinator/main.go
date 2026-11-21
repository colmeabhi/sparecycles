// Command coordinator runs one SpareCycles coordinator.
package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/colmeabhi/sparecycles/internal/config"
	"github.com/colmeabhi/sparecycles/internal/coordinator"
	"github.com/colmeabhi/sparecycles/internal/election"
	"github.com/colmeabhi/sparecycles/internal/lease"
	"github.com/colmeabhi/sparecycles/internal/logging"
	"github.com/colmeabhi/sparecycles/internal/membership"
	"github.com/colmeabhi/sparecycles/internal/metrics"
	"github.com/colmeabhi/sparecycles/internal/replication"
	"github.com/colmeabhi/sparecycles/internal/scheduler"
	"github.com/colmeabhi/sparecycles/internal/security"
	"github.com/colmeabhi/sparecycles/internal/tasktable"
	"github.com/colmeabhi/sparecycles/internal/verify"
	"github.com/colmeabhi/sparecycles/internal/workload"
)

func main() {
	config.LoadDotEnv()
	log := logging.New("coordinator")

	cfg, err := config.LoadCoordinator()
	if err != nil {
		log.Error("bad configuration", "err", err)
		os.Exit(2)
	}
	log = log.With("node", cfg.NodeID)

	m := metrics.NewCoordinator()
	elector := election.NewStatic(cfg.NodeID, cfg.Peers)
	srv := coordinator.New(coordinator.Deps{
		NodeID:       cfg.NodeID,
		Table:        tasktable.New(),
		Elector:      elector,
		Replicator:   replication.Noop{},
		Membership:   membership.NewStatic(),
		Leases:       lease.NoExpiry{Length: cfg.LeaseDuration},
		Verifier:     verify.FirstResult{},
		Scheduler:    scheduler.FIFO{},
		Auth:         security.SharedToken{Token: cfg.VolunteerToken},
		Signer:       security.NoSign{},
		Workloads:    workload.Default(),
		Metrics:      m,
		VoteReplicas: cfg.VoteReplicas,
		Log:          log,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		log.Error("listen", "addr", cfg.ListenAddr, "err", err)
		os.Exit(1)
	}
	g := grpc.NewServer()
	srv.RegisterServices(g)

	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler(m.Registry))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	httpSrv := &http.Server{Addr: cfg.MetricsAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	errc := make(chan error, 3)
	go func() { errc <- g.Serve(lis) }()
	go func() {
		if err := httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()
	go func() { errc <- srv.Run(ctx) }()

	if len(cfg.VolunteerToken) == 0 {
		log.Warn("SC_VOLUNTEER_TOKEN is empty: every volunteer is accepted")
	}
	leader, _ := elector.Leader()
	log.Info("coordinator started", "grpc", cfg.ListenAddr, "metrics", cfg.MetricsAddr,
		"peers", len(cfg.Peers), "leader", leader, "is_leader", elector.IsLeader())

	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case err := <-errc:
		if err != nil {
			log.Error("server stopped", "err", err)
		}
	}
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	g.GracefulStop()
}
