package metrics

import "github.com/prometheus/client_golang/prometheus"

type ReplicationsMetrics struct {
	PointsQueued *prometheus.CounterVec
	BytesQueued  *prometheus.CounterVec
}

func NewReplicationsMetrics() *ReplicationsMetrics {
	const namespace = "replications"
	const subsystem = "what_is_this_for"

	return &ReplicationsMetrics{
		PointsQueued: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "points_queued",
			Help:      "The number of points enqueued to the replication stream",
		}, []string{"replicationID"}),
		BytesQueued: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "bytes_queued",
			Help:      "The number bytes enqueued to the replication stream",
		}, []string{"replicationID"}),
	}
}

// PrometheusCollectors satisfies the prom.PrometheusCollector interface.
func (rm *ReplicationsMetrics) PrometheusCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		rm.PointsQueued,
	}
}
