package coordinator

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/leaderclient"
)

// submitJob submits a job to the leader through the pool.
func submitJob(t *testing.T, ctx context.Context, pool *leaderclient.Pool, params string) *pb.SubmitJobResponse {
	t.Helper()
	var resp *pb.SubmitJobResponse
	err := pool.Do(ctx, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pb.NewClientServiceClient(conn).SubmitJob(ctx, &pb.SubmitJobRequest{Workload: "fractal", Params: []byte(params)})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// After the leader crashes, a new leader is elected and a new job finishes.
// Without replication the job in progress is lost, so the test submits a new one.
func TestLeaderCrashNewJobFinishes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	nodes := startCluster(t, ctx, 3, clusterOptions{lease: time.Minute})
	waitLeader(t, ctx, nodes[2])

	pool := leaderclient.FromConns(nodes[0].conn, nodes[1].conn, nodes[2].conn)
	startVolunteers(ctx, pool, 2)

	nodes[2].stop()
	waitLeader(t, ctx, nodes[1])

	resp := submitJob(t, ctx, pool, `{"width":64,"height":64,"tile_size":32,"max_iter":50}`)
	job := waitForJob(t, ctx, pb.NewClientServiceClient(nodes[1].conn), resp.GetJobId())
	if job.GetState() != pb.JobState_JOB_STATE_DONE {
		t.Fatalf("job state = %v, error = %q", job.GetState(), job.GetError())
	}
}
