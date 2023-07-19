package abstractions

import "k8s.io/client-go/kubernetes"

// Used to synchronize events
type ISynchronizable interface {
	Sync(sourceContext string, kubeClients *map[string]*kubernetes.Clientset)
}
