// Command client submits a job, waits for it, and saves the result.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"google.golang.org/grpc"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/config"
	"github.com/colmeabhi/sparecycles/internal/leaderclient"
)

func main() {
	config.LoadDotEnv()
	cfg := config.LoadClient()

	workloadName := flag.String("workload", "fractal", "workload name")
	// Empty parameters mean the defaults of the chosen workload. For fractal they
	// are a 1024×1024 image with 256-pixel tiles and 1000 iterations.
	params := flag.String("params", "", "workload parameters as JSON (empty: the workload's defaults)")
	out := flag.String("out", "result.png", "file for the result")
	poll := flag.Duration("poll", time.Second, "time between progress checks")
	timeout := flag.Duration("timeout", 30*time.Minute, "maximum time to wait")
	flag.Parse()

	if err := run(cfg.Coordinators, *workloadName, *params, *out, *poll, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(addrs []string, workloadName, params, out string, poll, timeout time.Duration) error {
	pool, err := leaderclient.Dial(addrs)
	if err != nil {
		return err
	}
	defer pool.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var jobID string
	err = pool.Do(ctx, func(conn *grpc.ClientConn) error {
		resp, err := pb.NewClientServiceClient(conn).SubmitJob(ctx, &pb.SubmitJobRequest{
			Workload: workloadName, Params: []byte(params),
		})
		if err != nil {
			return err
		}
		jobID = resp.GetJobId()
		fmt.Printf("submitted %s with %d units\n", jobID, resp.GetUnits())
		return nil
	})
	if err != nil {
		return fmt.Errorf("submit: %w", err)
	}

	start := time.Now()
	t := time.NewTicker(poll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
		var job *pb.GetJobResponse
		err := pool.Do(ctx, func(conn *grpc.ClientConn) error {
			var err error
			job, err = pb.NewClientServiceClient(conn).GetJob(ctx, &pb.GetJobRequest{JobId: jobID})
			return err
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "progress check failed:", err)
			continue
		}
		fmt.Printf("\r%d/%d units done", job.GetUnitsDone(), job.GetUnitsTotal())
		switch job.GetState() {
		case pb.JobState_JOB_STATE_DONE:
			fmt.Printf("\ndone in %s\n", time.Since(start).Round(time.Millisecond))
			if err := os.WriteFile(out, job.GetResult(), 0o644); err != nil {
				return err
			}
			fmt.Printf("saved %s (%d bytes)\n", out, len(job.GetResult()))
			return nil
		case pb.JobState_JOB_STATE_FAILED:
			fmt.Println()
			return fmt.Errorf("job failed: %s", job.GetError())
		}
	}
}
