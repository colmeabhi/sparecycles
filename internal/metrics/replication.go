package metrics

import "github.com/prometheus/client_golang/prometheus"

// Replication holds the task-table replication metrics.
type Replication struct {
	Seconds           prometheus.Histogram
	BackupLag         *prometheus.GaugeVec // label: peer (node ID)
	SnapshotsServed   prometheus.Counter
	SnapshotsRestored prometheus.Counter
}

// NewReplication creates the replication metrics and registers them in reg.
func NewReplication(reg prometheus.Registerer) *Replication {
	r := &Replication{
		Seconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "sc_replication_seconds", Help: "Time until enough backups have a table change.",
			Buckets: prometheus.ExponentialBuckets(0.0005, 2, 14),
		}),
		BackupLag: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "sc_replication_backup_lag", Help: "Leader table index minus the backup index, from the last answer of each backup.",
		}, []string{"peer"}),
		SnapshotsServed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_replication_snapshots_served_total", Help: "Table snapshots this coordinator sent to another coordinator.",
		}),
		SnapshotsRestored: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sc_replication_snapshots_restored_total", Help: "Table snapshots this coordinator loaded from another coordinator.",
		}),
	}
	reg.MustRegister(r.Seconds, r.BackupLag, r.SnapshotsServed, r.SnapshotsRestored)
	return r
}
