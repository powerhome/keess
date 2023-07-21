package kube_syncer

import (
	"context"
	abstractions "keess/kube_syncer/abstractions"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// Represents a base structure for any syncer.
type NamespaceWatcher struct {
	// The kubeapi client.
	kubeClient *kubernetes.Clientset

	// The logger object.
	logger *zap.SugaredLogger
}

func (w NamespaceWatcher) Watch() <-chan abstractions.ISynchronizable {
	namespacesChan := make(chan abstractions.ISynchronizable)

	go func() {
		watcher, _ := w.kubeClient.CoreV1().Namespaces().Watch(context.Background(), metav1.ListOptions{})

		w.logger.Info("Watching namespaces events.")

		for event := range watcher.ResultChan() {

			// If it's not a valid event jumps to the next.
			if !abstractions.IsAValidEvent(string(event.Type)) {
				w.logger.Debug("Invalid event type '%s'.", event.Type)
				continue
			}

			var namespaceEvent abstractions.NamespaceEvent
			namespaceEvent.Entity = event.Object

			switch event.Type {
			case watch.Added:
				namespaceEvent.Type = abstractions.Added
			case watch.Modified:
				namespaceEvent.Type = abstractions.Modified
			case watch.Deleted:
				namespaceEvent.Type = abstractions.Deleted
			}

			namespacesChan <- namespaceEvent
		}
	}()

	return namespacesChan
}
