// Package tasktable holds the state of all jobs and work units.
//
// The table changes only through Apply. The leader replicates each Change to
// the backups before it applies it, so all coordinators hold the same table.
package tasktable

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"
)

// UnitState is the state of a work unit.
type UnitState int

const (
	UnitOpen UnitState = iota // needs more results
	UnitDone                  // has an accepted output
)

// JobState is the state of a job.
type JobState int

const (
	JobRunning JobState = iota
	JobDone
	JobFailed
)

// Assignment is an active lease of a unit to a volunteer.
type Assignment struct {
	VolunteerID string    `json:"volunteer_id"`
	LeaseExpiry time.Time `json:"lease_expiry"`
}

// Result is the output that one volunteer sent for a unit.
// Output is dropped when the unit is accepted; Hash stays.
type Result struct {
	VolunteerID string `json:"volunteer_id"`
	Output      []byte `json:"output,omitempty"`
	Hash        string `json:"hash"` // hex SHA-256 of Output
}

// Unit is one piece of a job.
//
// The table drops bytes that no step needs any more, so that its size (and the
// size of a catch-up snapshot) does not grow with every finished job:
// UnitAccepted drops Payload and the Output of each Result, and JobFinished
// drops Output. The hashes stay, so late copies can still be checked.
type Unit struct {
	ID          string                `json:"id"`
	JobID       string                `json:"job_id"`
	Index       int                   `json:"index"`
	Payload     []byte                `json:"payload,omitempty"`
	State       UnitState             `json:"state"`
	Copies      int                   `json:"copies"` // number of volunteers that must run the unit
	Assignments map[string]Assignment `json:"assignments"`
	Results     []Result              `json:"results"`
	Output      []byte                `json:"output,omitempty"`      // accepted output, until the job finishes
	OutputHash  string                `json:"output_hash,omitempty"` // hex SHA-256 of the accepted output
}

// HasVolunteer reports whether the volunteer holds a lease on the unit or already sent a result.
func (u Unit) HasVolunteer(volunteerID string) bool {
	if _, ok := u.Assignments[volunteerID]; ok {
		return true
	}
	for _, r := range u.Results {
		if r.VolunteerID == volunteerID {
			return true
		}
	}
	return false
}

// NeedsVolunteer reports whether the unit needs one more volunteer.
func (u Unit) NeedsVolunteer() bool {
	return u.State == UnitOpen && len(u.Assignments)+len(u.Results) < u.Copies
}

