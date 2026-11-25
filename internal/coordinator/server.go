// Package coordinator implements the client and volunteer RPCs of a coordinator.
//
// The server connects the parts: it asks the Scheduler for a unit, the lease
// Policy for lease lengths, the Verifier for decisions, and it sends each table
// change through the Replicator before it applies the change.
package coordinator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/election"
	"github.com/colmeabhi/sparecycles/internal/lease"
	"github.com/colmeabhi/sparecycles/internal/membership"
	"github.com/colmeabhi/sparecycles/internal/metrics"
	"github.com/colmeabhi/sparecycles/internal/replication"
	"github.com/colmeabhi/sparecycles/internal/scheduler"
	"github.com/colmeabhi/sparecycles/internal/security"
	"github.com/colmeabhi/sparecycles/internal/tasktable"
	"github.com/colmeabhi/sparecycles/internal/verify"
	"github.com/colmeabhi/sparecycles/internal/workload"
)

// noWorkRetry is how long a volunteer waits when no work is available.
const noWorkRetry = time.Second

// Deps are the parts that a Server uses. Each part has a base stub.
type Deps struct {
	Table        *tasktable.Table
	Elector      election.Elector
	Replicator   replication.Replicator
	Membership   membership.Membership
	Leases       lease.Policy
	Verifier     verify.Verifier
	Scheduler    scheduler.Scheduler
	Auth         security.Authenticator
	Signer       security.Signer
	Workloads    *workload.Registry
	Metrics      *metrics.Coordinator
	VoteReplicas int
	Log          *slog.Logger
	Now          func() time.Time // for tests; nil means time.Now
	TickInterval time.Duration    // leader background work; zero means 1 second
}

// Server is a coordinator.
type Server struct {
	pb.UnimplementedClientServiceServer
	pb.UnimplementedVolunteerServiceServer
	d Deps

	commitMu sync.Mutex // orders all changes

	decideMu   sync.Mutex // one result at a time, from record to vote; guards lateCopies
	lateCopies lateCopies // leader-local; copies still running when their unit was accepted

	mu         sync.Mutex
	volunteers map[string]scheduler.Volunteer  // leader-local; volunteers register again after a failover
	sessions   map[string]security.SessionHash // leader-local; by volunteer ID, removed with the volunteer
	waiting    map[string]bool                 // leader-local; units already reported as without a volunteer
	submitted  map[string]time.Time            // leader-local; for the job duration metric
}

// New returns a Server.
func New(d Deps) *Server {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.TickInterval <= 0 {
		d.TickInterval = time.Second
	}
	return &Server{
		d:          d,
		volunteers: map[string]scheduler.Volunteer{},
		sessions:   map[string]security.SessionHash{},
		submitted:  map[string]time.Time{},
		lateCopies: lateCopies{},
	}
}

// RegisterServices adds the coordinator services, and the services of its parts, to a gRPC server.
func (s *Server) RegisterServices(g *grpc.Server) {
	pb.RegisterClientServiceServer(g, s)
	pb.RegisterVolunteerServiceServer(g, s)
	pb.RegisterElectionServiceServer(g, s.d.Elector)
	pb.RegisterReplicationServiceServer(g, s.d.Replicator)
	pb.RegisterGossipServiceServer(g, s.d.Membership)
}

// Run starts the background loops and blocks until ctx ends.
func (s *Server) Run(ctx context.Context) error {
	if err := s.d.Elector.Start(ctx); err != nil {
		return fmt.Errorf("start election: %w", err)
	}
	if err := s.d.Membership.Start(ctx); err != nil {
		return fmt.Errorf("start membership: %w", err)
	}
	tick := time.NewTicker(s.d.TickInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			s.tick(ctx)
		}
	}
}

// tick runs the periodic work of the leader.
func (s *Server) tick(ctx context.Context) {
	if !s.d.Elector.IsLeader() {
		s.d.Metrics.IsLeader.Set(0)
		return
	}
	s.d.Metrics.IsLeader.Set(1)
	s.releaseExpiredLeases(ctx)
	s.releaseDeadVolunteers(ctx)
	s.retryStuckVotes(ctx)
	s.reportUnitsWithoutVolunteer()
}

func (s *Server) releaseExpiredLeases(ctx context.Context) {
	for _, e := range s.d.Leases.Expired(s.d.Table.OpenUnits(), s.d.Now()) {
		err := s.commit(ctx, tasktable.Change{Kind: tasktable.LeaseReleased, UnitID: e.UnitID, VolunteerID: e.VolunteerID})
		if err != nil {
			s.d.Log.Warn("release expired lease", "unit", e.UnitID, "volunteer", e.VolunteerID, "err", err)
			continue
		}
		s.d.Metrics.LeasesExpired.Inc()
		s.d.Log.Info("lease expired", "unit", e.UnitID, "volunteer", e.VolunteerID)
	}
}

