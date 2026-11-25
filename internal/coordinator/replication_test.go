package coordinator

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/leaderclient"
)

// The leader crashes in the middle of a job. The new leader continues from the
// replicated table, the job finishes, and no unit accepted before the crash runs again.
func TestLeaderCrashJobContinues(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// After the failover, volunteers register again under new IDs, so the leases
	// of their old IDs must end. A short lease does that quickly.
	nodes := startCluster(t, ctx, 3, clusterOptions{replicate: true, lease: 500 * time.Millisecond})
	oldLeader, newLeader := nodes[2], nodes[1]
	waitLeader(t, ctx, oldLeader)

	pool := leaderclient.FromConns(nodes[0].conn, nodes[1].conn, nodes[2].conn)
	startVolunteers(ctx, pool, 2)
	resp := submitJob(t, ctx, pool, `{"width":256,"height":256,"tile_size":16,"max_iter":200}`)
	total := int(resp.GetUnits())

	doneBefore := waitDone(t, ctx, oldLeader, resp.GetJobId(), 20)
	oldLeader.stop()
	if job, _ := oldLeader.table.Job(resp.GetJobId()); job.Done > doneBefore {
		doneBefore = job.Done // units accepted between the check and the stop
	}
	if doneBefore >= total {
		t.Fatalf("the job finished before the crash (%d units); make it larger", total)
	}

	waitLeader(t, ctx, newLeader)
	job := waitForJob(t, ctx, pb.NewClientServiceClient(newLeader.conn), resp.GetJobId())
	if job.GetState() != pb.JobState_JOB_STATE_DONE {
		t.Fatalf("job state = %v, error = %q", job.GetState(), job.GetError())
	}
	acceptedByNew := int(testutil.ToFloat64(newLeader.metrics.UnitsAccepted))
	if acceptedByNew > total-doneBefore {
		t.Fatalf("new leader accepted %d units, but only %d were left: accepted units ran again", acceptedByNew, total-doneBefore)
	}
	t.Logf("%d of %d units were done before the crash; the new leader accepted %d", doneBefore, total, acceptedByNew)
}

// waitDone waits until the node's table has at least n accepted units in the job.
func waitDone(t *testing.T, ctx context.Context, node *clusterNode, jobID string, n int) int {
	t.Helper()
	for {
		if job, ok := node.table.Job(jobID); ok && job.Done >= n {
			return job.Done
		}
		select {
		case <-ctx.Done():
			t.Fatalf("fewer than %d units done before the deadline", n)
		case <-time.After(time.Millisecond):
		}
	}
}
