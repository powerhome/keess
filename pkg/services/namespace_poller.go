package services

import (
	"context"
	"time"

	"go.uber.org/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A struct that can poll for namespaces in a Kubernetes cluster.
type NamespacePoller struct {
	KubeClient IKubeClient
	Logger     *zap.SugaredLogger
}

// Poll for namespaces in a Kubernetes cluster.
func (w *NamespacePoller) PollNamespaces(ctx context.Context, opts metav1.ListOptions, pollInterval time.Duration) (<-chan *PacNamespace, error) {
	namespacesChan := make(chan *PacNamespace)

	go func() {
		defer close(namespacesChan)

		for {
			select {
			case <-time.After(pollInterval):
				namespaces, err := w.KubeClient.CoreV1().Namespaces().List(ctx, opts)
				if err != nil {
					w.Logger.Error("Failed to list namespaces: ", err)
					return
				} else {
					w.Logger.Debugf("Found %d namespaces.", len(namespaces.Items))
				}

				for _, ns := range namespaces.Items {
					pacNamespace := &PacNamespace{
						Namespace: &ns,
					}
					namespacesChan <- pacNamespace
				}

			case <-ctx.Done():
				w.Logger.Warn("Namespace polling stopped by context cancellation.")
				return
			}
		}
	}()

	return namespacesChan, nil
}
