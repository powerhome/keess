package abstractions

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	errorsTypes "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

type KubernetesEntity struct {
	Entity               runtime.Object
	Type                 KubernetesEntityType
	SourceNamespace      string
	DestinationNamespace string
	SourceContext        string
	DestinationContext   string
	Client               *kubernetes.Clientset
}

func NewKubernetesEntity(clients map[string]*kubernetes.Clientset, entity runtime.Object, entityType KubernetesEntityType, sourceNamespace, destinationNamespace, sourceContext, destinationContext string) KubernetesEntity {
	clientList := clients
	return KubernetesEntity{
		Entity:               entity,
		Type:                 entityType,
		SourceNamespace:      sourceNamespace,
		DestinationNamespace: destinationNamespace,
		SourceContext:        sourceContext,
		DestinationContext:   destinationContext,
		Client:               clientList[destinationContext],
	}
}

func (e *KubernetesEntity) Create() error {
	if r := recover(); r != nil {
		Logger.Debugln("Program recovered.")
	}

	if e.Type == ConfigMapEntity {
		client := e.Client.CoreV1().ConfigMaps(e.DestinationNamespace)

		sourceEntity := e.Entity.(*corev1.ConfigMap)
		entity := getNewConfigMap(*sourceEntity, e.DestinationNamespace, e.SourceContext)

		_, error := client.Create(context.TODO(), &entity, v1.CreateOptions{})

		if error == nil {
			Logger.Infof("The configMap '%s' was added in the namespace '%s' on context '%s'.", entity.Name, entity.Namespace, e.DestinationContext)
		} else {
			if !errorsTypes.IsAlreadyExists(error) {
				Logger.Error(error)
			} else {
				// If alredy exists it need to be updated.
				_, error := client.Update(context.TODO(), &entity, v1.UpdateOptions{})
				if error == nil {
					Logger.Infof("The configMap '%s' was updated in the namespace '%s' on context '%s'.", entity.Name, entity.Namespace, e.DestinationContext)
				} else {
					Logger.Error(error)
				}
			}
		}

		return error
	}

	if e.Type == SecretEntity {
		client := e.Client.CoreV1().Secrets(e.DestinationNamespace)

		sourceEntity := e.Entity.(*corev1.Secret)
		entity := getNewSecret(*sourceEntity, e.DestinationNamespace, e.SourceContext)

		_, error := client.Create(context.TODO(), &entity, v1.CreateOptions{})

		if error == nil {
			Logger.Infof("The secret '%s' was added in the namespace '%s' on context '%s'.", entity.Name, entity.Namespace, e.DestinationContext)
		} else {
			if !errorsTypes.IsAlreadyExists(error) {
				Logger.Error(error)
			} else {
				// If alredy exists it need to be updated.
				_, error := client.Update(context.TODO(), &entity, v1.UpdateOptions{})
				if error == nil {
					Logger.Infof("The secret '%s' was updated in the namespace '%s' on context '%s'.", entity.Name, entity.Namespace, e.DestinationContext)
				} else {
					Logger.Error(error)
				}
			}
		}

		return error
	}

	return errors.New("unsuported type")
}

func (e *KubernetesEntity) Update() error {
	if r := recover(); r != nil {
		Logger.Debugln("Program recovered.")
	}

	if e.Type == ConfigMapEntity {
		client := e.Client.CoreV1().ConfigMaps(e.DestinationNamespace)

		sourceEntity := e.Entity.(*corev1.ConfigMap)
		entity := getNewConfigMap(*sourceEntity, e.DestinationNamespace, e.SourceContext)

		_, error := client.Update(context.TODO(), &entity, v1.UpdateOptions{})

		if error == nil {
			Logger.Infof("The configmap '%s' was updated in the namespace '%s' on context '%s'.", entity.Name, entity.Namespace, e.DestinationContext)
		} else {
			if !errorsTypes.IsNotFound(error) {
				Logger.Error(error)
			} else {
				// If not exists it need to be created.
				_, error := client.Create(context.TODO(), &entity, v1.CreateOptions{})
				if error == nil {
					Logger.Infof("The configmap '%s' was created in the namespace '%s' on context '%s'.", entity.Name, entity.Namespace, e.DestinationContext)
				} else {
					Logger.Error(error)
				}
			}
		}

		return error
	}

	if e.Type == SecretEntity {
		client := e.Client.CoreV1().Secrets(e.DestinationNamespace)

		sourceEntity := e.Entity.(*corev1.Secret)
		entity := getNewSecret(*sourceEntity, e.DestinationNamespace, e.SourceContext)

		_, error := client.Update(context.TODO(), &entity, v1.UpdateOptions{})

		if error == nil {
			Logger.Infof("The secret '%s' was updated in the namespace '%s' on context '%s'.", entity.Name, entity.Namespace, e.DestinationContext)
		} else {
			if !errorsTypes.IsNotFound(error) {
				Logger.Error(error)
			} else {
				// If not exists it need to be created.
				_, error := client.Create(context.TODO(), &entity, v1.CreateOptions{})
				if error == nil {
					Logger.Infof("The secret '%s' was created in the namespace '%s' on context '%s'.", entity.Name, entity.Namespace, e.DestinationContext)
				} else {
					Logger.Error(error)
				}
			}
		}

		return error
	}

	return errors.New("unsuported type")
}

