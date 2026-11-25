package scheduler

// TrustRecorder is an optional interface for a Scheduler that learns from votes.
// The coordinator calls it after each vote, if its Scheduler implements it.
type TrustRecorder interface {
	// RecordResult reports whether a result of this volunteer agreed with the accepted output.
	RecordResult(volunteerID string, correct bool)
}

// Forgetter is an optional interface for a Scheduler that keeps state per
// volunteer. The coordinator calls Forget when gossip declares a volunteer dead,
// because a dead volunteer never asks again, and one that was wrongly declared
// dead registers again with a new ID.
type Forgetter interface {
	Forget(volunteerID string)
}

// trustScore counts the results of one volunteer that the vote checked.
//
// The value is a beta reputation, as in VECTrust [4]: (correct + 1) / (correct + wrong + 2).
// A new volunteer starts at 0.5 (neutral). Each correct result moves it toward 1,
// each wrong result toward 0, and a long good history is not lost to one fault.
type trustScore struct {
	correct, wrong int
}

func (t trustScore) value() float64 {
	return float64(t.correct+1) / float64(t.correct+t.wrong+2)
}
