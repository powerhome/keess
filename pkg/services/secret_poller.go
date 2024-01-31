package services

import (
	"context"
	"time"

	"go.uber.org/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A struct that can poll for secrets in a Kubernetes cluster.
type SecretPoller struct {
	KubeClient IKubeClient
	Logger     *zap.SugaredLogger
}

// Poll for secrets in a Kubernetes cluster.
func (w *SecretPoller) PollSecrets(ctx context.Context, opts metav1.ListOptions, pollInterval time.Duration) (<-chan *PacSecret, error) {
	secretsChan := make(chan *PacSecret)

	go func() {
		defer close(secretsChan)

		for {
			select {
			case <-time.After(pollInterval):
				secrets, err := w.KubeClient.CoreV1().Secrets(metav1.NamespaceAll).List(ctx, opts)
				if err != nil {
					w.Logger.Error("Failed to list secrets: ", err)
					return
				} else {
					w.Logger.Debugf("Found %d secrets.", len(secrets.Items))
				}

				for _, ns := range secrets.Items {
					pacSecret := &PacSecret{
						Secret: &ns,
					}
					secretsChan <- pacSecret
				}

			case <-ctx.Done():
				w.Logger.Warn("Secret polling stopped by context cancellation.")
				return
			}
		}
	}()

	return secretsChan, nil
}