func (e *KubernetesEntity) Delete() error {
	if r := recover(); r != nil {
		Logger.Debugln("Program recovered.")
	}

	if e.Type == ConfigMapEntity {
		client := e.Client.CoreV1().ConfigMaps(e.DestinationNamespace)

		sourceEntity := e.Entity.(*corev1.ConfigMap)
		entity := getNewConfigMap(*sourceEntity, e.DestinationNamespace, e.SourceContext)

		error := client.Delete(context.TODO(), entity.Name, v1.DeleteOptions{})

		if error == nil {
			Logger.Infof("The configmap '%s' was deleted from namespace '%s' on context '%s'.", entity.Name, entity.Namespace, e.DestinationContext)
		} else {
			if !errorsTypes.IsNotFound(error) {
				Logger.Error(error)
			} else {
				Logger.Debugf("The configmap '%s' was already deleted from namespace '%s' on context '%s'.", entity.Name, entity.Namespace, e.DestinationContext)
			}
		}

		return error
	}

	if e.Type == SecretEntity {
		client := e.Client.CoreV1().Secrets(e.DestinationNamespace)

		sourceEntity := e.Entity.(*corev1.Secret)
		entity := getNewSecret(*sourceEntity, e.DestinationNamespace, e.SourceContext)

		error := client.Delete(context.TODO(), entity.Name, v1.DeleteOptions{})

		if error == nil {
			Logger.Infof("The secret '%s' was deleted from namespace '%s' on context '%s'.", entity.Name, entity.Namespace, e.DestinationContext)
		} else {
			if !errorsTypes.IsNotFound(error) {
				Logger.Error(error)
			} else {
				Logger.Debugf("The secret '%s' was already deleted from namespace '%s' on context '%s'.", entity.Name, entity.Namespace, e.DestinationContext)
			}
		}

		return error
	}

	return errors.New("unsuported type")
}

func getNewConfigMap(sourceConfigMap corev1.ConfigMap, namespace, sourceContext string) corev1.ConfigMap {
	destinationConfigMap := sourceConfigMap.DeepCopy()

	destinationConfigMap.UID = ""
	destinationConfigMap.Labels = map[string]string{}
	destinationConfigMap.Labels[ManagedLabelSelector] = "true"
	destinationConfigMap.Annotations = map[string]string{}
	destinationConfigMap.Annotations[SourceClusterAnnotation] = sourceContext
	destinationConfigMap.Annotations[SourceNamespaceAnnotation] = sourceConfigMap.Namespace
	destinationConfigMap.Annotations[SourceResourceVersionAnnotation] = sourceConfigMap.ResourceVersion
	destinationConfigMap.Namespace = namespace
	destinationConfigMap.ResourceVersion = ""

	return *destinationConfigMap
}

func getNewSecret(sourceSecret corev1.Secret, namespace, sourceContext string) corev1.Secret {
	destinationSecret := sourceSecret.DeepCopy()

	destinationSecret.UID = ""
	destinationSecret.Labels = map[string]string{}
	destinationSecret.Labels[ManagedLabelSelector] = "true"
	destinationSecret.Annotations = map[string]string{}
	destinationSecret.Annotations[SourceClusterAnnotation] = sourceContext
	destinationSecret.Annotations[SourceNamespaceAnnotation] = sourceSecret.Namespace
	destinationSecret.Annotations[SourceResourceVersionAnnotation] = sourceSecret.ResourceVersion
	destinationSecret.Namespace = namespace
	destinationSecret.ResourceVersion = ""

	return *destinationSecret
}
