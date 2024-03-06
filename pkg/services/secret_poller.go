package services

import (
	"context"
	"time"

	"go.uber.org/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A struct that can poll for secrets in a Kubernetes cluster.
type SecretPoller struct {
	cluster    string
	kubeClient IKubeClient
	logger     *zap.SugaredLogger
	startup    bool
}

// Create a new SecretPoller.
func NewSecretPoller(cluster string, kubeClient IKubeClient, logger *zap.SugaredLogger) *SecretPoller {
	return &SecretPoller{
		cluster:    cluster,
		kubeClient: kubeClient,
		logger:     logger,
		startup:    true,
	}
}

// Poll for secrets in a Kubernetes cluster.
func (w *SecretPoller) PollSecrets(ctx context.Context, opts metav1.ListOptions, pollInterval time.Duration) (<-chan PacSecret, error) {
	secretsChan := make(chan PacSecret)
	var interval time.Duration

	go func() {
		defer close(secretsChan)

		for {

			if w.startup {
				interval = 0
			} else {
				interval = pollInterval
			}

			select {
			case <-time.After(interval):
				secrets, err := w.kubeClient.CoreV1().Secrets(metav1.NamespaceAll).List(ctx, opts)
				if err != nil {
					w.logger.Error("Failed to list secrets: ", err)
					return
				} else {
					w.logger.Debugf("Found %d secrets.", len(secrets.Items))
				}

				for _, sc := range secrets.Items {
					pacSecret := PacSecret{
						Cluster: w.cluster,
						Secret:  sc,
					}
					secretsChan <- pacSecret
				}

			case <-ctx.Done():
				w.logger.Warn("Secret polling stopped by context cancellation.")
				return
			}
			w.startup = false
		}
	}()

	return secretsChan, nil
}
