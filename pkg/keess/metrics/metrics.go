package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	ErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "keess_errors_total",
			Help: "Total number of errors encountered by the operator.",
		},
	)

	// Resources counts the number of resources managed by the operator, labeled by resource type
	//
	// This is an informational metric (not meant to aid debugging problems, usually), to
	// understand the scale at which the operator is being used and quickly check which
	// types of resources are being managed.
	Resources = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "keess_resources_managed_total",
			Help: "Total number of resources managed by the operator.",
		},
		[]string{"resource_type"}, // e.g., "service", "configmap", "secret", "namespace"
	)

	// OrphansDetected counts the number of orphaned resources detected by the operator, labeled by resource type
	//
	// This metric must be incremented as soon as an orphan is detected.
	//
	// Note that if an orphan can NOT be deleted for some reason, it will be counted again
	// the next time it is detected, leading to grow indefinitely while the orphan exists.
	// Such a increase, or the divergence between this and OrphansRemoved, can be used
	// to detect or alert that we have orphans that may need manual cleaning.
	OrphansDetected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "keess_resources_orphan_detections_total",
			Help: "Total number of orphaned resources detected by the operator.",
		},
		[]string{"resource_type"},
	)

	// OrphansRemoved counts the number of orphaned resources removed by the operator, labeled by resource type
	//
	// This metric must be incremented only when an orphan is actually deleted. See
	// OrphansDetected for the relation between both.
	OrphansRemoved = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "keess_resources_orphan_removals_total",
			Help: "Total number of orphaned resources removed by the operator.",
		},
		[]string{"resource_type"},
	)

	// RemoteUp indicates if Keess can reach and access the remote cluster (1 for up, 0 for down).
	//
	// This metric is labeled by remote cluster name, so we can track the status of
	// multiple remote clusters independently.
	RemoteUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "keess_remote_up",
			Help: "Indicates if the remote cluster is reachable (1 for up, 0 for down).",
		},
		[]string{"remote_name"}, // e.g., "cluster1", "cluster2"
	)

	// GoroutinesUp tracks the number of active goroutines by resource type
	//
	// This metric provides insight into the concurrency patterns and resource usage
	// of the Keess operator. It can help detect goroutine leaks or performance issues.
	// Resource types: configmap, secret, service, namespace, kubeconfig
	GoroutinesUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "keess_goroutines_up",
			Help: "Number of active goroutines by resource type.",
		},
		[]string{"resource_type"}, // e.g., "configmap", "secret", "service", "namespace", "kubeconfig"
	)
)

// RegisterMetrics registers all prometheus metrics
func RegisterMetrics() {
	prometheus.MustRegister(ErrorCount)
	prometheus.MustRegister(Resources)
	prometheus.MustRegister(OrphansDetected)
	prometheus.MustRegister(OrphansRemoved)
	prometheus.MustRegister(RemoteUp)
	prometheus.MustRegister(GoroutinesUp)

	// For Vector metrics, prometheus requires at least one value to be set to show the metric as available
	// So we preset them to 0 with the known labels
	Resources.WithLabelValues("namespace").Add(0) // namespace label makes sense only to Resources metric
	for _, label := range []string{"service", "configmap", "secret"} {
		Resources.WithLabelValues(label).Add(0)
		OrphansDetected.WithLabelValues(label).Add(0)
		OrphansRemoved.WithLabelValues(label).Add(0)
	}

	// Initialize goroutine metrics to 0 for all known resource types
	for _, resourceType := range []string{"configmap", "secret", "service", "namespace", "kubeconfig"} {
		GoroutinesUp.WithLabelValues(resourceType).Set(0)
	}
}
