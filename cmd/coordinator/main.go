// Command coordinator runs one SpareCycles coordinator.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

// shutdownTimeout is how long the HTTP server has to finish open requests.
const shutdownTimeout = 5 * time.Second

func main() {
	config.LoadDotEnv()
	log := logging.New("coordinator")

	cfg, err := config.LoadCoordinator()
	if err != nil {
		log.Error("bad configuration", "err", err)
		os.Exit(2)
	}
	log = log.With("node", cfg.NodeID)

	if err := run(cfg, log); err != nil {
		log.Error("coordinator stopped", "err", err)
		os.Exit(1)
	}
}

// run starts all servers and blocks until a signal or a server error.
func run(cfg config.Coordinator, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	m := metrics.NewCoordinator()
	elector := election.NewStatic(cfg.NodeID, cfg.Peers)
	transport := membership.NewGRPCTransport()
	defer transport.Close()
	srv := newServer(cfg, elector, newMembership(cfg, transport, m, log), m, log)

	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.ListenAddr, err)
	}
	grpcSrv := grpc.NewServer()
	srv.RegisterServices(grpcSrv)
	httpSrv := newHTTPServer(cfg.MetricsAddr, m)

	errc := make(chan error, 3)
	go func() { errc <- grpcSrv.Serve(lis) }()
	go func() {
		if err := httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()
	go func() { errc <- srv.Run(ctx) }()

	logStart(cfg, elector, log)

	var runErr error
	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case runErr = <-errc:
	}

	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx) // best effort; the process exits next
	grpcSrv.GracefulStop()
	return runErr
}

// newServer builds the coordinator with the current implementation of each part.
// To replace a stub, change only its line here (see README.md).
func newServer(cfg config.Coordinator, elector election.Elector, members membership.Membership, m *metrics.Coordinator, log *slog.Logger) *coordinator.Server {
	return coordinator.New(coordinator.Deps{
		Table:        tasktable.New(),
		Elector:      elector,
		Replicator:   replication.Noop{},
		Membership:   members,
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
}

// newMembership builds the SWIM member of this coordinator. The other coordinators are the seeds.
func newMembership(cfg config.Coordinator, t membership.Transport, m *metrics.Coordinator, log *slog.Logger) *membership.SWIM {
	seeds := make([]membership.Member, 0, len(cfg.Peers))
	for id, addr := range cfg.Peers {
		seeds = append(seeds, membership.Member{ID: coordinatorMemberID(id), Address: addr, Coordinator: true})
	}
	return membership.NewSWIM(membership.SWIMConfig{
		Self:           membership.Member{ID: coordinatorMemberID(cfg.NodeID), Address: cfg.AdvertiseAddr, Coordinator: true},
		Seeds:          seeds,
		Transport:      t,
		Interval:       cfg.Gossip.Interval,
		PingTimeout:    cfg.Gossip.PingTimeout,
		SuspectTimeout: cfg.Gossip.SuspectTimeout,
		IndirectProbes: cfg.Gossip.IndirectProbes,
		Metrics:        m.Gossip,
		Log:            log,
	})
}

func coordinatorMemberID(nodeID int) string {
	return fmt.Sprintf("coord-%d", nodeID)
}

// newHTTPServer serves /metrics for Prometheus and /healthz for health checks.
func newHTTPServer(addr string, m *metrics.Coordinator) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler(m.Registry))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}

func logStart(cfg config.Coordinator, elector election.Elector, log *slog.Logger) {
	if cfg.VolunteerToken == "" {
		log.Warn("SC_VOLUNTEER_TOKEN is empty: every volunteer is accepted")
	}
	leader, _ := elector.Leader()
	log.Info("coordinator started", "grpc", cfg.ListenAddr, "metrics", cfg.MetricsAddr,
		"peers", len(cfg.Peers), "leader", leader, "is_leader", elector.IsLeader())
}
