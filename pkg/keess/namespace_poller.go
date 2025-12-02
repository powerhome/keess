package keess

import (
	"context"
	"keess/pkg/keess/metrics"
	"sync"
	"time"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A struct that can poll for namespaces in a Kubernetes cluster.
type NamespacePoller struct {
	kubeClient IKubeClient
	logger     *zap.SugaredLogger
	Namespaces map[string]*PacNamespace
	mutex      sync.Mutex // Add mutex field
	startup    bool
}

// Create a new NamespacePoller.
func NewNamespacePoller(kubeClient IKubeClient, logger *zap.SugaredLogger) *NamespacePoller {
	return &NamespacePoller{
		kubeClient: kubeClient,
		logger:     logger,
		Namespaces: make(map[string]*PacNamespace),
		startup:    true,
	}
}

// Poll for namespaces in a Kubernetes cluster.
func (w *NamespacePoller) PollNamespaces(ctx context.Context, opts metav1.ListOptions, pollInterval time.Duration, cluster string) error {

	var interval time.Duration
	go func() {
		w.logger.Debug("Namespace poller goroutine started")
		metrics.GoroutinesInactive.WithLabelValues("namespace").Dec()
		defer metrics.GoroutinesInactive.WithLabelValues("namespace").Inc()
		defer w.logger.Debug("Namespace poller goroutine stopped")

		for {
			if w.startup {
				interval = 0
			} else {
				interval = pollInterval
			}

			select {
			case <-time.After(interval):
				namespaces, err := w.kubeClient.CoreV1().Namespaces().List(ctx, opts)
				if err != nil {
					metrics.ErrorCount.Inc()
					w.logger.Error("Failed to list namespaces: ", err)
					continue
				}

				w.logger.Debugf("Found %d namespaces.", len(namespaces.Items))
				w.updateNamespacesMap(namespaces.Items, cluster)

			case <-ctx.Done():
				w.logger.Warn("Namespace polling stopped by context cancellation.")
				return
			}
			w.startup = false
		}
	}()
	return nil
}

// Update the namespaces map.
func (w *NamespacePoller) updateNamespacesMap(currentNamespaces []corev1.Namespace, cluster string) {
	// Ensure thread-safe access if needed
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Clear existing map
	w.Namespaces = make(map[string]*PacNamespace)

	// Populate map with current namespaces
	for _, ns := range currentNamespaces {
		w.Namespaces[ns.Name] = &PacNamespace{
			Namespace: ns.DeepCopy(),
			Cluster:   cluster,
		}
	}
}
