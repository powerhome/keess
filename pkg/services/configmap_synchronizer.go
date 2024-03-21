package services

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A struct that can synchronize configMaps in a Kubernetes cluster.
type ConfigMapSynchronizer struct {
	localKubeClient   IKubeClient
	remoteKubeClients map[string]IKubeClient
	configMapPooller  *ConfigMapPoller
	namespacePoller   *NamespacePoller
	logger            *zap.SugaredLogger
	ConfigMaps        map[string]*PacConfigMap
}

func NewConfigMapSynchronizer(
	localKubeClient IKubeClient,
	remoteKubeClients map[string]IKubeClient,
	configMapPooller *ConfigMapPoller,
	namespacePoller *NamespacePoller,
	logger *zap.SugaredLogger,
) *ConfigMapSynchronizer {
	return &ConfigMapSynchronizer{
		localKubeClient:   localKubeClient,
		remoteKubeClients: remoteKubeClients,
		configMapPooller:  configMapPooller,
		namespacePoller:   namespacePoller,
		logger:            logger,
		ConfigMaps:        make(map[string]*PacConfigMap),
	}
}

// Start the configMap synchronizer.
func (s *ConfigMapSynchronizer) Start(ctx context.Context, pollInterval time.Duration, housekeepingInterval time.Duration) error {
	err := s.startSyncyng(ctx, pollInterval)
	if err != nil {
		return err
	}

	err = s.deleteOrphans(ctx, housekeepingInterval)
	if err != nil {
		return err
	}

	return nil
}

// Delete orphan configMaps in the local Kubernetes cluster.
func (s *ConfigMapSynchronizer) deleteOrphans(ctx context.Context, pollInterval time.Duration) error {
	configMapsChan, err := s.configMapPooller.PollConfigMaps(ctx, v1.ListOptions{
		LabelSelector: ManagedLabelSelector,
	}, pollInterval)

	if err != nil {
		s.logger.Error("Failed to list managed configMaps: ", err)
		return err
	}

	go func() {
		for {
			select {
			case configMap, ok := <-configMapsChan:
				if !ok {
					// Channel closed, stop the goroutine
					return
				}

				// Process the configMap
				sourceCluster := configMap.ConfigMap.Annotations[SourceClusterAnnotation]
				sourceNamespace := configMap.ConfigMap.Annotations[SourceNamespaceAnnotation]

				var isLocal bool = false
				var remoteKubeClient IKubeClient
				if sourceCluster == configMap.Cluster {
					remoteKubeClient = s.localKubeClient
					isLocal = true
				} else {

					if _, ok := s.remoteKubeClients[sourceCluster]; !ok {
						s.logger.Error("Remote client not found: ", sourceCluster)
						continue
					}

					remoteKubeClient = s.remoteKubeClients[sourceCluster]
				}

				// Check if the configMap is orphan
				remoteConfigMap, err := remoteKubeClient.CoreV1().ConfigMaps(sourceNamespace).Get(ctx, configMap.ConfigMap.Name, v1.GetOptions{})

				// Delete the orphan configMap
				if errors.IsNotFound(err) {
					err := s.localKubeClient.CoreV1().ConfigMaps(configMap.ConfigMap.Namespace).Delete(ctx, configMap.ConfigMap.Name, v1.DeleteOptions{})
					if err == nil {
						s.logger.Infof("Orphan configMap %s deleted on cluster %s in namespace %s", configMap.ConfigMap.Name, configMap.Cluster, configMap.ConfigMap.Namespace)
					} else {
						s.logger.Errorf("Failed to delete orphan configMap %s deleted on cluster %s in namespace %s: %s", configMap.ConfigMap.Name, configMap.Cluster, configMap.ConfigMap.Namespace, err)
					}
				}

				if remoteConfigMap.Labels[LabelSelector] == "cluster" && isLocal {
					err := s.localKubeClient.CoreV1().ConfigMaps(configMap.ConfigMap.Namespace).Delete(ctx, configMap.ConfigMap.Name, v1.DeleteOptions{})
					if err == nil {
						s.logger.Infof("Orphan configMap %s deleted on cluster %s in namespace %s", configMap.ConfigMap.Name, configMap.Cluster, configMap.ConfigMap.Namespace)
					} else {
						s.logger.Errorf("Failed to delete orphan configMap %s deleted on cluster %s in namespace %s: %s", configMap.ConfigMap.Name, configMap.Cluster, configMap.ConfigMap.Namespace, err)
					}
				}

				// Check if the label and annotations of the remote configMap means that this configMap should exists, otherwise delete this configMap
				keep := false
				if remoteConfigMap.Labels[LabelSelector] == "namespace" {
					if remoteConfigMap.Annotations[NamespaceLabelAnnotation] != "" {
						keyValue := splitAndTrim(remoteConfigMap.Annotations[NamespaceLabelAnnotation], "=")
						key := keyValue[0]
						value := strings.Trim(keyValue[1], "\"")

						namespace, err := s.localKubeClient.CoreV1().Namespaces().Get(ctx, configMap.ConfigMap.Namespace, v1.GetOptions{})
						if err != nil {
							s.logger.Error("Failed to get namespace: ", err)
							continue
						}
						if namespace.Labels[key] == value {
							keep = true
						}
					}

					if remoteConfigMap.Annotations[NamespaceNameAnnotation] != "" {
						if remoteConfigMap.Annotations[NamespaceNameAnnotation] == "all" {
							keep = true
						} else {
							namespaces := splitAndTrim(remoteConfigMap.Annotations[NamespaceNameAnnotation], ",")
							for _, namespace := range namespaces {
								if namespace == configMap.ConfigMap.Namespace {
									keep = true
									break
								}
							}
						}
					}
				}

				if remoteConfigMap.Labels[LabelSelector] == "cluster" {
					destinationClusters := splitAndTrim(remoteConfigMap.Annotations[ClusterAnnotation], ",")
					for _, cluster := range destinationClusters {
						if cluster == configMap.Cluster {
							keep = true
						}
					}
				}

				if !keep {
					err := s.localKubeClient.CoreV1().ConfigMaps(configMap.ConfigMap.Namespace).Delete(ctx, configMap.ConfigMap.Name, v1.DeleteOptions{})
					if err == nil {
						s.logger.Infof("Orphan configMap %s deleted on cluster %s in namespace %s", configMap.ConfigMap.Name, configMap.Cluster, configMap.ConfigMap.Namespace)
					} else {
						s.logger.Errorf("Failed to delete orphan configMap %s deleted on cluster %s in namespace %s: %s", configMap.ConfigMap.Name, configMap.Cluster, configMap.ConfigMap.Namespace, err)
					}
				}

			case <-ctx.Done():
				s.logger.Warn("ConfigMap synchronization stopped by context cancellation.")
				return
			}
		}
	}()

	return nil
}

