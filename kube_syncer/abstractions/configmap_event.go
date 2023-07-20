package abstractions

import (
	"strings"

	str "github.com/appscode/go/strings"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type ConfigMapEvent struct {
	EntityEvent
}

func (c ConfigMapEvent) Sync(sourceContext string, kubeClients *map[string]*kubernetes.Clientset) {
	configMap := c.Entity.(*corev1.ConfigMap)
	sourceNamespace := configMap.Namespace

	// Check the synchronization type
	syncType := GetSyncType(configMap.Labels[LabelSelector])

	// Treating namespace synchronization
	if syncType == Namespace {

		namespaceNameAnnotation := configMap.Annotations[NamespaceNameAnnotation]
		namespaceLabelAnnotation := configMap.Annotations[NamespaceLabelAnnotation]

		var namespaces []string

		// If the replication is by name
		if !str.IsEmpty(&namespaceNameAnnotation) {

			// Getting the namespaces to replicate
			if namespaceNameAnnotation != All {
				namespaces = StringToSlice(namespaceNameAnnotation)
				delete(EntitiesToAllNamespaces["ConfigMaps"], configMap.Name)
			} else {
				// Getting all existing namespaces
				for key := range Namespaces {
					namespaces = append(namespaces, key)
				}
				EntitiesToAllNamespaces["ConfigMaps"][configMap.Name] = configMap
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
						Logger.Debugf("The namespace '%s' contains the synchronization label '%s'. The configmap '%s' will be synchronized.", namespaceName, namespaceLabelAnnotation, configMap.Name)
					}
				}
				EntitiesToLabeledNamespaces["ConfigMaps"][configMap.Name] = configMap
			}
		}

		for _, destinationNamespace := range namespaces {
			if configMap.Namespace == destinationNamespace {
				continue
			}

			kubeEntity := NewKubernetesEntity(*kubeClients, configMap, ConfigMapEntity, sourceNamespace, destinationNamespace, sourceContext, sourceContext)

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
		annotation := configMap.Annotations[ClusterAnnotation]
		clusters := StringToSlice(annotation)

		for _, destinationContext := range clusters {
			if sourceContext == destinationContext {
				continue
			}

			kubeEntity := NewKubernetesEntity(*kubeClients, configMap, ConfigMapEntity, sourceNamespace, sourceNamespace, sourceContext, destinationContext)

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

	if c.Type == Modified {
		namespaceNameAnnotation := configMap.Annotations[NamespaceNameAnnotation]
		if namespaceNameAnnotation != All {
			delete(EntitiesToAllNamespaces["ConfigMaps"], configMap.Name)
		}

		namespaceLabelAnnotation := configMap.Annotations[NamespaceLabelAnnotation]
		if namespaceLabelAnnotation == "" {
			delete(EntitiesToLabeledNamespaces["ConfigMaps"], configMap.Name)
		}
	}

	if c.Type == Deleted {
		delete(EntitiesToAllNamespaces["ConfigMaps"], configMap.Name)
		delete(EntitiesToLabeledNamespaces["ConfigMaps"], configMap.Name)
	}
}
