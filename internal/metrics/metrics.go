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
	Registry              *prometheus.Registry
	IsLeader              prometheus.Gauge
	Election              *Election    // see election.go
	Replication           *Replication // see replication.go
	JobsSubmitted         prometheus.Counter
	JobsFinished          *prometheus.CounterVec // label: state (done, failed)
	JobDuration           prometheus.Histogram
	VolunteersRegistered  prometheus.Counter
	RegistrationsRejected prometheus.Counter
	RequestsRejected      prometheus.Counter
	UnitsAssigned         prometheus.Counter
	ResultsReceived       prometheus.Counter
	UnitsAccepted         prometheus.Counter
	WrongResults          prometheus.Counter
	NoMajority            prometheus.Counter
	UnitsWithoutVolunteer prometheus.Gauge
	LeasesExpired         prometheus.Counter
	LeasesDeadVolunteer   prometheus.Counter
	ReplicationErrors     prometheus.Counter
	Gossip                *Gossip
}

// NewCoordinator creates the coordinator metrics in a new registry.
func NewCoordinator() *Coordinator {
	reg := prometheus.NewRegistry()
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
		VolunteersRegistered: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_volunteers_registered_total", Help: "Volunteer registrations.",
		}),
		RegistrationsRejected: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_registrations_rejected_total", Help: "Volunteer registrations rejected for a bad token.",
		}),
		RequestsRejected: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_requests_rejected_total", Help: "Volunteer requests rejected for a missing or wrong session secret.",
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
		NoMajority: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_votes_no_majority_total", Help: "Voting rounds with no majority, after which a unit got more copies.",
		}),
		UnitsWithoutVolunteer: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sc_units_without_volunteer", Help: "Open units that need another volunteer, but every registered volunteer has already run them.",
		}),
		LeasesExpired: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_leases_expired_total", Help: "Leases released because they expired.",
		}),
		LeasesDeadVolunteer: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_leases_dead_volunteer_total", Help: "Leases released because gossip marked the volunteer dead.",
		}),
		ReplicationErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_replication_errors_total", Help: "Changes that could not be replicated.",
		}),
		Gossip: NewGossip(reg),
	}
	reg.MustRegister(
		m.IsLeader,
		m.JobsSubmitted,
		m.JobsFinished,
		m.JobDuration,
		m.VolunteersRegistered,
		m.RegistrationsRejected,
		m.RequestsRejected,
		m.UnitsAssigned,
		m.ResultsReceived,
		m.UnitsAccepted,
		m.WrongResults,
		m.NoMajority,
		m.UnitsWithoutVolunteer,
		m.LeasesExpired,
		m.LeasesDeadVolunteer,
		m.ReplicationErrors,
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)
	m.Election = NewElection(reg)
	m.Replication = NewReplication(reg)
	return m
}

// Gossip holds the SWIM failure detector metrics.
type Gossip struct {
	Members          *prometheus.GaugeVec // label: state (alive, suspect, dead)
	PingsSent        prometheus.Counter
	PingFailures     prometheus.Counter
	IndirectProbes   prometheus.Counter
	Refutations      prometheus.Counter
	DetectionSeconds prometheus.Histogram
}

// NewGossip creates the gossip metrics and registers them in reg.
func NewGossip(reg prometheus.Registerer) *Gossip {
	g := &Gossip{
		Members: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "sc_gossip_members", Help: "Known members, by state.",
		}, []string{"state"}),
		PingsSent: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_gossip_pings_total", Help: "Direct pings sent.",
		}),
		PingFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_gossip_ping_failures_total", Help: "Direct pings with no valid ack.",
		}),
		IndirectProbes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_gossip_indirect_probes_total", Help: "Indirect ping requests sent to helper members.",
		}),
		Refutations: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_gossip_refutations_total", Help: "Times this node refuted a suspicion about itself.",
		}),
		DetectionSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "sc_gossip_detection_seconds", Help: "Time from the last contact with a member to marking it dead.",
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 10),
		}),
	}
	reg.MustRegister(g.Members, g.PingsSent, g.PingFailures, g.IndirectProbes, g.Refutations, g.DetectionSeconds)
	return g
}

// Handler returns the HTTP handler for /metrics.
func Handler(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}
