package abstractions

import (
	"strings"

	str "github.com/appscode/go/strings"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type SecretEvent struct {
	EntityEvent
}

func (c SecretEvent) Sync(sourceContext string, kubeClients *map[string]*kubernetes.Clientset) {
	secret := c.Entity.(*corev1.Secret)
	sourceNamespace := secret.Namespace

	// Check the synchronization type
	syncType := GetSyncType(secret.Labels[LabelSelector])

	// Treating namespace synchronization
	if syncType == Namespace {

		namespaceNameAnnotation := secret.Annotations[NamespaceNameAnnotation]
		namespaceLabelAnnotation := secret.Annotations[NamespaceLabelAnnotation]

		var namespaces []string

		// If the replication is by name
		if !str.IsEmpty(&namespaceNameAnnotation) {

			// Getting the namespaces to replicate
			if namespaceNameAnnotation != All {
				namespaces = StringToSlice(namespaceNameAnnotation)
				delete(EntitiesToAllNamespaces["Secrets"], secret.Name)
			} else {
				// Getting all existing namespaces
				for key := range Namespaces {
					namespaces = append(namespaces, key)
				}
				EntitiesToAllNamespaces["Secrets"][secret.Name] = secret
			}

		}

		// If the replication is by label
		if !str.IsEmpty(&namespaceLabelAnnotation) {
			label, value, found := strings.Cut(namespaceLabelAnnotation, "=")

			if !found {
				Logger.Warnf("The value '%s' for label '%s' is invalid.", namespaceLabelAnnotation, NamespaceLabelAnnotation)
			} else {
				// Getting all existing namespaces
				for namespaceName, namespace := range Namespaces {

					if namespace.Labels[label] == strings.Trim(value, "\"") {
						namespaces = append(namespaces, namespaceName)
						Logger.Debugf("The namespace '%s' contains the synchronization label '%s'. The secret '%s' will be synchronized.", namespaceName, namespaceLabelAnnotation, secret.Name)
					}

				}
				EntitiesToLabeledNamespaces["Secrets"][secret.Name] = secret
			}
		}

		if c.Type == Deleted {
			delete(EntitiesToAllNamespaces["Secrets"], secret.Name)
		}

		for _, destinationNamespace := range namespaces {
			if secret.Namespace == destinationNamespace {
				continue
			}

			kubeEntity := NewKubernetesEntity(*kubeClients, secret, SecretEntity, sourceNamespace, destinationNamespace, sourceContext, sourceContext)

			switch c.Type {
			case Added:
				kubeEntity.Create()
			case Modified:
				kubeEntity.Update()
			case Deleted:
				kubeEntity.Delete()
			}
		}
	}

	if syncType == Cluster {

		// Getting the configuration annotation
		annotation := secret.Annotations[ClusterAnnotation]
		clusters := StringToSlice(annotation)

		var removedClusters []string = []string{}
		for _, destinationContext := range ConnectedClusters {
			contains := false
			for _, cluster := range clusters {
				contains = contains || cluster == destinationContext
			}
			if !contains {
				removedClusters = append(removedClusters, destinationContext)
			}
		}

		for _, destinationContext := range clusters {
			if sourceContext == destinationContext {
				continue
			}

			kubeEntity := NewKubernetesEntity(*kubeClients, secret, SecretEntity, sourceNamespace, sourceNamespace, sourceContext, destinationContext)

			switch c.Type {
			case Added:
				kubeEntity.Create()
			case Modified:
				kubeEntity.Update()
			case Deleted:
				kubeEntity.Delete()
			}
		}

		for _, removedCluster := range removedClusters {
			kubeEntity := NewKubernetesEntity(*kubeClients, secret, SecretEntity, sourceNamespace, sourceNamespace, sourceContext, removedCluster)
			kubeEntity.Delete()
		}
	}

	if c.Type == Modified {
		namespaceNameAnnotation := secret.Annotations[NamespaceNameAnnotation]
		if namespaceNameAnnotation != All {
			delete(EntitiesToAllNamespaces["Secrets"], secret.Name)
		}

		namespaceLabelAnnotation := secret.Annotations[NamespaceLabelAnnotation]
		if namespaceLabelAnnotation == "" {
			delete(EntitiesToLabeledNamespaces["Secrets"], secret.Name)
		}
	}

	if c.Type == Deleted {
		delete(EntitiesToAllNamespaces["Secrets"], secret.Name)
		delete(EntitiesToLabeledNamespaces["Secrets"], secret.Name)
	}
}
