package metrics

import "github.com/prometheus/client_golang/prometheus"

// Election holds the leader election metrics.
type Election struct {
	Term            prometheus.Gauge
	Started         prometheus.Counter
	Won             prometheus.Counter
	StepDowns       prometheus.Counter
	FailoverSeconds prometheus.Histogram
}

// NewElection creates the election metrics and registers them in reg.
func NewElection(reg prometheus.Registerer) *Election {
	e := &Election{
		Term: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sc_election_term", Help: "Current election term of this coordinator.",
		}),
		Started: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_elections_started_total", Help: "Elections this coordinator started.",
		}),
		Won: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_elections_won_total", Help: "Elections this coordinator won and then started to lead.",
		}),
		StepDowns: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_election_stepdowns_total", Help: "Times this coordinator stopped being the leader.",
		}),
		FailoverSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "sc_election_failover_seconds", Help: "Time from the last contact with the old leader to a new leader.",
			Buckets: prometheus.ExponentialBuckets(0.25, 2, 10),
		}),
	}
	reg.MustRegister(e.Term, e.Started, e.Won, e.StepDowns, e.FailoverSeconds)
	return e
}
