package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	Registry = prometheus.NewRegistry()

	ErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "keess_errors_total",
			Help: "Total number of errors encountered by the operator.",
		},
	)

	// ManagedResources counts the number of resources managed by the operator, labeled by resource type
	//
	// These are resources matching the ManagedLabelSelector label, the destination
	// resources being synced FROM other namespaces/clusters.
	//
	// This is an informational metric (not meant to aid debugging problems, usually), to
	// understand the scale at which the operator is being used and quickly check which
	// types of resources are being managed.
	ManagedResources = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "keess_resources_managed_total",
			Help: "Total number of resources managed by the operator.",
		},
		[]string{"resource_type"}, // e.g., "service", "configmap", "secret", "namespace"
	)

	// SyncResources counts the number of resources being synced by the operator, labeled by resource type
	//
	// These are resources matching the LabelSelector label, the origin resources being
	// synced TO other namespaces/clusters.
	//
	// This is an informational metric (not meant to aid debugging problems, usually), to
	// understand the scale at which the operator is being used and quickly check which
	// types of resources are being synced.
	SyncResources = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "keess_resources_sync_total",
			Help: "Total number of resources being synced by the operator.",
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

	// RemoteInitFailed indicates if the remote cluster initialization has failed (1 for failed, 0 for successful)
	//
	// This metric is labeled by remote cluster name, so we can track the status of
	// multiple remote clusters independently.
	RemoteInitFailed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "keess_remote_initialization_failed",
			Help: "Indicates if the remote cluster initialization has failed (1 for failed, 0 for successful).",
		},
		[]string{"remote_name"}, // e.g., "cluster1", "cluster2"
	)

	// Goroutines tracks the number of active Keess goroutines by resource type
	//
	// This metric tracks the number of active goroutines, but only for the main goroutines
	// created by Keess to poll, sync, and delete resources, and watch the kubeconfig file.
	// Resource types: configmap, secret, service, namespace, kubeconfig
	Goroutines = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "keess_goroutines",
			Help: "Number of active goroutines by resource type.",
		},
		[]string{"resource_type"}, // e.g., "configmap", "secret", "service", "namespace", "kubeconfig"
	)
)

// RegisterMetrics registers all prometheus metrics
func RegisterMetrics() {
	Registry.MustRegister(ErrorCount)
	Registry.MustRegister(ManagedResources)
	Registry.MustRegister(SyncResources)
	Registry.MustRegister(OrphansDetected)
	Registry.MustRegister(OrphansRemoved)
	Registry.MustRegister(RemoteInitFailed)
	Registry.MustRegister(Goroutines)

	// For Vector metrics, prometheus requires at least one value to be set to show the metric as available
	// So we preset them to 0 with the known labels
	ManagedResources.WithLabelValues("namespace").Set(0) // namespace label makes sense only to Resources metrics
	SyncResources.WithLabelValues("namespace").Set(0)
	for _, label := range []string{"service", "configmap", "secret"} {
		ManagedResources.WithLabelValues(label).Set(0)
		SyncResources.WithLabelValues(label).Set(0)
		OrphansDetected.WithLabelValues(label).Add(0)
		OrphansRemoved.WithLabelValues(label).Add(0)
	}

	// Initialize goroutine metrics to 0 for all known resource types
	for _, resourceType := range []string{"configmap", "secret", "service", "namespace", "kubeconfig"} {
		Goroutines.WithLabelValues(resourceType).Set(0)
	}
}
