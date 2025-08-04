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
	err := s.startSyncing(ctx, pollInterval)
	if err != nil {
		return err
	}

	err = s.deleteOrphans(ctx, housekeepingInterval)
	if err != nil {
		return err
	}

	return nil
}

// Start syncing services (Poller and Synchronizer).
func (s *ServiceSynchronizer) startSyncing(ctx context.Context, pollInterval time.Duration) error {
	servicesChan, err := s.servicePoller.PollServices(ctx, v1.ListOptions{
		LabelSelector: ClusterLabelSelector, // remember service sync does not make sense for namespace sync
	}, pollInterval)

	if err != nil {
		s.logger.Error("Failed to start service poller: ", err)
		return err
	}

	go func() {
		for {
			select {
			case service, ok := <-servicesChan:
				if !ok {
					// Channel closed, stop the goroutine
					s.logger.Warn("[Service][startSyncing] Service syncing stopped because services channel was closed.")
					return
				}
				s.logger.Debugf("[Service][Synchronizer] Processing service %s/%s", service.Service.Namespace, service.Service.Name)

				// Process the service
				err := s.sync(ctx, service)
				if err != nil {
					s.logger.Error("[Service][startSyncing] Failed to sync service: ", err)
				}

			case <-ctx.Done():
				s.logger.Warn("[Service][startSyncing] Service syncing stopped by context cancellation.")
				return
			}
		}
	}()

	return nil
}

// Sync a service to all remote clusters
func (s *ServiceSynchronizer) sync(ctx context.Context, pacService PacService) error {

	// Check if the service has the clusters annotation
	clustersAnnotation, exists := pacService.Service.Annotations[ClusterAnnotation]
	if !exists {
		// TODO: unit test
		s.logger.Debug("[Service][sync] Service is marked for sync but does not have clusters annotation, skipping sync: ", pacService.Service.Name)
		return nil
	}

	// Parse the clusters
	clusters := splitAndTrim(clustersAnnotation, ",")

	// Sync to each cluster
	for _, cluster := range clusters {
		if cluster == pacService.Cluster {
			// TODO: unit test
			s.logger.Debug("[Service][sync] Service is marked for sync to its own cluster, skipping remote sync: ", pacService.Service.Name)
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

// Sync a service to a remote cluster
// This function assumes that the remote cluster client is already set up in the ServiceSynchronizer.
// It will create or update the service in the remote cluster.
func (s *ServiceSynchronizer) syncRemote(ctx context.Context, pacService PacService, cluster string) error {

	k, exists := s.remoteKubeClients[cluster]
	if !exists {
		s.logger.Error("[Service][syncRemote] Remote client not found: ", cluster)
		return nil
	}

	// Check if the namespace exists in the remote cluster
	_, err := k.CoreV1().Namespaces().Get(ctx, pacService.Service.Namespace, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			s.CreateNamespaceForService(ctx, k, cluster, pacService)
		} else {
			s.logger.Error("[Service][syncRemote] Failed to get namespace in remote cluster: ", err)
			return err
		}
	}

	err = s.createOrUpdate(ctx, k, pacService, cluster, pacService.Service.Namespace)
	if err != nil {
		s.logger.Error("[Service][syncRemote] Failed to create or update service: ", err)
	}

	return nil
}

func (s *ServiceSynchronizer) CreateNamespaceForService(ctx context.Context, k IKubeClient, cluster string, svc PacService) error {
	ns := &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: svc.Service.Namespace,
			Labels: map[string]string{
				ManagedLabelSelector: "true",
			},
			Annotations: map[string]string{
				SourceClusterAnnotation:   svc.Cluster,
				SourceNamespaceAnnotation: svc.Service.Namespace,
			},
		},
	}

	_, err := k.CoreV1().Namespaces().Create(ctx, ns, v1.CreateOptions{})
	if err != nil {
		s.logger.Error("[Service][syncRemote] Failed to create namespace in remote cluster: ", err)
		return err
	}

	s.logger.Infof("[Service][syncRemote] Namespace %s created in cluster %s", svc.Service.Namespace, cluster)
	return nil
}

// Create or update a service
func (s *ServiceSynchronizer) createOrUpdate(ctx context.Context, k IKubeClient, pacSvc PacService, cluster string, ns string) error {
	// Prepare the service for the target namespace
	svc := pacSvc.Prepare(ns)

	// Check if the service already exists
	existingService, err := k.CoreV1().Services(ns).Get(ctx, svc.Name, v1.GetOptions{})

	if errors.IsNotFound(err) {
		// Service does not exist, create it
		_, err = k.CoreV1().Services(ns).Create(ctx, &svc, v1.CreateOptions{})
		if err == nil {
			s.logger.Infof("[Service][createOrUpdate] Created service %s/%s in cluster %s", ns, svc.Name, cluster)
		} else {
			s.logger.Error("[Service][createOrUpdate] Failed to create service: ", err)
		}
		return err
	} else if err != nil {
		s.logger.Error("[Service][createOrUpdate] Failed to get existing services: ", err)
		return err
	}

	// Service exist, check if we should manage it
	managed, ok := existingService.Labels[ManagedLabelSelector]
	if !ok {
		s.logger.Warnf("[Service][createOrUpdate] Cluster %s already has unmanaged Service %s/%s, skipping create/update", cluster, ns, svc.Name)
		return nil
	}
	if managed != "true" {
		s.logger.Warnf("[Service][createOrUpdate] Service %s/%s on Cluster %s has management disabled, skipping create/update", ns, svc.Name, cluster)
		return nil
	}

	// Check if the service has changed
	if pacSvc.HasChanged(*existingService) {
		// Update the service
		svc.ResourceVersion = existingService.ResourceVersion
		_, err = k.CoreV1().Services(ns).Update(ctx, &svc, v1.UpdateOptions{})
		if err == nil {
			s.logger.Infof("[Service][createOrUpdate] Updated service %s/%s in cluster %s", ns, svc.Name, cluster)
		} else {
			s.logger.Error("[Service][createOrUpdate] Failed to update service: ", err)
		}
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
				s.logger.Debugf("[Service][deleteOrphans] Processing service %s/%s", service.Service.Namespace, service.Service.Name)

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
