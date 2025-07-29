package services

import (
	"context"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A struct that can synchronize services in a Kubernetes cluster.
type ServiceSynchronizer struct {
	localKubeClient   IKubeClient
	remoteKubeClients map[string]IKubeClient
	servicePoller     *ServicePoller
	namespacePoller   *NamespacePoller
	logger            *zap.SugaredLogger
	Services          map[string]*PacService
}

func NewServiceSynchronizer(
	localKubeClient IKubeClient,
	remoteKubeClients map[string]IKubeClient,
	servicePoller *ServicePoller,
	namespacePoller *NamespacePoller,
	logger *zap.SugaredLogger,
) *ServiceSynchronizer {
	return &ServiceSynchronizer{
		localKubeClient:   localKubeClient,
		remoteKubeClients: remoteKubeClients,
		servicePoller:     servicePoller,
		namespacePoller:   namespacePoller,
		logger:            logger,
		Services:          make(map[string]*PacService),
	}
}

// Start the service synchronizer.
func (s *ServiceSynchronizer) Start(ctx context.Context, pollInterval time.Duration, housekeepingInterval time.Duration) error {
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

// Delete orphan services in the local Kubernetes cluster.
func (s *ServiceSynchronizer) deleteOrphans(ctx context.Context, pollInterval time.Duration) error {
	servicesChan, err := s.servicePoller.PollServices(ctx, v1.ListOptions{
		LabelSelector: ManagedLabelSelector,
	}, pollInterval)

	if err != nil {
		s.logger.Error("Failed to list managed services: ", err)
		return err
	}

	go func() {
		for {
			select {
			case service, ok := <-servicesChan:
				if !ok {
					// Channel closed, stop the goroutine
					return
				}

				// Process the service
				sourceCluster := service.Service.Annotations[SourceClusterAnnotation]
				sourceNamespace := service.Service.Annotations[SourceNamespaceAnnotation]

				var remoteKubeClient IKubeClient
				if sourceCluster == service.Cluster {
					// TODO: this case should be impossible for services, since there is no namespace sync for services
					remoteKubeClient = s.localKubeClient
				} else {

					if _, ok := s.remoteKubeClients[sourceCluster]; !ok {
						s.logger.Error("[Service][deleteOrphans] Remote client not found: ", sourceCluster)
						continue
					}

					remoteKubeClient = s.remoteKubeClients[sourceCluster]
				}
				// TODO: extract this to a function

				// Check if the service is orphan
				_, err := remoteKubeClient.CoreV1().Services(sourceNamespace).Get(ctx, service.Service.Name, v1.GetOptions{})

				// Delete the orphan service
				if errors.IsNotFound(err) {
					// Check if the service has local endpoints before deleting
					hasLocalEndpoints := s.hasLocalEndpoints(ctx, service.Service)
					if hasLocalEndpoints {
						s.logger.Infof("[Service][deleteOrphans] Service %s/%s has local endpoints, skipping deletion", service.Service.Namespace, service.Service.Name)
						continue
					}

					err := s.localKubeClient.CoreV1().Services(service.Service.Namespace).Delete(ctx, service.Service.Name, v1.DeleteOptions{})
					if err == nil {
						s.logger.Infof("[Service][deleteOrphans] Deleted orphan service %s/%s", service.Service.Namespace, service.Service.Name)
					} else {
						s.logger.Error("[Service][deleteOrphans] Failed to delete orphan service: ", err)
					}

					// Check if namespace is managed and empty
					if s.isNamespaceManaged(service.Service.Namespace) {
						s.checkAndDeleteEmptyNamespace(ctx, service.Service.Namespace)
					}
				} else if err != nil {
					s.logger.Error("[Service][deleteOrphans] Failed to get remote service: ", err)
				}

			case <-ctx.Done():
				s.logger.Warn("[Service][deleteOrphans] Orphan deletion stopped by context cancellation.")
				return
			}
		}
	}()

	return nil
}

// Check if a service has local endpoints
func (s *ServiceSynchronizer) hasLocalEndpoints(ctx context.Context, service corev1.Service) bool {
	// Check if the service has a non-empty selector
	if len(service.Spec.Selector) == 0 {
		return false
	}

	// Get endpoints for the service
	endpoints, err := s.localKubeClient.CoreV1().Endpoints(service.Namespace).Get(ctx, service.Name, v1.GetOptions{})
	if err != nil {
		s.logger.Debugf("[Service][hasLocalEndpoints] Failed to get endpoints for service %s/%s: %v", service.Namespace, service.Name, err)
		return false
	}

	// TODO: check if endpoints are really local, or if they are remote
	// Check if there are any subsets with addresses
	for _, subset := range endpoints.Subsets {
		if len(subset.Addresses) > 0 {
			return true
		}
	}

	return false
}

// Check if namespace is managed by keess
func (s *ServiceSynchronizer) isNamespaceManaged(namespace string) bool {
	// This would need to be implemented based on how keess tracks managed namespaces
	// For now, we'll assume all namespaces are potentially managed
	// TODO: implement this
	return true
}

// Check and delete empty namespace
func (s *ServiceSynchronizer) checkAndDeleteEmptyNamespace(ctx context.Context, namespace string) {
	// TODO: rewrite this to check for all cluster resources. Also, decouple check from deletion.
	// List all resources in the namespace
	services, err := s.localKubeClient.CoreV1().Services(namespace).List(ctx, v1.ListOptions{})
	if err != nil {
		s.logger.Error("[Service][checkAndDeleteEmptyNamespace] Failed to list services in namespace: ", err)
		return
	}

	// If namespace is empty (only managed services), delete it
	if len(services.Items) == 0 {
		err := s.localKubeClient.CoreV1().Namespaces().Delete(ctx, namespace, v1.DeleteOptions{})
		if err == nil {
			s.logger.Infof("[Service][checkAndDeleteEmptyNamespace] Deleted empty namespace %s", namespace)
		} else {
			s.logger.Error("[Service][checkAndDeleteEmptyNamespace] Failed to delete namespace: ", err)
		}
	}
}

// Start syncing services
func (s *ServiceSynchronizer) startSyncyng(ctx context.Context, pollInterval time.Duration) error {
	servicesChan, err := s.servicePoller.PollServices(ctx, v1.ListOptions{
		LabelSelector: LabelSelector,
	}, pollInterval)

	if err != nil {
		s.logger.Error("Failed to list services: ", err) //TODO: bad error message
		return err
	}

	go func() {
		for {
			select {
			case service, ok := <-servicesChan:
				if !ok {
					// Channel closed, stop the goroutine
					return
				}

				// Process the service
				err := s.sync(ctx, service)
				if err != nil {
					s.logger.Error("[Service][startSyncyng] Failed to sync service: ", err)
				}

			case <-ctx.Done():
				s.logger.Warn("[Service][startSyncyng] Service syncing stopped by context cancellation.")
				return
			}
		}
	}()

	return nil
}

// Sync a service
func (s *ServiceSynchronizer) sync(ctx context.Context, pacService PacService) error {
	// Check if the service has the clusters annotation
	clustersAnnotation, exists := pacService.Service.Annotations[ClusterAnnotation]
	if !exists {
		return nil
	}

	// Parse the clusters
	clusters := splitAndTrim(clustersAnnotation, ",")

	// Sync to each cluster
	for _, cluster := range clusters {
		if cluster == pacService.Cluster {
			// Sync locally
			err := s.syncLocal(ctx, pacService)
			if err != nil {
				s.logger.Error("[Service][sync] Failed to sync service locally: ", err)
			}
		} else {
			// Sync remotely
			err := s.syncRemote(ctx, pacService, cluster)
			if err != nil {
				s.logger.Error("[Service][sync] Failed to sync service remotely: ", err)
			}
		}
	}

	return nil
}

// Sync a service locally
func (s *ServiceSynchronizer) syncLocal(ctx context.Context, pacService PacService) error {
	// Get the namespaces to sync to
	namespaces := s.namespacePoller.Namespaces

	// Sync to each namespace
	for _, namespace := range namespaces {
		err := s.createOrUpdate(ctx, s.localKubeClient, pacService, pacService.Cluster, namespace.Namespace.Name)
		if err != nil {
			s.logger.Error("[Service][syncLocal] Failed to create or update service: ", err)
		}
	}

	return nil
}

// Sync a service remotely
func (s *ServiceSynchronizer) syncRemote(ctx context.Context, pacService PacService, cluster string) error {
	// Get the remote client
	remoteKubeClient, exists := s.remoteKubeClients[cluster]
	if !exists {
		s.logger.Error("[Service][syncRemote] Remote client not found: ", cluster)
		return nil
	}

	// Get the namespaces to sync to
	namespaces := s.namespacePoller.Namespaces

	// Sync to each namespace
	for _, namespace := range namespaces {
		err := s.createOrUpdate(ctx, remoteKubeClient, pacService, cluster, namespace.Namespace.Name)
		if err != nil {
			s.logger.Error("[Service][syncRemote] Failed to create or update service: ", err)
		}
	}

	return nil
}

// Create or update a service
func (s *ServiceSynchronizer) createOrUpdate(ctx context.Context, client IKubeClient, pacService PacService, cluster string, namespace string) error {
	// Prepare the service for the target namespace
	newService := pacService.Prepare(namespace)

	// TODO: we need to create the namespace first, if it doesn't exist
	// Check if the service already exists
	existingService, err := client.CoreV1().Services(namespace).Get(ctx, newService.Name, v1.GetOptions{})

	if errors.IsNotFound(err) {
		// Create the service
		_, err = client.CoreV1().Services(namespace).Create(ctx, &newService, v1.CreateOptions{})
		if err == nil {
			s.logger.Infof("[Service][createOrUpdate] Created service %s/%s in cluster %s", namespace, newService.Name, cluster)
		} else {
			s.logger.Error("[Service][createOrUpdate] Failed to create service: ", err)
		}
		return err
	} else if err != nil {
		s.logger.Error("[Service][createOrUpdate] Failed to get existing service: ", err)
		return err
	}

	// Check if the service has changed
	if pacService.HasChanged(*existingService) {
		// Update the service
		newService.ResourceVersion = existingService.ResourceVersion
		_, err = client.CoreV1().Services(namespace).Update(ctx, &newService, v1.UpdateOptions{})
		if err == nil {
			s.logger.Infof("[Service][createOrUpdate] Updated service %s/%s in cluster %s", namespace, newService.Name, cluster)
		} else {
			s.logger.Error("[Service][createOrUpdate] Failed to update service: ", err)
		}
		return err
	}

	return nil
}
