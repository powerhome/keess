package services

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A struct that can synchronize secrets in a Kubernetes cluster.
type SecretSynchronizer struct {
	localKubeClient   IKubeClient
	remoteKubeClients map[string]IKubeClient
	secretPooller     *SecretPoller
	namespacePoller   *NamespacePoller
	logger            *zap.SugaredLogger
	Secrets           map[string]*PacSecret
}

func NewSecretSynchronizer(
	localKubeClient IKubeClient,
	remoteKubeClients map[string]IKubeClient,
	secretPooller *SecretPoller,
	namespacePoller *NamespacePoller,
	logger *zap.SugaredLogger,
) *SecretSynchronizer {
	return &SecretSynchronizer{
		localKubeClient:   localKubeClient,
		remoteKubeClients: remoteKubeClients,
		secretPooller:     secretPooller,
		namespacePoller:   namespacePoller,
		logger:            logger,
		Secrets:           make(map[string]*PacSecret),
	}
}

// Start the secret synchronizer.
func (s *SecretSynchronizer) Start(ctx context.Context, pollInterval time.Duration, housekeepingInterval time.Duration) error {
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

// Delete orphan secrets in the local Kubernetes cluster.
func (s *SecretSynchronizer) deleteOrphans(ctx context.Context, pollInterval time.Duration) error {
	secretsChan, err := s.secretPooller.PollSecrets(ctx, v1.ListOptions{
		LabelSelector: ManagedLabelSelector,
	}, pollInterval)

	if err != nil {
		s.logger.Error("Failed to list managed secrets: ", err)
		return err
	}

	go func() {
		for {
			select {
			case secret, ok := <-secretsChan:
				if !ok {
					// Channel closed, stop the goroutine
					return
				}

				// Process the secret
				sourceCluster := secret.Secret.Annotations[SourceClusterAnnotation]
				sourceNamespace := secret.Secret.Annotations[SourceNamespaceAnnotation]

				var isLocal bool = false
				var remoteKubeClient IKubeClient
				if sourceCluster == secret.Cluster {
					remoteKubeClient = s.localKubeClient
					isLocal = true
				} else {

					if _, ok := s.remoteKubeClients[sourceCluster]; !ok {
						s.logger.Error("Remote client not found: ", sourceCluster)
						continue
					}

					remoteKubeClient = s.remoteKubeClients[sourceCluster]
				}

				// Check if the secret is orphan
				remoteSecret, err := remoteKubeClient.CoreV1().Secrets(sourceNamespace).Get(ctx, secret.Secret.Name, v1.GetOptions{})

				// Delete the orphan secret
				if errors.IsNotFound(err) {
					err := s.localKubeClient.CoreV1().Secrets(secret.Secret.Namespace).Delete(ctx, secret.Secret.Name, v1.DeleteOptions{})
					if err == nil {
						s.logger.Infof("Orphan secret %s deleted on cluster %s in namespace %s", secret.Secret.Name, secret.Cluster, secret.Secret.Namespace)
					} else {
						s.logger.Errorf("Failed to delete orphan secret %s deleted on cluster %s in namespace %s: %s", secret.Secret.Name, secret.Cluster, secret.Secret.Namespace, err)
					}
				}

				if remoteSecret.Labels[LabelSelector] == "cluster" && isLocal {
					err := s.localKubeClient.CoreV1().Secrets(secret.Secret.Namespace).Delete(ctx, secret.Secret.Name, v1.DeleteOptions{})
					if err == nil {
						s.logger.Infof("Orphan secret %s deleted on cluster %s in namespace %s", secret.Secret.Name, secret.Cluster, secret.Secret.Namespace)
					} else {
						s.logger.Errorf("Failed to delete orphan secret %s deleted on cluster %s in namespace %s: %s", secret.Secret.Name, secret.Cluster, secret.Secret.Namespace, err)
					}
				}

				// Check if the label and annotations of the remote secret means that this secret should exists, otherwise delete this secret
				keep := false
				if remoteSecret.Labels[LabelSelector] == "namespace" {
					if remoteSecret.Annotations[NamespaceLabelAnnotation] != "" {
						keyValue := splitAndTrim(remoteSecret.Annotations[NamespaceLabelAnnotation], "=")
						key := keyValue[0]
						value := strings.Trim(keyValue[1], "\"")

						namespace, err := s.localKubeClient.CoreV1().Namespaces().Get(ctx, secret.Secret.Namespace, v1.GetOptions{})
						if err != nil {
							s.logger.Error("Failed to get namespace: ", err)
							continue
						}
						if namespace.Labels[key] == value {
							keep = true
						}
					}

					if remoteSecret.Annotations[NamespaceNameAnnotation] != "" {
						if remoteSecret.Annotations[NamespaceNameAnnotation] == "all" {
							keep = true
						} else {
							namespaces := splitAndTrim(remoteSecret.Annotations[NamespaceNameAnnotation], ",")
							for _, namespace := range namespaces {
								if namespace == secret.Secret.Namespace {
									keep = true
									break
								}
							}
						}
					}
				}

				if remoteSecret.Labels[LabelSelector] == "cluster" {
					destinationClusters := splitAndTrim(remoteSecret.Annotations[ClusterAnnotation], ",")
					for _, cluster := range destinationClusters {
						if cluster == secret.Cluster {
							keep = true
						}
					}
				}

				if !keep {
					err := s.localKubeClient.CoreV1().Secrets(secret.Secret.Namespace).Delete(ctx, secret.Secret.Name, v1.DeleteOptions{})
					if err == nil {
						s.logger.Infof("Orphan secret %s deleted on cluster %s in namespace %s", secret.Secret.Name, secret.Cluster, secret.Secret.Namespace)
					} else {
						s.logger.Errorf("Failed to delete orphan secret %s deleted on cluster %s in namespace %s: %s", secret.Secret.Name, secret.Cluster, secret.Secret.Namespace, err)
					}
				}

			case <-ctx.Done():
				s.logger.Warn("Secret synchronization stopped by context cancellation.")
				return
			}
		}
	}()

	return nil
}

// Synchronize secrets in Kubernetes clusters.
func (s *SecretSynchronizer) startSyncyng(ctx context.Context, pollInterval time.Duration) error {
	s.Secrets = make(map[string]*PacSecret)

	secretsChan, err := s.secretPooller.PollSecrets(ctx, v1.ListOptions{
		LabelSelector: LabelSelector,
	}, pollInterval)

	if err != nil {
		s.logger.Error("Failed to list secrets: ", err)
		return err
	}

	go func() {
		for {
			select {
			case secret, ok := <-secretsChan:
				if !ok {
					// Channel closed, stop the goroutine
					return
				}

				s.sync(ctx, secret)

			case <-ctx.Done():
				s.logger.Warn("Secret synchronization stopped by context cancellation.")
				return
			}
		}
	}()

	return nil
}

// Synchronize the secret in Kubernetes clusters.
func (s *SecretSynchronizer) sync(ctx context.Context, pacSecret PacSecret) error {

	if pacSecret.Secret.Labels[LabelSelector] == "namespace" {
		return s.syncLocal(ctx, pacSecret)
	}

	if pacSecret.Secret.Labels[LabelSelector] == "cluster" {
		return s.syncRemote(ctx, pacSecret)
	}

	return nil
}

// Synchronize the secret in the local Kubernetes cluster.
func (s *SecretSynchronizer) syncLocal(ctx context.Context, pacSecret PacSecret) error {
	// Synchronize based on the namespace label
	if namespaceLabelAnnotationValue, ok := pacSecret.Secret.Annotations[NamespaceLabelAnnotation]; ok {
		keyValue := splitAndTrim(namespaceLabelAnnotationValue, "=")
		key := keyValue[0]
		value := strings.Trim(keyValue[1], "\"")

		for _, namespace := range s.namespacePoller.Namespaces {

			// Skip the source namespace
			if namespace.Namespace.Name == pacSecret.Secret.Namespace {
				s.logger.Debugf("Skipping source namespace: %s when synchronizing secret: %s", namespace.Namespace.Name, pacSecret.Secret.Name)
				continue
			}

			// Synchronize the secret if the namespace has the label
			if labelValue, ok := namespace.Namespace.Labels[key]; ok && labelValue == value {
				s.createOrUpdate(ctx, s.localKubeClient, pacSecret, pacSecret.Cluster, namespace.Namespace.Name)
			}
		}

		return nil
	}

	// Synchronize based on the namespace name
	if namespaceNameAnnotationValue, ok := pacSecret.Secret.Annotations[NamespaceNameAnnotation]; ok && namespaceNameAnnotationValue != "all" {
		namespaces := splitAndTrim(namespaceNameAnnotationValue, ",")
		for _, namespace := range namespaces {
			_, ok := s.namespacePoller.Namespaces[namespace]
			if !ok {
				s.logger.Warn("Namespace not found: ", namespace)
				continue
			}

			// Skip the source namespace
			if pacSecret.Secret.Namespace == namespace {
				s.logger.Debugf("Skipping source namespace: %s when synchronizing secret: %s", namespace, pacSecret.Secret.Name)
				continue
			}

			s.createOrUpdate(ctx, s.localKubeClient, pacSecret, pacSecret.Cluster, namespace)
		}
		return nil
	}

	// Synchronize all namespaces
	if namespaceNameAnnotationValue, ok := pacSecret.Secret.Annotations[NamespaceNameAnnotation]; ok && namespaceNameAnnotationValue == "all" {
		namespaces := s.namespacePoller.Namespaces

		for _, namespace := range namespaces {

			// Skip the source namespace
			if pacSecret.Secret.Namespace == namespace.Namespace.Name {
				s.logger.Debugf("Skipping source namespace: %s when synchronizing secret: %s", namespace.Namespace.Name, pacSecret.Secret.Name)
				continue
			}

			s.createOrUpdate(ctx, s.localKubeClient, pacSecret, pacSecret.Cluster, namespace.Namespace.Name)
		}
		return nil
	}

	return nil
}

// Synchronize the secret in the remote Kubernetes clusters.
func (s *SecretSynchronizer) syncRemote(ctx context.Context, pacSecret PacSecret) error {
	clusters := splitAndTrim(pacSecret.Secret.Annotations[ClusterAnnotation], ",")

	for _, cluster := range clusters {
		client, ok := s.remoteKubeClients[cluster]

		if !ok {
			s.logger.Error("Remote client not found: ", cluster)
			continue
		}

		// Skip the source cluster.
		if pacSecret.Cluster == cluster {
			s.logger.Debugf("Skipping source cluster: %s when synchronizing secret: %s", cluster, pacSecret.Secret.Name)
			continue
		}

		s.createOrUpdate(ctx, client, pacSecret, cluster, pacSecret.Secret.Namespace)
	}

	return nil
}

// Create or update the secret in the Kubernetes cluster.
func (s *SecretSynchronizer) createOrUpdate(ctx context.Context, client IKubeClient, pacSecret PacSecret, cluster string, namespace string) error {
	gotSecret, err := client.CoreV1().Secrets(namespace).Get(ctx, pacSecret.Secret.Name, v1.GetOptions{})

	if err != nil {

		if errors.IsNotFound(err) {
			newSecret := pacSecret.Prepare(namespace)
			_, err := client.CoreV1().Secrets(namespace).Create(ctx, &newSecret, v1.CreateOptions{})

			if err == nil {
				s.logger.Infof("Secret %s created on cluster %s in namespace %s", pacSecret.Secret.Name, cluster, namespace)
			} else {
				s.logger.Errorf("Failed to create secret %s on cluster %s in namespace %s: %s", pacSecret.Secret.Name, cluster, namespace, err)
			}

			return err
		} else {
			s.logger.Error("Failed to get secret: ", err)
		}

		return err
	}

	// Update the secret if it has changed
	if pacSecret.HasChanged(*gotSecret) {
		updatedSecret := pacSecret.Prepare(namespace)
		_, err := client.CoreV1().Secrets(namespace).Update(ctx, &updatedSecret, v1.UpdateOptions{})

		if err == nil {
			s.logger.Infof("Secret %s updated on cluster %s in namespace %s", pacSecret.Secret.Name, cluster, namespace)
		} else {
			s.logger.Errorf("Failed to update secret %s on cluster %s in namespace %s: %s", pacSecret.Secret.Name, cluster, namespace, err)
		}

		return err
	} else {
		s.logger.Debugf("Secret %s is up do date on cluster %s in namespace %s", pacSecret.Secret.Name, cluster, namespace)
	}

	return err
}
