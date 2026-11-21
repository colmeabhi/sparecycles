// Package volunteer implements the volunteer loop: register, pull work, run it, send the result.
package volunteer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/leaderclient"
	"github.com/colmeabhi/sparecycles/internal/security"
	"github.com/colmeabhi/sparecycles/internal/workload"
)

// Config holds what the volunteer tells the leader about itself.
type Config struct {
	Name       string
	Token      string
	Address    string // gossip address; empty if the volunteer accepts no gossip
	CPUCores   int
	MemoryMB   int
	SpeedScore float64
}

// Volunteer runs units for the leader.
type Volunteer struct {
	cfg       Config
	pool      *leaderclient.Pool
	workloads *workload.Registry
	signer    security.Signer
	log       *slog.Logger
	id        string
}

// New returns a Volunteer.
func New(cfg Config, pool *leaderclient.Pool, workloads *workload.Registry, signer security.Signer, log *slog.Logger) *Volunteer {
	if log == nil {
		log = slog.Default()
	}
	return &Volunteer{cfg: cfg, pool: pool, workloads: workloads, signer: signer, log: log}
}

// Run works until ctx ends.
func (v *Volunteer) Run(ctx context.Context) error {
	backoff := newBackoff()
	for ctx.Err() == nil {
		if v.id == "" {
			if err := v.register(ctx); err != nil {
				if status.Code(err) == codes.Unauthenticated {
					return err // a wrong token does not fix itself
				}
				v.log.Warn("register failed", "err", err)
				sleep(ctx, backoff.next())
				continue
			}
			backoff.reset()
		}
		wait, err := v.step(ctx)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				v.log.Info("leader does not know this volunteer, registering again")
				v.id = ""
				continue
			}
			if ctx.Err() == nil {
				v.log.Warn("work step failed", "err", err)
			}
			sleep(ctx, backoff.next())
			continue
		}
		backoff.reset()
		sleep(ctx, wait)
	}
	return nil
}

func (v *Volunteer) register(ctx context.Context) error {
	req := &pb.RegisterRequest{
		Token: v.cfg.Token, Address: v.cfg.Address, Name: v.cfg.Name,
		Resources: &pb.Resources{
			CpuCores: uint32(v.cfg.CPUCores), MemoryMb: uint64(v.cfg.MemoryMB), SpeedScore: v.cfg.SpeedScore,
		},
	}
	return v.pool.Do(ctx, func(conn *grpc.ClientConn) error {
		resp, err := pb.NewVolunteerServiceClient(conn).Register(ctx, req)
		if err != nil {
			return err
		}
		v.id = resp.GetVolunteerId()
		v.log.Info("registered", "volunteer", v.id)
		return nil
	})
}

// step asks for one unit, runs it and sends the result.
// It returns how long to wait before the next step.
func (v *Volunteer) step(ctx context.Context) (time.Duration, error) {
	var unit *pb.WorkUnit
	var retry time.Duration
	err := v.pool.Do(ctx, func(conn *grpc.ClientConn) error {
		resp, err := pb.NewVolunteerServiceClient(conn).RequestWork(ctx, &pb.RequestWorkRequest{VolunteerId: v.id})
		if err != nil {
			return err
		}
		unit = resp.GetUnit()
		retry = time.Duration(resp.GetRetryAfterMs()) * time.Millisecond
		return nil
	})
	if err != nil {
		return 0, err
	}
	if unit == nil {
		return retry, nil
	}

	output, err := v.run(ctx, unit)
	if err != nil {
		// Do not send a result. The lease expires and another volunteer gets the unit.
		v.log.Warn("unit failed", "unit", unit.GetUnitId(), "err", err)
		return 0, nil
	}

	return 0, v.pool.Do(ctx, func(conn *grpc.ClientConn) error {
		resp, err := pb.NewVolunteerServiceClient(conn).SubmitResult(ctx, &pb.SubmitResultRequest{
			VolunteerId: v.id, UnitId: unit.GetUnitId(), Output: output,
		})
		if err != nil {
			return err
		}
		v.log.Info("result sent", "unit", unit.GetUnitId(), "recorded", resp.GetRecorded())
		return nil
	})
}

func (v *Volunteer) run(ctx context.Context, unit *pb.WorkUnit) ([]byte, error) {
	if err := v.signer.Verify(unit.GetPayload(), unit.GetSignature()); err != nil {
		return nil, fmt.Errorf("bad signature: %w", err)
	}
	w, err := v.workloads.Get(unit.GetWorkload())
	if err != nil {
		return nil, err
	}
	deadline := time.UnixMilli(unit.GetLeaseExpiresUnixMs())
	runCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	start := time.Now()
	out, err := w.Run(runCtx, unit.GetPayload())
	if errors.Is(err, context.DeadlineExceeded) {
		return nil, errors.New("lease expired before the unit finished")
	}
	if err != nil {
		return nil, err
	}
	v.log.Debug("unit done", "unit", unit.GetUnitId(), "took", time.Since(start))
	return out, nil
}

type backoff struct{ d time.Duration }

func newBackoff() *backoff { return &backoff{d: 250 * time.Millisecond} }

func (b *backoff) next() time.Duration {
	d := b.d
	b.d = min(b.d*2, 10*time.Second)
	return d
}

func (b *backoff) reset() { b.d = 250 * time.Millisecond }

func sleep(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
