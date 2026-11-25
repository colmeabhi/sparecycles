package coordinator

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/security"
	"github.com/colmeabhi/sparecycles/internal/workload"
)

// TestRequestsUnderAnotherVolunteersIDAreRejected checks that knowing a
// volunteer's ID (which gossip shares with everyone) is not enough to act as
// that volunteer: the attacker cannot take work or send results in its name,
// and the victim's lease and result are not changed.
func TestRequestsUnderAnotherVolunteersIDAreRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, m := startSignedNode(t, ctx, security.NoSign{})
	vc := pb.NewVolunteerServiceClient(conn)
	cc := pb.NewClientServiceClient(conn)

	victim, err := vc.Register(ctx, &pb.RegisterRequest{Token: "token-a"})
	if err != nil {
		t.Fatal(err)
	}
	attacker, err := vc.Register(ctx, &pb.RegisterRequest{Token: "token-b"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := cc.SubmitJob(ctx, &pb.SubmitJobRequest{
		Workload: "fractal", Params: []byte(`{"width":16,"height":16,"tile_size":16,"max_iter":10}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	work, err := vc.RequestWork(ctx, &pb.RequestWorkRequest{
		VolunteerId: victim.GetVolunteerId(), SessionSecret: victim.GetSessionSecret(),
	})
	if err != nil || work.GetUnit() == nil {
		t.Fatalf("victim got no unit: %v", err)
	}
	unit := work.GetUnit()

	attempts := map[string]func() error{
		"work, no secret": func() error {
			_, err := vc.RequestWork(ctx, &pb.RequestWorkRequest{VolunteerId: victim.GetVolunteerId()})
			return err
		},
		"work, attacker's own secret": func() error {
			_, err := vc.RequestWork(ctx, &pb.RequestWorkRequest{
				VolunteerId: victim.GetVolunteerId(), SessionSecret: attacker.GetSessionSecret(),
			})
			return err
		},
		"result, no secret": func() error {
			_, err := vc.SubmitResult(ctx, &pb.SubmitResultRequest{
				VolunteerId: victim.GetVolunteerId(), UnitId: unit.GetUnitId(), Output: []byte("garbage"),
			})
			return err
		},
		"result, attacker's own secret": func() error {
			_, err := vc.SubmitResult(ctx, &pb.SubmitResultRequest{
				VolunteerId: victim.GetVolunteerId(), SessionSecret: attacker.GetSessionSecret(),
				UnitId: unit.GetUnitId(), Output: []byte("garbage"),
			})
			return err
		},
	}
	for name, attempt := range attempts {
		if code := status.Code(attempt()); code != codes.Unauthenticated {
			t.Errorf("%s: code = %v, want Unauthenticated", name, code)
		}
	}
	if got := testutil.ToFloat64(m.RequestsRejected); got != float64(len(attempts)) {
		t.Fatalf("sc_requests_rejected_total = %v, want %d", got, len(attempts))
	}
	if got := testutil.ToFloat64(m.ResultsReceived); got != 0 {
		t.Fatalf("sc_results_received_total = %v, want 0: a forged result was recorded", got)
	}

	// The victim still holds its lease, and its real result finishes the job.
	w, err := workload.Default().Get(unit.GetWorkload())
	if err != nil {
		t.Fatal(err)
	}
	output, err := w.Run(ctx, unit.GetPayload())
	if err != nil {
		t.Fatal(err)
	}
	res, err := vc.SubmitResult(ctx, &pb.SubmitResultRequest{
		VolunteerId: victim.GetVolunteerId(), SessionSecret: victim.GetSessionSecret(),
		UnitId: unit.GetUnitId(), Output: output,
	})
	if err != nil || !res.GetRecorded() {
		t.Fatalf("victim's result: recorded=%v err=%v", res.GetRecorded(), err)
	}
	if got := waitForJob(t, ctx, cc, job.GetJobId()); got.GetState() != pb.JobState_JOB_STATE_DONE {
		t.Fatalf("job state = %v, error = %q", got.GetState(), got.GetError())
	}
}

// TestNewRegistrationGetsNewSecret checks that each registration has its own
// secret, so a secret from one registration cannot be used for another.
func TestNewRegistrationGetsNewSecret(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _ := startSignedNode(t, ctx, security.NoSign{})
	vc := pb.NewVolunteerServiceClient(conn)

	first, err := vc.Register(ctx, &pb.RegisterRequest{Token: "token-a"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := vc.Register(ctx, &pb.RegisterRequest{Token: "token-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.GetSessionSecret()) == 0 {
		t.Fatal("registration returned no session secret")
	}
	_, err = vc.RequestWork(ctx, &pb.RequestWorkRequest{
		VolunteerId: second.GetVolunteerId(), SessionSecret: first.GetSessionSecret(),
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("old secret on a new registration: code = %v, want Unauthenticated", status.Code(err))
	}
}
