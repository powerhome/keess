package services

import (
	"context"
	"time"

	"go.uber.org/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A struct that can poll for configMaps in a Kubernetes cluster.
type ConfigMapPoller struct {
	cluster    string
	kubeClient IKubeClient
	logger     *zap.SugaredLogger
	startup    bool
}

// Create a new ConfigMapPoller.
func NewConfigMapPoller(cluster string, kubeClient IKubeClient, logger *zap.SugaredLogger) *ConfigMapPoller {
	return &ConfigMapPoller{
		cluster:    cluster,
		kubeClient: kubeClient,
		logger:     logger,
		startup:    true,
	}
}

// Poll for configMaps in a Kubernetes cluster.
func (w *ConfigMapPoller) PollConfigMaps(ctx context.Context, opts metav1.ListOptions, pollInterval time.Duration) (<-chan PacConfigMap, error) {
	configMapsChan := make(chan PacConfigMap)
	var interval time.Duration

	go func() {
		defer close(configMapsChan)

		for {

			if w.startup {
				interval = 0
			} else {
				interval = pollInterval
			}

			select {
			case <-time.After(interval):
				configMaps, err := w.kubeClient.CoreV1().ConfigMaps(metav1.NamespaceAll).List(ctx, opts)
				if err != nil {
					w.logger.Error("Failed to list configMaps: ", err)
					return
				} else {
					w.logger.Debugf("Found %d configMaps.", len(configMaps.Items))
				}

				for _, sc := range configMaps.Items {
					pacConfigMap := PacConfigMap{
						Cluster:   w.cluster,
						ConfigMap: sc,
					}
					configMapsChan <- pacConfigMap
				}

			case <-ctx.Done():
				w.logger.Warn("ConfigMap polling stopped by context cancellation.")
				return
			}
			w.startup = false
		}
	}()

	return configMapsChan, nil
}
