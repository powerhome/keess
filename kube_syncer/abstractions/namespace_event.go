package abstractions

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	errorsTypes "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

type NamespaceEvent struct {
	EntityEvent
}

func (c NamespaceEvent) Sync(sourceContext string, kubeClients *map[string]*kubernetes.Clientset) {
	namespace := c.Entity.(*corev1.Namespace)
	namespaceName := namespace.Name

	clients := *kubeClients

	switch c.Type {
	case Added:
		Namespaces[namespaceName] = namespace.DeepCopy()
		c.addConfigMaps(sourceContext, clients, namespaceName)
		c.addSecrets(sourceContext, clients, namespaceName)
	case Deleted:
		delete(Namespaces, namespaceName)
	default:
		// Do nothing.
	}
}

// Creates the ConfigMaps that should be synched to every namespace in this namespace.
func (n NamespaceEvent) addConfigMaps(sourceContext string, kubeClients map[string]*kubernetes.Clientset, namespace string) {

	for _, entity := range EntitiesToAllNamespaces["ConfigMaps"] {
		configMap := entity.(*corev1.ConfigMap)
		sourceNamespace := configMap.Namespace

		entity := NewKubernetesEntity(kubeClients, configMap, ConfigMapEntity, sourceNamespace, namespace, sourceContext, sourceContext)

		err := entity.Create()
		if err != nil {
			if !errorsTypes.IsAlreadyExists(err) {
				Logger.Error(err)
			} else {
				Logger.Debugf("The configmap '%s' already exists in namespace '%s' on context '%s'.", configMap.Name, namespace, sourceContext)
			}
		} else {
			Logger.Infof("The configmap '%s' was added in the namespace '%s' on context '%s'.", configMap.Name, namespace, sourceContext)
		}
	}

	for _, entity := range EntitiesToLabeledNamespaces["ConfigMaps"] {
		configMap := entity.(*corev1.ConfigMap)
		namespaceLabelAnnotation := configMap.Annotations[NamespaceLabelAnnotation]
		label, value, _ := strings.Cut(namespaceLabelAnnotation, "=")

		namespaceEntity := n.Entity.(*corev1.Namespace)
		currentNamespaceLabelAnnotation := namespaceEntity.Annotations[label]

		if currentNamespaceLabelAnnotation != value {
			continue
		}

		sourceNamespace := configMap.Namespace
		entity := NewKubernetesEntity(kubeClients, configMap, ConfigMapEntity, sourceNamespace, namespace, sourceContext, sourceContext)

		err := entity.Create()
		if err != nil {
			if !errorsTypes.IsAlreadyExists(err) {
				Logger.Error(err)
			} else {
				Logger.Debugf("The configmap '%s' already exists in namespace '%s' on context '%s'.", configMap.Name, namespace, sourceContext)
			}
		} else {
			Logger.Infof("The configmap '%s' was added in the namespace '%s' on context '%s'.", configMap.Name, namespace, sourceContext)
		}
	}
}

// Creates the Secrets that should be synched to every namespace in this namespace.
func (n NamespaceEvent) addSecrets(sourceContext string, kubeClients map[string]*kubernetes.Clientset, namespace string) {

	for _, entity := range EntitiesToAllNamespaces["Secrets"] {
		secret := entity.(*corev1.Secret)
		sourceNamespace := secret.Namespace

		entity := NewKubernetesEntity(kubeClients, secret, SecretEntity, sourceNamespace, namespace, sourceContext, sourceContext)

		err := entity.Create()
		if err != nil {
			if !errorsTypes.IsAlreadyExists(err) {
				Logger.Error(err)
			} else {
				Logger.Debugf("The secret '%s' already exists in namespace '%s' on context '%s'.", secret.Name, namespace, sourceContext)
			}
		} else {
			Logger.Infof("The secret '%s' was added in the namespace '%s' on context '%s'.", secret.Name, namespace, sourceContext)
		}
	}

	for _, entity := range EntitiesToLabeledNamespaces["Secrets"] {
		secret := entity.(*corev1.Secret)
		namespaceLabelAnnotation := secret.Annotations[NamespaceLabelAnnotation]
		label, value, _ := strings.Cut(namespaceLabelAnnotation, "=")

		namespaceEntity := n.Entity.(*corev1.Namespace)
		currentNamespaceLabelAnnotation := namespaceEntity.Annotations[label]

		if currentNamespaceLabelAnnotation != value {
			continue
		}

		sourceNamespace := secret.Namespace
		entity := NewKubernetesEntity(kubeClients, secret, SecretEntity, sourceNamespace, namespace, sourceContext, sourceContext)

		err := entity.Create()
		if err != nil {
			if !errorsTypes.IsAlreadyExists(err) {
				Logger.Error(err)
			} else {
				Logger.Debugf("The secret '%s' already exists in namespace '%s' on context '%s'.", secret.Name, namespace, sourceContext)
			}
		} else {
			Logger.Infof("The secret '%s' was added in the namespace '%s' on context '%s'.", secret.Name, namespace, sourceContext)
		}
	}
}
