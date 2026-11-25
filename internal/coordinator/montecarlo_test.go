package coordinator

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/leaderclient"
	"github.com/colmeabhi/sparecycles/internal/workload"
)

// A small Monte Carlo job runs through the coordinator and 3 volunteers, and
// gives the same result as running every unit in this process.
func TestEndToEndMonteCarlo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn := startNode(t, ctx, 1, nil, 3)
	startVolunteers(ctx, leaderclient.FromConns(conn), 3)

	params := []byte(`{"units":4,"samples_per_unit":100000,"seed":7}`)
	client := pb.NewClientServiceClient(conn)
	resp, err := client.SubmitJob(ctx, &pb.SubmitJobRequest{Workload: "montecarlo", Params: params})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetUnits() != 4 {
		t.Fatalf("units = %d, want 4", resp.GetUnits())
	}

	job := waitForJob(t, ctx, client, resp.GetJobId())
	if job.GetState() != pb.JobState_JOB_STATE_DONE {
		t.Fatalf("job state = %v, error = %q", job.GetState(), job.GetError())
	}
	var r workload.MonteCarloResult
	if err := json.Unmarshal(job.GetResult(), &r); err != nil {
		t.Fatalf("result %s is not JSON: %v", job.GetResult(), err)
	}
	if r.Total != 400_000 || r.Units != 4 || math.Abs(r.Estimate-math.Pi) > 0.02 {
		t.Fatalf("result = %+v, want 400000 samples in 4 units and an estimate near pi", r)
	}
	if want := localMonteCarlo(t, params); !bytes.Equal(job.GetResult(), want) {
		t.Fatalf("result = %s, want %s (the same job run in this process)", job.GetResult(), want)
	}
}

// A Monte Carlo job with bad parameters is refused at submit time.
func TestMonteCarloBadParamsAreRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := pb.NewClientServiceClient(startNode(t, ctx, 1, nil, 3))
	_, err := client.SubmitJob(ctx, &pb.SubmitJobRequest{Workload: "montecarlo", Params: []byte(`{"sample_per_unit":10}`)})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("a job with an unknown parameter gave %v, want InvalidArgument", err)
	}
}

func localMonteCarlo(t *testing.T, params []byte) []byte {
	t.Helper()
	w := workload.MonteCarlo{}
	units, err := w.Split(params)
	if err != nil {
		t.Fatal(err)
	}
	outputs := make([][]byte, len(units))
	for i, u := range units {
		if outputs[i], err = w.Run(context.Background(), u); err != nil {
			t.Fatal(err)
		}
	}
	result, err := w.Merge(params, outputs)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
