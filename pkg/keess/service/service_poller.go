package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"keess/pkg/keess"
)

// A ServicePoller polls services from a Kubernetes cluster.
type ServicePoller struct {
	cluster    string
	kubeClient keess.IKubeClient
	logger     *zap.SugaredLogger
	startup    bool
}

// NewServicePoller creates a new ServicePoller.
func NewServicePoller(cluster string, kubeClient keess.IKubeClient, logger *zap.SugaredLogger) *ServicePoller {
	return &ServicePoller{
		cluster:    cluster,
		kubeClient: kubeClient,
		logger:     logger,
		startup:    true,
	}
}

// Poll for services in a Kubernetes cluster.
func (w *ServicePoller) PollServices(ctx context.Context, opts metav1.ListOptions, pollInterval time.Duration) (<-chan PacService, error) {
	servicesChan := make(chan PacService)
	var interval time.Duration

	go func() {
		defer close(servicesChan)

		for {

			if w.startup {
				interval = 0
			} else {
				interval = pollInterval
			}

			select {
			case <-time.After(interval):
				services, err := w.kubeClient.CoreV1().Services(metav1.NamespaceAll).List(ctx, opts)
				if err != nil {
					w.logger.Error("Failed to list services: ", err)
					return
				} else {
					w.logger.Debugf("Found %d services.", len(services.Items))
				}

				for _, svc := range services.Items {
					pacService := PacService{
						Cluster: w.cluster,
						Service: svc,
					}
					servicesChan <- pacService
				}

			case <-ctx.Done():
				w.logger.Warn("Service polling stopped by context cancellation.")
				return
			}
			w.startup = false
		}
	}()

	return servicesChan, nil
}