// Synchronize configMaps in Kubernetes clusters.
func (s *ConfigMapSynchronizer) startSyncyng(ctx context.Context, pollInterval time.Duration) error {
	s.ConfigMaps = make(map[string]*PacConfigMap)

	configMapsChan, err := s.configMapPooller.PollConfigMaps(ctx, v1.ListOptions{
		LabelSelector: LabelSelector,
	}, pollInterval)

	if err != nil {
		s.logger.Error("Failed to list configMaps: ", err)
		return err
	}

	go func() {
		for {
			select {
			case configMap, ok := <-configMapsChan:
				if !ok {
					// Channel closed, stop the goroutine
					return
				}

				s.sync(ctx, configMap)

			case <-ctx.Done():
				s.logger.Warn("ConfigMap synchronization stopped by context cancellation.")
				return
			}
		}
	}()

	return nil
}

// Synchronize the configMap in Kubernetes clusters.
func (s *ConfigMapSynchronizer) sync(ctx context.Context, pacConfigMap PacConfigMap) error {

	if pacConfigMap.ConfigMap.Labels[LabelSelector] == "namespace" {
		return s.syncLocal(ctx, pacConfigMap)
	}

	if pacConfigMap.ConfigMap.Labels[LabelSelector] == "cluster" {
		return s.syncRemote(ctx, pacConfigMap)
	}

	return nil
}

