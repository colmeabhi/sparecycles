// Command loadgen submits many jobs with several concurrent clients, waits for
// all of them, and reports throughput and job time. It can append the result to
// a CSV file, so that runs with different settings can be compared.
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/config"
	"github.com/colmeabhi/sparecycles/internal/leaderclient"
)

// pollInterval is the time between two progress checks of one job.
const pollInterval = 100 * time.Millisecond

type options struct {
	jobs        int
	concurrency int
	workload    string
	params      string
	label       string
	volunteers  int
	csvPath     string
	timeout     time.Duration
}

func main() {
	config.LoadDotEnv()
	var o options
	flag.IntVar(&o.jobs, "jobs", 8, "number of jobs to submit")
	flag.IntVar(&o.concurrency, "concurrency", 2, "number of clients that submit at the same time")
	flag.StringVar(&o.workload, "workload", "fractal", "workload name")
	// Empty parameters mean the defaults of the chosen workload, as in the client.
	// A fixed fractal default would break every other workload.
	flag.StringVar(&o.params, "params", "", "workload parameters as JSON (empty: the workload's defaults)")
	flag.StringVar(&o.label, "label", "full", "name of this setup in the CSV, for example full or baseline")
	flag.IntVar(&o.volunteers, "volunteers", 0, "number of volunteers in the cluster, recorded in the CSV")
	flag.StringVar(&o.csvPath, "csv", "", "append the result to this CSV file")
	flag.DurationVar(&o.timeout, "timeout", 30*time.Minute, "maximum time for the whole run")
	flag.Parse()

	if err := run(o, config.LoadClient().Coordinators); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(o options, coordinators []string) error {
	if o.jobs < 1 || o.concurrency < 1 {
		return errors.New("-jobs and -concurrency must be 1 or more")
	}
	pool, err := leaderclient.Dial(coordinators)
	if err != nil {
		return err
	}
	defer pool.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	start := time.Now()
	results := runJobs(ctx, pool, o)
	total := time.Since(start)

	var durations []time.Duration
	failed, units := 0, 0
	for _, r := range results {
		if r.err != nil {
			failed++
			fmt.Fprintln(os.Stderr, "job failed:", r.err)
			continue
		}
		durations = append(durations, r.duration)
		units += r.units
	}
	s := summarize(durations, failed, units, total)
	printSummary(o, s)
	if o.csvPath != "" {
		if err := appendCSV(o, s); err != nil {
			return fmt.Errorf("write %s: %w", o.csvPath, err)
		}
		fmt.Println("appended to", o.csvPath)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d jobs failed", failed, o.jobs)
	}
	return nil
}

type jobResult struct {
	duration time.Duration
	units    int
	err      error
}

// runJobs runs o.jobs jobs with o.concurrency workers and returns one result per job.
func runJobs(ctx context.Context, pool *leaderclient.Pool, o options) []jobResult {
	next := make(chan int)
	results := make([]jobResult, o.jobs)
	var wg sync.WaitGroup
	for w := 0; w < o.concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				results[i] = runOneJob(ctx, pool, o)
				fmt.Printf("job %d/%d done in %s\n", i+1, o.jobs, results[i].duration.Round(time.Millisecond))
			}
		}()
	}
	for i := 0; i < o.jobs; i++ {
		next <- i
	}
	close(next)
	wg.Wait()
	return results
}

// runOneJob submits one job and polls it until it is done or failed.
func runOneJob(ctx context.Context, pool *leaderclient.Pool, o options) jobResult {
	start := time.Now()
	var jobID string
	var units int
	err := pool.Do(ctx, func(conn *grpc.ClientConn) error {
		resp, err := pb.NewClientServiceClient(conn).SubmitJob(ctx, &pb.SubmitJobRequest{Workload: o.workload, Params: []byte(o.params)})
		if err != nil {
			return err
		}
		jobID, units = resp.GetJobId(), int(resp.GetUnits())
		return nil
	})
	if err != nil {
		return jobResult{err: fmt.Errorf("submit: %w", err)}
	}
	for {
		var job *pb.GetJobResponse
		err := pool.Do(ctx, func(conn *grpc.ClientConn) error {
			var err error
			job, err = pb.NewClientServiceClient(conn).GetJob(ctx, &pb.GetJobRequest{JobId: jobID})
			return err
		})
		switch {
		case ctx.Err() != nil:
			return jobResult{err: fmt.Errorf("job %s: %w", jobID, ctx.Err())}
		case err != nil:
			// A failover can make one check fail. Try again on the next tick.
		case job.GetState() == pb.JobState_JOB_STATE_DONE:
			return jobResult{duration: time.Since(start), units: units}
		case job.GetState() == pb.JobState_JOB_STATE_FAILED:
			return jobResult{err: fmt.Errorf("job %s failed: %s", jobID, job.GetError())}
		}
		select {
		case <-ctx.Done():
		case <-time.After(pollInterval):
		}
	}
}

func printSummary(o options, s summary) {
	fmt.Println()
	fmt.Printf("setup        %s, %d volunteers, %d clients\n", o.label, o.volunteers, o.concurrency)
	fmt.Printf("jobs         %d (%d failed)\n", s.Jobs, s.Failed)
	fmt.Printf("units        %d\n", s.Units)
	fmt.Printf("total time   %s\n", s.Total.Round(time.Millisecond))
	fmt.Printf("throughput   %.2f jobs/min, %.1f units/s\n", s.JobsPerMin, s.UnitsPerS)
	fmt.Printf("job time     mean %s, p95 %s\n", s.Mean.Round(time.Millisecond), s.P95.Round(time.Millisecond))
}

var csvHeader = []string{
	"time", "label", "volunteers", "clients", "jobs", "failed", "units",
	"total_s", "jobs_per_min", "units_per_s", "mean_job_s", "p95_job_s", "cpus", "arch",
}

// appendCSV appends one row, and writes the header if the file is new.
func appendCSV(o options, s summary) error {
	_, statErr := os.Stat(o.csvPath)
	isNew := errors.Is(statErr, os.ErrNotExist)
	f, err := os.OpenFile(o.csvPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if isNew {
		if err := w.Write(csvHeader); err != nil {
			return err
		}
	}
	f3 := func(x float64) string { return strconv.FormatFloat(x, 'f', 3, 64) }
	row := []string{
		time.Now().UTC().Format(time.RFC3339), o.label, strconv.Itoa(o.volunteers), strconv.Itoa(o.concurrency),
		strconv.Itoa(s.Jobs), strconv.Itoa(s.Failed), strconv.Itoa(s.Units),
		f3(s.Total.Seconds()), f3(s.JobsPerMin), f3(s.UnitsPerS), f3(s.Mean.Seconds()), f3(s.P95.Seconds()),
		strconv.Itoa(runtime.NumCPU()), runtime.GOARCH,
	}
	if err := w.Write(row); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}
