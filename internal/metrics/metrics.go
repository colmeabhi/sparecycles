// Package metrics defines the Prometheus metrics of SpareCycles.
//
// Add a metric here for each main event of a feature (see the definition of done in AGENTS.md).
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Coordinator holds the coordinator metrics.
type Coordinator struct {
	Registry          *prometheus.Registry
	IsLeader          prometheus.Gauge
	JobsSubmitted     prometheus.Counter
	JobsFinished      *prometheus.CounterVec // label: state (done, failed)
	JobDuration       prometheus.Histogram
	VolunteersJoined  prometheus.Counter
	UnitsAssigned     prometheus.Counter
	ResultsReceived   prometheus.Counter
	UnitsAccepted     prometheus.Counter
	WrongResults      prometheus.Counter
	LeasesExpired     prometheus.Counter
	ReplicationErrors prometheus.Counter
}

// NewCoordinator creates the coordinator metrics in a new registry.
func NewCoordinator() *Coordinator {
	reg := prometheus.NewRegistry()
	f := func(c prometheus.Collector) { reg.MustRegister(c) }
	m := &Coordinator{
		Registry: reg,
		IsLeader: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sc_coordinator_is_leader", Help: "1 if this coordinator is the leader, else 0.",
		}),
		JobsSubmitted: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_jobs_submitted_total", Help: "Jobs submitted by clients.",
		}),
		JobsFinished: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sc_jobs_finished_total", Help: "Jobs finished, by final state.",
		}, []string{"state"}),
		JobDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "sc_job_duration_seconds", Help: "Time from job submission to the final result.",
			Buckets: prometheus.ExponentialBuckets(1, 2, 14),
		}),
		VolunteersJoined: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_volunteers_registered_total", Help: "Volunteer registrations.",
		}),
		UnitsAssigned: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_units_assigned_total", Help: "Unit leases given to volunteers.",
		}),
		ResultsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_results_received_total", Help: "Results sent by volunteers.",
		}),
		UnitsAccepted: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_units_accepted_total", Help: "Units with an accepted output.",
		}),
		WrongResults: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_wrong_results_total", Help: "Results that disagreed with the accepted output.",
		}),
		LeasesExpired: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_leases_expired_total", Help: "Leases released because they expired.",
		}),
		ReplicationErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_replication_errors_total", Help: "Changes that could not be replicated.",
		}),
	}
	f(m.IsLeader)
	f(m.JobsSubmitted)
	f(m.JobsFinished)
	f(m.JobDuration)
	f(m.VolunteersJoined)
	f(m.UnitsAssigned)
	f(m.ResultsReceived)
	f(m.UnitsAccepted)
	f(m.WrongResults)
	f(m.LeasesExpired)
	f(m.ReplicationErrors)
	reg.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return m
}

// Handler returns the HTTP handler for /metrics.
func Handler(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}