// Job is a client job.
type Job struct {
	ID       string   `json:"id"`
	Workload string   `json:"workload"`
	Params   []byte   `json:"params"`
	UnitIDs  []string `json:"unit_ids"`
	Done     int      `json:"done"`
	State    JobState `json:"state"`
	Result   []byte   `json:"result,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// Table is the task table. It is safe for concurrent use.
type Table struct {
	mu    sync.RWMutex
	index uint64
	jobs  map[string]*Job
	units map[string]*Unit
	order []string // unit IDs in the order they were added
}

// New returns an empty table.
func New() *Table {
	return &Table{jobs: map[string]*Job{}, units: map[string]*Unit{}}
}

// UnitID returns the ID of unit number index in a job.
func UnitID(jobID string, index int) string {
	return fmt.Sprintf("%s-%d", jobID, index)
}

// Index returns the number of changes applied so far.
func (t *Table) Index() uint64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.index
}

// Apply applies one change. It returns an error if the change does not fit the current table.
// A change that fails does not modify the table.
func (t *Table) Apply(c Change) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.apply(c, false); err != nil {
		return fmt.Errorf("apply %s: %w", c.Kind, err)
	}
	t.index++
	return nil
}

// Validate reports whether Apply would accept the change. It does not modify the table.
func (t *Table) Validate(c Change) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if err := t.apply(c, true); err != nil {
		return fmt.Errorf("apply %s: %w", c.Kind, err)
	}
	return nil
}

// apply checks the change and, if dry is false, modifies the table.
// Each case must do all its checks before its first modification.
func (t *Table) apply(c Change, dry bool) error {
	switch c.Kind {
	case JobAdded:
		if _, ok := t.jobs[c.JobID]; ok {
			return fmt.Errorf("job %s already exists", c.JobID)
		}
		if c.Copies < 1 {
			return fmt.Errorf("copies must be 1 or more")
		}
		if dry {
			return nil
		}
		job := &Job{ID: c.JobID, Workload: c.Workload, Params: c.Params, State: JobRunning}
		for i, payload := range c.Units {
			id := UnitID(c.JobID, i)
			t.units[id] = &Unit{
				ID: id, JobID: c.JobID, Index: i, Payload: payload,
				State: UnitOpen, Copies: c.Copies, Assignments: map[string]Assignment{},
			}
			t.order = append(t.order, id)
			job.UnitIDs = append(job.UnitIDs, id)
		}
		t.jobs[c.JobID] = job

	case UnitAssigned:
		u, err := t.openUnit(c.UnitID)
		if err != nil {
			return err
		}
		if u.HasVolunteer(c.VolunteerID) {
			return fmt.Errorf("volunteer %s already has unit %s", c.VolunteerID, c.UnitID)
		}
		if !u.NeedsVolunteer() {
			return fmt.Errorf("unit %s has all its copies", c.UnitID)
		}
		if dry {
			return nil
		}
		u.Assignments[c.VolunteerID] = Assignment{VolunteerID: c.VolunteerID, LeaseExpiry: c.LeaseExpiry}

	case LeaseReleased:
		u, ok := t.units[c.UnitID]
		if !ok {
			return fmt.Errorf("unknown unit %s", c.UnitID)
		}
		if _, ok := u.Assignments[c.VolunteerID]; !ok {
			return fmt.Errorf("volunteer %s has no lease on unit %s", c.VolunteerID, c.UnitID)
		}
		if dry {
			return nil
		}
		delete(u.Assignments, c.VolunteerID)

	case ResultRecorded:
		u, err := t.openUnit(c.UnitID)
		if err != nil {
			return err
		}
		if _, ok := u.Assignments[c.VolunteerID]; !ok {
			return fmt.Errorf("volunteer %s has no lease on unit %s", c.VolunteerID, c.UnitID)
		}
		if dry {
			return nil
		}
		delete(u.Assignments, c.VolunteerID)
		u.Results = append(u.Results, Result{VolunteerID: c.VolunteerID, Output: c.Output, Hash: Hash(c.Output)})

	case CopiesRaised:
		u, err := t.openUnit(c.UnitID)
		if err != nil {
			return err
		}
		if c.Copies <= u.Copies {
			return fmt.Errorf("copies %d is not higher than %d", c.Copies, u.Copies)
		}
		if dry {
			return nil
		}
		u.Copies = c.Copies

	case UnitAccepted:
		u, err := t.openUnit(c.UnitID)
		if err != nil {
			return err
		}
		if dry {
			return nil
		}
		u.State = UnitDone
		u.Output, u.OutputHash = c.Output, Hash(c.Output)
		u.Assignments = map[string]Assignment{} // other leases are no longer needed
		u.Payload = nil                         // a done unit is never sent again
		for i := range u.Results {
			u.Results[i].Output = nil // the vote is over; the hash is enough
		}
		t.jobs[u.JobID].Done++

	case JobFinished:
		job, ok := t.jobs[c.JobID]
		if !ok {
			return fmt.Errorf("unknown job %s", c.JobID)
		}
		if job.State != JobRunning {
			return fmt.Errorf("job %s is already finished", c.JobID)
		}
		if dry {
			return nil
		}
		if c.Error != "" {
			job.State, job.Error = JobFailed, c.Error
		} else {
			job.State, job.Result = JobDone, c.Result
		}
		t.dropOutputs(job)

	default:
		return fmt.Errorf("unknown change kind %q", c.Kind)
	}
	return nil
}

// dropOutputs removes the accepted outputs of a finished job. The merged
// result in the job replaces them.
func (t *Table) dropOutputs(job *Job) {
	for _, id := range job.UnitIDs {
		if u := t.units[id]; u.State == UnitDone {
			u.Output = nil
		}
	}
}

func (t *Table) openUnit(id string) (*Unit, error) {
	u, ok := t.units[id]
	if !ok {
		return nil, fmt.Errorf("unknown unit %s", id)
	}
	if u.State != UnitOpen {
		return nil, fmt.Errorf("unit %s is already done", id)
	}
	return u, nil
}

// Job returns a copy of a job.
func (t *Table) Job(id string) (Job, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	j, ok := t.jobs[id]
	if !ok {
		return Job{}, false
	}
	return cloneJob(j), true
}

// RunningJobs returns the IDs of the jobs that are not finished yet, sorted.
func (t *Table) RunningJobs() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var ids []string
	for id, j := range t.jobs {
		if j.State == JobRunning {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// Unit returns a copy of a unit.
func (t *Table) Unit(id string) (Unit, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	u, ok := t.units[id]
	if !ok {
		return Unit{}, false
	}
	return cloneUnit(u), true
}

// OpenUnits returns copies of all units that are not done, in the order they were added.
func (t *Table) OpenUnits() []Unit {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out []Unit
	for _, id := range t.order {
		if u := t.units[id]; u.State == UnitOpen {
			out = append(out, cloneUnit(u))
		}
	}
	return out
}

// Outputs returns the accepted outputs of a running job, in unit order.
// It returns false if the job is unknown or finished, or a unit is not done.
func (t *Table) Outputs(jobID string) ([][]byte, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	job, ok := t.jobs[jobID]
	if !ok || job.State != JobRunning {
		return nil, false
	}
	out := make([][]byte, len(job.UnitIDs))
	for i, id := range job.UnitIDs {
		u := t.units[id]
		if u.State != UnitDone {
			return nil, false
		}
		out[i] = u.Output
	}
	return out, true
}

type snapshot struct {
	Index uint64           `json:"index"`
	Jobs  map[string]*Job  `json:"jobs"`
	Units map[string]*Unit `json:"units"`
	Order []string         `json:"order"`
}

// Snapshot returns the full table as bytes. A backup that is behind can load it with Restore.
func (t *Table) Snapshot() ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return json.Marshal(snapshot{Index: t.index, Jobs: t.jobs, Units: t.units, Order: t.order})
}

// Restore replaces the table with a snapshot made by Snapshot.
func (t *Table) Restore(b []byte) error {
	var s snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s.Jobs == nil {
		s.Jobs = map[string]*Job{}
	}
	if s.Units == nil {
		s.Units = map[string]*Unit{}
	}
	for _, u := range s.Units {
		if u.Assignments == nil {
			u.Assignments = map[string]Assignment{}
		}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.index, t.jobs, t.units, t.order = s.Index, s.Jobs, s.Units, s.Order
	return nil
}

// Hash returns the hex SHA-256 of b.
func Hash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func cloneJob(j *Job) Job {
	c := *j
	c.UnitIDs = append([]string(nil), j.UnitIDs...)
	return c
}

func cloneUnit(u *Unit) Unit {
	c := *u
	c.Assignments = make(map[string]Assignment, len(u.Assignments))
	for k, v := range u.Assignments {
		c.Assignments[k] = v
	}
	c.Results = append([]Result(nil), u.Results...)
	return c
}
