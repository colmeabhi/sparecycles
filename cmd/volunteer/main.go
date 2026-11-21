// Command volunteer runs one SpareCycles volunteer.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/colmeabhi/sparecycles/internal/config"
	"github.com/colmeabhi/sparecycles/internal/leaderclient"
	"github.com/colmeabhi/sparecycles/internal/logging"
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

	pool, err := leaderclient.Dial(cfg.Coordinators)
	if err != nil {
		log.Error("connect to coordinators", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	v := volunteer.New(volunteer.Config{
		Name: cfg.Name, Token: cfg.Token,
		CPUCores: cfg.CPUCores, MemoryMB: cfg.MemoryMB, SpeedScore: cfg.SpeedScore,
	}, pool, workload.Default(), security.NoSign{}, log)

	log.Info("volunteer started", "coordinators", cfg.Coordinators)
	if err := v.Run(ctx); err != nil {
		log.Error("volunteer stopped", "err", err)
		os.Exit(1)
	}
}
