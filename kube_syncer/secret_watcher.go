package kube_syncer

import (
	"context"
	abstractions "keess/kube_syncer/abstractions"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// The secret watcher.
type SecretWatcher struct {
	// The kubeapi client.
	kubeClient *kubernetes.Clientset

	// The logger object.
	logger *zap.SugaredLogger
}

func (w SecretWatcher) Watch() <-chan abstractions.ISynchronizable {
	secretsChan := make(chan abstractions.ISynchronizable)

	go func() {
		watcher, _ := w.kubeClient.CoreV1().Secrets(metav1.NamespaceAll).Watch(context.Background(), metav1.ListOptions{
			LabelSelector: abstractions.LabelSelector,
		})

		w.logger.Info("Watching secrets events.")

		for event := range watcher.ResultChan() {

			// If it's not a valid event jumps to the next.
			if !abstractions.IsAValidEvent(string(event.Type)) {
				w.logger.Debugf("Invalid event type '%s'.", event.Type)
				continue
			}

			var secretEvent abstractions.SecretEvent
			secretEvent.Entity = event.Object

			switch event.Type {
			case watch.Added:
				secretEvent.Type = abstractions.Added
			case watch.Modified:
				secretEvent.Type = abstractions.Modified
			case watch.Deleted:
				secretEvent.Type = abstractions.Deleted
			}

			item := event.Object.(*corev1.Secret)

			if !abstractions.IsAValidLabelValue(item.Labels[abstractions.LabelSelector]) {
				w.logger.Warnf("Invalid label value '%s' for '%s'. Accepted values are: '%s' and '%s' ", item.Labels[abstractions.LabelSelector], abstractions.LabelSelector, abstractions.Namespace, abstractions.Cluster)
				continue
			}

			// If this config map don't contains the mandatory configuration annotation, skip to the next one.
			if !abstractions.ContainsAValidAnnotation(item.Annotations) {
				w.logger.Warnf("Missing or invalid configuration annotation on Secret '%s' in namespace '%s'.", item.Name, item.Namespace)
			}

			secretsChan <- secretEvent
		}
	}()

	return secretsChan
}