// releaseDeadVolunteers releases the leases of volunteers that gossip declared dead,
// so their units move on before the lease ends. It also forgets those volunteers:
// if one was wrongly declared dead, its next request gets NotFound and it registers again.
func (s *Server) releaseDeadVolunteers(ctx context.Context) {
	for _, u := range s.d.Table.OpenUnits() {
		for volunteerID := range u.Assignments {
			if !s.d.Membership.Dead(volunteerID) {
				continue
			}
			err := s.commit(ctx, tasktable.Change{Kind: tasktable.LeaseReleased, UnitID: u.ID, VolunteerID: volunteerID})
			if err != nil {
				s.d.Log.Warn("release lease of dead volunteer", "unit", u.ID, "volunteer", volunteerID, "err", err)
				continue
			}
			s.d.Metrics.LeasesDeadVolunteer.Inc()
			s.d.Log.Info("lease released: volunteer dead", "unit", u.ID, "volunteer", volunteerID)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.volunteers {
		if s.d.Membership.Dead(id) {
			delete(s.volunteers, id)
			delete(s.sessions, id)
			s.d.Log.Info("volunteer removed: dead", "volunteer", id)
		}
	}
}

// commit replicates a change and then applies it to the local table.
func (s *Server) commit(ctx context.Context, c tasktable.Change) error {
	s.commitMu.Lock()
	defer s.commitMu.Unlock()
	// Check first, so that backups never get a change that the leader cannot apply.
	if err := s.d.Table.Validate(c); err != nil {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	raw, err := c.Encode()
	if err != nil {
		return err
	}
	if err := s.d.Replicator.Send(ctx, s.d.Table.Index()+1, raw); err != nil {
		s.d.Metrics.ReplicationErrors.Inc()
		return status.Errorf(codes.Unavailable, "replicate %s: %v", c.Kind, err)
	}
	if err := s.d.Table.Apply(c); err != nil {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	return nil
}

func (s *Server) requireLeader() error {
	if s.d.Elector.IsLeader() {
		return nil
	}
	leader, term := s.d.Elector.Leader()
	return status.Errorf(codes.FailedPrecondition, "not the leader: leader is node %d (term %d)", leader, term)
}

// ---------------------------------------------------------------------------
// ClientService
// ---------------------------------------------------------------------------

// SubmitJob splits a job into units and adds it to the table.
func (s *Server) SubmitJob(ctx context.Context, req *pb.SubmitJobRequest) (*pb.SubmitJobResponse, error) {
	if err := s.requireLeader(); err != nil {
		return nil, err
	}
	w, err := s.d.Workloads.Get(req.GetWorkload())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	units, err := w.Split(req.GetParams())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if len(units) == 0 {
		return nil, status.Error(codes.InvalidArgument, "the job has no units")
	}
	jobID := newID("job")
	err = s.commit(ctx, tasktable.Change{
		Kind: tasktable.JobAdded, JobID: jobID, Workload: w.Name(),
		Params: req.GetParams(), Units: units, Copies: s.d.VoteReplicas,
	})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.submitted[jobID] = s.d.Now()
	s.mu.Unlock()
	s.d.Metrics.JobsSubmitted.Inc()
	s.d.Log.Info("job submitted", "job", jobID, "workload", w.Name(), "units", len(units), "copies", s.d.VoteReplicas)
	return &pb.SubmitJobResponse{JobId: jobID, Units: uint32(len(units))}, nil
}

// GetJob returns the progress of a job and its result when it is done.
func (s *Server) GetJob(_ context.Context, req *pb.GetJobRequest) (*pb.GetJobResponse, error) {
	if err := s.requireLeader(); err != nil {
		return nil, err
	}
	job, ok := s.d.Table.Job(req.GetJobId())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown job %q", req.GetJobId())
	}
	resp := &pb.GetJobResponse{
		JobId: job.ID, UnitsTotal: uint32(len(job.UnitIDs)), UnitsDone: uint32(job.Done), Error: job.Error,
	}
	switch job.State {
	case tasktable.JobRunning:
		resp.State = pb.JobState_JOB_STATE_RUNNING
	case tasktable.JobDone:
		resp.State = pb.JobState_JOB_STATE_DONE
		resp.Result = job.Result
	case tasktable.JobFailed:
		resp.State = pb.JobState_JOB_STATE_FAILED
	}
	return resp, nil
}

// ---------------------------------------------------------------------------
// VolunteerService
// ---------------------------------------------------------------------------

// Register adds a volunteer.
func (s *Server) Register(_ context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if err := s.requireLeader(); err != nil {
		return nil, err
	}
	tokenName, err := s.d.Auth.Authenticate(req.GetToken())
	if err != nil {
		s.d.Metrics.RegistrationsRejected.Inc()
		s.d.Log.Warn("registration rejected", "name", req.GetName(), "reason", err)
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	r := req.GetResources()
	v := scheduler.Volunteer{
		ID: newID("vol"), Name: req.GetName(), Address: req.GetAddress(),
		Resources: scheduler.Resources{
			CPUCores: int(r.GetCpuCores()), MemoryMB: int(r.GetMemoryMb()), SpeedScore: r.GetSpeedScore(),
		},
	}
	secret, hash := security.NewSession()
	s.mu.Lock()
	s.volunteers[v.ID] = v
	s.sessions[v.ID] = hash
	s.mu.Unlock()
	s.d.Membership.Join(membership.Member{ID: v.ID, Address: v.Address})
	s.d.Metrics.VolunteersRegistered.Inc()
	s.d.Log.Info("volunteer registered", "volunteer", v.ID, "name", v.Name, "token", tokenName,
		"cpu", v.Resources.CPUCores, "speed", v.Resources.SpeedScore)
	return &pb.RegisterResponse{VolunteerId: v.ID, SessionSecret: secret}, nil
}

// volunteer returns a registered volunteer if the request carries its session
// secret. Gossip shares volunteer IDs with every member, so the ID alone does not
// prove who sends the request. An unknown ID gives NotFound and a wrong secret
// gives Unauthenticated; in both cases the volunteer registers again.
func (s *Server) volunteer(id string, secret []byte) (scheduler.Volunteer, error) {
	s.mu.Lock()
	v, ok := s.volunteers[id]
	hash := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return v, status.Errorf(codes.NotFound, "unknown volunteer %q: register again", id)
	}
	if err := hash.Check(secret); err != nil {
		s.d.Metrics.RequestsRejected.Inc()
		s.d.Log.Warn("request rejected: bad session", "volunteer", id)
		return scheduler.Volunteer{}, status.Error(codes.Unauthenticated, err.Error())
	}
	return v, nil
}

// RequestWork gives a unit to a volunteer.
func (s *Server) RequestWork(ctx context.Context, req *pb.RequestWorkRequest) (*pb.RequestWorkResponse, error) {
	if err := s.requireLeader(); err != nil {
		return nil, err
	}
	v, err := s.volunteer(req.GetVolunteerId(), req.GetSessionSecret())
	if err != nil {
		return nil, err
	}
	noWork := &pb.RequestWorkResponse{RetryAfterMs: uint32(noWorkRetry.Milliseconds())}

	var candidates []tasktable.Unit
	for _, u := range s.d.Table.OpenUnits() {
		if u.NeedsVolunteer() && !u.HasVolunteer(v.ID) {
			candidates = append(candidates, u)
		}
	}
	u := s.d.Scheduler.Pick(candidates, v)
	if u == nil {
		return noWork, nil
	}
	expiry := s.d.Now().Add(s.d.Leases.Duration(v)).UTC()
	err = s.commit(ctx, tasktable.Change{Kind: tasktable.UnitAssigned, UnitID: u.ID, VolunteerID: v.ID, LeaseExpiry: expiry})
	if err != nil {
		// Another volunteer can take the last copy at the same time. Ask again later.
		if status.Code(err) == codes.FailedPrecondition {
			return noWork, nil
		}
		return nil, err
	}
	job, _ := s.d.Table.Job(u.JobID)
	s.d.Metrics.UnitsAssigned.Inc()
	s.d.Log.Debug("unit assigned", "unit", u.ID, "volunteer", v.ID, "lease_expiry", expiry)
	msg := security.TaskMessage(u.ID, job.Workload, expiry.UnixMilli(), u.Payload)
	return &pb.RequestWorkResponse{Unit: &pb.WorkUnit{
		UnitId: u.ID, JobId: u.JobID, Workload: job.Workload, Payload: u.Payload,
		LeaseExpiresUnixMs: expiry.UnixMilli(), Signature: s.d.Signer.Sign(msg),
	}}, nil
}

// SubmitResult records a result and accepts the unit when the Verifier agrees.
//
// It holds decideMu from recording the result to the end of the vote, so each
// result is recorded and voted on in one step. Without it, a result recorded
// while another call is between reading the unit and accepting it would never be
// compared, and two results that arrive together could both accept the unit (or
// both raise its copies): the second commit would then fail and return an error
// to a volunteer whose result was already recorded.
func (s *Server) SubmitResult(ctx context.Context, req *pb.SubmitResultRequest) (*pb.SubmitResultResponse, error) {
	if err := s.requireLeader(); err != nil {
		return nil, err
	}
	v, err := s.volunteer(req.GetVolunteerId(), req.GetSessionSecret())
	if err != nil {
		return nil, err
	}
	s.decideMu.Lock()
	defer s.decideMu.Unlock()
	err = s.commit(ctx, tasktable.Change{
		Kind: tasktable.ResultRecorded, UnitID: req.GetUnitId(), VolunteerID: v.ID, Output: req.GetOutput(),
	})
	if err != nil {
		// The lease expired, or the unit is already done. The result is not needed,
		// but a copy that was still running at the accept is still checked.
		if status.Code(err) == codes.FailedPrecondition {
			s.d.Log.Debug("result not recorded", "unit", req.GetUnitId(), "volunteer", v.ID, "reason", err)
			s.checkLateResult(req.GetUnitId(), v.ID, req.GetOutput())
			return &pb.SubmitResultResponse{Recorded: false}, nil
		}
		return nil, err
	}
	s.d.Metrics.ResultsReceived.Inc()
	if err := s.decide(ctx, req.GetUnitId()); err != nil {
		return nil, err
	}
	return &pb.SubmitResultResponse{Recorded: true}, nil
}

// decide asks the Verifier about a unit and commits the outcome.
// The caller holds decideMu. It also runs the job merge when the last unit is
// accepted, so a large merge pauses other votes for that time.
func (s *Server) decide(ctx context.Context, unitID string) error {
	u, ok := s.d.Table.Unit(unitID)
	if !ok || u.State != tasktable.UnitOpen {
		return nil
	}
	d := s.d.Verifier.Decide(u)
	if d.ExtraCopies > 0 {
		err := s.commit(ctx, tasktable.Change{Kind: tasktable.CopiesRaised, UnitID: unitID, Copies: u.Copies + d.ExtraCopies})
		if err != nil {
			return err
		}
		s.d.Metrics.NoMajority.Inc()
		s.d.Log.Info("no majority, more copies", "unit", unitID, "copies", u.Copies+d.ExtraCopies)
	}
	if !d.Accept {
		return nil
	}
	if err := s.commit(ctx, tasktable.Change{Kind: tasktable.UnitAccepted, UnitID: unitID, Output: d.Output}); err != nil {
		return err
	}
	// Count wrong results only after the accept is committed. If the commit
	// fails, the vote runs again on a later tick and must not count them twice.
	for _, id := range d.Wrong {
		s.d.Metrics.WrongResults.Inc()
		s.d.Log.Warn("wrong result", "unit", unitID, "volunteer", id)
	}
	s.lateCopies.add(unitID, u.Assignments, s.d.Now())
	s.d.Metrics.UnitsAccepted.Inc()
	return s.finishJobIfDone(ctx, u.JobID)
}

func (s *Server) finishJobIfDone(ctx context.Context, jobID string) error {
	outputs, ok := s.d.Table.Outputs(jobID)
	if !ok {
		return nil
	}
	job, _ := s.d.Table.Job(jobID)
	change := tasktable.Change{Kind: tasktable.JobFinished, JobID: jobID}
	state := "done"
	w, err := s.d.Workloads.Get(job.Workload)
	if err == nil {
		change.Result, err = w.Merge(job.Params, outputs)
	}
	if err != nil {
		change.Error, state = err.Error(), "failed"
	}
	if err := s.commit(ctx, change); err != nil {
		return err
	}
	s.d.Metrics.JobsFinished.WithLabelValues(state).Inc()
	s.mu.Lock()
	if start, ok := s.submitted[jobID]; ok {
		s.d.Metrics.JobDuration.Observe(s.d.Now().Sub(start).Seconds())
		delete(s.submitted, jobID)
	}
	s.mu.Unlock()
	s.d.Log.Info("job finished", "job", jobID, "state", state, "error", change.Error)
	return nil
}

// newID returns a random ID such as "job-1a2b3c4d5e6f".
func newID(prefix string) string {
	b := make([]byte, 6)
	// Since Go 1.24, crypto/rand.Read never returns an error.
	_, _ = rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}
