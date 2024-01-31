package services

import (
	"context"
	"time"

	"go.uber.org/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A struct that can poll for configMaps in a Kubernetes cluster.
type ConfigMapPoller struct {
	KubeClient IKubeClient
	Logger     *zap.SugaredLogger
}

// Poll for configMaps in a Kubernetes cluster.
func (w *ConfigMapPoller) PollConfigMaps(ctx context.Context, opts metav1.ListOptions, pollInterval time.Duration) (<-chan *PacConfigMap, error) {
	configMapsChan := make(chan *PacConfigMap)

	go func() {
		defer close(configMapsChan)

		for {
			select {
			case <-time.After(pollInterval):
				configMaps, err := w.KubeClient.CoreV1().ConfigMaps(metav1.NamespaceAll).List(ctx, opts)
				if err != nil {
					w.Logger.Error("Failed to list configMaps: ", err)
					return
				} else {
					w.Logger.Debugf("Found %d configMaps.", len(configMaps.Items))
				}

				for _, ns := range configMaps.Items {
					pacConfigMap := &PacConfigMap{
						ConfigMap: &ns,
					}
					configMapsChan <- pacConfigMap
				}

			case <-ctx.Done():
				w.Logger.Warn("ConfigMap polling stopped by context cancellation.")
				return
			}
		}
	}()

	return configMapsChan, nil
}
