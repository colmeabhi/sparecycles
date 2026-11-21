package tasktable

import (
	"encoding/json"
	"time"
)

// ChangeKind names one type of change to the task table.
type ChangeKind string

// The leader changes the table only through these changes.
// The same changes, applied in the same order, give the same table on every coordinator.
const (
	// JobAdded adds a job and its units. Fields: JobID, Workload, Params, Units, Copies.
	JobAdded ChangeKind = "job_added"
	// UnitAssigned gives a lease on a unit to a volunteer. Fields: UnitID, VolunteerID, LeaseExpiry.
	UnitAssigned ChangeKind = "unit_assigned"
	// LeaseReleased removes a lease without a result, for example when it expires. Fields: UnitID, VolunteerID.
	LeaseReleased ChangeKind = "lease_released"
	// ResultRecorded stores a result from a volunteer and removes its lease. Fields: UnitID, VolunteerID, Output.
	ResultRecorded ChangeKind = "result_recorded"
	// CopiesRaised sets a higher number of volunteers for a unit. Fields: UnitID, Copies.
	CopiesRaised ChangeKind = "copies_raised"
	// UnitAccepted marks a unit as done with its accepted output. Fields: UnitID, Output.
	UnitAccepted ChangeKind = "unit_accepted"
	// JobFinished stores the merged result, or an error. Fields: JobID, Result, Error.
	JobFinished ChangeKind = "job_finished"
)

// Change is one entry in the replicated change log.
type Change struct {
	Kind        ChangeKind `json:"kind"`
	JobID       string     `json:"job_id,omitempty"`
	UnitID      string     `json:"unit_id,omitempty"`
	VolunteerID string     `json:"volunteer_id,omitempty"`
	Workload    string     `json:"workload,omitempty"`
	Params      []byte     `json:"params,omitempty"`
	Units       [][]byte   `json:"units,omitempty"`
	Copies      int        `json:"copies,omitempty"`
	LeaseExpiry time.Time  `json:"lease_expiry,omitempty"`
	Output      []byte     `json:"output,omitempty"`
	Result      []byte     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// Encode returns the change as bytes for replication.
func (c Change) Encode() ([]byte, error) {
	return json.Marshal(c)
}

// DecodeChange reads a change from bytes made by Encode.
func DecodeChange(b []byte) (Change, error) {
	var c Change
	err := json.Unmarshal(b, &c)
	return c, err
}