// Synchronize the configMap in the local Kubernetes cluster.
func (s *ConfigMapSynchronizer) syncLocal(ctx context.Context, pacConfigMap PacConfigMap) error {
	// Synchronize based on the namespace label
	if namespaceLabelAnnotationValue, ok := pacConfigMap.ConfigMap.Annotations[NamespaceLabelAnnotation]; ok {
		keyValue := splitAndTrim(namespaceLabelAnnotationValue, "=")
		key := keyValue[0]
		value := strings.Trim(keyValue[1], "\"")

		for _, namespace := range s.namespacePoller.Namespaces {

			// Skip the source namespace
			if namespace.Namespace.Name == pacConfigMap.ConfigMap.Namespace {
				s.logger.Debugf("Skipping source namespace: %s when synchronizing configMap: %s", namespace.Namespace.Name, pacConfigMap.ConfigMap.Name)
				continue
			}

			// Synchronize the configMap if the namespace has the label
			if labelValue, ok := namespace.Namespace.Labels[key]; ok && labelValue == value {
				s.createOrUpdate(ctx, s.localKubeClient, pacConfigMap, pacConfigMap.Cluster, namespace.Namespace.Name)
			}
		}

		return nil
	}

	// Synchronize based on the namespace name
	if namespaceNameAnnotationValue, ok := pacConfigMap.ConfigMap.Annotations[NamespaceNameAnnotation]; ok && namespaceNameAnnotationValue != "all" {
		namespaces := splitAndTrim(namespaceNameAnnotationValue, ",")
		for _, namespace := range namespaces {
			_, ok := s.namespacePoller.Namespaces[namespace]
			if !ok {
				s.logger.Warn("Namespace not found: ", namespace)
				continue
			}

			// Skip the source namespace
			if pacConfigMap.ConfigMap.Namespace == namespace {
				s.logger.Debugf("Skipping source namespace: %s when synchronizing configMap: %s", namespace, pacConfigMap.ConfigMap.Name)
				continue
			}

			s.createOrUpdate(ctx, s.localKubeClient, pacConfigMap, pacConfigMap.Cluster, namespace)
		}
		return nil
	}

	// Synchronize all namespaces
	if namespaceNameAnnotationValue, ok := pacConfigMap.ConfigMap.Annotations[NamespaceNameAnnotation]; ok && namespaceNameAnnotationValue == "all" {
		namespaces := s.namespacePoller.Namespaces

		for _, namespace := range namespaces {

			// Skip the source namespace
			if pacConfigMap.ConfigMap.Namespace == namespace.Namespace.Name {
				s.logger.Debugf("Skipping source namespace: %s when synchronizing configMap: %s", namespace.Namespace.Name, pacConfigMap.ConfigMap.Name)
				continue
			}

			s.createOrUpdate(ctx, s.localKubeClient, pacConfigMap, pacConfigMap.Cluster, namespace.Namespace.Name)
		}
		return nil
	}

	return nil
}

// Synchronize the configMap in the remote Kubernetes clusters.
func (s *ConfigMapSynchronizer) syncRemote(ctx context.Context, pacConfigMap PacConfigMap) error {
	clusters := splitAndTrim(pacConfigMap.ConfigMap.Annotations[ClusterAnnotation], ",")

	for _, cluster := range clusters {
		client, ok := s.remoteKubeClients[cluster]

		if !ok {
			s.logger.Error("Remote client not found: ", cluster)
			continue
		}

		// Skip the source cluster.
		if pacConfigMap.Cluster == cluster {
			s.logger.Debugf("Skipping source cluster: %s when synchronizing configMap: %s", cluster, pacConfigMap.ConfigMap.Name)
			continue
		}

		s.createOrUpdate(ctx, client, pacConfigMap, cluster, pacConfigMap.ConfigMap.Namespace)
	}

	return nil
}

// Create or update the configMap in the Kubernetes cluster.
func (s *ConfigMapSynchronizer) createOrUpdate(ctx context.Context, client IKubeClient, pacConfigMap PacConfigMap, cluster string, namespace string) error {
	gotConfigMap, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, pacConfigMap.ConfigMap.Name, v1.GetOptions{})

	if err != nil {

		if errors.IsNotFound(err) {
			newConfigMap := pacConfigMap.Prepare(namespace)
			_, err := client.CoreV1().ConfigMaps(namespace).Create(ctx, &newConfigMap, v1.CreateOptions{})

			if err == nil {
				s.logger.Infof("ConfigMap %s created on cluster %s in namespace %s", pacConfigMap.ConfigMap.Name, cluster, namespace)
			} else {
				s.logger.Errorf("Failed to create configMap %s on cluster %s in namespace %s: %s", pacConfigMap.ConfigMap.Name, cluster, namespace, err)
			}

			return err
		} else {
			s.logger.Error("Failed to get configMap: ", err)
		}

		return err
	}

	// Update the configMap if it has changed
	if pacConfigMap.HasChanged(*gotConfigMap) {
		updatedConfigMap := pacConfigMap.Prepare(namespace)
		_, err := client.CoreV1().ConfigMaps(namespace).Update(ctx, &updatedConfigMap, v1.UpdateOptions{})

		if err == nil {
			s.logger.Infof("ConfigMap %s updated on cluster %s in namespace %s", pacConfigMap.ConfigMap.Name, cluster, namespace)
		} else {
			s.logger.Errorf("Failed to update configMap %s on cluster %s in namespace %s: %s", pacConfigMap.ConfigMap.Name, cluster, namespace, err)
		}

		return err
	} else {
		s.logger.Debugf("ConfigMap %s is up do date on cluster %s in namespace %s", pacConfigMap.ConfigMap.Name, cluster, namespace)
	}

	return err
}
