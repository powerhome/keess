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

// Represents a base structure for any syncer.
type ConfigMapWatcher struct {
	// The kubeapi client.
	kubeClient *kubernetes.Clientset

	// The logger object.
	logger *zap.SugaredLogger
}

func (w ConfigMapWatcher) Watch() <-chan abstractions.ISynchronizable {
	configMapsChan := make(chan abstractions.ISynchronizable)

	go func() {
		watcher, _ := w.kubeClient.CoreV1().ConfigMaps(metav1.NamespaceAll).Watch(context.Background(), metav1.ListOptions{
			LabelSelector: abstractions.LabelSelector,
		})
		w.logger.Info("Watching configmaps events.")

		for event := range watcher.ResultChan() {

			// If it's not a valid event jumps to the next.
			if !abstractions.IsAValidEvent(string(event.Type)) {
				w.logger.Debug("Invalid event type.")
				continue
			}

			var configMapEvent abstractions.ConfigMapEvent
			configMapEvent.Entity = event.Object

			switch event.Type {
			case watch.Added:
				configMapEvent.Type = abstractions.Added
			case watch.Modified:
				configMapEvent.Type = abstractions.Modified
			case watch.Deleted:
				configMapEvent.Type = abstractions.Deleted
			}

			item := event.Object.(*corev1.ConfigMap)

			if !abstractions.IsAValidLabelValue(item.Labels[abstractions.LabelSelector]) {
				w.logger.Warnf("Invalid label value '%s' for '%s'. Accepted values are: '%s' and '%s' ", item.Labels[abstractions.LabelSelector], abstractions.LabelSelector, abstractions.Namespace, abstractions.Cluster)
				continue
			}

			// If this config map don't contains the mandatory configuration annotation, skip to the next one.
			if !abstractions.ContainsAValidAnnotation(item.Annotations) {
				w.logger.Warnf("Missing or invalid configuration annotation on ConfigMap '%s' in namespace '%s'.", item.Name, item.Namespace)
			}

			configMapsChan <- configMapEvent
		}
	}()

	return configMapsChan
}
